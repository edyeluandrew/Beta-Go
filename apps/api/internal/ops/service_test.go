package ops

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/commission"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/vendor"
)

func TestAcceptReservesAndCompleteCaptures(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	shops.SetAvailable(kitchen.ID, 50000)
	cook := uuid.New()
	shops.BindUser(cook, kitchen.ID)
	diner := uuid.New()
	orders := order.NewMemoryDirectory()
	pays := payment.NewMemoryLedger(orders)
	placed := seedPaid(t, orders, pays, diner, kitchen.ID, "one", 20000, 2000, time.Now().Add(10*time.Minute))
	pool := commission.NewService(shops, commission.NewMemoryBooks(shops), orders, nil, nil, "")
	svc := NewService(orders, shops, pool, payment.NewService(pays, orders, nil, nil, ""), NewMemoryRecords(), nil)

	accepted, err := svc.Accept(ctx, cook, placed.ID)
	if err != nil || accepted.Status != order.StatusConfirmed {
		t.Fatalf("accept %#v %v", accepted, err)
	}
	account, err := pool.AccountForAdmin(ctx, kitchen.ID)
	if err != nil || account.Available != 48000 || account.Reserved != 2000 {
		t.Fatalf("reserved %#v %v", account, err)
	}
	again, err := svc.Accept(ctx, cook, placed.ID)
	if err != nil || again.Status != order.StatusConfirmed {
		t.Fatalf("replay %#v %v", again, err)
	}
	for _, next := range []string{order.StatusPreparing, order.StatusReady, order.StatusFulfilled} {
		moved, err := svc.Advance(ctx, cook, placed.ID, next)
		if err != nil || moved.Status != next {
			t.Fatalf("advance %s %#v %v", next, moved, err)
		}
	}
	done, err := svc.Confirm(ctx, diner, placed.ID)
	if err != nil || done.Status != order.StatusCompleted {
		t.Fatalf("confirm %#v %v", done, err)
	}
	account, err = pool.AccountForAdmin(ctx, kitchen.ID)
	if err != nil || account.Reserved != 0 || account.Available != 48000 {
		t.Fatalf("captured %#v %v", account, err)
	}
}

func TestRejectAndPaidCancelMarkRefund(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, _ := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusActive})
	shops.SetAvailable(kitchen.ID, 50000)
	cook := uuid.New()
	shops.BindUser(cook, kitchen.ID)
	diner := uuid.New()
	orders := order.NewMemoryDirectory()
	pays := payment.NewMemoryLedger(orders)
	records := NewMemoryRecords()
	svc := NewService(orders, shops, commission.NewService(shops, commission.NewMemoryBooks(shops), orders, nil, nil, ""), payment.NewService(pays, orders, nil, nil, ""), records, nil)
	rejected := seedPaid(t, orders, pays, diner, kitchen.ID, "rej", 20000, 2000, time.Now().Add(time.Hour))
	moved, err := svc.Reject(ctx, cook, rejected.ID, ReasonVendorUnable)
	if err != nil || moved.Status != order.StatusRejected {
		t.Fatalf("reject %#v %v", moved, err)
	}
	refund, err := records.FindRefund(ctx, rejected.ID)
	if err != nil || refund.Status != RefundRequested {
		t.Fatalf("refund %#v %v", refund, err)
	}
	cancelled := seedPaid(t, orders, pays, diner, kitchen.ID, "can", 20000, 2000, time.Now().Add(time.Hour))
	closed, err := svc.CustomerCancel(ctx, diner, cancelled.ID)
	if err != nil || closed.Status != order.StatusCancelled {
		t.Fatalf("cancel %#v %v", closed, err)
	}
}

func seedPaid(t *testing.T, orders *order.MemoryDirectory, pays *payment.MemoryLedger, diner, vendorID uuid.UUID, key string, total, commission int, deadline time.Time) order.Order {
	t.Helper()
	placed, err := orders.Create(context.Background(), order.Order{
		CustomerID: diner, VendorID: vendorID, Status: order.StatusAwaitingVendor,
		TotalUGX: total, FoodSubtotalUGX: total, CommissionUGX: commission,
		IdempotencyKey: key, DecisionDeadline: deadline,
	})
	if err != nil {
		t.Fatal(err)
	}
	pays.SeedPaid(placed.ID, total)
	return placed
}
