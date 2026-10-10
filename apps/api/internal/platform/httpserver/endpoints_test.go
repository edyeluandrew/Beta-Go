package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestFoundationEndpoints(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{profile: identity.GoogleProfile{Sub: "sub-ada", Email: "ada@gmail.com", Name: "Ada"}}
	engine := testEngine(t, users, sessions, google, "admin@gmail.com")

	t.Run("health", func(t *testing.T) {
		status, body := call(t, engine, http.MethodGet, "/healthz", "", nil)
		if status != http.StatusOK || !strings.Contains(body, `"ok"`) {
			t.Fatalf("status %d body %s", status, body)
		}
	})

	t.Run("me requires a session", func(t *testing.T) {
		status, body := call(t, engine, http.MethodGet, "/api/v1/me", "", nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", status, body)
		}
	})

	t.Run("google token is required", func(t *testing.T) {
		status, _ := call(t, engine, http.MethodPost, "/api/v1/auth/google", `{}`, nil)
		if status != http.StatusBadRequest {
			t.Fatalf("status %d", status)
		}
	})

	t.Run("bad google token is rejected", func(t *testing.T) {
		google.err = auth.ErrUnauthorized
		defer func() { google.err = nil }()
		status, _ := call(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"bad"}`, nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("status %d", status)
		}
	})

	var customerCookies []*http.Cookie
	t.Run("google sign-in opens a customer", func(t *testing.T) {
		status, body, cookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
		if status != http.StatusOK {
			t.Fatalf("status %d body %s", status, body)
		}
		customerCookies = cookies
		var payload struct {
			User struct {
				Email string   `json:"email"`
				Roles []string `json:"roles"`
			} `json:"user"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.User.Email != "ada@gmail.com" || !contains(payload.User.Roles, identity.RoleCustomer) || contains(payload.User.Roles, identity.RoleSuperAdmin) {
			t.Fatalf("user = %#v", payload.User)
		}
	})

	t.Run("customer can open the marketplace and nothing else", func(t *testing.T) {
		status, _ := call(t, engine, http.MethodGet, "/api/v1/me", "", customerCookies)
		if status != http.StatusOK {
			t.Fatalf("me status %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/customer/home", "", customerCookies)
		if status != http.StatusOK {
			t.Fatalf("customer home %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/vendor/home", "", customerCookies)
		if status != http.StatusForbidden {
			t.Fatalf("vendor home %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/admin/home", "", customerCookies)
		if status != http.StatusForbidden {
			t.Fatalf("admin home %d", status)
		}
	})

	t.Run("refresh keeps the session and logout ends it", func(t *testing.T) {
		status, _, next := callCookies(t, engine, http.MethodPost, "/api/v1/auth/refresh", "", customerCookies)
		if status != http.StatusOK {
			t.Fatalf("refresh %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/me", "", next)
		if status != http.StatusOK {
			t.Fatalf("me after refresh %d", status)
		}
		status, _ = call(t, engine, http.MethodPost, "/api/v1/auth/logout", "", next)
		if status != http.StatusNoContent {
			t.Fatalf("logout %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/me", "", next)
		if status != http.StatusUnauthorized {
			t.Fatalf("me after logout %d", status)
		}
	})

	t.Run("bootstrap email also receives super admin", func(t *testing.T) {
		google.profile = identity.GoogleProfile{Sub: "sub-admin", Email: "admin@gmail.com", Name: "Admin"}
		status, body, cookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
		if status != http.StatusOK {
			t.Fatalf("status %d body %s", status, body)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/admin/home", "", cookies)
		if status != http.StatusOK {
			t.Fatalf("admin home %d", status)
		}
		status, _ = call(t, engine, http.MethodGet, "/api/v1/vendor/home", "", cookies)
		if status != http.StatusForbidden {
			t.Fatalf("vendor home %d", status)
		}
	})
}

func TestGoogleSignInWithoutClientID(t *testing.T) {
	cfg := testConfig("")
	cfg.GoogleClientID = ""
	handler := auth.NewHandler(auth.NewService(cfg, newMemUsers(), newMemSessions(), fakeGoogle{}), cfg)
	engine := New(cfg, nil, handler, nil, nil, nil, nil, nil, nil)
	status, body := call(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "google_not_configured") {
		t.Fatalf("status %d body %s", status, body)
	}
}

func testEngine(t *testing.T, users *memUsers, sessions *memSessions, google auth.Verifier, admin string) http.Handler {
	t.Helper()
	cfg := testConfig(admin)
	handler := auth.NewHandler(auth.NewService(cfg, users, sessions, google), cfg)
	return New(cfg, nil, handler, nil, nil, nil, nil, nil, nil)
}

func testConfig(admin string) config.Config {
	cfg := config.Config{
		Env:            "test",
		SessionSecret:  "dev-only-session-secret-change-before-deploy",
		GoogleClientID: "test-client",
		WebOrigin:      "http://localhost:5173",
		AccessTTL:      time.Minute,
		RefreshTTL:     time.Hour,
	}
	if admin != "" {
		cfg.BootstrapAdminEmails = []string{admin}
	}
	return cfg
}

func call(t *testing.T, handler http.Handler, method, path, payload string, cookies []*http.Cookie) (int, string) {
	t.Helper()
	status, body, _ := callCookies(t, handler, method, path, payload, cookies)
	return status, body
}

func callCookies(t *testing.T, handler http.Handler, method, path, payload string, cookies []*http.Cookie) (int, string, []*http.Cookie) {
	t.Helper()
	var reader io.Reader
	if payload != "" {
		reader = strings.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if payload != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.Code, string(body), res.Result().Cookies()
}

func contains(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

type fakeGoogle struct {
	profile identity.GoogleProfile
	err     error
}

func (f fakeGoogle) Verify(context.Context, string) (identity.GoogleProfile, error) {
	if f.err != nil {
		return identity.GoogleProfile{}, f.err
	}
	return f.profile, nil
}

type memUsers struct {
	mu   sync.Mutex
	byID map[uuid.UUID]identity.User
}

func newMemUsers() *memUsers {
	return &memUsers{byID: map[uuid.UUID]identity.User{}}
}

func (m *memUsers) UpsertGoogleUser(_ context.Context, profile identity.GoogleProfile, extra []string) (identity.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var user identity.User
	for _, existing := range m.byID {
		if existing.GoogleSub == profile.Sub {
			user = existing
			break
		}
	}
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
		user.GoogleSub = profile.Sub
		user.Roles = []string{}
	}
	user.Email = profile.Email
	user.Name = profile.Name
	user.PictureURL = profile.Picture
	user.Roles = mergeRoles(user.Roles, append([]string{identity.RoleCustomer}, extra...))
	m.byID[user.ID] = user
	return user, nil
}

func (m *memUsers) GrantRole(_ context.Context, id uuid.UUID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.byID[id]
	if !ok {
		return identity.ErrNotFound
	}
	user.Roles = mergeRoles(user.Roles, []string{role})
	m.byID[id] = user
	return nil
}

func (m *memUsers) FindByEmail(_ context.Context, email string) (identity.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.byID {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return identity.User{}, identity.ErrNotFound
}

func (m *memUsers) Find(_ context.Context, id uuid.UUID) (identity.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.byID[id]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	return user, nil
}

func (m *memUsers) UpdateProfile(_ context.Context, id uuid.UUID, name, phone, area string) (identity.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.byID[id]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	if name != "" {
		user.Name = name
	}
	user.Phone = phone
	user.Area = area
	m.byID[id] = user
	return user, nil
}

func mergeRoles(current, extra []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(current)+len(extra))
	for _, role := range append(current, extra...) {
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}

type memSession struct {
	id      uuid.UUID
	userID  uuid.UUID
	hash    string
	expires time.Time
	revoked bool
}

type memSessions struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*memSession
	byHash map[string]*memSession
}

func newMemSessions() *memSessions {
	return &memSessions{byID: map[uuid.UUID]*memSession{}, byHash: map[string]*memSession{}}
}

func (m *memSessions) Create(_ context.Context, userID uuid.UUID, refreshHash string, expiresAt time.Time, _, _ string) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := &memSession{id: uuid.New(), userID: userID, hash: refreshHash, expires: expiresAt}
	m.byID[row.id] = row
	m.byHash[refreshHash] = row
	return row.id, nil
}

func (m *memSessions) FindLive(_ context.Context, refreshHash string) (auth.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.live(refreshHash)
	if row == nil {
		return auth.Session{}, auth.ErrUnauthorized
	}
	return auth.Session{ID: row.id, UserID: row.userID}, nil
}

func (m *memSessions) Rotate(_ context.Context, currentHash, nextHash string, expiresAt time.Time) (auth.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.live(currentHash)
	if row == nil {
		return auth.Session{}, auth.ErrUnauthorized
	}
	delete(m.byHash, currentHash)
	row.hash = nextHash
	row.expires = expiresAt
	m.byHash[nextHash] = row
	return auth.Session{ID: row.id, UserID: row.userID}, nil
}

func (m *memSessions) Revoke(_ context.Context, refreshHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row, ok := m.byHash[refreshHash]; ok {
		row.revoked = true
	}
	return nil
}

func (m *memSessions) Active(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.byID[id]
	if !ok || row.revoked || !row.expires.After(time.Now()) {
		return auth.ErrUnauthorized
	}
	return nil
}

func (m *memSessions) live(hash string) *memSession {
	row, ok := m.byHash[hash]
	if !ok || row.revoked || !row.expires.After(time.Now()) {
		return nil
	}
	return row
}
