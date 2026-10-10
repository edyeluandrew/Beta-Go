package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/vendor"
)

func TestVendorOnboardingFromTheSpec(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	authSvc := auth.NewService(cfg, users, sessions, google)
	authHandler := auth.NewHandler(authSvc, cfg)
	vendorHandler := vendor.NewHandler(vendor.NewService(vendor.NewMemoryDirectory(), users), authSvc, cfg)
	engine := New(cfg, nil, authHandler, vendorHandler, nil, nil, nil, nil, nil)

	google.profile = identity.GoogleProfile{Sub: "cook", Email: "cook@gmail.com", Name: "Cook"}
	_, _, cookCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	status, _ := call(t, engine, http.MethodPost, "/api/v1/admin/vendors", `{"businessName":"Kiosk","contactName":"Ada","contactEmail":"cook@gmail.com","phone":"0700","address":"Kabale"}`, cookCookies)
	if status != http.StatusForbidden {
		t.Fatalf("customer create status %d", status)
	}

	google.profile = identity.GoogleProfile{Sub: "admin", Email: "admin@gmail.com", Name: "Admin"}
	_, _, adminCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	status, body := call(t, engine, http.MethodPost, "/api/v1/admin/vendors", `{"businessName":"Kiosk","contactName":"Ada","contactEmail":"cook@gmail.com","phone":"0700","address":"Kabale"}`, adminCookies)
	if status != http.StatusCreated {
		t.Fatalf("create status %d body %s", status, body)
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

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/invitations", "", adminCookies)
	if status != http.StatusCreated {
		t.Fatalf("invite status %d body %s", status, body)
	}
	var invite struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &invite); err != nil || invite.Token == "" {
		t.Fatalf("invite body %s", body)
	}

	status, _, cookCookies = callCookies(t, engine, http.MethodPost, "/api/v1/vendor/invitations/accept", `{"token":"`+invite.Token+`"}`, cookCookies)
	if status != http.StatusOK {
		t.Fatalf("accept status %d", status)
	}

	status, _ = call(t, engine, http.MethodPut, "/api/v1/vendor/hours", hoursJSON(), cookCookies)
	if status != http.StatusNoContent {
		t.Fatalf("hours status %d", status)
	}
	status, _ = call(t, engine, http.MethodPatch, "/api/v1/vendor/shop", `{"description":"Rollex and tea","shopPhotoUrl":"https://example.com/shop.jpg","deliveryFeeUgx":2000,"isOpen":true,"acceptingOrders":true}`, cookCookies)
	if status != http.StatusOK {
		t.Fatalf("shop status %d", status)
	}
	for _, name := range []string{"Rolex", "Chapati", "Tea"} {
		status, body = call(t, engine, http.MethodPost, "/api/v1/vendor/items", `{"name":"`+name+`","description":"hot","priceUgx":5000}`, cookCookies)
		if status != http.StatusCreated {
			t.Fatalf("item %s status %d body %s", name, status, body)
		}
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/approve", "", adminCookies)
	if status != http.StatusConflict {
		t.Fatalf("approve before destination status %d body %s", status, body)
	}

	status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/destination", `{"provider":"mtn","account":"0772000000"}`, adminCookies)
	if status != http.StatusOK {
		t.Fatalf("destination status %d", status)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/destination/verify", "", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("verify status %d", status)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/approve", "", adminCookies)
	if status != http.StatusOK || !strings.Contains(body, vendor.StatusInactive) {
		t.Fatalf("approve status %d body %s", status, body)
	}
}

func hoursJSON() string {
	return `{"hours":[
		{"weekday":0,"closed":true},
		{"weekday":1,"opensAt":"08:00","closesAt":"21:00"},
		{"weekday":2,"opensAt":"08:00","closesAt":"21:00"},
		{"weekday":3,"opensAt":"08:00","closesAt":"21:00"},
		{"weekday":4,"opensAt":"08:00","closesAt":"21:00"},
		{"weekday":5,"opensAt":"08:00","closesAt":"21:00"},
		{"weekday":6,"opensAt":"08:00","closesAt":"21:00"}
	]}`
}
