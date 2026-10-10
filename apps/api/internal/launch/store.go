package launch

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists audit and backup checks. It never stores secrets.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Write(ctx context.Context, actorID, entityID uuid.UUID, action, reason string) error {
	var actor any
	if actorID != uuid.Nil {
		actor = actorID
	}
	var entity any
	if entityID != uuid.Nil {
		entity = entityID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_id, reason) VALUES ($1, $2, $3, $4)
	`, actor, action, entity, reason)
	return err
}

func (s *Store) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(actor_user_id, '00000000-0000-0000-0000-000000000000'), action,
		       COALESCE(entity_id, '00000000-0000-0000-0000-000000000000'), reason, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.Action, &entry.EntityID, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *Store) SaveBackup(ctx context.Context, check BackupCheck) (BackupCheck, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO backup_checks (actor_user_id, note) VALUES ($1, $2) RETURNING id, created_at
	`, check.ActorID, check.Note).Scan(&check.ID, &check.CreatedAt)
	return check, err
}

func (s *Store) ListBackups(ctx context.Context, limit int) ([]BackupCheck, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(actor_user_id, '00000000-0000-0000-0000-000000000000'), note, created_at
		FROM backup_checks
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupCheck
	for rows.Next() {
		var check BackupCheck
		if err := rows.Scan(&check.ID, &check.ActorID, &check.Note, &check.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, check)
	}
	return out, rows.Err()
}
