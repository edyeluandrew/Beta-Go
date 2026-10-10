package commission

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StreamAvailable = "available"
	StreamReserved  = "reserved"
	StreamRevenue   = "revenue"
	StreamExternal  = "external"

	DirDebit  = "debit"
	DirCredit = "credit"

	OpTopUp      = "TOP_UP"
	OpReserve    = "RESERVE"
	OpCapture    = "CAPTURE"
	OpRelease    = "RELEASE"
	OpAdjustment = "ADJUSTMENT"

	TopUpPending = "PENDING"
	TopUpPaid    = "PAID"
	TopUpFailed  = "FAILED"

	AdjPending = "PENDING"
	AdjApplied = "APPLIED"

	ProviderMTN    = "mtn"
	ProviderAirtel = "airtel"

	DefaultFloor     = 10000
	DefaultWarning   = 20000
	DefaultTopUpMin  = 10000
	DefaultTopUpMax  = 1000000
	DefaultDualLimit = 50000
)

// Account is the kitchen's prepaid pool. Available and reserved are never added to food revenue.
type Account struct {
	VendorID     uuid.UUID
	Available    int
	Reserved     int
	VendorStatus string
	LowBalance   bool
}

// Entry is one posted ledger line. Posted lines are never edited.
type Entry struct {
	ID          uuid.UUID
	VendorID    uuid.UUID
	Stream      string
	Direction   string
	AmountUGX   int
	Operation   string
	OperationID uuid.UUID
	OrderID     uuid.UUID
	CreatedAt   time.Time
}

// TopUp is a verified credit into available. It is not a food payment.
type TopUp struct {
	ID             uuid.UUID
	VendorID       uuid.UUID
	AmountUGX      int
	Provider       string
	Phone          string
	Status         string
	Reference      string
	IdempotencyKey string
}

// Adjustment is a staff change to available. Large amounts wait for a second person.
type Adjustment struct {
	ID          uuid.UUID
	VendorID    uuid.UUID
	AmountUGX   int
	Reason      string
	RequestedBy uuid.UUID
	ApprovedBy  uuid.UUID
	Status      string
}

var (
	ErrNotFound      = errors.New("not found")
	ErrBadInput      = errors.New("bad input")
	ErrConflict      = errors.New("conflict")
	ErrInsufficient  = errors.New("insufficient")
	ErrNotConfigured = errors.New("not configured")
	ErrBadSignature  = errors.New("bad signature")
	ErrForbidden     = errors.New("forbidden")
)
