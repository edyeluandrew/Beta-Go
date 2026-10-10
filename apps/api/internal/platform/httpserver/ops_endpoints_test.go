package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/commission"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/ops"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/vendor"
)

func TestKitchenAcceptsThenCustomerConfirms(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	authSvc := auth.NewService(cfg, users, sessions, google)
	authHandler := auth.NewHandler(authSvc, cfg)
	shops := vendor.NewMemoryDirectory()
	vendorHandler := vendor.NewHandler(vendor.NewService(shops, users), authSvc, cfg)
	orders := order.NewMemoryDirectory()
	pays := payment.NewMemoryLedger(orders)
	pool := commission.NewService(shops, commission.NewMemoryBooks(shops), orders, nil, nil, "")
	paySvc := payment.NewService(pays, orders, nil, nil, "")
	opsHandler := ops.NewHandler(ops.NewService(orders, shops, pool, paySvc, ops.NewMemoryRecords(), nil))
	engine := New(cfg, nil, authHandler, vendorHandler, order.NewHandler(order.NewService(nil, nil, orders, order.FixedSettings{})), nil, nil, opsHandler, nil)

	google.profile = identity.GoogleProfile{Sub: "cook", Email: "cook@gmail.com", Name: "Cook"}
	_, _, cookCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "diner", Email: "diner@gmail.com", Name: "Diner"}
	_, meBody, dinerCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "admin", Email: "admin@gmail.com", Name: "Admin"}
	_, _, adminCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)

	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal([]byte(meBody), &me); err != nil {
		t.Fatal(err)
	}
	dinerID, _ := uuid.Parse(me.User.ID)

	status, body := call(t, engine, http.MethodPost, "/api/v1/admin/vendors", `{"businessName":"Kiosk","contactName":"Ada","contactEmail":"cook@gmail.com","phone":"0700","address":"Kabale"}`, adminCookies)
	if status != http.StatusCreated {
		t.Fatalf("create %d %s", status, body)
	}
	var created struct {
		Vendor struct {
			ID string `json:"id"`
		} `json:"vendor"`
	}
	_ = json.Unmarshal([]byte(body), &created)
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+created.Vendor.ID+"/invitations", "", adminCookies)
	var invite struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal([]byte(body), &invite)
	status, _, cookCookies = callCookies(t, engine, http.MethodPost, "/api/v1/vendor/invitations/accept", `{"token":"`+invite.Token+`"}`, cookCookies)
	if status != http.StatusOK {
		t.Fatalf("accept invite %d", status)
	}
	vendorID, _ := uuid.Parse(created.Vendor.ID)
	shops.SetAvailable(vendorID, 50000)

	placed, err := orders.Create(context.Background(), order.Order{
		CustomerID: dinerID, VendorID: vendorID, Status: order.StatusAwaitingVendor,
		TotalUGX: 20000, FoodSubtotalUGX: 20000, CommissionUGX: 2000,
		IdempotencyKey: "paid-1", DecisionDeadline: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	pays.SeedPaid(placed.ID, 20000)

	status, body = call(t, engine, http.MethodPost, "/api/v1/vendor/orders/"+placed.ID.String()+"/accept", "", cookCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusConfirmed) {
		t.Fatalf("accept %d %s", status, body)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/vendor/orders/"+placed.ID.String()+"/preparing", "", cookCookies)
	if status != http.StatusOK {
		t.Fatalf("preparing %d", status)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/vendor/orders/"+placed.ID.String()+"/ready", "", cookCookies)
	if status != http.StatusOK {
		t.Fatalf("ready %d", status)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/vendor/orders/"+placed.ID.String()+"/fulfilled", "", cookCookies)
	if status != http.StatusOK {
		t.Fatalf("fulfilled %d", status)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/orders/"+placed.ID.String()+"/confirm", "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusCompleted) {
		t.Fatalf("confirm %d %s", status, body)
	}

	other, err := orders.Create(context.Background(), order.Order{
		CustomerID: dinerID, VendorID: vendorID, Status: order.StatusAwaitingVendor,
		TotalUGX: 20000, CommissionUGX: 2000, IdempotencyKey: "paid-2", DecisionDeadline: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	pays.SeedPaid(other.ID, 20000)
	status, body = call(t, engine, http.MethodPost, "/api/v1/vendor/orders/"+other.ID.String()+"/reject", `{"reason":"VENDOR_UNABLE"}`, cookCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusRejected) {
		t.Fatalf("reject %d %s", status, body)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/orders/"+placed.ID.String()+"/cancel", "", dinerCookies)
	if status != http.StatusForbidden {
		t.Fatalf("cancel completed %d", status)
	}
}
