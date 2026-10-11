package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/order"
)

// Ledger stores payment rows and applies a provider result once.
type Ledger interface {
	Find(ctx context.Context, orderID uuid.UUID) (Payment, error)
	SaveInitiation(ctx context.Context, payment Payment) (Payment, error)
	ApplyProviderResult(ctx context.Context, reference, eventID, providerStatus string, at time.Time, recordUnchanged bool) (Payment, bool, error)
	Pending(ctx context.Context) ([]Payment, error)
	MarkRefundDue(ctx context.Context, orderID uuid.UUID) (Payment, error)
}

// Orders identifies the customer order a payment belongs to.
type Orders interface {
	Get(ctx context.Context, id uuid.UUID) (order.Order, error)
}

// Settings reads the unpaid-order window.
type Settings interface {
	Int(ctx context.Context, key string) (int, error)
}

// Summary is the result of asking the provider about payments still pending.
type Summary struct {
	Checked       int
	Paid          int
	Expired       int
	RefundPending int
	Unchanged     int
}

// Service starts payments and accepts provider results. It never trusts a status sent by the customer.
type Service struct {
	ledger   Ledger
	orders   Orders
	settings Settings
	provider Provider
	secret   string
	notes    announcer
	clock    func() time.Time
}

func NewService(ledger Ledger, orders Orders, settings Settings, provider Provider, secret string) *Service {
	return &Service{
		ledger:   ledger,
		orders:   orders,
		settings: settings,
		provider: provider,
		secret:   secret,
		clock:    time.Now,
	}
}

// SetClock fixes the clock used for the 15-minute unpaid window.
func (s *Service) SetClock(now func() time.Time) {
	s.clock = now
}

func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

// Initiate asks the provider to collect the order total. The amount comes from the order.
func (s *Service) Initiate(ctx context.Context, customerID, orderID uuid.UUID, providerName, phone string) (Payment, bool, error) {
	if s.provider == nil || s.secret == "" {
		return Payment{}, false, ErrNotConfigured
	}
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName != ProviderMTN && providerName != ProviderAirtel {
		return Payment{}, false, ErrBadInput
	}
	normalized, err := normalizePhone(phone)
	if err != nil {
		return Payment{}, false, ErrBadInput
	}
	placed, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return Payment{}, false, mapOrderErr(err)
	}
	if placed.CustomerID != customerID || placed.TotalUGX <= 0 {
		return Payment{}, false, ErrNotFound
	}
	if placed.Status != order.StatusPendingPayment {
		return Payment{}, false, ErrConflict
	}
	existing, err := s.ledger.Find(ctx, orderID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Payment{}, false, err
	}
	if existing.Status == StatusPaid {
		return existing, false, nil
	}
	if existing.Status == StatusExpired || existing.Status == StatusRefundPending {
		return Payment{}, false, ErrConflict
	}
	expires := s.expiry(ctx, existing)
	if !expires.IsZero() && !s.now().Before(expires) {
		return Payment{}, false, ErrConflict
	}
	if existing.Reference != "" && existing.Status == StatusPending {
		if existing.Provider == providerName && existing.Phone == normalized {
			return existing, false, nil
		}
		return Payment{}, false, ErrConflict
	}
	reference, err := s.provider.Initiate(ctx, orderID.String())
	if err != nil {
		return Payment{}, false, err
	}
	first := existing.Reference == ""
	saved, err := s.ledger.SaveInitiation(ctx, Payment{
		ID:         existing.ID,
		OrderID:    orderID,
		CustomerID: customerID,
		AmountUGX:  placed.TotalUGX,
		Provider:   providerName,
		Phone:      normalized,
		Reference:  reference,
		ExpiresAt:  expires,
	})
	if err != nil {
		return Payment{}, false, err
	}
	return saved, first, nil
}

func (s *Service) expiry(ctx context.Context, existing Payment) time.Time {
	if !existing.ExpiresAt.IsZero() {
		return existing.ExpiresAt
	}
	minutes := 15
	if s.settings != nil {
		if value, err := s.settings.Int(ctx, "unpaid_expiry_minutes"); err == nil && value > 0 {
			minutes = value
		}
	}
	return s.now().Add(time.Duration(minutes) * time.Minute)
}

// MarkRefundDue records that a paid order must be refunded. Starting a refund is not finishing one.
func (s *Service) MarkRefundDue(ctx context.Context, orderID uuid.UUID) (Payment, error) {
	return s.ledger.MarkRefundDue(ctx, orderID)
}

// Get returns the payment stored for this customer. It does not ask the provider.
func (s *Service) Get(ctx context.Context, customerID, orderID uuid.UUID) (Payment, error) {
	placed, err := s.orders.Get(ctx, orderID)
	if err != nil || placed.CustomerID != customerID {
		return Payment{}, ErrNotFound
	}
	return s.ledger.Find(ctx, orderID)
}

// AcceptWebhook applies one signed provider notice. A repeated event returns the stored result.
func (s *Service) AcceptWebhook(ctx context.Context, body []byte, signature string) (Payment, error) {
	if s.provider == nil || s.secret == "" {
		return Payment{}, ErrNotConfigured
	}
	if !validSignature(s.secret, body, signature) {
		return Payment{}, ErrBadSignature
	}
	var payload struct {
		EventID   string `json:"eventId"`
		Reference string `json:"reference"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Payment{}, ErrBadInput
	}
	payload.Status = strings.ToUpper(strings.TrimSpace(payload.Status))
	payload.EventID = strings.TrimSpace(payload.EventID)
	payload.Reference = strings.TrimSpace(payload.Reference)
	if payload.EventID == "" || payload.Reference == "" {
		return Payment{}, ErrBadInput
	}
	switch payload.Status {
	case ProviderSuccess, ProviderFailed, ProviderPending:
	default:
		return Payment{}, ErrBadInput
	}
	payment, duplicate, err := s.ledger.ApplyProviderResult(ctx, payload.Reference, payload.EventID, payload.Status, s.now(), true)
	if err == nil && !duplicate && payment.Status == StatusPaid {
		s.notePaid(ctx, payment)
	}
	return payment, err
}

// Reconcile asks the provider about payments that are still pending.
// A provider that cannot be reached leaves those payments pending.
func (s *Service) Reconcile(ctx context.Context) (Summary, error) {
	if s.provider == nil || s.secret == "" {
		return Summary{}, ErrNotConfigured
	}
	pending, err := s.ledger.Pending(ctx)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	for _, payment := range pending {
		status, err := s.provider.Query(ctx, payment.Reference)
		if err != nil {
			summary.Unchanged++
			continue
		}
		status = strings.ToUpper(strings.TrimSpace(status))
		at := s.now()
		windowOpen := payment.ExpiresAt.IsZero() || at.Before(payment.ExpiresAt)
		if status == ProviderPending && windowOpen {
			summary.Checked++
			summary.Unchanged++
			continue
		}
		updated, _, err := s.ledger.ApplyProviderResult(ctx, payment.Reference, "", status, at, false)
		if err != nil {
			summary.Unchanged++
			continue
		}
		summary.Checked++
		switch updated.Status {
		case StatusPaid:
			summary.Paid++
			s.notePaid(ctx, updated)
		case StatusExpired:
			summary.Expired++
		case StatusRefundPending:
			summary.RefundPending++
		default:
			summary.Unchanged++
		}
	}
	return summary, nil
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

func mapOrderErr(err error) error {
	if errors.Is(err, order.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
