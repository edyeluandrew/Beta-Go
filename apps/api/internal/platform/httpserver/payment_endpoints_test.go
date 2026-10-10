package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
)

func TestPaymentSucceedsOnlyFromSignedProviderResult(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	authHandler := auth.NewHandler(auth.NewService(cfg, users, sessions, google), cfg)
	directory := order.NewMemoryDirectory()
	orderHandler := order.NewHandler(order.NewService(nil, nil, directory, order.FixedSettings{}))
	sandbox := payment.NewSandbox()
	secret := "payment-webhook-secret-for-tests-only"
	paySvc := payment.NewService(payment.NewMemoryLedger(directory), directory, order.FixedSettings{
		"unpaid_expiry_minutes": 15,
	}, sandbox, secret)
	start := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	paySvc.SetClock(func() time.Time { return start })
	engine := New(cfg, nil, authHandler, nil, orderHandler, payment.NewHandler(paySvc), nil, nil, nil)

	google.profile = identity.GoogleProfile{Sub: "diner", Email: "diner@gmail.com", Name: "Diner"}
	_, meBody, dinerCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	google.profile = identity.GoogleProfile{Sub: "other", Email: "other@gmail.com", Name: "Other"}
	_, _, otherCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
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
	customerID, err := uuid.Parse(me.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	payID := seedOrder(t, directory, customerID, "pay")
	cancelID := seedOrder(t, directory, customerID, "cancel")
	expireID := seedOrder(t, directory, customerID, "expire")
	lateID := seedOrder(t, directory, customerID, "late")

	status, _ := call(t, engine, http.MethodPost, "/api/v1/orders/"+payID.String()+"/payment", `{"provider":"mtn","phone":"0772000111","status":"PAID"}`, dinerCookies)
	if status != http.StatusCreated {
		t.Fatalf("initiate %d", status)
	}
	status, body := call(t, engine, http.MethodPost, "/api/v1/orders/"+payID.String()+"/payment", `{"provider":"mtn","phone":"0772000111"}`, dinerCookies)
	if status != http.StatusOK || sandbox.Calls() != 1 || !strings.Contains(body, `"status":"PENDING"`) || strings.Contains(body, `"status":"PAID"`) {
		t.Fatalf("replay initiate calls %d %d %s", sandbox.Calls(), status, body)
	}
	var started struct {
		Payment struct {
			Reference string `json:"providerReference"`
			Amount    int    `json:"amountUgx"`
		} `json:"payment"`
	}
	if err := json.Unmarshal([]byte(body), &started); err != nil {
		t.Fatal(err)
	}
	if started.Payment.Amount != 8000 || started.Payment.Reference == "" {
		t.Fatalf("amount %d ref %s", started.Payment.Amount, started.Payment.Reference)
	}
	status, _ = call(t, engine, http.MethodGet, "/api/v1/orders/"+payID.String()+"/payment", "", otherCookies)
	if status != http.StatusNotFound {
		t.Fatalf("other customer %d", status)
	}

	notice := `{"eventId":"evt-pay","reference":"` + started.Payment.Reference + `","status":"SUCCESS"}`
	status, _ = callSigned(t, engine, notice, "not-the-signature")
	if status != http.StatusUnauthorized {
		t.Fatalf("bad signature %d", status)
	}
	status, body = callSigned(t, engine, notice, signPayment(secret, notice))
	if status != http.StatusOK || !strings.Contains(body, `"status":"PAID"`) || strings.Contains(body, `"lateSuccess":true`) {
		t.Fatalf("webhook %d %s", status, body)
	}
	status, again := callSigned(t, engine, notice, signPayment(secret, notice))
	if status != http.StatusOK || !strings.Contains(again, `"status":"PAID"`) {
		t.Fatalf("webhook replay %d %s", status, again)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/orders/"+payID.String(), "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusAwaitingVendor) {
		t.Fatalf("order after pay %d %s", status, body)
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/orders/"+cancelID.String()+"/payment", `{"provider":"airtel","phone":"256770000222"}`, dinerCookies)
	if status != http.StatusCreated {
		t.Fatalf("cancel initiate %d %s", status, body)
	}
	var cancelPay struct {
		Payment struct {
			Reference string `json:"providerReference"`
		} `json:"payment"`
	}
	if err := json.Unmarshal([]byte(body), &cancelPay); err != nil {
		t.Fatal(err)
	}
	status, _ = call(t, engine, http.MethodPost, "/api/v1/orders/"+cancelID.String()+"/cancel", "", dinerCookies)
	if status != http.StatusOK {
		t.Fatalf("cancel %d", status)
	}
	cancelNotice := `{"eventId":"evt-cancel","reference":"` + cancelPay.Payment.Reference + `","status":"SUCCESS"}`
	status, body = callSigned(t, engine, cancelNotice, signPayment(secret, cancelNotice))
	if status != http.StatusOK || !strings.Contains(body, `"lateSuccess":true`) || !strings.Contains(body, payment.StatusRefundPending) {
		t.Fatalf("late after cancel %d %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/orders/"+cancelID.String(), "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusCancelled) {
		t.Fatalf("cancelled order %d %s", status, body)
	}

	for _, id := range []uuid.UUID{expireID, lateID} {
		status, body = call(t, engine, http.MethodPost, "/api/v1/orders/"+id.String()+"/payment", `{"provider":"mtn","phone":"0772000333"}`, dinerCookies)
		if status != http.StatusCreated {
			t.Fatalf("seed pay %s %d %s", id, status, body)
		}
		if id == lateID {
			var startedLate struct {
				Payment struct {
					Reference string `json:"providerReference"`
				} `json:"payment"`
			}
			if err := json.Unmarshal([]byte(body), &startedLate); err != nil {
				t.Fatal(err)
			}
			sandbox.SetResult(startedLate.Payment.Reference, payment.ProviderSuccess)
		}
	}
	paySvc.SetClock(func() time.Time { return start.Add(16 * time.Minute) })
	status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/payments/reconcile", "", dinerCookies)
	if status != http.StatusForbidden {
		t.Fatalf("customer reconcile %d", status)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/payments/reconcile", "", adminCookies)
	if status != http.StatusOK || !strings.Contains(body, `"expired":1`) || !strings.Contains(body, `"refundPending":1`) {
		t.Fatalf("reconcile %d %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/orders/"+expireID.String(), "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, order.StatusExpired) {
		t.Fatalf("expired order %d %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/orders/"+lateID.String()+"/payment", "", dinerCookies)
	if status != http.StatusOK || !strings.Contains(body, `"lateSuccess":true`) {
		t.Fatalf("late reconcile %d %s", status, body)
	}
}

func TestPaymentRoutesStayClosedWithoutProvider(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("")
	authHandler := auth.NewHandler(auth.NewService(cfg, users, sessions, google), cfg)
	directory := order.NewMemoryDirectory()
	paySvc := payment.NewService(payment.NewMemoryLedger(directory), directory, order.FixedSettings{}, nil, "")
	engine := New(cfg, nil, authHandler, nil, order.NewHandler(order.NewService(nil, nil, directory, order.FixedSettings{})), payment.NewHandler(paySvc), nil, nil, nil)
	google.profile = identity.GoogleProfile{Sub: "diner", Email: "diner@gmail.com", Name: "Diner"}
	_, meBody, cookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal([]byte(meBody), &me); err != nil {
		t.Fatal(err)
	}
	customerID, err := uuid.Parse(me.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	orderID := seedOrder(t, directory, customerID, "closed")
	status, body := call(t, engine, http.MethodPost, "/api/v1/orders/"+orderID.String()+"/payment", `{"provider":"mtn","phone":"0772000111"}`, cookies)
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "payment_not_configured") {
		t.Fatalf("closed %d %s", status, body)
	}
}

func seedOrder(t *testing.T, directory *order.MemoryDirectory, customerID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	created, err := directory.Create(context.Background(), order.Order{
		CustomerID:      customerID,
		VendorID:        uuid.New(),
		Status:          order.StatusPendingPayment,
		Fulfilment:      order.FulfilmentCollection,
		FoodSubtotalUGX: 8000,
		TotalUGX:        8000,
		CommissionRate:  10,
		CommissionUGX:   800,
		IdempotencyKey:  key,
		RequestHash:     key,
	})
	if err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func signPayment(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func callSigned(t *testing.T, handler http.Handler, body, signature string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", strings.NewReader(body))
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
