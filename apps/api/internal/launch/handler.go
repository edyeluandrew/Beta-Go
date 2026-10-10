package launch

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/platform/httpx"
)

// Handler is super-admin launch prep only.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Security(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.SecurityReport())
}

func (h *Handler) ListAudit(c *gin.Context) {
	entries, err := h.svc.ListAudit(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "internal", "could not load audit")
		return
	}
	out := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		out = append(out, gin.H{
			"id":        entry.ID,
			"actorId":   entry.ActorID,
			"action":    entry.Action,
			"entityId":  entry.EntityID,
			"reason":    entry.Reason,
			"createdAt": entry.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) RecordBackup(c *gin.Context) {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	actor, ok := actorID(c)
	if !ok {
		return
	}
	check, err := h.svc.RecordBackup(c.Request.Context(), actor, body.Note)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "internal", "could not record backup check")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": check.ID, "note": check.Note, "createdAt": check.CreatedAt})
}

func (h *Handler) ListBackups(c *gin.Context) {
	checks, err := h.svc.ListBackups(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "internal", "could not load backup checks")
		return
	}
	out := make([]gin.H, 0, len(checks))
	for _, check := range checks {
		out = append(out, gin.H{"id": check.ID, "note": check.Note, "createdAt": check.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *Handler) GrantStaff(c *gin.Context) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "bad_request", "email and role are required")
		return
	}
	actor, ok := actorID(c)
	if !ok {
		return
	}
	user, err := h.svc.GrantStaff(c.Request.Context(), actor, body.Email, body.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			httpx.Error(c, http.StatusForbidden, "forbidden", "only support and finance can be granted here")
		case errors.Is(err, ErrNotFound):
			httpx.Error(c, http.StatusNotFound, "not_found", "that person has not signed in yet")
		case errors.Is(err, ErrBadInput):
			httpx.Error(c, http.StatusBadRequest, "bad_request", "email is required")
		default:
			httpx.Error(c, http.StatusInternalServerError, "internal", "could not grant role")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "email": user.Email, "roles": user.Roles})
}

func actorID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, false
	}
	return principal.UserID, true
}
