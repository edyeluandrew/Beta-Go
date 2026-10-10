package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the process configuration. Secrets come from the environment, never from code.
type Config struct {
	Env                  string
	HTTPAddr             string
	DatabaseURL          string
	GoogleClientID       string
	SessionSecret        string
	WebOrigin            string
	CookieSecure         bool
	AccessTTL            time.Duration
	RefreshTTL           time.Duration
	BootstrapAdminEmails []string
	PaymentProvider      string
	PaymentWebhookSecret string
}

// Load reads .env files without overriding variables already set in the process.
// The API is started from apps/api. The first file that defines a variable wins:
// apps/api/.env, then apps/.env, then the repo root.
func Load() (Config, error) {
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		_ = godotenv.Load(path)
	}
	return FromEnv()
}

// FromEnv builds a Config from the current environment.
func FromEnv() (Config, error) {
	env := value("ENV", "development")
	cfg := Config{
		Env:                  env,
		HTTPAddr:             listenAddr(),
		DatabaseURL:          strings.TrimSpace(os.Getenv("DATABASE_URL")),
		GoogleClientID:       strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		SessionSecret:        os.Getenv("SESSION_SECRET"),
		WebOrigin:            value("WEB_ORIGIN", "http://localhost:5173"),
		CookieSecure:         boolValue("COOKIE_SECURE", env == "production"),
		AccessTTL:            durationValue("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:           durationValue("REFRESH_TOKEN_TTL", 14*24*time.Hour),
		BootstrapAdminEmails: parseEmails(os.Getenv("BOOTSTRAP_ADMIN_EMAILS")),
		PaymentProvider:      value("PAYMENT_PROVIDER", ""),
		PaymentWebhookSecret: strings.TrimSpace(os.Getenv("PAYMENT_WEBHOOK_SECRET")),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if len(cfg.SessionSecret) < 32 {
		missing = append(missing, "SESSION_SECRET (32 characters or more)")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing configuration: %s. Copy .env.example to .env and fill it in", strings.Join(missing, ", "))
	}
	if problems := cfg.ProductionProblems(); len(problems) > 0 {
		return Config{}, fmt.Errorf("production is not safe to start: %s", strings.Join(problems, "; "))
	}
	return cfg, nil
}

const exampleSessionSecret = "dev-only-session-secret-change-before-deploy"

// ProductionProblems lists launch blockers. Development may keep local values.
func (c Config) ProductionProblems() []string {
	if c.Env != "production" {
		return nil
	}
	var problems []string
	if !c.CookieSecure {
		problems = append(problems, "COOKIE_SECURE must be true")
	}
	if c.SessionSecret == exampleSessionSecret || strings.Contains(c.SessionSecret, "dev-only") {
		problems = append(problems, "SESSION_SECRET is still the example value")
	}
	if c.PaymentProvider == "sandbox" {
		problems = append(problems, "PAYMENT_PROVIDER=sandbox is not allowed")
	}
	if strings.Contains(c.WebOrigin, "localhost") || strings.Contains(c.WebOrigin, "127.0.0.1") {
		problems = append(problems, "WEB_ORIGIN still points at this machine")
	}
	return problems
}

// SecurityReport is safe to show a super admin. It never includes secret values.
func (c Config) SecurityReport() map[string]any {
	return map[string]any{
		"env":                   c.Env,
		"cookieSecure":          c.CookieSecure,
		"googleConfigured":      c.GoogleClientID != "",
		"paymentProvider":       c.PaymentProvider,
		"paymentWebhookSet":     c.PaymentWebhookSecret != "",
		"sessionSecretSet":       len(c.SessionSecret) >= 32,
		"sessionSecretIsExample": c.SessionSecret == exampleSessionSecret || strings.Contains(c.SessionSecret, "dev-only"),
		"webOrigin":             c.WebOrigin,
		"sandboxBlocked":        c.Env == "production" && c.PaymentProvider == "sandbox",
	}
}

// IsBootstrapAdmin reports whether this Google email should receive super_admin.
func (c Config) IsBootstrapAdmin(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, candidate := range c.BootstrapAdminEmails {
		if candidate == email {
			return true
		}
	}
	return false
}

func listenAddr() string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return value("HTTP_ADDR", ":8080")
}

func value(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func boolValue(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}

func durationValue(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseEmails(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}
