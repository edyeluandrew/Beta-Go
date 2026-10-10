package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/commission"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/vendor"
)

func TestVendorPoolTopUpAndStaffAdjustment(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	cfg.BootstrapAdminEmails = []string{"admin@gmail.com", "finance@gmail.com"}
	authSvc := auth.NewService(cfg, users, sessions, google)
	authHandler := auth.NewHandler(authSvc, cfg)
	shops := vendor.NewMemoryDirectory()
	vendorHandler := vendor.NewHandler(vendor.NewService(shops, users), authSvc, cfg)
	sandbox := payment.NewSandbox()
	secret := "payment-webhook-secret-for-tests-only"
	pool := commission.NewService(shops, commission.NewMemoryBooks(shops), order.NewMemoryDirectory(), nil, sandbox, secret)
	engine := New(cfg, nil, authHandler, vendorHandler, nil, nil, commission.NewHandler(pool), nil, nil)

	google.profile = identity.GoogleProfile{Sub: "cook", Email: "cook@gmail.com", Name: "Cook"}
	_, _, cookCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "admin", Email: "admin@gmail.com", Name: "Admin"}
	_, _, adminCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "finance", Email: "finance@gmail.com", Name: "Finance"}
	_, _, financeCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)

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

	status, body = call(t, engine, http.MethodPost, "/api/v1/vendor/commission/top-ups", `{"amountUgx":50000,"provider":"mtn","phone":"0772000111","idempotencyKey":"top-1","screenshotUrl":"https://example.com/shot.jpg"}`, cookCookies)
	if status != http.StatusCreated || strings.Contains(body, `"status":"PAID"`) {
		t.Fatalf("top-up %d %s", status, body)
	}
	var started struct {
		TopUp struct {
			ID        string `json:"id"`
			Reference string `json:"providerReference"`
			Status    string `json:"status"`
		} `json:"topUp"`
	}
	if err := json.Unmarshal([]byte(body), &started); err != nil {
		t.Fatal(err)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/vendor/commission", "", cookCookies)
	if status != http.StatusOK || !strings.Contains(body, `"availableUgx":0`) {
		t.Fatalf("empty pool %d %s", status, body)
	}

	notice := `{"eventId":"evt-pool","reference":"` + started.TopUp.Reference + `","status":"SUCCESS"}`
	status, body = postSigned(t, engine, "/api/v1/commission/top-ups/webhook", notice, signPayment(secret, notice))
	if status != http.StatusOK {
		t.Fatalf("webhook %d %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/vendor/commission", "", cookCookies)
	if status != http.StatusOK || !strings.Contains(body, `"availableUgx":50000`) {
		t.Fatalf("credited %d %s", status, body)
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/vendors/"+vendorID+"/commission/adjustments", `{"amountUgx":80000,"reason":"launch credit"}`, adminCookies)
	if status != http.StatusCreated || !strings.Contains(body, `"status":"PENDING"`) {
		t.Fatalf("dual %d %s", status, body)
	}
	var pending struct {
		Adjustment struct {
			ID string `json:"id"`
		} `json:"adjustment"`
	}
	if err := json.Unmarshal([]byte(body), &pending); err != nil {
		t.Fatal(err)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/commission/adjustments/"+pending.Adjustment.ID+"/approve", "", adminCookies)
	if status != http.StatusForbidden {
		t.Fatalf("self approve %d", status)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/commission/adjustments/"+pending.Adjustment.ID+"/approve", "", financeCookies)
	if status != http.StatusOK || !strings.Contains(body, `"availableUgx":130000`) {
		t.Fatalf("approved %d %s", status, body)
	}

	status, _ = call(t, engine, http.MethodGet, "/api/v1/admin/vendors/"+vendorID+"/commission", "", cookCookies)
	if status != http.StatusForbidden {
		t.Fatalf("cook admin %d", status)
	}
	if _, err := uuid.Parse(started.TopUp.ID); err != nil {
		t.Fatal(err)
	}
}

func postSigned(t *testing.T, handler http.Handler, path, body, signature string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Payment-Signature", signature)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.Code, string(payload)
}
