package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/launch"
)

func TestLaunchPrepIsSuperAdminOnly(t *testing.T) {
	users := newMemUsers()
	sessions := newMemSessions()
	google := &fakeGoogle{}
	cfg := testConfig("admin@gmail.com")
	cfg.DatabaseURL = "postgres://secret-user:secret-pass@localhost/db"
	cfg.PaymentWebhookSecret = "webhook-secret-value"
	cfg.PaymentProvider = "sandbox"
	authSvc := auth.NewService(cfg, users, sessions, google)
	authHandler := auth.NewHandler(authSvc, cfg)
	engine := New(cfg, nil, authHandler, nil, nil, nil, nil, nil, launch.NewHandler(launch.NewService(launch.NewMemoryBooks(), users, cfg)))

	google.profile = identity.GoogleProfile{Sub: "diner", Email: "diner@gmail.com", Name: "Diner"}
	_, _, dinerCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	status, _ := call(t, engine, http.MethodGet, "/api/v1/admin/launch/security", "", dinerCookies)
	if status != http.StatusForbidden {
		t.Fatalf("customer security %d", status)
	}

	google.profile = identity.GoogleProfile{Sub: "help", Email: "help@gmail.com", Name: "Help"}
	_, _, supportCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	help, err := users.FindByEmail(context.Background(), "help@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := users.GrantRole(context.Background(), help.ID, identity.RoleSupport); err != nil {
		t.Fatal(err)
	}
	_, _, supportCookies = callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)
	status, _ = call(t, engine, http.MethodPost, "/api/v1/admin/launch/staff", `{"email":"diner@gmail.com","role":"finance"}`, supportCookies)
	if status != http.StatusForbidden {
		t.Fatalf("support grant %d", status)
	}

	google.profile = identity.GoogleProfile{Sub: "admin", Email: "admin@gmail.com", Name: "Admin"}
	_, _, adminCookies := callCookies(t, engine, http.MethodPost, "/api/v1/auth/google", `{"idToken":"good"}`, nil)

	status, body := call(t, engine, http.MethodGet, "/api/v1/admin/launch/security", "", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("security %d body %s", status, body)
	}
	for _, secret := range []string{"secret-pass", "webhook-secret-value", "dev-only-session-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("security leaked %q: %s", secret, body)
		}
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/launch/staff", `{"email":"diner@gmail.com","role":"super_admin"}`, adminCookies)
	if status != http.StatusForbidden {
		t.Fatalf("super_admin grant %d body %s", status, body)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/launch/staff", `{"email":"diner@gmail.com","role":"vendor"}`, adminCookies)
	if status != http.StatusForbidden {
		t.Fatalf("vendor grant %d body %s", status, body)
	}
	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/launch/staff", `{"email":"diner@gmail.com","role":"finance"}`, adminCookies)
	if status != http.StatusOK || !strings.Contains(body, identity.RoleFinance) {
		t.Fatalf("finance grant %d body %s", status, body)
	}

	status, body = call(t, engine, http.MethodPost, "/api/v1/admin/launch/backups", `{"note":"Neon PITR checked"}`, adminCookies)
	if status != http.StatusCreated {
		t.Fatalf("backup %d body %s", status, body)
	}
	status, body = call(t, engine, http.MethodGet, "/api/v1/admin/launch/audit", "", adminCookies)
	if status != http.StatusOK {
		t.Fatalf("audit %d body %s", status, body)
	}
	var audit struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Items) < 2 {
		t.Fatalf("audit items = %#v", audit.Items)
	}
}
