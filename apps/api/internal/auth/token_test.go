package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"beta-go/api/internal/identity"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	user := identity.User{
		ID:    uuid.New(),
		Email: "ada@gmail.com",
		Roles: []string{identity.RoleCustomer, identity.RoleSuperAdmin},
	}
	sessionID := uuid.New()
	secret := "dev-only-session-secret-change-before-deploy"

	signed, err := issueAccess(secret, user, sessionID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := parseAccess(secret, signed)
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != user.ID || principal.SessionID != sessionID || principal.Email != user.Email {
		t.Fatalf("principal = %#v", principal)
	}
	if !identity.HasAny(principal.Roles, identity.RoleSuperAdmin) {
		t.Fatalf("roles = %#v", principal.Roles)
	}
}

func TestAccessTokenRejectsOtherSecret(t *testing.T) {
	user := identity.User{ID: uuid.New(), Email: "ada@gmail.com", Roles: []string{identity.RoleCustomer}}
	signed, err := issueAccess("dev-only-session-secret-change-before-deploy", user, uuid.New(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseAccess("a-different-secret-that-is-long-enough", signed); err == nil {
		t.Fatal("expected rejection")
	}
}
