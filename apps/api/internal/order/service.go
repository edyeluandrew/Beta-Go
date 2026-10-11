package order

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/vendor"
)

// Kitchens answers the marketplace checks from the vendor rules.
type Kitchens interface {
	CanReceiveOrder(ctx context.Context, vendorID uuid.UUID, commissionUGX int, at time.Time) error
	Get(ctx context.Context, id uuid.UUID) (vendor.Vendor, vendor.Destination, bool, error)
}

// Menu is the priced catalog used to snapshot an order.
type Menu interface {
	ListItems(ctx context.Context, vendorID uuid.UUID) ([]vendor.Item, error)
}

// Directory stores orders.
type Directory interface {
	FindByKey(ctx context.Context, customerID uuid.UUID, key string) (Order, bool, error)
	Create(ctx context.Context, order Order) (Order, error)
	ListForCustomer(ctx context.Context, customerID uuid.UUID) ([]Order, error)
	ListForVendor(ctx context.Context, vendorID uuid.UUID) ([]Order, error)
	ListByStatus(ctx context.Context, status string) ([]Order, error)
	Get(ctx context.Context, id uuid.UUID) (Order, error)
	Cancel(ctx context.Context, id, actor uuid.UUID, reason string) (Order, error)
	Transition(ctx context.Context, id uuid.UUID, from, to string, actor uuid.UUID, reason string) (Order, error)
	SetCancelRequested(ctx context.Context, id uuid.UUID, requested bool) (Order, error)
}

// Settings reads whole-number policy values.
type Settings interface {
	Int(ctx context.Context, key string) (int, error)
}

// Service places and reads single-vendor orders.
type Service struct {
	kitchens Kitchens
	menu     Menu
	dir      Directory
	settings Settings
	notes    announcer
	now      func() time.Time
}

func NewService(kitchens Kitchens, menu Menu, dir Directory, settings Settings) *Service {
	return &Service{
		kitchens: kitchens,
		menu:     menu,
		dir:      dir,
		settings: settings,
		now:      time.Now,
	}
}

// SetClock fixes the clock used for opening hours. Tests use this.
func (s *Service) SetClock(now func() time.Time) {
	s.now = now
}

type Line struct {
	ItemID   uuid.UUID
	Quantity int
}

type CreateInput struct {
	CustomerID     uuid.UUID
	VendorID       uuid.UUID
	IdempotencyKey string
	Fulfilment     string
	Address        string
	Phone          string
	Lines          []Line
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Order, error) {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Fulfilment = strings.TrimSpace(in.Fulfilment)
	in.Address = strings.TrimSpace(in.Address)
	in.Phone = strings.TrimSpace(in.Phone)
	if in.IdempotencyKey == "" || len(in.Lines) == 0 {
		return Order{}, ErrBadInput
	}
	if in.Fulfilment != FulfilmentCollection && in.Fulfilment != FulfilmentDelivery {
		return Order{}, ErrBadInput
	}
	if in.Fulfilment == FulfilmentDelivery && (in.Address == "" || in.Phone == "") {
		return Order{}, ErrBadInput
	}
	if in.Fulfilment == FulfilmentCollection {
		in.Address = ""
		in.Phone = ""
	}

	hash, err := requestHash(in)
	if err != nil {
		return Order{}, err
	}
	existing, found, err := s.dir.FindByKey(ctx, in.CustomerID, in.IdempotencyKey)
	if err != nil {
		return Order{}, err
	}
	if found {
		if existing.RequestHash != hash {
			return Order{}, ErrConflict
		}
		return existing, nil
	}

	kitchen, _, _, err := s.kitchens.Get(ctx, in.VendorID)
	if err != nil {
		if errors.Is(err, vendor.ErrNotFound) {
			return Order{}, ErrClosed
		}
		return Order{}, err
	}
	items, err := s.menu.ListItems(ctx, in.VendorID)
	if err != nil {
		return Order{}, err
	}
	byID := map[uuid.UUID]vendor.Item{}
	for _, item := range items {
		byID[item.ID] = item
	}
	var snapshots []Item
	food := 0
	seen := map[uuid.UUID]bool{}
	for _, line := range in.Lines {
		if line.Quantity < 1 || line.Quantity > 99 || seen[line.ItemID] {
			return Order{}, ErrBadInput
		}
		seen[line.ItemID] = true
		item, ok := byID[line.ItemID]
		if !ok || item.VendorID != in.VendorID || !item.Available {
			return Order{}, ErrBadInput
		}
		subtotal := item.PriceUGX * line.Quantity
		food += subtotal
		snapshots = append(snapshots, Item{
			MenuItemID: line.ItemID, Name: item.Name, Quantity: line.Quantity,
			UnitPriceUGX: item.PriceUGX, SubtotalUGX: subtotal,
		})
	}
	minimum, err := s.settings.Int(ctx, "minimum_order_ugx")
	if err != nil {
		return Order{}, err
	}
	if food < minimum {
		return Order{}, ErrBadInput
	}
	rate, err := s.settings.Int(ctx, "commission_rate_percent")
	if err != nil {
		return Order{}, err
	}
	delivery := 0
	if in.Fulfilment == FulfilmentDelivery {
		delivery = kitchen.DeliveryFeeUGX
	}
	commission := roundHalfUp(food, rate)
	if err := s.kitchens.CanReceiveOrder(ctx, in.VendorID, commission, s.now()); err != nil {
		var notReady vendor.NotReadyError
		if errors.Is(err, vendor.ErrNotFound) || errors.As(err, &notReady) {
			return Order{}, ErrClosed
		}
		return Order{}, err
	}
	pickup := ""
	if in.Fulfilment == FulfilmentCollection {
		pickup = newPickupCode()
	}
	created, err := s.dir.Create(ctx, Order{
		CustomerID: in.CustomerID, VendorID: in.VendorID, Status: StatusPendingPayment,
		Fulfilment: in.Fulfilment, DeliveryAddress: in.Address, CustomerPhone: in.Phone,
		FoodSubtotalUGX: food, DeliveryFeeUGX: delivery, TotalUGX: food + delivery,
		CommissionRate: rate, CommissionUGX: commission,
		IdempotencyKey: in.IdempotencyKey, RequestHash: hash, PickupCode: pickup, Items: snapshots,
	})
	if err != nil {
		return Order{}, err
	}
	s.announce(ctx, "Order saved", "Pay the kitchen and they will see it.", "/c/orders/"+created.ID.String(), []uuid.UUID{created.CustomerID}, uuid.Nil, nil)
	return created, nil
}

func (s *Service) List(ctx context.Context, customerID uuid.UUID) ([]Order, error) {
	return s.dir.ListForCustomer(ctx, customerID)
}

func (s *Service) Get(ctx context.Context, customerID, id uuid.UUID) (Order, error) {
	order, err := s.dir.Get(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if order.CustomerID != customerID {
		return Order{}, ErrNotFound
	}
	return order, nil
}

func (s *Service) Cancel(ctx context.Context, customerID, id uuid.UUID) (Order, error) {
	order, err := s.Get(ctx, customerID, id)
	if err != nil {
		return Order{}, err
	}
	if order.Status != StatusPendingPayment {
		return Order{}, ErrForbidden
	}
	return s.dir.Cancel(ctx, id, customerID, ReasonCustomer)
}

func roundHalfUp(subtotal, percent int) int {
	if percent <= 0 || subtotal <= 0 {
		return 0
	}
	return (subtotal*percent + 50) / 100
}

func requestHash(in CreateInput) (string, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func newPickupCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "PICKUP"
	}
	for i, b := range raw {
		raw[i] = letters[int(b)%len(letters)]
	}
	return string(raw)
}
