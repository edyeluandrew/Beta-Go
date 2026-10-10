package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists payments in Postgres and moves the order in the same transaction.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Find(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	rows, err := s.pool.Query(ctx, paymentSelect+` WHERE p.order_id = $1`, orderID)
	if err != nil {
		return Payment{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Payment{}, err
		}
		return Payment{}, ErrNotFound
	}
	payment, _, err := scanPayment(rows)
	return payment, err
}

func (s *Store) SaveInitiation(ctx context.Context, payment Payment) (Payment, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE payments SET
			provider = $2,
			payer_phone = $3,
			provider_reference = $4,
			status = 'PENDING',
			initiated_at = COALESCE(initiated_at, now()),
			expires_at = COALESCE(expires_at, $5)
		WHERE order_id = $1 AND status IN ('PENDING', 'FAILED')
	`, payment.OrderID, payment.Provider, payment.Phone, payment.Reference, payment.ExpiresAt)
	if err != nil {
		return Payment{}, err
	}
	if tag.RowsAffected() == 1 {
		return s.Find(ctx, payment.OrderID)
	}
	return Payment{}, ErrConflict
}

func (s *Store) Pending(ctx context.Context) ([]Payment, error) {
	rows, err := s.pool.Query(ctx, paymentSelect+`
		WHERE p.status = 'PENDING' AND p.provider_reference IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		payment, _, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, payment)
	}
	return out, rows.Err()
}

func (s *Store) ApplyProviderResult(ctx context.Context, reference, eventID, providerStatus string, at time.Time, recordUnchanged bool) (Payment, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Payment{}, false, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, paymentSelect+`
		WHERE p.provider_reference = $1
		FOR UPDATE OF p, o
	`, reference)
	if err != nil {
		return Payment{}, false, err
	}
	if !rows.Next() {
		rows.Close()
		if err := rows.Err(); err != nil {
			return Payment{}, false, err
		}
		return Payment{}, false, ErrNotFound
	}
	payment, orderStatus, err := scanPayment(rows)
	rows.Close()
	if err != nil {
		return Payment{}, false, err
	}

	decision := decide(State{
		PaymentStatus: payment.Status,
		OrderStatus:   orderStatus,
		ExpiresAt:     payment.ExpiresAt,
	}, providerStatus, at)
	if !decision.Changed && !recordUnchanged {
		return payment, false, nil
	}
	storedID := eventID
	if storedID == "" {
		storedID = fmt.Sprintf("local:%s:%s:%s", reference, decision.PaymentStatus, decision.OrderStatus)
	}
	var eventUUID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO payment_events (provider, event_id, payment_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, event_id) DO NOTHING
		RETURNING id
	`, eventProvider(payment.Provider), storedID, payment.ID).Scan(&eventUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return payment, true, nil
	}
	if err != nil {
		return Payment{}, false, err
	}
	if !decision.Changed {
		if err := tx.Commit(ctx); err != nil {
			return Payment{}, false, err
		}
		return payment, false, nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payments SET
			status = $2,
			late_success = late_success OR $3,
			paid_at = CASE WHEN $2 = 'PAID' THEN COALESCE(paid_at, now()) ELSE paid_at END
		WHERE id = $1
	`, payment.ID, decision.PaymentStatus, decision.Late); err != nil {
		return Payment{}, false, err
	}
	if decision.OrderStatus != "" && decision.OrderStatus != orderStatus {
		tag, err := tx.Exec(ctx, `
			UPDATE orders SET
				status = $2,
				decision_deadline_at = CASE WHEN $2 = 'AWAITING_VENDOR' THEN now() + interval '10 minutes' ELSE decision_deadline_at END,
				updated_at = now()
			WHERE id = $1 AND status = 'PENDING_PAYMENT'
		`, payment.OrderID, decision.OrderStatus)
		if err != nil {
			return Payment{}, false, err
		}
		if tag.RowsAffected() != 1 {
			return Payment{}, false, ErrConflict
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, previous_status, new_status, reason)
			VALUES ($1, 'PENDING_PAYMENT', $2, $3)
		`, payment.OrderID, decision.OrderStatus, decision.Reason); err != nil {
			return Payment{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Payment{}, false, err
	}
	payment.Status = decision.PaymentStatus
	if decision.Late {
		payment.LateSuccess = true
	}
	return payment, false, nil
}

func (s *Store) MarkRefundDue(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE payments SET status = 'REFUND_PENDING'
		WHERE order_id = $1 AND status IN ('PAID', 'REFUND_PENDING')
	`, orderID)
	if err != nil {
		return Payment{}, err
	}
	if tag.RowsAffected() == 0 {
		return Payment{}, ErrConflict
	}
	return s.Find(ctx, orderID)
}

func eventProvider(provider string) string {
	if provider == "" {
		return "unknown"
	}
	return provider
}

const paymentSelect = `
	SELECT p.id, p.order_id, o.customer_user_id, o.status, p.status, p.amount_ugx,
	       COALESCE(p.provider, ''), p.payer_phone, COALESCE(p.provider_reference, ''),
	       p.expires_at, p.late_success
	FROM payments p
	JOIN orders o ON o.id = p.order_id
`

type scanner interface {
	Scan(dest ...any) error
}

func scanPayment(row scanner) (Payment, string, error) {
	var payment Payment
	var orderStatus string
	var expiresAt *time.Time
	err := row.Scan(
		&payment.ID, &payment.OrderID, &payment.CustomerID, &orderStatus, &payment.Status, &payment.AmountUGX,
		&payment.Provider, &payment.Phone, &payment.Reference, &expiresAt, &payment.LateSuccess,
	)
	if expiresAt != nil {
		payment.ExpiresAt = *expiresAt
	}
	return payment, orderStatus, err
}
