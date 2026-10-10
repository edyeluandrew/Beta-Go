package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleCustomer   = "customer"
	RoleVendor     = "vendor"
	RoleSupport    = "support"
	RoleFinance    = "finance"
	RoleSuperAdmin = "super_admin"
)

// User is a person who signs in with Google.
type User struct {
	ID         uuid.UUID
	GoogleSub  string
	Email      string
	Name       string
	PictureURL string
	Phone      string
	Area       string
	Roles      []string
	Disabled   bool
}

// GoogleProfile is the verified identity Google returned.
type GoogleProfile struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

// Store reads and writes users and roles.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// UpsertGoogleUser creates or updates the person and always leaves them with the customer role.
// extraRoles are granted in the same transaction. Public sign-in passes none.
func (s *Store) UpsertGoogleUser(ctx context.Context, profile GoogleProfile, extraRoles []string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin user upsert: %w", err)
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (google_sub, email, name, picture_url)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			name = CASE WHEN users.name = '' THEN EXCLUDED.name ELSE users.name END,
			picture_url = EXCLUDED.picture_url,
			updated_at = now()
		RETURNING id, google_sub, email, name, COALESCE(picture_url, ''), COALESCE(phone, ''), COALESCE(area, ''), disabled_at IS NOT NULL
	`, profile.Sub, profile.Email, profile.Name, profile.Picture).Scan(
		&user.ID, &user.GoogleSub, &user.Email, &user.Name, &user.PictureURL, &user.Phone, &user.Area, &user.Disabled,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("upsert user: %w", err)
	}

	roles := append([]string{RoleCustomer}, extraRoles...)
	for _, role := range roles {
		if err := grantRole(ctx, tx, user.ID, role); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit user upsert: %w", err)
	}

	return s.Find(ctx, user.ID)
}

// FindByEmail loads a person for a controlled staff grant. Public callers never search this.
func (s *Store) FindByEmail(ctx context.Context, email string) (User, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, strings.TrimSpace(email)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return s.Find(ctx, id)
}

// Find loads a user and their roles.
func (s *Store) Find(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		SELECT id, google_sub, email, name, COALESCE(picture_url, ''), COALESCE(phone, ''), COALESCE(area, ''), disabled_at IS NOT NULL
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.GoogleSub, &user.Email, &user.Name, &user.PictureURL, &user.Phone, &user.Area, &user.Disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT r.name
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.id
	`, id)
	if err != nil {
		return User{}, fmt.Errorf("find roles: %w", err)
	}
	defer rows.Close()

	user.Roles = []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return User{}, fmt.Errorf("scan role: %w", err)
		}
		user.Roles = append(user.Roles, role)
	}
	if err := rows.Err(); err != nil {
		return User{}, fmt.Errorf("read roles: %w", err)
	}
	return user, nil
}

// UpdateProfile stores the delivery details a customer gives when they create an account.
func (s *Store) UpdateProfile(ctx context.Context, id uuid.UUID, name, phone, area string) (User, error) {
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	area = strings.TrimSpace(area)
	if len(name) > 80 || len(phone) > 32 || len(area) > 80 {
		return User{}, ErrBadProfile
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE users
		SET name = CASE WHEN $2 = '' THEN name ELSE $2 END,
		    phone = $3,
		    area = $4,
		    updated_at = now()
		WHERE id = $1
	`, id, name, phone, area)
	if err != nil {
		return User{}, fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return User{}, ErrNotFound
	}
	return s.Find(ctx, id)
}

// GrantRole adds a role through a controlled process. Public sign-in never calls this.
func (s *Store) GrantRole(ctx context.Context, userID uuid.UUID, role string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin grant role: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := grantRole(ctx, tx, userID, role); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit grant role: %w", err)
	}
	return nil
}

func grantRole(ctx context.Context, tx pgx.Tx, userID uuid.UUID, role string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM roles WHERE name = $1)`, role).Scan(&exists); err != nil {
		return fmt.Errorf("check role %s: %w", role, err)
	}
	if !exists {
		return fmt.Errorf("unknown role %s", role)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT DO NOTHING
	`, userID, role); err != nil {
		return fmt.Errorf("grant role %s: %w", role, err)
	}
	return nil
}

var (
	// ErrNotFound means the user id does not exist.
	ErrNotFound = errors.New("user not found")
	// ErrEmailTaken means another account already uses this Google email.
	ErrEmailTaken = errors.New("email taken")
	// ErrBadProfile means the customer details were empty in a way we cannot store.
	ErrBadProfile = errors.New("bad profile")
)

// IsStaff reports whether the roles include an administrator permission.
func IsStaff(roles []string) bool {
	return HasAny(roles, RoleSupport, RoleFinance, RoleSuperAdmin)
}

// HasAny reports whether roles contains one of the wanted names.
func HasAny(roles []string, wanted ...string) bool {
	for _, role := range roles {
		for _, name := range wanted {
			if role == name {
				return true
			}
		}
	}
	return false
}
