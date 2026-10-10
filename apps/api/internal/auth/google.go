package auth

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/api/idtoken"

	"beta-go/api/internal/identity"
)

// GoogleIdentity is a verified Google account.
type GoogleIdentity = identity.GoogleProfile

// Verifier checks a Google ID token.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}

type googleVerifier struct {
	clientID string
}

func NewGoogleVerifier(clientID string) Verifier {
	return googleVerifier{clientID: clientID}
}

func (v googleVerifier) Verify(ctx context.Context, raw string) (GoogleIdentity, error) {
	payload, err := idtoken.Validate(ctx, raw, v.clientID)
	if err != nil {
		return GoogleIdentity{}, ErrUnauthorized
	}

	email, _ := payload.Claims["email"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if payload.Subject == "" || email == "" || !claimTrue(payload.Claims["email_verified"]) {
		return GoogleIdentity{}, ErrUnauthorized
	}

	name, _ := payload.Claims["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		name = email
	}
	if len(name) > 200 {
		name = name[:200]
	}
	picture, _ := payload.Claims["picture"].(string)
	if len(picture) > 2048 {
		picture = ""
	}

	return GoogleIdentity{
		Sub:     payload.Subject,
		Email:   email,
		Name:    name,
		Picture: picture,
	}, nil
}

func claimTrue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

// ErrAccountDisabled means the person cannot sign in.
var ErrAccountDisabled = errors.New("account disabled")
