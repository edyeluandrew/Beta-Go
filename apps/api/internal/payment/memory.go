package payment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/order"
)

// OrderBook reads an order and applies the payment outcome in tests.
type OrderBook interface {
	Get(ctx context.Context, id uuid.UUID) (order.Order, error)
	MoveFromPending(ctx context.Context, id uuid.UUID, next, reason string) error
}

// MemoryLedger stores payments for tests.
type MemoryLedger struct {
	mu     sync.Mutex
	orders OrderBook
	byID   map[uuid.UUID]Payment
	byRef  map[string]uuid.UUID
	byOrdr map[uuid.UUID]uuid.UUID
	events map[string]struct{}
}

func NewMemoryLedger(orders OrderBook) *MemoryLedger {
	return &MemoryLedger{
		orders: orders,
		byID:   map[uuid.UUID]Payment{},
		byRef:  map[string]uuid.UUID{},
		byOrdr: map[uuid.UUID]uuid.UUID{},
		events: map[string]struct{}{},
	}
}

func (m *MemoryLedger) Find(_ context.Context, orderID uuid.UUID) (Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byOrdr[orderID]
	if !ok {
		return Payment{}, ErrNotFound
	}
	return m.byID[id], nil
}

func (m *MemoryLedger) SaveInitiation(_ context.Context, payment Payment) (Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if payment.ID == uuid.Nil {
		payment.ID = uuid.New()
	}
	payment.Status = StatusPending
	m.byID[payment.ID] = payment
	m.byOrdr[payment.OrderID] = payment.ID
	if payment.Reference != "" {
		m.byRef[payment.Reference] = payment.ID
	}
	return payment, nil
}

func (m *MemoryLedger) Pending(_ context.Context) ([]Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Payment
	for _, payment := range m.byID {
		if payment.Status == StatusPending && payment.Reference != "" {
			out = append(out, payment)
		}
	}
	return out, nil
}

func (m *MemoryLedger) ApplyProviderResult(ctx context.Context, reference, eventID, providerStatus string, at time.Time, recordUnchanged bool) (Payment, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byRef[reference]
	if !ok {
		return Payment{}, false, ErrNotFound
	}
	payment := m.byID[id]
	if eventID != "" {
		if _, seen := m.events[eventKey(payment.Provider, eventID)]; seen {
			return payment, true, nil
		}
	}
	placed, err := m.orders.Get(ctx, payment.OrderID)
	if err != nil {
		return Payment{}, false, err
	}
	decision := decide(State{
		PaymentStatus: payment.Status,
		OrderStatus:   placed.Status,
		ExpiresAt:     payment.ExpiresAt,
	}, providerStatus, at)
	if !decision.Changed && !recordUnchanged {
		return payment, false, nil
	}
	storedID := eventID
	if storedID == "" {
		storedID = fmt.Sprintf("local:%s:%s:%s", reference, decision.PaymentStatus, decision.OrderStatus)
	}
	key := eventKey(payment.Provider, storedID)
	if _, seen := m.events[key]; seen {
		return payment, true, nil
	}
	if decision.Changed && decision.OrderStatus != "" && decision.OrderStatus != placed.Status {
		if err := m.orders.MoveFromPending(ctx, payment.OrderID, decision.OrderStatus, decision.Reason); err != nil {
			return Payment{}, false, err
		}
	}
	if decision.Changed {
		payment.Status = decision.PaymentStatus
		if decision.Late {
			payment.LateSuccess = true
		}
		m.byID[payment.ID] = payment
	}
	m.events[key] = struct{}{}
	return payment, false, nil
}

func (m *MemoryLedger) MarkRefundDue(_ context.Context, orderID uuid.UUID) (Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byOrdr[orderID]
	if !ok {
		return Payment{}, ErrNotFound
	}
	payment := m.byID[id]
	if payment.Status == StatusRefundPending || payment.Status == StatusRefunded {
		return payment, nil
	}
	if payment.Status != StatusPaid {
		return Payment{}, ErrConflict
	}
	payment.Status = StatusRefundPending
	m.byID[id] = payment
	return payment, nil
}

// SeedPaid stores a paid payment for tests that start after checkout.
func (m *MemoryLedger) SeedPaid(orderID uuid.UUID, amount int) Payment {
	m.mu.Lock()
	defer m.mu.Unlock()
	payment := Payment{ID: uuid.New(), OrderID: orderID, Status: StatusPaid, AmountUGX: amount}
	m.byID[payment.ID] = payment
	m.byOrdr[orderID] = payment.ID
	return payment
}

func eventKey(provider, eventID string) string {
	if provider == "" {
		provider = "unknown"
	}
	return provider + "|" + eventID
}
