package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"beta-go/api/internal/identity"
)

const accessIssuer = "beta-go"

type accessClaims struct {
	jwt.RegisteredClaims
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"sid"`
}

// Principal is the person attached to a valid access token.
type Principal struct {
	UserID    uuid.UUID
	Email     string
	Roles     []string
	SessionID uuid.UUID
}

func issueAccess(secret string, user identity.User, sessionID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessIssuer,
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Email:     user.Email,
		Roles:     user.Roles,
		SessionID: sessionID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func parseAccess(secret, raw string) (Principal, error) {
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithIssuer(accessIssuer))
	if err != nil || !token.Valid {
		return Principal{}, ErrUnauthorized
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	roles := claims.Roles
	if roles == nil {
		roles = []string{}
	}
	return Principal{
		UserID:    userID,
		Email:     claims.Email,
		Roles:     roles,
		SessionID: sessionID,
	}, nil
}

func newRefreshToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ErrUnauthorized means the session or token cannot be trusted.
var ErrUnauthorized = errors.New("unauthorized")
