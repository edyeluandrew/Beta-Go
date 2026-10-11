package ops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/commission"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/vendor"
)

// Orders is the kitchen's work queue and the customer's receipt.
type Orders interface {
	Get(ctx context.Context, id uuid.UUID) (order.Order, error)
	ListForVendor(ctx context.Context, vendorID uuid.UUID) ([]order.Order, error)
	ListByStatus(ctx context.Context, status string) ([]order.Order, error)
	Transition(ctx context.Context, id uuid.UUID, from, to string, actor uuid.UUID, reason string) (order.Order, error)
	SetCancelRequested(ctx context.Context, id uuid.UUID, requested bool) (order.Order, error)
	Cancel(ctx context.Context, id, actor uuid.UUID, reason string) (order.Order, error)
}

// Shops maps a cook to one kitchen.
type Shops interface {
	VendorIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// Pool reserves, captures, and releases commission for an order.
type Pool interface {
	Reserve(ctx context.Context, orderID uuid.UUID) (commission.Account, error)
	Capture(ctx context.Context, orderID uuid.UUID) (commission.Account, error)
	Release(ctx context.Context, orderID uuid.UUID) (commission.Account, error)
}

// Payments marks a paid order for refund. It does not finish the provider refund.
type Payments interface {
	MarkRefundDue(ctx context.Context, orderID uuid.UUID) (payment.Payment, error)
}

// Records stores refunds, disputes, and alerts.
type Records interface {
	SaveRefund(ctx context.Context, refund Refund) (Refund, error)
	FindRefund(ctx context.Context, orderID uuid.UUID) (Refund, error)
	SaveDispute(ctx context.Context, dispute Dispute) (Dispute, error)
	FindDispute(ctx context.Context, orderID uuid.UUID) (Dispute, error)
	ListOpenDisputes(ctx context.Context) ([]Dispute, error)
	ResolveDispute(ctx context.Context, id, actor uuid.UUID, resolution string) (Dispute, error)
	SaveAlert(ctx context.Context, alert Alert) (Alert, error)
	HasAlert(ctx context.Context, orderID uuid.UUID, kind string) (bool, error)
}

// Settings reads policy minutes and hours.
type Settings interface {
	Int(ctx context.Context, key string) (int, error)
}

// Service is the paid-order life after checkout.
type Service struct {
	orders   Orders
	shops    Shops
	pool     Pool
	payments Payments
	records  Records
	settings Settings
	notes    announcer
	now      func() time.Time
}

func NewService(orders Orders, shops Shops, pool Pool, payments Payments, records Records, settings Settings) *Service {
	return &Service{orders: orders, shops: shops, pool: pool, payments: payments, records: records, settings: settings, now: time.Now}
}

func (s *Service) SetClock(now func() time.Time) {
	s.now = now
}

func (s *Service) ListVendor(ctx context.Context, userID uuid.UUID) ([]order.Order, error) {
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return nil, mapShop(err)
	}
	return s.orders.ListForVendor(ctx, vendorID)
}

func (s *Service) GetVendor(ctx context.Context, userID, orderID uuid.UUID) (order.Order, error) {
	return s.ownedByVendor(ctx, userID, orderID)
}

func (s *Service) Accept(ctx context.Context, userID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.ownedByVendor(ctx, userID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status == order.StatusConfirmed || placed.Status == order.StatusPreparing || placed.Status == order.StatusReady || placed.Status == order.StatusFulfilled || placed.Status == order.StatusCompleted {
		return placed, nil
	}
	if placed.Status != order.StatusAwaitingVendor {
		return order.Order{}, ErrConflict
	}
	if s.decisionExpired(placed) {
		expired, err := s.expire(ctx, placed, userID)
		if err != nil {
			return order.Order{}, err
		}
		return expired, ErrClosed
	}
	if _, err := s.pool.Reserve(ctx, orderID); err != nil {
		return order.Order{}, err
	}
	moved, err := s.orders.Transition(ctx, orderID, order.StatusAwaitingVendor, order.StatusConfirmed, userID, "")
	if err != nil {
		return order.Order{}, err
	}
	s.tellCustomer(ctx, moved, "Kitchen accepted", "Your order is confirmed.")
	return moved, nil
}

func (s *Service) Reject(ctx context.Context, userID, orderID uuid.UUID, reason string) (order.Order, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = ReasonVendorUnable
	}
	placed, err := s.ownedByVendor(ctx, userID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status == order.StatusRejected {
		return placed, nil
	}
	if placed.Status != order.StatusAwaitingVendor {
		return order.Order{}, ErrConflict
	}
	moved, err := s.orders.Transition(ctx, orderID, order.StatusAwaitingVendor, order.StatusRejected, userID, reason)
	if err != nil {
		return order.Order{}, err
	}
	_ = s.dueRefund(ctx, moved, reason)
	s.tellCustomer(ctx, moved, "Kitchen declined", "This order was not accepted.")
	return moved, nil
}

func (s *Service) Advance(ctx context.Context, userID, orderID uuid.UUID, next string) (order.Order, error) {
	from, ok := nextFrom(next)
	if !ok {
		return order.Order{}, ErrBadInput
	}
	placed, err := s.ownedByVendor(ctx, userID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status == next {
		return placed, nil
	}
	if placed.Status != from {
		return order.Order{}, ErrConflict
	}
	moved, err := s.orders.Transition(ctx, orderID, from, next, userID, "")
	if err != nil {
		return order.Order{}, err
	}
	s.tellCustomer(ctx, moved, "Order update", statusLine(next))
	return moved, nil
}

func (s *Service) VendorCancel(ctx context.Context, userID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.ownedByVendor(ctx, userID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	switch placed.Status {
	case order.StatusCancelled:
		return placed, nil
	case order.StatusConfirmed, order.StatusPreparing, order.StatusReady:
	default:
		return order.Order{}, ErrForbidden
	}
	moved, err := s.orders.Transition(ctx, orderID, placed.Status, order.StatusCancelled, userID, ReasonVendorUnable)
	if err != nil {
		return order.Order{}, err
	}
	if _, err := s.pool.Release(ctx, orderID); err != nil {
		return order.Order{}, err
	}
	_ = s.dueRefund(ctx, moved, ReasonVendorUnable)
	s.tellCustomer(ctx, moved, "Order cancelled", "The kitchen cancelled this order.")
	return moved, nil
}

func (s *Service) ApproveCustomerCancel(ctx context.Context, userID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.ownedByVendor(ctx, userID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status != order.StatusConfirmed || !placed.CancelRequested {
		return order.Order{}, ErrForbidden
	}
	moved, err := s.orders.Transition(ctx, orderID, order.StatusConfirmed, order.StatusCancelled, userID, ReasonCustomerRequest)
	if err != nil {
		return order.Order{}, err
	}
	if _, err := s.pool.Release(ctx, orderID); err != nil {
		return order.Order{}, err
	}
	_ = s.dueRefund(ctx, moved, ReasonCustomerRequest)
	s.tellCustomer(ctx, moved, "Cancellation approved", "The kitchen approved the cancellation.")
	return moved, nil
}

func (s *Service) CustomerCancel(ctx context.Context, customerID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.ownedByCustomer(ctx, customerID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	switch placed.Status {
	case order.StatusPendingPayment:
		return s.orders.Cancel(ctx, orderID, customerID, ReasonCustomerRequest)
	case order.StatusCancelled:
		return placed, nil
	case order.StatusAwaitingVendor:
		moved, err := s.orders.Transition(ctx, orderID, order.StatusAwaitingVendor, order.StatusCancelled, customerID, ReasonCustomerRequest)
		if err != nil {
			return order.Order{}, err
		}
		_ = s.dueRefund(ctx, moved, ReasonCustomerRequest)
		s.announce(ctx, "Order cancelled", "The customer cancelled before the kitchen accepted.", "/v/orders/"+moved.ID.String(), nil, moved.VendorID, nil)
		return moved, nil
	case order.StatusConfirmed:
		return s.orders.SetCancelRequested(ctx, orderID, true)
	default:
		return order.Order{}, ErrForbidden
	}
}

func (s *Service) Confirm(ctx context.Context, customerID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.ownedByCustomer(ctx, customerID, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status == order.StatusCompleted {
		return placed, nil
	}
	if placed.Status != order.StatusFulfilled {
		return order.Order{}, ErrConflict
	}
	if open, _ := s.openDispute(ctx, orderID); open {
		return order.Order{}, ErrConflict
	}
	moved, err := s.orders.Transition(ctx, orderID, order.StatusFulfilled, order.StatusCompleted, customerID, "")
	if err != nil {
		return order.Order{}, err
	}
	if _, err := s.pool.Capture(ctx, orderID); err != nil {
		return order.Order{}, err
	}
	s.announce(ctx, "Order confirmed", "The customer confirmed this order.", "/v/orders/"+moved.ID.String(), nil, moved.VendorID, nil)
	return moved, nil
}

func (s *Service) OpenDispute(ctx context.Context, customerID, orderID uuid.UUID, reason string) (Dispute, order.Order, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Dispute{}, order.Order{}, ErrBadInput
	}
	placed, err := s.ownedByCustomer(ctx, customerID, orderID)
	if err != nil {
		return Dispute{}, order.Order{}, err
	}
	if existing, err := s.records.FindDispute(ctx, orderID); err == nil {
		return existing, placed, nil
	}
	if placed.Status != order.StatusFulfilled {
		return Dispute{}, order.Order{}, ErrForbidden
	}
	hours := s.intOr("dispute_window_hours", 24)
	if !placed.FulfilledAt.IsZero() && s.now().After(placed.FulfilledAt.Add(time.Duration(hours)*time.Hour)) {
		return Dispute{}, order.Order{}, ErrForbidden
	}
	moved, err := s.orders.Transition(ctx, orderID, order.StatusFulfilled, order.StatusDisputed, customerID, ReasonDispute)
	if err != nil {
		return Dispute{}, order.Order{}, err
	}
	saved, err := s.records.SaveDispute(ctx, Dispute{
		OrderID: orderID, CustomerID: customerID, Reason: reason, Status: DisputeOpen,
		OpenedAt: s.now(), EscalateAt: s.now().Add(time.Duration(s.intOr("dispute_escalate_hours", 48)) * time.Hour),
	})
	if err != nil {
		return Dispute{}, moved, err
	}
	s.announce(ctx, "Dispute opened", reason, "/a/reconcile", nil, uuid.Nil, []string{"support", "super_admin"})
	return saved, moved, nil
}

func (s *Service) ResolveDispute(ctx context.Context, actor, disputeOrderID uuid.UUID, outcome, resolution string) (order.Order, error) {
	placed, err := s.orders.Get(ctx, disputeOrderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.Status != order.StatusDisputed {
		return order.Order{}, ErrConflict
	}
	dispute, err := s.records.FindDispute(ctx, disputeOrderID)
	if err != nil {
		return order.Order{}, err
	}
	if _, err := s.records.ResolveDispute(ctx, dispute.ID, actor, resolution); err != nil {
		return order.Order{}, err
	}
	switch outcome {
	case order.StatusCompleted:
		moved, err := s.orders.Transition(ctx, disputeOrderID, order.StatusDisputed, order.StatusCompleted, actor, resolution)
		if err != nil {
			return order.Order{}, err
		}
		if _, err := s.pool.Capture(ctx, disputeOrderID); err != nil && !errors.Is(err, commission.ErrConflict) {
			return order.Order{}, err
		}
		s.tellCustomer(ctx, moved, "Dispute closed", "This order was completed.")
		return moved, nil
	case order.StatusCancelled:
		moved, err := s.orders.Transition(ctx, disputeOrderID, order.StatusDisputed, order.StatusCancelled, actor, resolution)
		if err != nil {
			return order.Order{}, err
		}
		if _, err := s.pool.Release(ctx, disputeOrderID); err != nil && !errors.Is(err, commission.ErrConflict) {
			return order.Order{}, err
		}
		_ = s.dueRefund(ctx, moved, resolution)
		s.tellCustomer(ctx, moved, "Dispute closed", "This order was cancelled.")
		return moved, nil
	default:
		return order.Order{}, ErrBadInput
	}
}

func (s *Service) Reconcile(ctx context.Context) error {
	awaiting, err := s.orders.ListByStatus(ctx, order.StatusAwaitingVendor)
	if err != nil {
		return err
	}
	for _, placed := range awaiting {
		if s.decisionExpired(placed) {
			if _, err := s.expire(ctx, placed, uuid.Nil); err != nil {
				return err
			}
			continue
		}
		if !placed.DecisionDeadline.IsZero() {
			reminderAt := placed.DecisionDeadline.Add(-5 * time.Minute)
			if !s.now().Before(reminderAt) {
				if seen, _ := s.records.HasAlert(ctx, placed.ID, AlertReminder); !seen {
					_, _ = s.records.SaveAlert(ctx, Alert{VendorID: placed.VendorID, OrderID: placed.ID, Kind: AlertReminder})
				}
			}
		}
	}
	fulfilled, err := s.orders.ListByStatus(ctx, order.StatusFulfilled)
	if err != nil {
		return err
	}
	auto := time.Duration(s.intOr("autocomplete_minutes", 120)) * time.Minute
	for _, placed := range fulfilled {
		if placed.FulfilledAt.IsZero() || s.now().Before(placed.FulfilledAt.Add(auto)) {
			continue
		}
		if open, _ := s.openDispute(ctx, placed.ID); open {
			continue
		}
		if _, err := s.orders.Transition(ctx, placed.ID, order.StatusFulfilled, order.StatusCompleted, uuid.Nil, ReasonAutoComplete); err != nil {
			return err
		}
		if _, err := s.pool.Capture(ctx, placed.ID); err != nil {
			return err
		}
	}
	disputes, err := s.records.ListOpenDisputes(ctx)
	if err != nil {
		return err
	}
	for _, dispute := range disputes {
		if dispute.EscalateAt.IsZero() || s.now().Before(dispute.EscalateAt) {
			continue
		}
		if seen, _ := s.records.HasAlert(ctx, dispute.OrderID, AlertEscalate); seen {
			continue
		}
		placed, err := s.orders.Get(ctx, dispute.OrderID)
		if err != nil {
			return err
		}
		if _, err := s.records.SaveAlert(ctx, Alert{VendorID: placed.VendorID, OrderID: placed.ID, Kind: AlertEscalate}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ownedByVendor(ctx context.Context, userID, orderID uuid.UUID) (order.Order, error) {
	vendorID, err := s.shops.VendorIDForUser(ctx, userID)
	if err != nil {
		return order.Order{}, mapShop(err)
	}
	placed, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.VendorID != vendorID {
		return order.Order{}, ErrNotFound
	}
	return placed, nil
}

func (s *Service) ownedByCustomer(ctx context.Context, customerID, orderID uuid.UUID) (order.Order, error) {
	placed, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return order.Order{}, err
	}
	if placed.CustomerID != customerID {
		return order.Order{}, ErrNotFound
	}
	return placed, nil
}

func (s *Service) decisionExpired(placed order.Order) bool {
	if placed.DecisionDeadline.IsZero() {
		return false
	}
	return !s.now().Before(placed.DecisionDeadline)
}

func (s *Service) expire(ctx context.Context, placed order.Order, actor uuid.UUID) (order.Order, error) {
	moved, err := s.orders.Transition(ctx, placed.ID, order.StatusAwaitingVendor, order.StatusExpired, actor, ReasonDecisionExpired)
	if err != nil {
		return order.Order{}, err
	}
	_ = s.dueRefund(ctx, moved, ReasonDecisionExpired)
	s.tellCustomer(ctx, moved, "Order expired", "The kitchen did not accept in time.")
	return moved, nil
}

func (s *Service) dueRefund(ctx context.Context, placed order.Order, reason string) error {
	if _, err := s.payments.MarkRefundDue(ctx, placed.ID); err != nil && !errors.Is(err, payment.ErrNotFound) && !errors.Is(err, payment.ErrConflict) {
		return err
	}
	_, err := s.records.SaveRefund(ctx, Refund{OrderID: placed.ID, AmountUGX: placed.TotalUGX, Status: RefundRequested, Reason: reason})
	return err
}

func (s *Service) openDispute(ctx context.Context, orderID uuid.UUID) (bool, error) {
	dispute, err := s.records.FindDispute(ctx, orderID)
	if err != nil {
		return false, nil
	}
	return dispute.Status == DisputeOpen, nil
}

func (s *Service) intOr(key string, fallback int) int {
	if s.settings == nil {
		return fallback
	}
	value, err := s.settings.Int(context.Background(), key)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func nextFrom(next string) (string, bool) {
	switch next {
	case order.StatusPreparing:
		return order.StatusConfirmed, true
	case order.StatusReady:
		return order.StatusPreparing, true
	case order.StatusFulfilled:
		return order.StatusReady, true
	default:
		return "", false
	}
}

func mapShop(err error) error {
	if errors.Is(err, vendor.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
