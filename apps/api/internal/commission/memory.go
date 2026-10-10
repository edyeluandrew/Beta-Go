package commission

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"beta-go/api/internal/vendor"
)

// MemoryBooks stores the pool for tests.
type MemoryBooks struct {
	mu          sync.Mutex
	shops       *vendor.MemoryDirectory
	accounts    map[uuid.UUID]Account
	entries     []Entry
	posted      map[string]struct{}
	events      map[string]struct{}
	topUps      map[uuid.UUID]TopUp
	topUpsByKey map[string]uuid.UUID
	topUpsByRef map[string]uuid.UUID
	adjustments map[uuid.UUID]Adjustment
}

func NewMemoryBooks(shops *vendor.MemoryDirectory) *MemoryBooks {
	return &MemoryBooks{
		shops:       shops,
		accounts:    map[uuid.UUID]Account{},
		posted:      map[string]struct{}{},
		events:      map[string]struct{}{},
		topUps:      map[uuid.UUID]TopUp{},
		topUpsByKey: map[string]uuid.UUID{},
		topUpsByRef: map[string]uuid.UUID{},
		adjustments: map[uuid.UUID]Adjustment{},
	}
}

func (m *MemoryBooks) Account(ctx context.Context, vendorID uuid.UUID) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accountLocked(ctx, vendorID)
}

func (m *MemoryBooks) accountLocked(ctx context.Context, vendorID uuid.UUID) (Account, error) {
	if account, ok := m.accounts[vendorID]; ok {
		return account, nil
	}
	available := 0
	if m.shops != nil {
		if balance, err := m.shops.AvailableBalance(ctx, vendorID); err == nil {
			available = balance
		} else if !errors.Is(err, vendor.ErrNotFound) {
			return Account{}, err
		} else {
			return Account{}, ErrNotFound
		}
	}
	account := Account{VendorID: vendorID, Available: available}
	m.accounts[vendorID] = account
	return account, nil
}

func (m *MemoryBooks) Post(ctx context.Context, posting Posting) (Account, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if posting.EventID != "" {
		key := posting.Provider + "|" + posting.EventID
		if _, seen := m.events[key]; seen {
			account, err := m.accountLocked(ctx, posting.VendorID)
			return account, true, err
		}
		m.events[key] = struct{}{}
	}
	opKey := posting.Operation + "|" + posting.OperationID.String()
	if _, seen := m.posted[opKey]; seen {
		account, err := m.accountLocked(ctx, posting.VendorID)
		return account, true, err
	}
	if posting.Available < 0 || posting.Reserved < 0 {
		return Account{}, false, ErrInsufficient
	}
	account := Account{VendorID: posting.VendorID, Available: posting.Available, Reserved: posting.Reserved, VendorStatus: posting.VendorStatus}
	m.accounts[posting.VendorID] = account
	for _, entry := range posting.Entries {
		entry.ID = uuid.New()
		m.entries = append(m.entries, entry)
	}
	m.posted[opKey] = struct{}{}
	if posting.TopUp != nil {
		saved := *posting.TopUp
		m.topUps[saved.ID] = saved
	}
	if posting.Adjustment != nil {
		saved := *posting.Adjustment
		m.adjustments[saved.ID] = saved
	}
	if m.shops != nil {
		m.shops.SetAvailable(posting.VendorID, posting.Available)
		_ = m.shops.SetStatus(ctx, posting.VendorID, posting.VendorStatus)
	}
	return account, false, nil
}

func (m *MemoryBooks) ListEntries(_ context.Context, vendorID uuid.UUID) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Entry
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].VendorID == vendorID {
			out = append(out, m.entries[i])
		}
		if len(out) == 50 {
			break
		}
	}
	return out, nil
}

func (m *MemoryBooks) SaveTopUp(_ context.Context, topUp TopUp) (TopUp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if topUp.ID == uuid.Nil {
		topUp.ID = uuid.New()
	}
	m.topUps[topUp.ID] = topUp
	m.topUpsByKey[topUp.VendorID.String()+"|"+topUp.IdempotencyKey] = topUp.ID
	if topUp.Reference != "" {
		m.topUpsByRef[topUp.Reference] = topUp.ID
	}
	return topUp, nil
}

func (m *MemoryBooks) FindTopUp(_ context.Context, id uuid.UUID) (TopUp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	topUp, ok := m.topUps[id]
	if !ok {
		return TopUp{}, ErrNotFound
	}
	return topUp, nil
}

func (m *MemoryBooks) FindTopUpByKey(_ context.Context, vendorID uuid.UUID, key string) (TopUp, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.topUpsByKey[vendorID.String()+"|"+key]
	if !ok {
		return TopUp{}, false, nil
	}
	return m.topUps[id], true, nil
}

func (m *MemoryBooks) FindTopUpByReference(_ context.Context, reference string) (TopUp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.topUpsByRef[reference]
	if !ok {
		return TopUp{}, ErrNotFound
	}
	return m.topUps[id], nil
}

func (m *MemoryBooks) SaveAdjustment(_ context.Context, adjustment Adjustment) (Adjustment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if adjustment.ID == uuid.Nil {
		adjustment.ID = uuid.New()
	}
	m.adjustments[adjustment.ID] = adjustment
	return adjustment, nil
}

func (m *MemoryBooks) FindAdjustment(_ context.Context, id uuid.UUID) (Adjustment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	adjustment, ok := m.adjustments[id]
	if !ok {
		return Adjustment{}, ErrNotFound
	}
	return adjustment, nil
}

func (m *MemoryBooks) Posted(_ context.Context, operation string, operationID uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.posted[operation+"|"+operationID.String()]
	return ok, nil
}
