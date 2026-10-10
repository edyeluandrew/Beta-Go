package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists orders in Postgres.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Int(ctx context.Context, key string) (int, error) {
	var value int
	err := s.pool.QueryRow(ctx, `SELECT value_int FROM platform_settings WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return value, err
}

func (s *Store) FindByKey(ctx context.Context, customerID uuid.UUID, key string) (Order, bool, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM orders WHERE customer_user_id = $1 AND idempotency_key = $2
	`, customerID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, err
	}
	order, err := s.Get(ctx, id)
	return order, err == nil, err
}

func (s *Store) Create(ctx context.Context, order Order) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			customer_user_id, vendor_id, status, fulfilment, delivery_address, customer_phone,
			food_subtotal_ugx, delivery_fee_ugx, total_ugx, commission_rate_percent, commission_ugx,
			idempotency_key, request_hash, pickup_code
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id
	`, order.CustomerID, order.VendorID, order.Status, order.Fulfilment, order.DeliveryAddress, order.CustomerPhone,
		order.FoodSubtotalUGX, order.DeliveryFeeUGX, order.TotalUGX, order.CommissionRate, order.CommissionUGX,
		order.IdempotencyKey, order.RequestHash, order.PickupCode).Scan(&order.ID)
	if err != nil {
		return Order{}, fmt.Errorf("create order: %w", err)
	}
	for _, item := range order.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, menu_item_id, name, quantity, unit_price_ugx, subtotal_ugx)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, order.ID, item.MenuItemID, item.Name, item.Quantity, item.UnitPriceUGX, item.SubtotalUGX); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, previous_status, new_status, actor_user_id, reason)
		VALUES ($1, NULL, $2, $3, '')
	`, order.ID, order.Status, order.CustomerID); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (order_id, status, amount_ugx) VALUES ($1, 'PENDING', $2)
	`, order.ID, order.TotalUGX); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return s.Get(ctx, order.ID)
}

func (s *Store) ListForCustomer(ctx context.Context, customerID uuid.UUID) ([]Order, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM orders WHERE customer_user_id = $1 ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
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
		order, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, order)
	}
	return out, nil
}

func (s *Store) ListForVendor(ctx context.Context, vendorID uuid.UUID) ([]Order, error) {
	return s.listBy(ctx, `SELECT id FROM orders WHERE vendor_id = $1 ORDER BY created_at DESC`, vendorID)
}

func (s *Store) ListByStatus(ctx context.Context, status string) ([]Order, error) {
	return s.listBy(ctx, `SELECT id FROM orders WHERE status = $1 ORDER BY created_at DESC`, status)
}

func (s *Store) listBy(ctx context.Context, query string, arg any) ([]Order, error) {
	rows, err := s.pool.Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	out := make([]Order, 0, len(ids))
	for _, id := range ids {
		order, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, order)
	}
	return out, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Order, error) {
	var order Order
	var deadline, fulfilled *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, customer_user_id, vendor_id, status, fulfilment, delivery_address, customer_phone,
		       food_subtotal_ugx, delivery_fee_ugx, total_ugx, commission_rate_percent, commission_ugx,
		       idempotency_key, request_hash, pickup_code, cancel_requested, decision_deadline_at, fulfilled_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(
		&order.ID, &order.CustomerID, &order.VendorID, &order.Status, &order.Fulfilment,
		&order.DeliveryAddress, &order.CustomerPhone, &order.FoodSubtotalUGX, &order.DeliveryFeeUGX,
		&order.TotalUGX, &order.CommissionRate, &order.CommissionUGX, &order.IdempotencyKey, &order.RequestHash,
		&order.PickupCode, &order.CancelRequested, &deadline, &fulfilled, &order.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if deadline != nil {
		order.DecisionDeadline = *deadline
	}
	if fulfilled != nil {
		order.FulfilledAt = *fulfilled
	}
	rows, err := s.pool.Query(ctx, `
		SELECT menu_item_id, name, quantity, unit_price_ugx, subtotal_ugx
		FROM order_items WHERE order_id = $1
	`, id)
	if err != nil {
		return Order{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.MenuItemID, &item.Name, &item.Quantity, &item.UnitPriceUGX, &item.SubtotalUGX); err != nil {
			return Order{}, err
		}
		order.Items = append(order.Items, item)
	}
	return order, rows.Err()
}

func (s *Store) Cancel(ctx context.Context, id, actor uuid.UUID, reason string) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status = 'CANCELLED', updated_at = now()
		WHERE id = $1 AND status = 'PENDING_PAYMENT'
	`, id)
	if err != nil {
		return Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return Order{}, ErrForbidden
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, previous_status, new_status, actor_user_id, reason)
		VALUES ($1, 'PENDING_PAYMENT', 'CANCELLED', $2, $3)
	`, id, actor, reason); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Transition(ctx context.Context, id uuid.UUID, from, to string, actor uuid.UUID, reason string) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)
	query := `
		UPDATE orders SET status = $3, updated_at = now()
		WHERE id = $1 AND status = $2
	`
	if to == StatusFulfilled {
		query = `
			UPDATE orders SET status = $3, fulfilled_at = COALESCE(fulfilled_at, now()), updated_at = now()
			WHERE id = $1 AND status = $2
		`
	}
	tag, err := tx.Exec(ctx, query, id, from, to)
	if err != nil {
		return Order{}, err
	}
	if tag.RowsAffected() == 0 {
		current, getErr := s.Get(ctx, id)
		if getErr != nil {
			return Order{}, getErr
		}
		if current.Status == to {
			return current, nil
		}
		return Order{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, previous_status, new_status, actor_user_id, reason)
		VALUES ($1, $2, $3, $4, $5)
	`, id, from, to, actor, reason); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) SetCancelRequested(ctx context.Context, id uuid.UUID, requested bool) (Order, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE orders SET cancel_requested = $2, updated_at = now() WHERE id = $1
	`, id, requested)
	if err != nil {
		return Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return Order{}, ErrNotFound
	}
	return s.Get(ctx, id)
}
