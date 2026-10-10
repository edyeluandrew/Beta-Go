package ops

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	ReasonVendorUnable    = "VENDOR_UNABLE"
	ReasonCustomerRequest = "CUSTOMER_REQUEST"
	ReasonDecisionExpired = "DECISION_EXPIRED"
	ReasonAutoComplete    = "AUTO_COMPLETE"
	ReasonDispute         = "CUSTOMER_DISPUTE"

	RefundRequested = "REQUESTED"
	RefundPending   = "PENDING"
	Refunded        = "REFUNDED"

	DisputeOpen     = "OPEN"
	DisputeResolved = "RESOLVED"

	AlertNewOrder = "VENDOR_NEW_ORDER"
	AlertReminder = "VENDOR_REMINDER"
	AlertEscalate = "DISPUTE_ESCALATE"
)

// Refund is the money-back record. Deciding, requesting, and confirming stay separate.
type Refund struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	AmountUGX int
	Status    string
	Reason    string
}

// Dispute blocks completion and commission capture.
type Dispute struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	Reason     string
	Status     string
	OpenedAt   time.Time
	EscalateAt time.Time
}

// Alert is a stored notice. SMS is still later.
type Alert struct {
	ID       uuid.UUID
	VendorID uuid.UUID
	OrderID  uuid.UUID
	Kind     string
}

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("bad input")
	ErrConflict  = errors.New("conflict")
	ErrClosed    = errors.New("closed")
)
