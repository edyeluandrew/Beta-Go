package vendor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists vendors in Postgres.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateVendor(ctx context.Context, vendor Vendor) (Vendor, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Vendor{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO vendors (business_name, contact_name, contact_email, phone, address, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, vendor.BusinessName, vendor.ContactName, vendor.ContactEmail, vendor.Phone, vendor.Address, vendor.Status).Scan(&vendor.ID)
	if err != nil {
		return Vendor{}, fmt.Errorf("create vendor: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO commission_accounts (vendor_id) VALUES ($1)`, vendor.ID); err != nil {
		return Vendor{}, fmt.Errorf("create commission account: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Vendor{}, err
	}
	return vendor, nil
}

func (s *Store) ListVendors(ctx context.Context) ([]Vendor, error) {
	rows, err := s.pool.Query(ctx, vendorSelect+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Vendor
	for rows.Next() {
		vendor, err := scanVendor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, vendor)
	}
	return out, rows.Err()
}

func (s *Store) GetVendor(ctx context.Context, id uuid.UUID) (Vendor, error) {
	row := s.pool.QueryRow(ctx, vendorSelect+` WHERE id = $1`, id)
	vendor, err := scanVendor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Vendor{}, ErrNotFound
	}
	return vendor, err
}

func (s *Store) UpdateIdentity(ctx context.Context, id uuid.UUID, businessName, contactName, phone, address string) (Vendor, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE vendors
		SET business_name = $2, contact_name = $3, phone = $4, address = $5, updated_at = now()
		WHERE id = $1
	`, id, businessName, contactName, phone, address)
	if err != nil {
		return Vendor{}, err
	}
	if tag.RowsAffected() == 0 {
		return Vendor{}, ErrNotFound
	}
	return s.GetVendor(ctx, id)
}

func (s *Store) UpdateShop(ctx context.Context, id uuid.UUID, description, logoURL, photoURL string, deliveryFee int, isOpen, accepting bool) (Vendor, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE vendors
		SET description = $2, logo_url = NULLIF($3, ''), shop_photo_url = NULLIF($4, ''),
		    delivery_fee_ugx = $5, is_open = $6, accepting_orders = $7, updated_at = now()
		WHERE id = $1
	`, id, description, logoURL, photoURL, deliveryFee, isOpen, accepting)
	if err != nil {
		return Vendor{}, err
	}
	if tag.RowsAffected() == 0 {
		return Vendor{}, ErrNotFound
	}
	return s.GetVendor(ctx, id)
}

func (s *Store) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE vendors SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SaveInvitation(ctx context.Context, vendorID uuid.UUID, email, tokenHash string, expiresAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE vendor_invitations SET expires_at = now()
		WHERE vendor_id = $1 AND used_at IS NULL
	`, vendorID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO vendor_invitations (vendor_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, vendorID, email, tokenHash, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) FindInvitation(ctx context.Context, tokenHash string) (Invitation, error) {
	var invite Invitation
	err := s.pool.QueryRow(ctx, `
		SELECT id, vendor_id, email, token_hash, expires_at, used_at IS NOT NULL
		FROM vendor_invitations WHERE token_hash = $1
	`, tokenHash).Scan(&invite.ID, &invite.VendorID, &invite.Email, &invite.TokenHash, &invite.ExpiresAt, &invite.Used)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, ErrNotFound
	}
	return invite, err
}

func (s *Store) ConsumeInvitation(ctx context.Context, invitationID, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var vendorID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE vendor_invitations SET used_at = now()
		WHERE id = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING vendor_id
	`, invitationID).Scan(&vendorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvitation
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO vendor_users (vendor_id, user_id) VALUES ($1, $2)`, vendorID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) VendorIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT vendor_id FROM vendor_users WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

func (s *Store) UpsertDestination(ctx context.Context, vendorID uuid.UUID, provider, account string) (Destination, error) {
	var destination Destination
	err := s.pool.QueryRow(ctx, `
		INSERT INTO vendor_payment_destinations (vendor_id, provider, account)
		VALUES ($1, $2, $3)
		ON CONFLICT (vendor_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			account = EXCLUDED.account,
			status = 'UNVERIFIED',
			updated_at = now()
		RETURNING provider, account, status
	`, vendorID, provider, account).Scan(&destination.Provider, &destination.Account, &destination.Status)
	if err != nil {
		return Destination{}, err
	}
	return destination, nil
}

func (s *Store) SetDestinationStatus(ctx context.Context, vendorID uuid.UUID, status string) (Destination, error) {
	var destination Destination
	err := s.pool.QueryRow(ctx, `
		UPDATE vendor_payment_destinations SET status = $2, updated_at = now()
		WHERE vendor_id = $1
		RETURNING provider, account, status
	`, vendorID, status).Scan(&destination.Provider, &destination.Account, &destination.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Destination{}, ErrNotFound
	}
	return destination, err
}

func (s *Store) GetDestination(ctx context.Context, vendorID uuid.UUID) (Destination, bool, error) {
	var destination Destination
	err := s.pool.QueryRow(ctx, `
		SELECT provider, account, status FROM vendor_payment_destinations WHERE vendor_id = $1
	`, vendorID).Scan(&destination.Provider, &destination.Account, &destination.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Destination{}, false, nil
	}
	if err != nil {
		return Destination{}, false, err
	}
	return destination, true, nil
}

func (s *Store) ReplaceHours(ctx context.Context, vendorID uuid.UUID, hours []Hours) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM vendor_hours WHERE vendor_id = $1`, vendorID); err != nil {
		return err
	}
	for _, day := range hours {
		if _, err := tx.Exec(ctx, `
			INSERT INTO vendor_hours (vendor_id, weekday, opens_at, closes_at, closed)
			VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
		`, vendorID, day.Weekday, day.OpensAt, day.ClosesAt, day.Closed); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListHours(ctx context.Context, vendorID uuid.UUID) ([]Hours, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT weekday, COALESCE(opens_at, ''), COALESCE(closes_at, ''), closed
		FROM vendor_hours WHERE vendor_id = $1 ORDER BY weekday
	`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hours
	for rows.Next() {
		var day Hours
		if err := rows.Scan(&day.Weekday, &day.OpensAt, &day.ClosesAt, &day.Closed); err != nil {
			return nil, err
		}
		out = append(out, day)
	}
	return out, rows.Err()
}

func (s *Store) AddCategory(ctx context.Context, vendorID uuid.UUID, name string) (Category, error) {
	var category Category
	err := s.pool.QueryRow(ctx, `
		INSERT INTO menu_categories (vendor_id, name) VALUES ($1, $2) RETURNING id, name
	`, vendorID, name).Scan(&category.ID, &category.Name)
	return category, err
}

func (s *Store) ListCategories(ctx context.Context, vendorID uuid.UUID) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM menu_categories WHERE vendor_id = $1 ORDER BY created_at`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			return nil, err
		}
		out = append(out, category)
	}
	return out, rows.Err()
}

func (s *Store) AddItem(ctx context.Context, item Item) (Item, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO menu_items (vendor_id, category_id, name, description, price_ugx, available, photo_url)
		VALUES ($1, NULLIF($2, '00000000-0000-0000-0000-000000000000'::uuid), $3, $4, $5, $6, $7)
		RETURNING id
	`, item.VendorID, item.CategoryID, item.Name, item.Description, item.PriceUGX, item.Available, item.PhotoURL).Scan(&item.ID)
	return item, err
}

func (s *Store) UpdateItem(ctx context.Context, vendorID, itemID uuid.UUID, name, description string, price int, available bool, photoURL string) (Item, error) {
	item := Item{ID: itemID, VendorID: vendorID, Name: name, Description: description, PriceUGX: price, Available: available, PhotoURL: photoURL}
	err := s.pool.QueryRow(ctx, `
		UPDATE menu_items
		SET name = $3, description = $4, price_ugx = $5, available = $6,
		    photo_url = CASE WHEN $7 = '' THEN photo_url ELSE $7 END, updated_at = now()
		WHERE id = $1 AND vendor_id = $2
		RETURNING COALESCE(category_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(photo_url, '')
	`, itemID, vendorID, name, description, price, available, photoURL).Scan(&item.CategoryID, &item.PhotoURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func (s *Store) ListItems(ctx context.Context, vendorID uuid.UUID) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, vendor_id, COALESCE(category_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       name, description, price_ugx, available, COALESCE(photo_url, '')
		FROM menu_items WHERE vendor_id = $1 ORDER BY created_at
	`, vendorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.VendorID, &item.CategoryID, &item.Name, &item.Description, &item.PriceUGX, &item.Available, &item.PhotoURL); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) AvailableBalance(ctx context.Context, vendorID uuid.UUID) (int, error) {
	var balance int
	err := s.pool.QueryRow(ctx, `SELECT available_ugx FROM commission_accounts WHERE vendor_id = $1`, vendorID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return balance, err
}

func (s *Store) Audit(ctx context.Context, actorID, entityID uuid.UUID, action, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_id, reason) VALUES ($1, $2, $3, $4)
	`, actorID, action, entityID, reason)
	return err
}

const vendorSelect = `
	SELECT id, business_name, contact_name, contact_email, phone, address, description,
	       COALESCE(logo_url, ''), COALESCE(shop_photo_url, ''), status, is_open, accepting_orders, delivery_fee_ugx
	FROM vendors`

type scanner interface {
	Scan(dest ...any) error
}

func scanVendor(row scanner) (Vendor, error) {
	var vendor Vendor
	err := row.Scan(
		&vendor.ID, &vendor.BusinessName, &vendor.ContactName, &vendor.ContactEmail, &vendor.Phone, &vendor.Address,
		&vendor.Description, &vendor.LogoURL, &vendor.ShopPhotoURL, &vendor.Status, &vendor.IsOpen, &vendor.AcceptingOrders, &vendor.DeliveryFeeUGX,
	)
	return vendor, err
}
