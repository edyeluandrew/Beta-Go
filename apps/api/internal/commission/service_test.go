package commission

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"

	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/vendor"
)

func TestPoolNeverGoesNegativeAndMovesOnce(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	shops.SetAvailable(kitchen.ID, 50000)
	cook := uuid.New()
	shops.BindUser(cook, kitchen.ID)
	orders := order.NewMemoryDirectory()
	placed, err := orders.Create(ctx, order.Order{
		CustomerID: uuid.New(), VendorID: kitchen.ID, Status: order.StatusAwaitingVendor,
		FoodSubtotalUGX: 20000, TotalUGX: 20000, CommissionUGX: 2000, IdempotencyKey: "one",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(shops, NewMemoryBooks(shops), orders, order.FixedSettings{
		"commission_activity_floor_ugx": 10000,
		"commission_low_balance_ugx":    20000,
	}, nil, "")

	account, err := svc.Reserve(ctx, placed.ID)
	if err != nil || account.Available != 48000 || account.Reserved != 2000 {
		t.Fatalf("reserve %#v %v", account, err)
	}
	again, err := svc.Reserve(ctx, placed.ID)
	if err != nil || again.Available != 48000 || again.Reserved != 2000 {
		t.Fatalf("reserve replay %#v %v", again, err)
	}
	if _, err := svc.Reserve(ctx, placed.ID); err != nil {
		t.Fatal(err)
	}

	other, err := orders.Create(ctx, order.Order{
		CustomerID: uuid.New(), VendorID: kitchen.ID, Status: order.StatusAwaitingVendor,
		FoodSubtotalUGX: 500000, TotalUGX: 500000, CommissionUGX: 50000, IdempotencyKey: "big",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reserve(ctx, other.ID); err != ErrInsufficient {
		t.Fatalf("overspend %v", err)
	}

	captured, err := svc.Capture(ctx, placed.ID)
	if err != nil || captured.Reserved != 0 || captured.Available != 48000 {
		t.Fatalf("capture %#v %v", captured, err)
	}
	replay, err := svc.Capture(ctx, placed.ID)
	if err != nil || replay.Reserved != 0 {
		t.Fatalf("capture replay %#v %v", replay, err)
	}
	if _, err := svc.Release(ctx, placed.ID); err != ErrConflict {
		t.Fatalf("release after capture %v", err)
	}
}

func TestReleaseReturnsFundsAndLowPoolHidesTheShop(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	shops.SetAvailable(kitchen.ID, 11000)
	orders := order.NewMemoryDirectory()
	placed, err := orders.Create(ctx, order.Order{
		CustomerID: uuid.New(), VendorID: kitchen.ID, CommissionUGX: 2000, IdempotencyKey: "hide",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(shops, NewMemoryBooks(shops), orders, nil, nil, "")
	account, err := svc.Reserve(ctx, placed.ID)
	if err != nil || account.Available != 9000 || account.VendorStatus != vendor.StatusInactive {
		t.Fatalf("hide %#v %v", account, err)
	}
	released, err := svc.Release(ctx, placed.ID)
	if err != nil || released.Available != 11000 || released.Reserved != 0 || released.VendorStatus != vendor.StatusActive {
		t.Fatalf("release %#v %v", released, err)
	}

	paused, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Paused", Status: vendor.StatusPaused})
	if err != nil {
		t.Fatal(err)
	}
	shops.SetAvailable(paused.ID, 11000)
	held, err := orders.Create(ctx, order.Order{
		CustomerID: uuid.New(), VendorID: paused.ID, CommissionUGX: 2000, IdempotencyKey: "pause",
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := svc.Reserve(ctx, held.ID)
	if err != nil || after.VendorStatus != vendor.StatusPaused {
		t.Fatalf("paused %#v %v", after, err)
	}
}

func TestTopUpIgnoresScreenshotsAndNeedsASignedSuccess(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusInactive})
	if err != nil {
		t.Fatal(err)
	}
	cook := uuid.New()
	shops.BindUser(cook, kitchen.ID)
	sandbox := payment.NewSandbox()
	secret := "pool-secret"
	svc := NewService(shops, NewMemoryBooks(shops), order.NewMemoryDirectory(), nil, sandbox, secret)

	if _, _, err := svc.StartTopUp(ctx, cook, 5000, ProviderMTN, "0772000111", "tiny"); err != ErrBadInput {
		t.Fatalf("small top-up %v", err)
	}
	topUp, first, err := svc.StartTopUp(ctx, cook, 50000, ProviderMTN, "0772000111", "top-1")
	if err != nil || !first || topUp.Status != TopUpPending {
		t.Fatalf("start %#v %v %v", topUp, first, err)
	}
	replay, first, err := svc.StartTopUp(ctx, cook, 50000, ProviderMTN, "0772000111", "top-1")
	if err != nil || first || replay.ID != topUp.ID || sandbox.Calls() != 1 {
		t.Fatalf("replay %#v first=%v calls=%d %v", replay, first, sandbox.Calls(), err)
	}

	body := []byte(`{"eventId":"evt-1","reference":"` + topUp.Reference + `","status":"SUCCESS"}`)
	if _, err := svc.AcceptTopUpWebhook(ctx, body, "bad"); err != ErrBadSignature {
		t.Fatalf("bad signature %v", err)
	}
	paid, err := svc.AcceptTopUpWebhook(ctx, body, sign(secret, body))
	if err != nil || paid.Status != TopUpPaid {
		t.Fatalf("paid %#v %v", paid, err)
	}
	again, err := svc.AcceptTopUpWebhook(ctx, body, sign(secret, body))
	if err != nil || again.Status != TopUpPaid {
		t.Fatalf("webhook replay %#v %v", again, err)
	}
	account, err := svc.AccountForVendor(ctx, cook)
	if err != nil || account.Available != 50000 || account.VendorStatus != vendor.StatusActive {
		t.Fatalf("credited %#v %v", account, err)
	}
}

func TestLargeAdjustmentNeedsASecondPerson(t *testing.T) {
	ctx := context.Background()
	shops := vendor.NewMemoryDirectory()
	kitchen, err := shops.CreateVendor(ctx, vendor.Vendor{BusinessName: "Kiosk", Status: vendor.StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	shops.SetAvailable(kitchen.ID, 20000)
	svc := NewService(shops, NewMemoryBooks(shops), order.NewMemoryDirectory(), nil, nil, "")
	first := uuid.New()
	second := uuid.New()
	pending, account, err := svc.RequestAdjustment(ctx, first, kitchen.ID, 80000, "promo credit")
	if err != nil || pending.Status != AdjPending || account.Available != 20000 {
		t.Fatalf("pending %#v %#v %v", pending, account, err)
	}
	if _, _, err := svc.ApproveAdjustment(ctx, first, pending.ID); err != ErrForbidden {
		t.Fatalf("self approve %v", err)
	}
	applied, account, err := svc.ApproveAdjustment(ctx, second, pending.ID)
	if err != nil || applied.Status != AdjApplied || account.Available != 100000 {
		t.Fatalf("approved %#v %#v %v", applied, account, err)
	}
	small, account, err := svc.RequestAdjustment(ctx, first, kitchen.ID, 10000, "small top")
	if err != nil || small.Status != AdjApplied || account.Available != 110000 {
		t.Fatalf("small %#v %#v %v", small, account, err)
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
