package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session is a stored refresh session.
type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID
}

// SessionStore keeps refresh sessions in Postgres.
type SessionStore struct {
	pool *pgxpool.Pool
}

func NewSessionStore(pool *pgxpool.Pool) *SessionStore {
	return &SessionStore{pool: pool}
}

func (s *SessionStore) Create(ctx context.Context, userID uuid.UUID, refreshHash string, expiresAt time.Time, userAgent, ip string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id
	`, userID, refreshHash, expiresAt, trimAgent(userAgent), ip).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

// FindLive returns the session for a refresh token that has not expired or been revoked.
func (s *SessionStore) FindLive(ctx context.Context, refreshHash string) (Session, error) {
	var session Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id
		FROM sessions
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()
	`, refreshHash).Scan(&session.ID, &session.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}
	return session, nil
}

// Rotate replaces the refresh hash when the presented token is still live.
func (s *SessionStore) Rotate(ctx context.Context, currentHash, nextHash string, expiresAt time.Time) (Session, error) {
	var session Session
	err := s.pool.QueryRow(ctx, `
		UPDATE sessions
		SET refresh_token_hash = $2, expires_at = $3
		WHERE refresh_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()
		RETURNING id, user_id
	`, currentHash, nextHash, expiresAt).Scan(&session.ID, &session.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, fmt.Errorf("rotate session: %w", err)
	}
	return session, nil
}

// Active reports whether this session id can still authorize a request.
func (s *SessionStore) Active(ctx context.Context, id uuid.UUID) error {
	var found bool
	err := s.pool.QueryRow(ctx, `
		SELECT true
		FROM sessions
		WHERE id = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()
	`, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !found) {
		return ErrUnauthorized
	}
	if err != nil {
		return fmt.Errorf("check session: %w", err)
	}
	return nil
}

func (s *SessionStore) Revoke(ctx context.Context, refreshHash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = now()
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL
	`, refreshHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func trimAgent(agent string) string {
	if len(agent) > 512 {
		return agent[:512]
	}
	return agent
}
