package commission

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists the pool in Postgres. Food payments never write these tables.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Account(ctx context.Context, vendorID uuid.UUID) (Account, error) {
	var account Account
	account.VendorID = vendorID
	err := s.pool.QueryRow(ctx, `
		SELECT available_ugx, reserved_ugx FROM commission_accounts WHERE vendor_id = $1
	`, vendorID).Scan(&account.Available, &account.Reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return account, err
}

func (s *Store) Post(ctx context.Context, posting Posting) (Account, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, false, err
	}
	defer tx.Rollback(ctx)

	if posting.EventID != "" {
		var eventID uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO commission_events (provider, event_id, top_up_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (provider, event_id) DO NOTHING
			RETURNING id
		`, eventProvider(posting.Provider), posting.EventID, posting.OperationID).Scan(&eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			account, err := s.Account(ctx, posting.VendorID)
			return account, true, err
		}
		if err != nil {
			return Account{}, false, err
		}
	}

	var available, reserved int
	err = tx.QueryRow(ctx, `
		SELECT available_ugx, reserved_ugx FROM commission_accounts WHERE vendor_id = $1 FOR UPDATE
	`, posting.VendorID).Scan(&available, &reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, ErrNotFound
	}
	if err != nil {
		return Account{}, false, err
	}

	posted := false
	err = tx.QueryRow(ctx, `
		SELECT true FROM commission_ledger
		WHERE operation = $1 AND operation_id = $2
		LIMIT 1
	`, posting.Operation, posting.OperationID).Scan(&posted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false, err
	}
	if posted {
		account := Account{VendorID: posting.VendorID, Available: available, Reserved: reserved, VendorStatus: posting.VendorStatus}
		return account, true, tx.Commit(ctx)
	}

	if posting.Available < 0 || posting.Reserved < 0 {
		return Account{}, false, ErrInsufficient
	}
	if _, err := tx.Exec(ctx, `
		UPDATE commission_accounts SET available_ugx = $2, reserved_ugx = $3 WHERE vendor_id = $1
	`, posting.VendorID, posting.Available, posting.Reserved); err != nil {
		return Account{}, false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE vendors SET status = $2, updated_at = now()
		WHERE id = $1 AND status IN ('ACTIVE', 'INACTIVE')
	`, posting.VendorID, posting.VendorStatus); err != nil {
		return Account{}, false, err
	}
	for _, entry := range posting.Entries {
		var orderID *uuid.UUID
		if entry.OrderID != uuid.Nil {
			id := entry.OrderID
			orderID = &id
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO commission_ledger (
				vendor_id, stream, direction, amount_ugx, operation, operation_id, order_id
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, posting.VendorID, entry.Stream, entry.Direction, entry.AmountUGX, entry.Operation, entry.OperationID, orderID); err != nil {
			return Account{}, false, err
		}
	}
	if posting.TopUp != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE commission_top_ups SET status = $2 WHERE id = $1
		`, posting.TopUp.ID, posting.TopUp.Status); err != nil {
			return Account{}, false, err
		}
	}
	if posting.Adjustment != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE commission_adjustments SET status = $2, approved_by = $3 WHERE id = $1
		`, posting.Adjustment.ID, posting.Adjustment.Status, posting.Adjustment.ApprovedBy); err != nil {
			return Account{}, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, false, err
	}
	return Account{
		VendorID: posting.VendorID, Available: posting.Available, Reserved: posting.Reserved,
		VendorStatus: posting.VendorStatus,
	}, false, nil
}

func (s *Store) ListEntries(ctx context.Context, vendorID uuid.UUID) ([]Entry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, vendor_id, stream, direction, amount_ugx, operation, operation_id, COALESCE(order_id, '00000000-0000-0000-0000-000000000000'), created_at
		FROM commission_ledger
		WHERE vendor_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.VendorID, &entry.Stream, &entry.Direction, &entry.AmountUGX, &entry.Operation, &entry.OperationID, &entry.OrderID, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *Store) SaveTopUp(ctx context.Context, topUp TopUp) (TopUp, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO commission_top_ups (vendor_id, amount_ugx, provider, payer_phone, status, provider_reference, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id
	`, topUp.VendorID, topUp.AmountUGX, topUp.Provider, topUp.Phone, topUp.Status, topUp.Reference, topUp.IdempotencyKey).Scan(&topUp.ID)
	return topUp, err
}

func (s *Store) FindTopUp(ctx context.Context, id uuid.UUID) (TopUp, error) {
	return scanTopUp(s.pool.QueryRow(ctx, topUpSelect+` WHERE id = $1`, id))
}

func (s *Store) FindTopUpByKey(ctx context.Context, vendorID uuid.UUID, key string) (TopUp, bool, error) {
	topUp, err := scanTopUp(s.pool.QueryRow(ctx, topUpSelect+` WHERE vendor_id = $1 AND idempotency_key = $2`, vendorID, key))
	if errors.Is(err, ErrNotFound) {
		return TopUp{}, false, nil
	}
	return topUp, err == nil, err
}

func (s *Store) FindTopUpByReference(ctx context.Context, reference string) (TopUp, error) {
	return scanTopUp(s.pool.QueryRow(ctx, topUpSelect+` WHERE provider_reference = $1`, reference))
}

func (s *Store) SaveAdjustment(ctx context.Context, adjustment Adjustment) (Adjustment, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO commission_adjustments (vendor_id, amount_ugx, reason, requested_by, status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id
	`, adjustment.VendorID, adjustment.AmountUGX, adjustment.Reason, adjustment.RequestedBy, adjustment.Status).Scan(&adjustment.ID)
	return adjustment, err
}

func (s *Store) FindAdjustment(ctx context.Context, id uuid.UUID) (Adjustment, error) {
	var adjustment Adjustment
	var approved *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id, vendor_id, amount_ugx, reason, requested_by, approved_by, status
		FROM commission_adjustments WHERE id = $1
	`, id).Scan(&adjustment.ID, &adjustment.VendorID, &adjustment.AmountUGX, &adjustment.Reason, &adjustment.RequestedBy, &approved, &adjustment.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Adjustment{}, ErrNotFound
	}
	if approved != nil {
		adjustment.ApprovedBy = *approved
	}
	return adjustment, err
}

func (s *Store) Posted(ctx context.Context, operation string, operationID uuid.UUID) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM commission_ledger WHERE operation = $1 AND operation_id = $2)
	`, operation, operationID).Scan(&found)
	return found, err
}

const topUpSelect = `
	SELECT id, vendor_id, amount_ugx, provider, payer_phone, status, COALESCE(provider_reference, ''), idempotency_key
	FROM commission_top_ups`

func scanTopUp(row pgx.Row) (TopUp, error) {
	var topUp TopUp
	err := row.Scan(&topUp.ID, &topUp.VendorID, &topUp.AmountUGX, &topUp.Provider, &topUp.Phone, &topUp.Status, &topUp.Reference, &topUp.IdempotencyKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return TopUp{}, ErrNotFound
	}
	return topUp, err
}

func eventProvider(provider string) string {
	if provider == "" {
		return "unknown"
	}
	return provider
}
