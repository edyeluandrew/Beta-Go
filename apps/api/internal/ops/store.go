package ops

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists refunds, disputes, and alerts.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) SaveRefund(ctx context.Context, refund Refund) (Refund, error) {
	if existing, err := s.FindRefund(ctx, refund.OrderID); err == nil {
		return existing, nil
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO refunds (order_id, amount_ugx, status, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, refund.OrderID, refund.AmountUGX, refund.Status, refund.Reason).Scan(&refund.ID)
	return refund, err
}

func (s *Store) FindRefund(ctx context.Context, orderID uuid.UUID) (Refund, error) {
	var refund Refund
	err := s.pool.QueryRow(ctx, `
		SELECT id, order_id, amount_ugx, status, reason
		FROM refunds WHERE order_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, orderID).Scan(&refund.ID, &refund.OrderID, &refund.AmountUGX, &refund.Status, &refund.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, ErrNotFound
	}
	return refund, err
}

func (s *Store) SaveDispute(ctx context.Context, dispute Dispute) (Dispute, error) {
	if existing, err := s.FindDispute(ctx, dispute.OrderID); err == nil {
		return existing, nil
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO disputes (order_id, customer_user_id, reason, status, opened_at, escalate_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, dispute.OrderID, dispute.CustomerID, dispute.Reason, dispute.Status, dispute.OpenedAt, dispute.EscalateAt).Scan(&dispute.ID)
	return dispute, err
}

func (s *Store) FindDispute(ctx context.Context, orderID uuid.UUID) (Dispute, error) {
	var dispute Dispute
	var escalate *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, order_id, customer_user_id, reason, status, opened_at, escalate_at
		FROM disputes WHERE order_id = $1
	`, orderID).Scan(&dispute.ID, &dispute.OrderID, &dispute.CustomerID, &dispute.Reason, &dispute.Status, &dispute.OpenedAt, &escalate)
	if errors.Is(err, pgx.ErrNoRows) {
		return Dispute{}, ErrNotFound
	}
	if escalate != nil {
		dispute.EscalateAt = *escalate
	}
	return dispute, err
}

func (s *Store) ListOpenDisputes(ctx context.Context) ([]Dispute, error) {
	rows, err := s.pool.Query(ctx, `SELECT order_id FROM disputes WHERE status = 'OPEN'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dispute
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		dispute, err := s.FindDispute(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, dispute)
	}
	return out, nil
}

func (s *Store) ResolveDispute(ctx context.Context, id, actor uuid.UUID, resolution string) (Dispute, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE disputes SET status = 'RESOLVED', resolved_at = now(), resolved_by = $2, resolution = $3
		WHERE id = $1
	`, id, actor, resolution)
	if err != nil {
		return Dispute{}, err
	}
	if tag.RowsAffected() == 0 {
		return Dispute{}, ErrNotFound
	}
	var orderID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT order_id FROM disputes WHERE id = $1`, id).Scan(&orderID); err != nil {
		return Dispute{}, err
	}
	return s.FindDispute(ctx, orderID)
}

func (s *Store) SaveAlert(ctx context.Context, alert Alert) (Alert, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO alerts (vendor_id, order_id, kind) VALUES ($1, $2, $3) RETURNING id
	`, alert.VendorID, alert.OrderID, alert.Kind).Scan(&alert.ID)
	return alert, err
}

func (s *Store) HasAlert(ctx context.Context, orderID uuid.UUID, kind string) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM alerts WHERE order_id = $1 AND kind = $2)
	`, orderID, kind).Scan(&found)
	return found, err
}
