package launch

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
)

// Books is the append-only audit and backup record.
type Books interface {
	Write(ctx context.Context, actorID, entityID uuid.UUID, action, reason string) error
	List(ctx context.Context, limit int) ([]Entry, error)
	SaveBackup(ctx context.Context, check BackupCheck) (BackupCheck, error)
	ListBackups(ctx context.Context, limit int) ([]BackupCheck, error)
}

// Accounts looks up people and grants staff roles. Public sign-in never calls this.
type Accounts interface {
	Find(ctx context.Context, id uuid.UUID) (identity.User, error)
	FindByEmail(ctx context.Context, email string) (identity.User, error)
	GrantRole(ctx context.Context, userID uuid.UUID, role string) error
}

// Service is launch prep: audit, backup confirmation, and staff RBAC.
type Service struct {
	books Books
	users Accounts
	cfg   config.Config
}

func NewService(books Books, users Accounts, cfg config.Config) *Service {
	return &Service{books: books, users: users, cfg: cfg}
}

func (s *Service) SecurityReport() map[string]any {
	return s.cfg.SecurityReport()
}

func (s *Service) ListAudit(ctx context.Context) ([]Entry, error) {
	return s.books.List(ctx, 100)
}

func (s *Service) RecordBackup(ctx context.Context, actor uuid.UUID, note string) (BackupCheck, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		note = "Neon recovery point checked"
	}
	saved, err := s.books.SaveBackup(ctx, BackupCheck{ActorID: actor, Note: note, CreatedAt: time.Now().UTC()})
	if err != nil {
		return BackupCheck{}, err
	}
	_ = s.books.Write(ctx, actor, saved.ID, "launch.backup.check", note)
	return saved, nil
}

func (s *Service) ListBackups(ctx context.Context) ([]BackupCheck, error) {
	return s.books.ListBackups(ctx, 20)
}

func (s *Service) GrantStaff(ctx context.Context, actor uuid.UUID, email, role string) (identity.User, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != identity.RoleSupport && role != identity.RoleFinance {
		return identity.User{}, ErrForbidden
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return identity.User{}, ErrBadInput
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if err == identity.ErrNotFound {
			return identity.User{}, ErrNotFound
		}
		return identity.User{}, err
	}
	if err := s.users.GrantRole(ctx, user.ID, role); err != nil {
		return identity.User{}, err
	}
	_ = s.books.Write(ctx, actor, user.ID, "identity.grant_role", role)
	return s.users.Find(ctx, user.ID)
}
