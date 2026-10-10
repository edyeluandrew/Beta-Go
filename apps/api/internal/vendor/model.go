package vendor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending   = "PENDING_APPROVAL"
	StatusRejected  = "REJECTED"
	StatusActive    = "ACTIVE"
	StatusInactive  = "INACTIVE"
	StatusPaused    = "PAUSED"
	StatusSuspended = "SUSPENDED"

	DestinationUnverified = "UNVERIFIED"
	DestinationVerified   = "VERIFIED"

	activityFloorUGX = 10000
	goLiveItemCount  = 3
)

// Vendor is one kitchen. Admins create it. The vendor builds the shop.
type Vendor struct {
	ID              uuid.UUID
	BusinessName    string
	ContactName     string
	ContactEmail    string
	Phone           string
	Address         string
	Description     string
	LogoURL         string
	ShopPhotoURL    string
	Status          string
	IsOpen          bool
	AcceptingOrders bool
	DeliveryFeeUGX  int
}

// Invitation is a single-use link bound to the vendor's Google email.
type Invitation struct {
	ID        uuid.UUID
	VendorID  uuid.UUID
	Email     string
	TokenHash string
	ExpiresAt time.Time
	Used      bool
}

// Destination is the vendor's MoMo or Airtel number. Vendors cannot edit it.
type Destination struct {
	Provider string
	Account  string
	Status   string
}

// Hours is one weekday. Weekday 0 is Sunday.
type Hours struct {
	Weekday  int
	OpensAt  string
	ClosesAt string
	Closed   bool
}

// Category groups menu items.
type Category struct {
	ID   uuid.UUID
	Name string
}

// Item is one dish. Price is whole shillings.
type Item struct {
	ID          uuid.UUID
	VendorID    uuid.UUID
	CategoryID  uuid.UUID
	Name        string
	Description string
	PriceUGX    int
	Available   bool
	PhotoURL    string
}

// Shop is what the signed-in vendor is allowed to see and edit.
type Shop struct {
	Vendor     Vendor
	Hours      []Hours
	Categories []Category
	Items      []Item
}

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrInvitation  = errors.New("invitation invalid")
	ErrInviteEmail = errors.New("invitation email mismatch")
	ErrInviteGone  = errors.New("invitation used or expired")
	ErrBadInput    = errors.New("bad input")
)

// NotReadyError lists the go-live checks that are still missing.
type NotReadyError struct {
	Missing []string
}

func (e NotReadyError) Error() string {
	return fmt.Sprintf("shop is not ready: %s", strings.Join(e.Missing, ", "))
}
