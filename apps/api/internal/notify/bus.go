package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/httpx"
)

// Note is one saved message for one person.
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Href      string    `json:"href"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

type pass struct {
	user  uuid.UUID
	until time.Time
}

type client struct {
	user uuid.UUID
	conn *websocket.Conn
	send chan []byte
}

// Bus stores notifications and pushes them to open sockets.
type Bus struct {
	pool    *pgxpool.Pool
	origin  string
	mu      sync.Mutex
	rooms   map[uuid.UUID]map[*client]struct{}
	tickets map[string]pass
	upgrade websocket.Upgrader
}

func New(pool *pgxpool.Pool, origin string) *Bus {
	bus := &Bus{
		pool:    pool,
		origin:  strings.TrimRight(origin, "/"),
		rooms:   map[uuid.UUID]map[*client]struct{}{},
		tickets: map[string]pass{},
	}
	bus.upgrade = websocket.Upgrader{CheckOrigin: bus.allowedOrigin}
	return bus
}

// Send saves one note for each person and pushes it if they are connected.
// A missing bus, or a failed save, does not stop the order or payment that caused it.
func (b *Bus) Send(ctx context.Context, title, body, href string, users []uuid.UUID, vendorID uuid.UUID, roles []string) {
	if b == nil || b.pool == nil {
		return
	}
	ids := append([]uuid.UUID{}, users...)
	if vendorID != uuid.Nil {
		more, err := b.vendorUsers(ctx, vendorID)
		if err != nil {
			slog.Error("notification audience", "err", err.Error())
		} else {
			ids = append(ids, more...)
		}
	}
	if len(roles) > 0 {
		more, err := b.roleUsers(ctx, roles)
		if err != nil {
			slog.Error("notification audience", "err", err.Error())
		} else {
			ids = append(ids, more...)
		}
	}
	seen := map[uuid.UUID]struct{}{}
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		note, err := b.insert(ctx, id, title, body, href)
		if err != nil {
			slog.Error("notification", "err", err.Error())
			continue
		}
		b.push(id, note)
	}
}

// Mount adds the list, the ticket, and the socket. The socket uses a ticket
// because the hosted site and the API are different hosts.
func Mount(router *gin.Engine, bus *Bus, sessions interface {
	Active(ctx context.Context, id uuid.UUID) error
}, cfg config.Config) {
	if bus == nil {
		return
	}
	authed := router.Group("/api/v1")
	authed.Use(auth.Authenticate(cfg, sessions))
	authed.GET("/notifications", bus.list)
	authed.POST("/notifications/ticket", bus.ticket)
	authed.POST("/notifications/:id/read", bus.markRead)
	staff := auth.RequireRole(identity.RoleSupport, identity.RoleFinance, identity.RoleSuperAdmin)
	authed.GET("/admin/overview", staff, bus.overview)
	router.GET("/api/v1/ws", bus.socket)
}

func (b *Bus) list(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	items, unread, err := b.recent(c.Request.Context(), principal.UserID)
	if err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, "notifications_unavailable", "Notifications could not be loaded.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unread})
}

func (b *Bus) ticket(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ticket": b.issue(principal.UserID),
		"url":    socketURL(c),
	})
}

func (b *Bus) markRead(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That notification is not valid.")
		return
	}
	if _, err := b.pool.Exec(c.Request.Context(), `
		UPDATE notifications SET read_at = COALESCE(read_at, now())
		WHERE id = $1 AND user_id = $2
	`, id, principal.UserID); err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, "notifications_unavailable", "That note could not be marked read.")
		return
	}
	c.Status(http.StatusNoContent)
}

func (b *Bus) overview(c *gin.Context) {
	var kitchens, waiting, disputes int
	err := b.pool.QueryRow(c.Request.Context(), `
		SELECT
			(SELECT count(*) FROM vendors WHERE status = 'ACTIVE'),
			(SELECT count(*) FROM orders WHERE status = 'AWAITING_VENDOR'),
			(SELECT count(*) FROM disputes WHERE status = 'OPEN')
	`).Scan(&kitchens, &waiting, &disputes)
	if err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, "overview_unavailable", "The dashboard could not be loaded.")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"activeKitchens": kitchens,
		"ordersWaiting":  waiting,
		"openDisputes":   disputes,
	})
}

func (b *Bus) socket(c *gin.Context) {
	userID, ok := b.take(c.Query("ticket"))
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "That live connection expired. Try again.")
		return
	}
	conn, err := b.upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	person := &client{user: userID, conn: conn, send: make(chan []byte, 8)}
	b.join(person)
	go person.write()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	b.leave(person)
	_ = conn.Close()
}

func (c *client) write() {
	for payload := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (b *Bus) join(person *client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	room := b.rooms[person.user]
	if room == nil {
		room = map[*client]struct{}{}
		b.rooms[person.user] = room
	}
	room[person] = struct{}{}
}

func (b *Bus) leave(person *client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	room := b.rooms[person.user]
	if _, ok := room[person]; !ok {
		return
	}
	delete(room, person)
	close(person.send)
	if len(room) == 0 {
		delete(b.rooms, person.user)
	}
}

func (b *Bus) push(user uuid.UUID, note Note) {
	payload, err := json.Marshal(note)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for person := range b.rooms[user] {
		select {
		case person.send <- payload:
		default:
		}
	}
}

func (b *Bus) issue(user uuid.UUID) string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	raw := hex.EncodeToString(buf)
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for key, item := range b.tickets {
		if now.After(item.until) {
			delete(b.tickets, key)
		}
	}
	b.tickets[raw] = pass{user: user, until: now.Add(45 * time.Second)}
	return raw
}

func (b *Bus) take(raw string) (uuid.UUID, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	item, ok := b.tickets[raw]
	if !ok {
		return uuid.Nil, false
	}
	delete(b.tickets, raw)
	if time.Now().After(item.until) {
		return uuid.Nil, false
	}
	return item.user, true
}

func (b *Bus) allowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == b.origin
}

func (b *Bus) insert(ctx context.Context, user uuid.UUID, title, body, href string) (Note, error) {
	note := Note{Title: title, Body: body, Href: href}
	var id uuid.UUID
	err := b.pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, title, body, href)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, user, title, body, href).Scan(&id, &note.CreatedAt)
	note.ID = id.String()
	return note, err
}

func (b *Bus) recent(ctx context.Context, user uuid.UUID) ([]Note, int, error) {
	rows, err := b.pool.Query(ctx, `
		SELECT id, title, body, href, read_at IS NOT NULL, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 30
	`, user)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Note{}
	for rows.Next() {
		var note Note
		var id uuid.UUID
		if err := rows.Scan(&id, &note.Title, &note.Body, &note.Href, &note.Read, &note.CreatedAt); err != nil {
			return nil, 0, err
		}
		note.ID = id.String()
		items = append(items, note)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var unread int
	if err := b.pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL
	`, user).Scan(&unread); err != nil {
		return nil, 0, err
	}
	return items, unread, nil
}

func (b *Bus) vendorUsers(ctx context.Context, vendorID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := b.pool.Query(ctx, `SELECT user_id FROM vendor_users WHERE vendor_id = $1`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func (b *Bus) roleUsers(ctx context.Context, roles []string) ([]uuid.UUID, error) {
	rows, err := b.pool.Query(ctx, `
		SELECT ur.user_id
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = ANY($1)
	`, roles)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

type scanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanIDs(rows scanner) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func socketURL(c *gin.Context) string {
	scheme := "ws"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	return scheme + "://" + c.Request.Host + "/api/v1/ws"
}
