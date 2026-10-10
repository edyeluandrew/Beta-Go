package commission

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"beta-go/api/internal/order"
	"beta-go/api/internal/vendor"
)

// Shops finds the kitchen and can hide it when the pool is low.
type Shops interface {
	VendorIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
	GetVendor(ctx context.Context, id uuid.UUID) (vendor.Vendor, error)
}

// Books is the locked pool and the posted ledger.
type Books interface {
	Account(ctx context.Context, vendorID uuid.UUID) (Account, error)
	Post(ctx context.Context, posting Posting) (Account, bool, error)
	ListEntries(ctx context.Context, vendorID uuid.UUID) ([]Entry, error)
	SaveTopUp(ctx context.Context, topUp TopUp) (TopUp, error)
	FindTopUp(ctx context.Context, id uuid.UUID) (TopUp, error)
	FindTopUpByKey(ctx context.Context, vendorID uuid.UUID, key string) (TopUp, bool, error)
	FindTopUpByReference(ctx context.Context, reference string) (TopUp, error)
	SaveAdjustment(ctx context.Context, adjustment Adjustment) (Adjustment, error)
	FindAdjustment(ctx context.Context, id uuid.UUID) (Adjustment, error)
	Posted(ctx context.Context, operation string, operationID uuid.UUID) (bool, error)
}

// Orders supplies the snapshotted commission for reserve, capture, and release.
type Orders interface {
	Get(ctx context.Context, id uuid.UUID) (order.Order, error)
}

// Settings reads whole-number policy values.
type Settings interface {
	Int(ctx context.Context, key string) (int, error)
}

// Provider starts a pool top-up. It is not the food-payment collector.
type Provider interface {
	Initiate(ctx context.Context, id string) (reference string, err error)
}

// Posting is one money movement. The store posts it once.
type Posting struct {
	VendorID     uuid.UUID
	Available    int
	Reserved     int
	VendorStatus string
	Operation    string
	OperationID  uuid.UUID
	OrderID      uuid.UUID
	Entries      []Entry
	TopUp        *TopUp
	Adjustment   *Adjustment
	EventID      string
	Provider     string
}

// Service is the Model A pool: top-up, reserve, capture, release, adjust.
type Service struct {
	shops    Shops
	books    Books
	orders   Orders
	settings Settings
	provider Provider
	secret   string
}

func NewService(shops Shops, books Books, orders Orders, settings Settings, provider Provider, secret string) *Service {
	return &Service{shops: shops, books: books, orders: orders, settings: settings, provider: provider, secret: secret}
}

func (s *Service) AccountForVendor(ctx context.Context, userID uuid.UUID) (Account, error) {
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return Account{}, mapShopErr(err)
	}
	return s.account(ctx, vendorID)
}

func (s *Service) AccountForAdmin(ctx context.Context, vendorID uuid.UUID) (Account, error) {
	if _, err := s.shops.GetVendor(ctx, vendorID); err != nil {
		return Account{}, mapShopErr(err)
	}
	return s.account(ctx, vendorID)
}

func (s *Service) LedgerForVendor(ctx context.Context, userID uuid.UUID) ([]Entry, error) {
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return nil, mapShopErr(err)
	}
	return s.books.ListEntries(ctx, vendorID)
}

func (s *Service) account(ctx context.Context, vendorID uuid.UUID) (Account, error) {
	account, err := s.books.Account(ctx, vendorID)
	if err != nil {
		return Account{}, err
	}
	kitchen, err := s.shops.GetVendor(ctx, vendorID)
	if err != nil {
		return Account{}, mapShopErr(err)
	}
	account.VendorStatus = kitchen.Status
	account.LowBalance = account.Available < s.intOr(ctx, "commission_low_balance_ugx", DefaultWarning)
	return account, nil
}

func (s *Service) StartTopUp(ctx context.Context, userID uuid.UUID, amount int, providerName, phone, key string) (TopUp, bool, error) {
	if s.provider == nil || s.secret == "" {
		return TopUp{}, false, ErrNotConfigured
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return TopUp{}, false, ErrBadInput
	}
	min := s.intOr(ctx, "commission_topup_min_ugx", DefaultTopUpMin)
	max := s.intOr(ctx, "commission_topup_max_ugx", DefaultTopUpMax)
	if amount < min || amount > max {
		return TopUp{}, false, ErrBadInput
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName != ProviderMTN && providerName != ProviderAirtel {
		return TopUp{}, false, ErrBadInput
	}
	normalized, err := normalizePhone(phone)
	if err != nil {
		return TopUp{}, false, ErrBadInput
	}
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return TopUp{}, false, mapShopErr(err)
	}
	if existing, ok, err := s.books.FindTopUpByKey(ctx, vendorID, key); err != nil {
		return TopUp{}, false, err
	} else if ok {
		if existing.AmountUGX != amount || existing.Provider != providerName || existing.Phone != normalized {
			return TopUp{}, false, ErrConflict
		}
		return existing, false, nil
	}
	reference, err := s.provider.Initiate(ctx, vendorID.String())
	if err != nil {
		return TopUp{}, false, err
	}
	saved, err := s.books.SaveTopUp(ctx, TopUp{
		VendorID: vendorID, AmountUGX: amount, Provider: providerName,
		Phone: normalized, Status: TopUpPending, Reference: reference, IdempotencyKey: key,
	})
	return saved, true, err
}

func (s *Service) GetTopUp(ctx context.Context, userID, id uuid.UUID) (TopUp, error) {
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return TopUp{}, mapShopErr(err)
	}
	topUp, err := s.books.FindTopUp(ctx, id)
	if err != nil || topUp.VendorID != vendorID {
		return TopUp{}, ErrNotFound
	}
	return topUp, nil
}

func (s *Service) AcceptTopUpWebhook(ctx context.Context, body []byte, signature string) (TopUp, error) {
	if s.provider == nil || s.secret == "" {
		return TopUp{}, ErrNotConfigured
	}
	if !validSignature(s.secret, body, signature) {
		return TopUp{}, ErrBadSignature
	}
	var payload struct {
		EventID   string `json:"eventId"`
		Reference string `json:"reference"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TopUp{}, ErrBadInput
	}
	payload.EventID = strings.TrimSpace(payload.EventID)
	payload.Reference = strings.TrimSpace(payload.Reference)
	payload.Status = strings.ToUpper(strings.TrimSpace(payload.Status))
	if payload.EventID == "" || payload.Reference == "" {
		return TopUp{}, ErrBadInput
	}
	topUp, err := s.books.FindTopUpByReference(ctx, payload.Reference)
	if err != nil {
		return TopUp{}, err
	}
	if payload.Status != "SUCCESS" {
		return topUp, nil
	}
	if topUp.Status == TopUpPaid {
		return topUp, nil
	}
	account, err := s.books.Account(ctx, topUp.VendorID)
	if err != nil {
		return TopUp{}, err
	}
	kitchen, err := s.shops.GetVendor(ctx, topUp.VendorID)
	if err != nil {
		return TopUp{}, mapShopErr(err)
	}
	nextAvailable := account.Available + topUp.AmountUGX
	paid := topUp
	paid.Status = TopUpPaid
	_, _, err = s.books.Post(ctx, Posting{
		VendorID: topUp.VendorID, Available: nextAvailable, Reserved: account.Reserved,
		VendorStatus: activityStatus(kitchen.Status, nextAvailable, s.intOr(ctx, "commission_activity_floor_ugx", DefaultFloor)),
		Operation:    OpTopUp, OperationID: topUp.ID,
		Entries: []Entry{
			line(topUp.VendorID, StreamExternal, DirDebit, topUp.AmountUGX, OpTopUp, topUp.ID, uuid.Nil),
			line(topUp.VendorID, StreamAvailable, DirCredit, topUp.AmountUGX, OpTopUp, topUp.ID, uuid.Nil),
		},
		TopUp: &paid, EventID: payload.EventID, Provider: topUp.Provider,
	})
	if err != nil {
		return TopUp{}, err
	}
	return s.books.FindTopUp(ctx, topUp.ID)
}

func (s *Service) Reserve(ctx context.Context, orderID uuid.UUID) (Account, error) {
	return s.moveOrder(ctx, orderID, OpReserve)
}

func (s *Service) Capture(ctx context.Context, orderID uuid.UUID) (Account, error) {
	return s.moveOrder(ctx, orderID, OpCapture)
}

func (s *Service) Release(ctx context.Context, orderID uuid.UUID) (Account, error) {
	return s.moveOrder(ctx, orderID, OpRelease)
}

func (s *Service) moveOrder(ctx context.Context, orderID uuid.UUID, operation string) (Account, error) {
	placed, err := s.orders.Get(ctx, orderID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			return Account{}, ErrNotFound
		}
		return Account{}, err
	}
	if placed.CommissionUGX <= 0 {
		return Account{}, ErrBadInput
	}
	account, err := s.books.Account(ctx, placed.VendorID)
	if err != nil {
		return Account{}, err
	}
	kitchen, err := s.shops.GetVendor(ctx, placed.VendorID)
	if err != nil {
		return Account{}, mapShopErr(err)
	}
	already, err := s.books.Posted(ctx, operation, orderID)
	if err != nil {
		return Account{}, err
	}
	if already {
		account.VendorStatus = kitchen.Status
		account.LowBalance = account.Available < s.intOr(ctx, "commission_low_balance_ugx", DefaultWarning)
		return account, nil
	}
	if operation == OpCapture {
		if released, err := s.books.Posted(ctx, OpRelease, orderID); err != nil {
			return Account{}, err
		} else if released {
			return Account{}, ErrConflict
		}
	}
	if operation == OpRelease {
		if captured, err := s.books.Posted(ctx, OpCapture, orderID); err != nil {
			return Account{}, err
		} else if captured {
			return Account{}, ErrConflict
		}
	}
	entries, available, reserved, err := planOrderMove(account, placed.CommissionUGX, placed.VendorID, orderID, operation)
	if err != nil {
		return Account{}, err
	}
	floor := s.intOr(ctx, "commission_activity_floor_ugx", DefaultFloor)
	posted, _, err := s.books.Post(ctx, Posting{
		VendorID: placed.VendorID, Available: available, Reserved: reserved,
		VendorStatus: activityStatus(kitchen.Status, available, floor),
		Operation:    operation, OperationID: orderID, OrderID: orderID, Entries: entries,
	})
	if err != nil {
		return Account{}, err
	}
	posted.LowBalance = posted.Available < s.intOr(ctx, "commission_low_balance_ugx", DefaultWarning)
	return posted, nil
}

func (s *Service) RequestAdjustment(ctx context.Context, actor, vendorID uuid.UUID, amount int, reason string) (Adjustment, Account, error) {
	reason = strings.TrimSpace(reason)
	if amount == 0 || reason == "" {
		return Adjustment{}, Account{}, ErrBadInput
	}
	if _, err := s.shops.GetVendor(ctx, vendorID); err != nil {
		return Adjustment{}, Account{}, mapShopErr(err)
	}
	saved, err := s.books.SaveAdjustment(ctx, Adjustment{
		VendorID: vendorID, AmountUGX: amount, Reason: reason, RequestedBy: actor, Status: AdjPending,
	})
	if err != nil {
		return Adjustment{}, Account{}, err
	}
	limit := s.intOr(ctx, "commission_adjustment_dual_approve_ugx", DefaultDualLimit)
	if abs(amount) > limit {
		account, err := s.account(ctx, vendorID)
		return saved, account, err
	}
	saved.ApprovedBy = actor
	return s.applyAdjustment(ctx, saved)
}

func (s *Service) ApproveAdjustment(ctx context.Context, actor, id uuid.UUID) (Adjustment, Account, error) {
	saved, err := s.books.FindAdjustment(ctx, id)
	if err != nil {
		return Adjustment{}, Account{}, err
	}
	if saved.Status == AdjApplied {
		account, err := s.account(ctx, saved.VendorID)
		return saved, account, err
	}
	if saved.RequestedBy == actor {
		return Adjustment{}, Account{}, ErrForbidden
	}
	saved.ApprovedBy = actor
	return s.applyAdjustment(ctx, saved)
}

func (s *Service) applyAdjustment(ctx context.Context, adjustment Adjustment) (Adjustment, Account, error) {
	account, err := s.books.Account(ctx, adjustment.VendorID)
	if err != nil {
		return Adjustment{}, Account{}, err
	}
	next := account.Available + adjustment.AmountUGX
	if next < 0 {
		return Adjustment{}, Account{}, ErrInsufficient
	}
	kitchen, err := s.shops.GetVendor(ctx, adjustment.VendorID)
	if err != nil {
		return Adjustment{}, Account{}, mapShopErr(err)
	}
	applied := adjustment
	applied.Status = AdjApplied
	amount := abs(adjustment.AmountUGX)
	var entries []Entry
	if adjustment.AmountUGX > 0 {
		entries = []Entry{
			line(adjustment.VendorID, StreamExternal, DirDebit, amount, OpAdjustment, adjustment.ID, uuid.Nil),
			line(adjustment.VendorID, StreamAvailable, DirCredit, amount, OpAdjustment, adjustment.ID, uuid.Nil),
		}
	} else {
		entries = []Entry{
			line(adjustment.VendorID, StreamAvailable, DirDebit, amount, OpAdjustment, adjustment.ID, uuid.Nil),
			line(adjustment.VendorID, StreamExternal, DirCredit, amount, OpAdjustment, adjustment.ID, uuid.Nil),
		}
	}
	posted, _, err := s.books.Post(ctx, Posting{
		VendorID: adjustment.VendorID, Available: next, Reserved: account.Reserved,
		VendorStatus: activityStatus(kitchen.Status, next, s.intOr(ctx, "commission_activity_floor_ugx", DefaultFloor)),
		Operation:    OpAdjustment, OperationID: adjustment.ID, Entries: entries, Adjustment: &applied,
	})
	if err != nil {
		return Adjustment{}, Account{}, err
	}
	posted.LowBalance = posted.Available < s.intOr(ctx, "commission_low_balance_ugx", DefaultWarning)
	return applied, posted, nil
}

func (s *Service) intOr(ctx context.Context, key string, fallback int) int {
	if s.settings == nil {
		return fallback
	}
	value, err := s.settings.Int(ctx, key)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func planOrderMove(account Account, commission int, vendorID, orderID uuid.UUID, operation string) ([]Entry, int, int, error) {
	switch operation {
	case OpReserve:
		if account.Available < commission {
			return nil, 0, 0, ErrInsufficient
		}
		return []Entry{
			line(vendorID, StreamAvailable, DirDebit, commission, OpReserve, orderID, orderID),
			line(vendorID, StreamReserved, DirCredit, commission, OpReserve, orderID, orderID),
		}, account.Available - commission, account.Reserved + commission, nil
	case OpCapture:
		if account.Reserved < commission {
			return nil, 0, 0, ErrConflict
		}
		return []Entry{
			line(vendorID, StreamReserved, DirDebit, commission, OpCapture, orderID, orderID),
			line(vendorID, StreamRevenue, DirCredit, commission, OpCapture, orderID, orderID),
		}, account.Available, account.Reserved - commission, nil
	case OpRelease:
		if account.Reserved < commission {
			return nil, 0, 0, ErrConflict
		}
		return []Entry{
			line(vendorID, StreamReserved, DirDebit, commission, OpRelease, orderID, orderID),
			line(vendorID, StreamAvailable, DirCredit, commission, OpRelease, orderID, orderID),
		}, account.Available + commission, account.Reserved - commission, nil
	default:
		return nil, 0, 0, ErrBadInput
	}
}

func line(vendorID uuid.UUID, stream, direction string, amount int, operation string, operationID, orderID uuid.UUID) Entry {
	return Entry{
		VendorID: vendorID, Stream: stream, Direction: direction, AmountUGX: amount,
		Operation: operation, OperationID: operationID, OrderID: orderID,
	}
}

func activityStatus(current string, available, floor int) string {
	switch current {
	case vendor.StatusPaused, vendor.StatusSuspended, vendor.StatusPending, vendor.StatusRejected:
		return current
	}
	if available < floor {
		return vendor.StatusInactive
	}
	if current == vendor.StatusActive || current == vendor.StatusInactive {
		return vendor.StatusActive
	}
	return current
}

func validSignature(secret string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	got := strings.ToLower(strings.TrimSpace(signature))
	return hmac.Equal([]byte(expected), []byte(got))
}

func normalizePhone(raw string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	switch {
	case len(digits) == 10 && strings.HasPrefix(digits, "07"):
		return "256" + digits[1:], nil
	case len(digits) == 12 && strings.HasPrefix(digits, "2567"):
		return digits, nil
	default:
		return "", ErrBadInput
	}
}

func mapShopErr(err error) error {
	if errors.Is(err, vendor.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
