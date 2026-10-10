package ops

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MemoryRecords stores refunds, disputes, and alerts for tests.
type MemoryRecords struct {
	mu       sync.Mutex
	refunds  map[uuid.UUID]Refund
	disputes map[uuid.UUID]Dispute
	byID     map[uuid.UUID]uuid.UUID
	alerts   []Alert
}

func NewMemoryRecords() *MemoryRecords {
	return &MemoryRecords{
		refunds:  map[uuid.UUID]Refund{},
		disputes: map[uuid.UUID]Dispute{},
		byID:     map[uuid.UUID]uuid.UUID{},
	}
}

func (m *MemoryRecords) SaveRefund(_ context.Context, refund Refund) (Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.refunds[refund.OrderID]; ok {
		return existing, nil
	}
	if refund.ID == uuid.Nil {
		refund.ID = uuid.New()
	}
	m.refunds[refund.OrderID] = refund
	return refund, nil
}

func (m *MemoryRecords) FindRefund(_ context.Context, orderID uuid.UUID) (Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	refund, ok := m.refunds[orderID]
	if !ok {
		return Refund{}, ErrNotFound
	}
	return refund, nil
}

func (m *MemoryRecords) SaveDispute(_ context.Context, dispute Dispute) (Dispute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.disputes[dispute.OrderID]; ok {
		return existing, nil
	}
	if dispute.ID == uuid.Nil {
		dispute.ID = uuid.New()
	}
	m.disputes[dispute.OrderID] = dispute
	m.byID[dispute.ID] = dispute.OrderID
	return dispute, nil
}

func (m *MemoryRecords) FindDispute(_ context.Context, orderID uuid.UUID) (Dispute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dispute, ok := m.disputes[orderID]
	if !ok {
		return Dispute{}, ErrNotFound
	}
	return dispute, nil
}

func (m *MemoryRecords) ListOpenDisputes(context.Context) ([]Dispute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Dispute
	for _, dispute := range m.disputes {
		if dispute.Status == DisputeOpen {
			out = append(out, dispute)
		}
	}
	return out, nil
}

func (m *MemoryRecords) ResolveDispute(_ context.Context, id, _ uuid.UUID, resolution string) (Dispute, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	orderID, ok := m.byID[id]
	if !ok {
		return Dispute{}, ErrNotFound
	}
	dispute := m.disputes[orderID]
	dispute.Status = DisputeResolved
	dispute.Reason = dispute.Reason
	_ = resolution
	m.disputes[orderID] = dispute
	return dispute, nil
}

func (m *MemoryRecords) SaveAlert(_ context.Context, alert Alert) (Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	m.alerts = append(m.alerts, alert)
	return alert, nil
}

func (m *MemoryRecords) HasAlert(_ context.Context, orderID uuid.UUID, kind string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, alert := range m.alerts {
		if alert.OrderID == orderID && alert.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}
