package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/order"
	"beta-go/api/internal/vendor"
)

func TestCustomerCanBrowseAndOrderOneKitchen(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	authSvc := auth.NewService(cfg, users, sessions, google)
	authHandler := auth.NewHandler(authSvc, cfg)
	directory := vendor.NewMemoryDirectory()
	vendorSvc := vendor.NewService(directory, users)
	vendorHandler := vendor.NewHandler(vendorSvc, authSvc, cfg)
	orderSvc := order.NewService(vendorSvc, directory, order.NewMemoryDirectory(), order.FixedSettings{
		"minimum_order_ugx":       5000,
		"commission_rate_percent": 10,
	})
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		t.Fatal(err)
	}
	orderSvc.SetClock(func() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, loc) })
	engine := New(cfg, nil, authHandler, vendorHandler, order.NewHandler(orderSvc), nil, nil, nil, nil)

	google.profile = identity.GoogleProfile{Sub: "cook", Email: "cook@gmail.com", Name: "Cook"}
	_, _, cookCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "diner", Email: "diner@gmail.com", Name: "Diner"}
	_, _, dinerCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "admin", Email: "admin@gmail.com", Name: "Admin"}
	_, _, adminCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)

	status, body := call(t, engine, http.MethodPost, "/api/v1/admin/vendors", `{"businessName":"Kiosk","contactName":"Ada","contactEmail":"cook@gmail.com","phone":"0700","address":"Kabale"}`, adminCookies)
	if status != http.StatusCreated {
		t.Fatalf("create %d %s", status, body)
	}
	var created struct {
		Vendor struct {
			ID string `json:"id"`
		} `json:"vendor"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatal(err)
	}
	vendorID := created.Vendor.ID

	status, body = call(t, engine, http.MethodGet, "/api/v1/vendors", "", nil)
	if status != http.StatusOK || strings.Contains(body, "Kiosk") {
		t.Fatalf("hidden before approval %d %s", status, body)
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/invitations", "", adminCookies)
	var invite struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &invite); err != nil || status != http.StatusCreated {
		t.Fatalf("invite %d %s", status, body)
	}
	status, _, cookCookies = callCookies(t, engine, http.MethodPost, "/api/v1/vendor/invitations/accept", `{"token":"`+invite.Token+`"}`, cookCookies)
	if status != http.StatusOK {
		t.Fatalf("accept %d", status)
	}
	if status, _ = call(t, engine, http.MethodPut, "/api/v1/vendor/hours", hoursJSON(), cookCookies); status != http.StatusNoContent {
		t.Fatalf("hours %d", status)
	}
	if status, _ = call(t, engine, http.MethodPatch, "/api/v1/vendor/shop", `{"shopPhotoUrl":"https://example.com/shop.jpg","deliveryFeeUgx":2000,"isOpen":true,"acceptingOrders":true}`, cookCookies); status != http.StatusOK {
		t.Fatalf("shop %d", status)
	}
	var itemID string
	for _, name := range []string{"Rolex", "Chapati", "Tea"} {
		status, body = call(t, engine, http.MethodPost, "/api/v1/vendor/items", `{"name":"`+name+`","priceUgx":5000}`, cookCookies)
		if status != http.StatusCreated {
			t.Fatalf("item %s %d %s", name, status, body)
		}
		if name == "Rolex" {
			var payload struct {
				Item struct {
					ID string `json:"id"`
				} `json:"item"`
			}
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatal(err)
			}
			itemID = payload.Item.ID
		}
	}
	parsed, err := uuid.Parse(vendorID)
	if err != nil {
		t.Fatal(err)
	}
	directory.SetAvailable(parsed, 50000)
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/destination", `{"provider":"mtn","account":"0772000000"}`, adminCookies)
	if status != http.StatusOK {
		t.Fatalf("destination %d %s", status, body)
	}
	if status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/destination/verify", "", adminCookies); status != http.StatusOK {
		t.Fatalf("verify %d", status)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/approve", "", adminCookies)
	if status != http.StatusOK || !strings.Contains(body, vendor.StatusActive) {
		t.Fatalf("approve %d %s", status, body)
	}

	status, body = call(t, engine, http.MethodGet, "/api/v1/vendors", "", nil)
	if status != http.StatusOK || !strings.Contains(body, "Kiosk") || strings.Contains(body, "0772000000") {
		t.Fatalf("public list %d %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/vendors/"+vendorID, "", nil)
	if status != http.StatusOK || !strings.Contains(body, "Rolex") || strings.Contains(body, "cook@gmail.com") {
		t.Fatalf("public menu %d %s", status, body)
	}

	orderBody := `{"idempotencyKey":"order-1","vendorId":"` + vendorID + `","fulfilment":"collection","items":[{"itemId":"` + itemID + `","quantity":1}]}`
	status, body = call(t, engine, http.MethodPost, "/api/v1/orders", orderBody, dinerCookies)
	if status != http.StatusCreated || !strings.Contains(body, `"totalUgx":5000`) || !strings.Contains(body, "PENDING_PAYMENT") {
		t.Fatalf("order %d %s", status, body)
	}
	var placed struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	if err := json.Unmarshal([]byte(body), &placed); err != nil {
		t.Fatal(err)
	}
	status, again := call(t, engine, http.MethodPost, "/api/v1/orders", orderBody, dinerCookies)
	if status != http.StatusCreated || !strings.Contains(again, placed.Order.ID) {
		t.Fatalf("replay %d %s", status, again)
	}
	changed := strings.Replace(orderBody, `"quantity":1`, `"quantity":2`, 1)
	status, _ = call(t, engine, http.MethodPost, "/api/v1/orders", changed, dinerCookies)
	if status != http.StatusConflict {
		t.Fatalf("idempotency conflict %d", status)
	}

	orderSvc.SetClock(func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, loc) })
	status, _ = call(t, engine, http.MethodPost, "/api/v1/orders", `{"idempotencyKey":"order-2","vendorId":"`+vendorID+`","fulfilment":"delivery","address":"Makanga","phone":"0700111222","items":[{"itemId":"`+itemID+`","quantity":1}]}`, dinerCookies)
	if status != http.StatusConflict {
		t.Fatalf("closed sunday %d", status)
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/orders/"+placed.Order.ID+"/cancel", "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, "CANCELLED") {
		t.Fatalf("cancel %d %s", status, body)
	}
	status, _ = call(t, engine, http.MethodGet, "/api/v1/orders/"+placed.Order.ID, "", cookCookies)
	if status != http.StatusNotFound {
		t.Fatalf("other customer %d", status)
	}
}
