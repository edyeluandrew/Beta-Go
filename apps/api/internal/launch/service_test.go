package launch

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
)

type memAccounts struct {
	users map[uuid.UUID]identity.User
}

func (m *memAccounts) Find(_ context.Context, id uuid.UUID) (identity.User, error) {
	user, ok := m.users[id]
	if !ok {
		return identity.User{}, identity.ErrNotFound
	}
	return user, nil
}

func (m *memAccounts) FindByEmail(_ context.Context, email string) (identity.User, error) {
	for _, user := range m.users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return identity.User{}, identity.ErrNotFound
}

func (m *memAccounts) GrantRole(_ context.Context, userID uuid.UUID, role string) error {
	user, ok := m.users[userID]
	if !ok {
		return identity.ErrNotFound
	}
	user.Roles = append(user.Roles, role)
	m.users[userID] = user
	return nil
}

func TestGrantStaffAllowsSupportAndFinanceOnly(t *testing.T) {
	id := uuid.New()
	svc := NewService(NewMemoryBooks(), &memAccounts{users: map[uuid.UUID]identity.User{
		id: {ID: id, Email: "cook@example.com", Roles: []string{identity.RoleCustomer}},
	}}, config.Config{Env: "development"})

	if _, err := svc.GrantStaff(context.Background(), uuid.New(), "cook@example.com", identity.RoleSuperAdmin); err != ErrForbidden {
		t.Fatalf("super_admin grant = %v", err)
	}
	if _, err := svc.GrantStaff(context.Background(), uuid.New(), "cook@example.com", identity.RoleVendor); err != ErrForbidden {
		t.Fatalf("vendor grant = %v", err)
	}
	user, err := svc.GrantStaff(context.Background(), uuid.New(), "cook@example.com", identity.RoleFinance)
	if err != nil {
		t.Fatal(err)
	}
	if len(user.Roles) == 0 || user.Roles[len(user.Roles)-1] != identity.RoleFinance {
		t.Fatalf("roles = %#v", user.Roles)
	}
}

func TestSecurityReportOmitsSecrets(t *testing.T) {
	svc := NewService(NewMemoryBooks(), &memAccounts{}, config.Config{
		Env:                  "development",
		DatabaseURL:          "postgres://secret-user:secret-pass@localhost/db",
		SessionSecret:        "dev-only-session-secret-change-before-deploy",
		PaymentWebhookSecret: "webhook-secret-value",
		GoogleClientID:       "client",
		WebOrigin:            "http://localhost:5173",
		PaymentProvider:      "sandbox",
	})
	report := svc.SecurityReport()
	blob := strings.ToLower(strings.Join(keysAndValues(report), " "))
	for _, secret := range []string{"secret-pass", "webhook-secret-value", "dev-only-session-secret"} {
		if strings.Contains(blob, secret) {
			t.Fatalf("report leaked %q: %#v", secret, report)
		}
	}
	if report["sessionSecretIsExample"] != true {
		t.Fatalf("expected example secret flag: %#v", report)
	}
}

func keysAndValues(in map[string]any) []string {
	var out []string
	for key, value := range in {
		out = append(out, key)
		out = append(out, strings.ToLower(strings.TrimSpace(toString(value))))
	}
	return out
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
