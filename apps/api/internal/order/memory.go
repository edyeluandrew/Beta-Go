package order

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryDirectory stores orders for tests.
type MemoryDirectory struct {
	mu     sync.Mutex
	orders map[uuid.UUID]Order
	byKey  map[string]uuid.UUID
}

func NewMemoryDirectory() *MemoryDirectory {
	return &MemoryDirectory{orders: map[uuid.UUID]Order{}, byKey: map[string]uuid.UUID{}}
}

func (m *MemoryDirectory) FindByKey(_ context.Context, customerID uuid.UUID, key string) (Order, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byKey[customerID.String()+"|"+key]
	if !ok {
		return Order{}, false, nil
	}
	return m.orders[id], true, nil
}

func (m *MemoryDirectory) Create(_ context.Context, order Order) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order.ID = uuid.New()
	copied := append([]Item(nil), order.Items...)
	order.Items = copied
	m.orders[order.ID] = order
	m.byKey[order.CustomerID.String()+"|"+order.IdempotencyKey] = order.ID
	return order, nil
}

func (m *MemoryDirectory) ListForCustomer(_ context.Context, customerID uuid.UUID) ([]Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Order
	for _, order := range m.orders {
		if order.CustomerID == customerID {
			out = append(out, order)
		}
	}
	return out, nil
}

func (m *MemoryDirectory) ListForVendor(_ context.Context, vendorID uuid.UUID) ([]Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Order
	for _, order := range m.orders {
		if order.VendorID == vendorID {
			out = append(out, order)
		}
	}
	return out, nil
}

func (m *MemoryDirectory) ListByStatus(_ context.Context, status string) ([]Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Order
	for _, order := range m.orders {
		if order.Status == status {
			out = append(out, order)
		}
	}
	return out, nil
}

func (m *MemoryDirectory) Get(_ context.Context, id uuid.UUID) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	return order, nil
}

// MoveFromPending records a payment outcome. Only an unpaid order can move.
func (m *MemoryDirectory) MoveFromPending(_ context.Context, id uuid.UUID, next, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return ErrNotFound
	}
	if order.Status == next {
		return nil
	}
	if order.Status != StatusPendingPayment || (next != StatusAwaitingVendor && next != StatusExpired) {
		return ErrConflict
	}
	order.Status = next
	if next == StatusAwaitingVendor && order.DecisionDeadline.IsZero() {
		order.DecisionDeadline = order.UpdatedAt
	}
	m.orders[id] = order
	return nil
}

func (m *MemoryDirectory) Transition(_ context.Context, id uuid.UUID, from, to string, _ uuid.UUID, _ string) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	if order.Status == to {
		return order, nil
	}
	if order.Status != from {
		return Order{}, ErrConflict
	}
	order.Status = to
	if to == StatusFulfilled {
		order.FulfilledAt = time.Now().UTC()
	}
	m.orders[id] = order
	return order, nil
}

func (m *MemoryDirectory) SetCancelRequested(_ context.Context, id uuid.UUID, requested bool) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	order.CancelRequested = requested
	m.orders[id] = order
	return order, nil
}

func (m *MemoryDirectory) Cancel(_ context.Context, id, _ uuid.UUID, _ string) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order, ok := m.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	order.Status = StatusCancelled
	m.orders[id] = order
	return order, nil
}

// FixedSettings returns policy numbers without a database.
type FixedSettings map[string]int

func (s FixedSettings) Int(_ context.Context, key string) (int, error) {
	value, ok := s[key]
	if !ok {
		return 0, ErrNotFound
	}
	return value, nil
}
