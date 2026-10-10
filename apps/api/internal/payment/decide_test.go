package payment

import (
	"testing"
	"time"

	"beta-go/api/internal/order"
)

func TestDecideTrustsOnlyProviderResults(t *testing.T) {
	open := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	late := open.Add(16 * time.Minute)
	pending := State{PaymentStatus: StatusPending, OrderStatus: order.StatusPendingPayment, ExpiresAt: open.Add(15 * time.Minute)}

	paid := decide(pending, ProviderSuccess, open)
	if !paid.Changed || paid.PaymentStatus != StatusPaid || paid.OrderStatus != order.StatusAwaitingVendor {
		t.Fatalf("paid %#v", paid)
	}
	claimed := decide(pending, "PAID", open)
	if claimed.Changed {
		t.Fatalf("client status counted %#v", claimed)
	}
	failed := decide(pending, ProviderFailed, open)
	if failed.PaymentStatus != StatusFailed || failed.OrderStatus != order.StatusPendingPayment {
		t.Fatalf("failed %#v", failed)
	}
	expired := decide(pending, ProviderPending, late)
	if expired.PaymentStatus != StatusExpired || expired.OrderStatus != order.StatusExpired {
		t.Fatalf("expired %#v", expired)
	}
	refund := decide(pending, ProviderSuccess, late)
	if !refund.Late || refund.PaymentStatus != StatusRefundPending || refund.OrderStatus != order.StatusExpired {
		t.Fatalf("late %#v", refund)
	}
	cancelled := decide(State{PaymentStatus: StatusPending, OrderStatus: order.StatusCancelled, ExpiresAt: pending.ExpiresAt}, ProviderSuccess, open)
	if !cancelled.Late || cancelled.OrderStatus != order.StatusCancelled || cancelled.PaymentStatus != StatusRefundPending {
		t.Fatalf("cancelled %#v", cancelled)
	}
}
