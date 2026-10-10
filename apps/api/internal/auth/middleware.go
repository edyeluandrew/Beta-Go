package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/httpx"
)

// Authenticate requires a valid access cookie whose session is still active.
// The UI is not the enforcer. Logout revokes the session immediately.
func Authenticate(cfg config.Config, sessions interface {
	Active(ctx context.Context, id uuid.UUID) error
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(accessCookie)
		if err != nil || raw == "" {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
			c.Abort()
			return
		}
		principal, err := parseAccess(cfg.SessionSecret, raw)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
			c.Abort()
			return
		}
		if err := sessions.Active(c.Request.Context(), principal.SessionID); err != nil {
			if errors.Is(err, ErrUnauthorized) {
				httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
			} else {
				httpx.Error(c, http.StatusServiceUnavailable, "session_check_failed", "Sign-in could not be checked. Try again.")
			}
			c.Abort()
			return
		}
		c.Set("principal", principal)
		c.Next()
	}
}

// RequireRole allows the request when the access token carries one of the roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok || !identity.HasAny(principal.Roles, roles...) {
			httpx.Error(c, http.StatusForbidden, "forbidden", "You do not have access to this workspace.")
			c.Abort()
			return
		}
		c.Next()
	}
}

// PrincipalFrom reads the person set by Authenticate.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	value, ok := c.Get("principal")
	if !ok {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}
