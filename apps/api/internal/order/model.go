package order

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPendingPayment = "PENDING_PAYMENT"
	StatusAwaitingVendor = "AWAITING_VENDOR"
	StatusConfirmed      = "CONFIRMED"
	StatusPreparing      = "PREPARING"
	StatusReady          = "READY"
	StatusFulfilled      = "FULFILLED"
	StatusCompleted      = "COMPLETED"
	StatusRejected       = "REJECTED"
	StatusCancelled      = "CANCELLED"
	StatusExpired        = "EXPIRED"
	StatusDisputed       = "DISPUTED"
	FulfilmentCollection = "collection"
	FulfilmentDelivery   = "delivery"
	ReasonCustomer       = "CUSTOMER_REQUEST"
)

// Order is one vendor's basket. Prices are copied onto the order.
type Order struct {
	ID               uuid.UUID
	CustomerID       uuid.UUID
	VendorID         uuid.UUID
	Status           string
	Fulfilment       string
	DeliveryAddress  string
	CustomerPhone    string
	FoodSubtotalUGX  int
	DeliveryFeeUGX   int
	TotalUGX         int
	CommissionRate   int
	CommissionUGX    int
	IdempotencyKey   string
	RequestHash      string
	PickupCode       string
	CancelRequested  bool
	DecisionDeadline time.Time
	FulfilledAt      time.Time
	UpdatedAt        time.Time
	Items            []Item
}

// Item is the saved name and price, not a live menu lookup.
type Item struct {
	MenuItemID   uuid.UUID
	Name         string
	Quantity     int
	UnitPriceUGX int
	SubtotalUGX  int
}

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("bad input")
	ErrConflict  = errors.New("conflict")
	ErrClosed    = errors.New("closed")
)
