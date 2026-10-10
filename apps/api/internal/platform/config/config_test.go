package config

import (
	"testing"
	"time"
)

func TestFromEnvRequiresCoreSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("SESSION_SECRET", "short")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected missing configuration error")
	}
}

func TestFromEnvParsesBootstrapAdmins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("GOOGLE_CLIENT_ID", "client")
	t.Setenv("SESSION_SECRET", "dev-only-session-secret-change-before-deploy")
	t.Setenv("BOOTSTRAP_ADMIN_EMAILS", " You@Gmail.com, you@gmail.com, ")
	t.Setenv("ACCESS_TOKEN_TTL", "10m")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsBootstrapAdmin("you@gmail.com") {
		t.Fatal("expected bootstrap admin match")
	}
	if cfg.IsBootstrapAdmin("other@gmail.com") {
		t.Fatal("unexpected bootstrap admin match")
	}
	if len(cfg.BootstrapAdminEmails) != 1 {
		t.Fatalf("emails = %#v", cfg.BootstrapAdminEmails)
	}
	if cfg.AccessTTL != 10*time.Minute {
		t.Fatalf("access ttl = %s", cfg.AccessTTL)
	}
}

func TestFromEnvRefusesUnsafeProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("GOOGLE_CLIENT_ID", "client")
	t.Setenv("SESSION_SECRET", "dev-only-session-secret-change-before-deploy")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("PAYMENT_PROVIDER", "sandbox")
	t.Setenv("WEB_ORIGIN", "http://localhost:5173")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected production refusal")
	}
}

func TestSecurityReportNeverIncludesSecretValues(t *testing.T) {
	cfg := Config{
		Env:                  "development",
		DatabaseURL:          "postgres://secret-user:secret-pass@localhost/db",
		SessionSecret:        "dev-only-session-secret-change-before-deploy",
		PaymentWebhookSecret: "webhook-secret-value",
		GoogleClientID:       "client",
		WebOrigin:            "http://localhost:5173",
		PaymentProvider:      "sandbox",
	}
	report := cfg.SecurityReport()
	if report["sessionSecretSet"] != true || report["sessionSecretIsExample"] != true {
		t.Fatalf("flags = %#v", report)
	}
	if report["paymentWebhookSet"] != true {
		t.Fatal("expected webhook flag")
	}
	if _, ok := report["databaseURL"]; ok {
		t.Fatal("database url must not appear")
	}
}
