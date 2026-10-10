package vendor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/identity"
)

// Directory is the persistence used by onboarding.
type Directory interface {
	CreateVendor(ctx context.Context, vendor Vendor) (Vendor, error)
	ListVendors(ctx context.Context) ([]Vendor, error)
	GetVendor(ctx context.Context, id uuid.UUID) (Vendor, error)
	UpdateIdentity(ctx context.Context, id uuid.UUID, businessName, contactName, phone, address string) (Vendor, error)
	UpdateShop(ctx context.Context, id uuid.UUID, description, logoURL, photoURL string, deliveryFee int, isOpen, accepting bool) (Vendor, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	SaveInvitation(ctx context.Context, vendorID uuid.UUID, email, tokenHash string, expiresAt time.Time) error
	FindInvitation(ctx context.Context, tokenHash string) (Invitation, error)
	ConsumeInvitation(ctx context.Context, invitationID, userID uuid.UUID) error
	VendorIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
	UpsertDestination(ctx context.Context, vendorID uuid.UUID, provider, account string) (Destination, error)
	SetDestinationStatus(ctx context.Context, vendorID uuid.UUID, status string) (Destination, error)
	GetDestination(ctx context.Context, vendorID uuid.UUID) (Destination, bool, error)
	ReplaceHours(ctx context.Context, vendorID uuid.UUID, hours []Hours) error
	ListHours(ctx context.Context, vendorID uuid.UUID) ([]Hours, error)
	AddCategory(ctx context.Context, vendorID uuid.UUID, name string) (Category, error)
	ListCategories(ctx context.Context, vendorID uuid.UUID) ([]Category, error)
	AddItem(ctx context.Context, item Item) (Item, error)
	UpdateItem(ctx context.Context, vendorID, itemID uuid.UUID, name, description string, price int, available bool, photoURL string) (Item, error)
	ListItems(ctx context.Context, vendorID uuid.UUID) ([]Item, error)
	AvailableBalance(ctx context.Context, vendorID uuid.UUID) (int, error)
	Audit(ctx context.Context, actorID, entityID uuid.UUID, action, reason string) error
}

// Accounts grants the vendor role. Public sign-in does not.
type Accounts interface {
	GrantRole(ctx context.Context, userID uuid.UUID, role string) error
	Find(ctx context.Context, id uuid.UUID) (identity.User, error)
}

// Service applies the onboarding rules from the specification.
type Service struct {
	dir   Directory
	users Accounts
	now   func() time.Time
}

func NewService(dir Directory, users Accounts) *Service {
	return &Service{dir: dir, users: users, now: func() time.Time { return time.Now().UTC() }}
}

type CreateInput struct {
	BusinessName string
	ContactName  string
	ContactEmail string
	Phone        string
	Address      string
}

func (s *Service) Create(ctx context.Context, actor uuid.UUID, in CreateInput) (Vendor, error) {
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.ContactName = strings.TrimSpace(in.ContactName)
	in.ContactEmail = strings.ToLower(strings.TrimSpace(in.ContactEmail))
	in.Phone = strings.TrimSpace(in.Phone)
	in.Address = strings.TrimSpace(in.Address)
	if in.BusinessName == "" || in.ContactName == "" || in.ContactEmail == "" || in.Phone == "" || in.Address == "" || !strings.Contains(in.ContactEmail, "@") {
		return Vendor{}, ErrBadInput
	}
	created, err := s.dir.CreateVendor(ctx, Vendor{
		BusinessName: in.BusinessName,
		ContactName:  in.ContactName,
		ContactEmail: in.ContactEmail,
		Phone:        in.Phone,
		Address:      in.Address,
		Status:       StatusPending,
	})
	if err != nil {
		return Vendor{}, err
	}
	_ = s.dir.Audit(ctx, actor, created.ID, "vendor.create", "")
	return created, nil
}

func (s *Service) List(ctx context.Context) ([]Vendor, error) {
	return s.dir.ListVendors(ctx)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Vendor, Destination, bool, error) {
	vendor, err := s.dir.GetVendor(ctx, id)
	if err != nil {
		return Vendor{}, Destination{}, false, err
	}
	destination, ok, err := s.dir.GetDestination(ctx, id)
	return vendor, destination, ok, err
}

func (s *Service) UpdateIdentity(ctx context.Context, actor, id uuid.UUID, businessName, contactName, phone, address string) (Vendor, error) {
	updated, err := s.dir.UpdateIdentity(ctx, id, strings.TrimSpace(businessName), strings.TrimSpace(contactName), strings.TrimSpace(phone), strings.TrimSpace(address))
	if err != nil {
		return Vendor{}, err
	}
	_ = s.dir.Audit(ctx, actor, id, "vendor.update", "")
	return updated, nil
}

// Invite returns the raw token once. Only the hash is stored.
func (s *Service) Invite(ctx context.Context, actor, vendorID uuid.UUID) (string, time.Time, error) {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return "", time.Time{}, err
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().Add(7 * 24 * time.Hour)
	if err := s.dir.SaveInvitation(ctx, vendor.ID, vendor.ContactEmail, hash, expires); err != nil {
		return "", time.Time{}, err
	}
	_ = s.dir.Audit(ctx, actor, vendor.ID, "vendor.invite", "")
	return raw, expires, nil
}

func (s *Service) SetDestination(ctx context.Context, actor, vendorID uuid.UUID, provider, account string) (Destination, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	account = strings.TrimSpace(account)
	if provider != "mtn" && provider != "airtel" || account == "" {
		return Destination{}, ErrBadInput
	}
	destination, err := s.dir.UpsertDestination(ctx, vendorID, provider, account)
	if err != nil {
		return Destination{}, err
	}
	_ = s.dir.Audit(ctx, actor, vendorID, "vendor.destination", "")
	return destination, nil
}

func (s *Service) VerifyDestination(ctx context.Context, actor, vendorID uuid.UUID) (Destination, error) {
	destination, err := s.dir.SetDestinationStatus(ctx, vendorID, DestinationVerified)
	if err != nil {
		return Destination{}, err
	}
	_ = s.dir.Audit(ctx, actor, vendorID, "vendor.destination.verify", "")
	return destination, nil
}

func (s *Service) Approve(ctx context.Context, actor, vendorID uuid.UUID) (Vendor, error) {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return Vendor{}, err
	}
	if vendor.Status != StatusPending {
		return Vendor{}, ErrForbidden
	}
	if missing, err := s.missingGoLive(ctx, vendor); err != nil {
		return Vendor{}, err
	} else if len(missing) > 0 {
		return Vendor{}, NotReadyError{Missing: missing}
	}
	balance, err := s.dir.AvailableBalance(ctx, vendorID)
	if err != nil {
		return Vendor{}, err
	}
	next := StatusActive
	if balance < activityFloorUGX {
		next = StatusInactive
	}
	if err := s.dir.SetStatus(ctx, vendorID, next); err != nil {
		return Vendor{}, err
	}
	_ = s.dir.Audit(ctx, actor, vendorID, "vendor.approve", "")
	return s.dir.GetVendor(ctx, vendorID)
}

func (s *Service) Reject(ctx context.Context, actor, vendorID uuid.UUID, reason string) (Vendor, error) {
	return s.transition(ctx, actor, vendorID, "vendor.reject", reason, func(status string) (string, error) {
		if status != StatusPending {
			return "", ErrForbidden
		}
		return StatusRejected, nil
	})
}

func (s *Service) Pause(ctx context.Context, actor, vendorID uuid.UUID, reason string) (Vendor, error) {
	return s.transition(ctx, actor, vendorID, "vendor.pause", reason, func(status string) (string, error) {
		if status != StatusActive && status != StatusInactive {
			return "", ErrForbidden
		}
		return StatusPaused, nil
	})
}

func (s *Service) Suspend(ctx context.Context, actor, vendorID uuid.UUID, reason string) (Vendor, error) {
	return s.transition(ctx, actor, vendorID, "vendor.suspend", reason, func(status string) (string, error) {
		if status == StatusRejected {
			return "", ErrForbidden
		}
		return StatusSuspended, nil
	})
}

func (s *Service) Reactivate(ctx context.Context, actor, vendorID uuid.UUID) (Vendor, error) {
	return s.transition(ctx, actor, vendorID, "vendor.reactivate", "", func(status string) (string, error) {
		if status != StatusPaused && status != StatusSuspended {
			return "", ErrForbidden
		}
		balance, err := s.dir.AvailableBalance(ctx, vendorID)
		if err != nil {
			return "", err
		}
		if balance < activityFloorUGX {
			return StatusInactive, nil
		}
		return StatusActive, nil
	})
}

func (s *Service) Accept(ctx context.Context, userID uuid.UUID, rawToken string) error {
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return ErrInvitation
	}
	invite, err := s.dir.FindInvitation(ctx, hashToken(strings.TrimSpace(rawToken)))
	if err != nil {
		return ErrInvitation
	}
	if invite.Used || !invite.ExpiresAt.After(s.now()) {
		return ErrInviteGone
	}
	if !strings.EqualFold(invite.Email, user.Email) {
		return ErrInviteEmail
	}
	if err := s.users.GrantRole(ctx, userID, identity.RoleVendor); err != nil {
		return err
	}
	if err := s.dir.ConsumeInvitation(ctx, invite.ID, userID); err != nil {
		return err
	}
	_ = s.dir.Audit(ctx, userID, invite.VendorID, "vendor.invite.accept", "")
	return nil
}

// ListVisible returns shops customers are allowed to see. Only ACTIVE kitchens.
func (s *Service) ListVisible(ctx context.Context) ([]Vendor, error) {
	all, err := s.dir.ListVendors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Vendor, 0, len(all))
	for _, vendor := range all {
		if vendor.Status == StatusActive {
			out = append(out, vendor)
		}
	}
	return out, nil
}

// PublicShop returns the menu for an ACTIVE kitchen. Other statuses stay hidden.
func (s *Service) PublicShop(ctx context.Context, id uuid.UUID) (Shop, error) {
	shop, err := s.loadShop(ctx, id)
	if err != nil {
		return Shop{}, err
	}
	if shop.Vendor.Status != StatusActive {
		return Shop{}, ErrNotFound
	}
	return shop, nil
}

// CanReceiveOrder checks the specification rules for starting a new order.
func (s *Service) CanReceiveOrder(ctx context.Context, vendorID uuid.UUID, commissionUGX int, at time.Time) error {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return err
	}
	if vendor.Status != StatusActive || !vendor.IsOpen || !vendor.AcceptingOrders {
		return ErrNotFound
	}
	hours, err := s.dir.ListHours(ctx, vendorID)
	if err != nil {
		return err
	}
	if !openAt(hours, at) {
		return ErrNotFound
	}
	missing, err := s.missingGoLive(ctx, vendor)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return NotReadyError{Missing: missing}
	}
	balance, err := s.dir.AvailableBalance(ctx, vendorID)
	if err != nil {
		return err
	}
	if balance < activityFloorUGX || balance < commissionUGX {
		return ErrNotFound
	}
	return nil
}

func openAt(hours []Hours, at time.Time) bool {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		loc = time.FixedZone("EAT", 3*60*60)
	}
	local := at.In(loc)
	clock := local.Format("15:04")
	weekday := int(local.Weekday())
	for _, day := range hours {
		if day.Weekday != weekday {
			continue
		}
		if day.Closed || !validClock(day.OpensAt) || !validClock(day.ClosesAt) || day.ClosesAt <= day.OpensAt {
			return false
		}
		return clock >= day.OpensAt && clock < day.ClosesAt
	}
	return false
}

func (s *Service) Shop(ctx context.Context, userID uuid.UUID) (Shop, error) {
	vendorID, err := s.dir.VendorIDForUser(ctx, userID)
	if err != nil {
		return Shop{}, err
	}
	return s.loadShop(ctx, vendorID)
}

func (s *Service) UpdateShop(ctx context.Context, userID uuid.UUID, description, logoURL, photoURL string, deliveryFee int, isOpen, accepting bool) (Vendor, error) {
	vendorID, err := s.own(ctx, userID)
	if err != nil {
		return Vendor{}, err
	}
	if deliveryFee < 0 || len(logoURL) > 2048 || len(photoURL) > 2048 {
		return Vendor{}, ErrBadInput
	}
	return s.dir.UpdateShop(ctx, vendorID, strings.TrimSpace(description), strings.TrimSpace(logoURL), strings.TrimSpace(photoURL), deliveryFee, isOpen, accepting)
}

func (s *Service) ReplaceHours(ctx context.Context, userID uuid.UUID, hours []Hours) error {
	vendorID, err := s.own(ctx, userID)
	if err != nil {
		return err
	}
	if err := validateHours(hours); err != nil {
		return err
	}
	return s.dir.ReplaceHours(ctx, vendorID, hours)
}

func (s *Service) AddCategory(ctx context.Context, userID uuid.UUID, name string) (Category, error) {
	vendorID, err := s.own(ctx, userID)
	if err != nil {
		return Category{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrBadInput
	}
	return s.dir.AddCategory(ctx, vendorID, name)
}

func (s *Service) AddItem(ctx context.Context, userID uuid.UUID, name, description string, price int, categoryID uuid.UUID, photoURL string) (Item, error) {
	vendorID, err := s.own(ctx, userID)
	if err != nil {
		return Item{}, err
	}
	name = strings.TrimSpace(name)
	photoURL = strings.TrimSpace(photoURL)
	if name == "" || price <= 0 || len(photoURL) > 2048 {
		return Item{}, ErrBadInput
	}
	return s.dir.AddItem(ctx, Item{
		VendorID:    vendorID,
		CategoryID:  categoryID,
		Name:        name,
		Description: strings.TrimSpace(description),
		PriceUGX:    price,
		Available:   true,
		PhotoURL:    photoURL,
	})
}

func (s *Service) UpdateItem(ctx context.Context, userID, itemID uuid.UUID, name, description string, price int, available bool, photoURL string) (Item, error) {
	vendorID, err := s.own(ctx, userID)
	if err != nil {
		return Item{}, err
	}
	name = strings.TrimSpace(name)
	photoURL = strings.TrimSpace(photoURL)
	if name == "" || price <= 0 || len(photoURL) > 2048 {
		return Item{}, ErrBadInput
	}
	return s.dir.UpdateItem(ctx, vendorID, itemID, name, strings.TrimSpace(description), price, available, photoURL)
}

func (s *Service) own(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return s.dir.VendorIDForUser(ctx, userID)
}

func (s *Service) loadShop(ctx context.Context, vendorID uuid.UUID) (Shop, error) {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return Shop{}, err
	}
	hours, err := s.dir.ListHours(ctx, vendorID)
	if err != nil {
		return Shop{}, err
	}
	categories, err := s.dir.ListCategories(ctx, vendorID)
	if err != nil {
		return Shop{}, err
	}
	items, err := s.dir.ListItems(ctx, vendorID)
	if err != nil {
		return Shop{}, err
	}
	return Shop{Vendor: vendor, Hours: hours, Categories: categories, Items: items}, nil
}

func (s *Service) GoLiveGaps(ctx context.Context, vendorID uuid.UUID) ([]string, error) {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return nil, err
	}
	return s.missingGoLive(ctx, vendor)
}

func (s *Service) missingGoLive(ctx context.Context, vendor Vendor) ([]string, error) {
	var missing []string
	items, err := s.dir.ListItems(ctx, vendor.ID)
	if err != nil {
		return nil, err
	}
	available := 0
	for _, item := range items {
		if item.Available && item.PriceUGX > 0 {
			available++
		}
	}
	if available < goLiveItemCount {
		missing = append(missing, "at least 3 available priced items")
	}
	if strings.TrimSpace(vendor.ShopPhotoURL) == "" {
		missing = append(missing, "one shop photo")
	}
	hours, err := s.dir.ListHours(ctx, vendor.ID)
	if err != nil {
		return nil, err
	}
	if len(hours) != 7 {
		missing = append(missing, "opening hours")
	}
	destination, ok, err := s.dir.GetDestination(ctx, vendor.ID)
	if err != nil {
		return nil, err
	}
	if !ok || destination.Status != DestinationVerified {
		missing = append(missing, "a verified MoMo or Airtel destination")
	}
	return missing, nil
}

func (s *Service) transition(ctx context.Context, actor, vendorID uuid.UUID, action, reason string, next func(string) (string, error)) (Vendor, error) {
	vendor, err := s.dir.GetVendor(ctx, vendorID)
	if err != nil {
		return Vendor{}, err
	}
	status, err := next(vendor.Status)
	if err != nil {
		return Vendor{}, err
	}
	if err := s.dir.SetStatus(ctx, vendorID, status); err != nil {
		return Vendor{}, err
	}
	_ = s.dir.Audit(ctx, actor, vendorID, action, strings.TrimSpace(reason))
	return s.dir.GetVendor(ctx, vendorID)
}

func validateHours(hours []Hours) error {
	if len(hours) != 7 {
		return ErrBadInput
	}
	seen := map[int]bool{}
	for _, day := range hours {
		if day.Weekday < 0 || day.Weekday > 6 || seen[day.Weekday] {
			return ErrBadInput
		}
		seen[day.Weekday] = true
		if day.Closed {
			continue
		}
		if !validClock(day.OpensAt) || !validClock(day.ClosesAt) {
			return ErrBadInput
		}
	}
	return nil
}

func validClock(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}

func newToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
