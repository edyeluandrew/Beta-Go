package payment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending       = "PENDING"
	StatusPaid          = "PAID"
	StatusFailed        = "FAILED"
	StatusExpired       = "EXPIRED"
	StatusRefundPending = "REFUND_PENDING"
	StatusRefunded      = "REFUNDED"
	StatusRefundFailed  = "REFUND_FAILED"

	ProviderMTN     = "mtn"
	ProviderAirtel  = "airtel"
	ProviderSandbox = "sandbox"

	ProviderSuccess = "SUCCESS"
	ProviderFailed  = "FAILED"
	ProviderPending = "PENDING"

	ReasonProviderSuccess = "PROVIDER_SUCCESS"
	ReasonLatePayment     = "LATE_PAYMENT"
	ReasonUnpaidExpired   = "UNPAID_EXPIRED"
)

// Payment is the money record for one order. Its status is not the order status.
type Payment struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	CustomerID  uuid.UUID
	Status      string
	AmountUGX   int
	Provider    string
	Phone       string
	Reference   string
	ExpiresAt   time.Time
	LateSuccess bool
}

var (
	ErrNotFound      = errors.New("not found")
	ErrBadInput      = errors.New("bad input")
	ErrConflict      = errors.New("conflict")
	ErrNotConfigured = errors.New("not configured")
	ErrBadSignature  = errors.New("bad signature")
)
