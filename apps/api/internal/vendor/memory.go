package vendor

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryDirectory is the in-memory store used by the onboarding tests.
type MemoryDirectory struct {
	mu           sync.Mutex
	vendors      map[uuid.UUID]Vendor
	order        []uuid.UUID
	invites      map[string]Invitation
	users        map[uuid.UUID]uuid.UUID
	destinations map[uuid.UUID]Destination
	hours        map[uuid.UUID][]Hours
	categories   map[uuid.UUID][]Category
	items        map[uuid.UUID][]Item
	balances     map[uuid.UUID]int
}

func NewMemoryDirectory() *MemoryDirectory {
	return &MemoryDirectory{
		vendors:      map[uuid.UUID]Vendor{},
		invites:      map[string]Invitation{},
		users:        map[uuid.UUID]uuid.UUID{},
		destinations: map[uuid.UUID]Destination{},
		hours:        map[uuid.UUID][]Hours{},
		categories:   map[uuid.UUID][]Category{},
		items:        map[uuid.UUID][]Item{},
		balances:     map[uuid.UUID]int{},
	}
}

func (m *MemoryDirectory) CreateVendor(_ context.Context, vendor Vendor) (Vendor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vendor.ID = uuid.New()
	m.vendors[vendor.ID] = vendor
	m.order = append(m.order, vendor.ID)
	m.balances[vendor.ID] = 0
	return vendor, nil
}

func (m *MemoryDirectory) ListVendors(context.Context) ([]Vendor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Vendor, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.vendors[id])
	}
	return out, nil
}

func (m *MemoryDirectory) GetVendor(_ context.Context, id uuid.UUID) (Vendor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vendor, ok := m.vendors[id]
	if !ok {
		return Vendor{}, ErrNotFound
	}
	return vendor, nil
}

func (m *MemoryDirectory) UpdateIdentity(_ context.Context, id uuid.UUID, businessName, contactName, phone, address string) (Vendor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vendor, ok := m.vendors[id]
	if !ok {
		return Vendor{}, ErrNotFound
	}
	vendor.BusinessName = businessName
	vendor.ContactName = contactName
	vendor.Phone = phone
	vendor.Address = address
	m.vendors[id] = vendor
	return vendor, nil
}

func (m *MemoryDirectory) UpdateShop(_ context.Context, id uuid.UUID, description, logoURL, photoURL string, deliveryFee int, isOpen, accepting bool) (Vendor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vendor, ok := m.vendors[id]
	if !ok {
		return Vendor{}, ErrNotFound
	}
	vendor.Description = description
	vendor.LogoURL = logoURL
	vendor.ShopPhotoURL = photoURL
	vendor.DeliveryFeeUGX = deliveryFee
	vendor.IsOpen = isOpen
	vendor.AcceptingOrders = accepting
	m.vendors[id] = vendor
	return vendor, nil
}

func (m *MemoryDirectory) SetStatus(_ context.Context, id uuid.UUID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vendor, ok := m.vendors[id]
	if !ok {
		return ErrNotFound
	}
	vendor.Status = status
	m.vendors[id] = vendor
	return nil
}

func (m *MemoryDirectory) SaveInvitation(_ context.Context, vendorID uuid.UUID, email, tokenHash string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for hash, invite := range m.invites {
		if invite.VendorID == vendorID && !invite.Used {
			invite.ExpiresAt = now
			m.invites[hash] = invite
		}
	}
	m.invites[tokenHash] = Invitation{ID: uuid.New(), VendorID: vendorID, Email: email, TokenHash: tokenHash, ExpiresAt: expiresAt}
	return nil
}

func (m *MemoryDirectory) FindInvitation(_ context.Context, tokenHash string) (Invitation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[tokenHash]
	if !ok {
		return Invitation{}, ErrNotFound
	}
	return invite, nil
}

func (m *MemoryDirectory) ConsumeInvitation(_ context.Context, invitationID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for hash, invite := range m.invites {
		if invite.ID != invitationID {
			continue
		}
		if invite.Used {
			return ErrInvitation
		}
		invite.Used = true
		m.invites[hash] = invite
		m.users[userID] = invite.VendorID
		return nil
	}
	return ErrNotFound
}

// BindUser attaches a cook to a shop in tests.
func (m *MemoryDirectory) BindUser(userID, vendorID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[userID] = vendorID
}

func (m *MemoryDirectory) VendorIDForUser(_ context.Context, userID uuid.UUID) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.users[userID]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return id, nil
}

func (m *MemoryDirectory) UpsertDestination(_ context.Context, vendorID uuid.UUID, provider, account string) (Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vendors[vendorID]; !ok {
		return Destination{}, ErrNotFound
	}
	destination := Destination{Provider: provider, Account: account, Status: DestinationUnverified}
	m.destinations[vendorID] = destination
	return destination, nil
}

func (m *MemoryDirectory) SetDestinationStatus(_ context.Context, vendorID uuid.UUID, status string) (Destination, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	destination, ok := m.destinations[vendorID]
	if !ok {
		return Destination{}, ErrNotFound
	}
	destination.Status = status
	m.destinations[vendorID] = destination
	return destination, nil
}

func (m *MemoryDirectory) GetDestination(_ context.Context, vendorID uuid.UUID) (Destination, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	destination, ok := m.destinations[vendorID]
	return destination, ok, nil
}

func (m *MemoryDirectory) ReplaceHours(_ context.Context, vendorID uuid.UUID, hours []Hours) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := append([]Hours(nil), hours...)
	m.hours[vendorID] = copied
	return nil
}

func (m *MemoryDirectory) ListHours(_ context.Context, vendorID uuid.UUID) ([]Hours, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Hours(nil), m.hours[vendorID]...), nil
}

func (m *MemoryDirectory) AddCategory(_ context.Context, vendorID uuid.UUID, name string) (Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	category := Category{ID: uuid.New(), Name: name}
	m.categories[vendorID] = append(m.categories[vendorID], category)
	return category, nil
}

func (m *MemoryDirectory) ListCategories(_ context.Context, vendorID uuid.UUID) ([]Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Category(nil), m.categories[vendorID]...), nil
}

func (m *MemoryDirectory) AddItem(_ context.Context, item Item) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item.ID = uuid.New()
	m.items[item.VendorID] = append(m.items[item.VendorID], item)
	return item, nil
}

func (m *MemoryDirectory) UpdateItem(_ context.Context, vendorID, itemID uuid.UUID, name, description string, price int, available bool, photoURL string) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.items[vendorID]
	for i, item := range items {
		if item.ID != itemID {
			continue
		}
		item.Name = name
		item.Description = description
		item.PriceUGX = price
		item.Available = available
		if photoURL != "" {
			item.PhotoURL = photoURL
		}
		items[i] = item
		m.items[vendorID] = items
		return item, nil
	}
	return Item{}, ErrNotFound
}

func (m *MemoryDirectory) ListItems(_ context.Context, vendorID uuid.UUID) ([]Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Item(nil), m.items[vendorID]...), nil
}

// SetAvailable lets tests fund a commission pool. Top-ups arrive in a later phase.
func (m *MemoryDirectory) SetAvailable(vendorID uuid.UUID, amount int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[vendorID] = amount
}

func (m *MemoryDirectory) AvailableBalance(_ context.Context, vendorID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	balance, ok := m.balances[vendorID]
	if !ok {
		return 0, ErrNotFound
	}
	return balance, nil
}

func (m *MemoryDirectory) Audit(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}
