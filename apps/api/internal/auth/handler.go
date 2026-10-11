package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"beta-go/api/internal/identity"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/httpx"
)

const (
	accessCookie  = "bg_access"
	refreshCookie = "bg_refresh"
	refreshPath   = "/api/v1/auth"
)

// Handler is the HTTP edge for sign-in.
type Handler struct {
	svc *Service
	cfg config.Config
}

func NewHandler(svc *Service, cfg config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

// Sessions exposes the session checker used by route protection.
func (h *Handler) Sessions() Sessions {
	return h.svc.sessions
}

func (h *Handler) Google(c *gin.Context) {
	var body struct {
		IDToken string `json:"idToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.IDToken == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Google sign-in did not include a token.")
		return
	}
	if h.cfg.GoogleClientID == "" {
		httpx.Error(c, http.StatusServiceUnavailable, "google_not_configured", "Google sign-in is not configured yet.")
		return
	}

	signedIn, err := h.svc.SignInWithGoogle(c.Request.Context(), body.IDToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.writeAuthError(c, err, "Google sign-in could not be verified. Try again.")
		return
	}
	h.setSessionCookies(c, signedIn)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(signedIn.User)})
}

func (h *Handler) Register(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		Area     string `json:"area"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Those account details could not be read.")
		return
	}
	signedIn, err := h.svc.Register(c.Request.Context(), body.Name, body.Phone, body.Area, body.Email, body.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		var input InputError
		if errors.As(err, &input) {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", input.Message)
			return
		}
		h.writeAuthError(c, err, "Create the account again.")
		return
	}
	h.setSessionCookies(c, signedIn)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(signedIn.User)})
}

func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Email == "" || body.Password == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Enter your email and password.")
		return
	}
	signedIn, err := h.svc.Login(c.Request.Context(), body.Email, body.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.writeAuthError(c, err, "Email or password is wrong.")
		return
	}
	h.setSessionCookies(c, signedIn)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(signedIn.User)})
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	signedIn, err := h.svc.Refresh(c.Request.Context(), raw)
	if err != nil {
		h.clearSessionCookies(c)
		h.writeAuthError(c, err, "Sign in to continue.")
		return
	}
	h.setSessionCookies(c, signedIn)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(signedIn.User)})
}

func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(refreshCookie)
	if err := h.svc.Logout(c.Request.Context(), raw); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "logout_failed", "Sign-out did not finish. Try again.")
		return
	}
	h.clearSessionCookies(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	principal, ok := PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	user, err := h.svc.CurrentUser(c.Request.Context(), principal)
	if err != nil {
		h.writeAuthError(c, err, "Sign in to continue.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	principal, ok := PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	var body struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
		Area  string `json:"area"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Those account details could not be read.")
		return
	}
	user, err := h.svc.UpdateProfile(c.Request.Context(), principal, body.Name, body.Phone, body.Area)
	if err != nil {
		if errors.Is(err, identity.ErrBadProfile) {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Name, phone, or area is too long.")
			return
		}
		h.writeAuthError(c, err, "Sign in to continue.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

// WriteSessionCookies stores a new session on the response.
func WriteSessionCookies(c *gin.Context, cfg config.Config, signedIn SignedIn) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessCookie, signedIn.AccessToken, int(cfg.AccessTTL.Seconds()), "/", "", cfg.CookieSecure, true)
	c.SetCookie(refreshCookie, signedIn.RefreshToken, int(cfg.RefreshTTL.Seconds()), refreshPath, "", cfg.CookieSecure, true)
}

func (h *Handler) setSessionCookies(c *gin.Context, signedIn SignedIn) {
	WriteSessionCookies(c, h.cfg, signedIn)
}

func (h *Handler) clearSessionCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(accessCookie, "", -1, "/", "", h.cfg.CookieSecure, true)
	c.SetCookie(refreshCookie, "", -1, refreshPath, "", h.cfg.CookieSecure, true)
}

func (h *Handler) writeAuthError(c *gin.Context, err error, unauthorized string) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", unauthorized)
	case errors.Is(err, ErrAccountDisabled):
		httpx.Error(c, http.StatusForbidden, "account_disabled", "This account is disabled.")
	case errors.Is(err, identity.ErrEmailTaken):
		httpx.Error(c, http.StatusConflict, "email_taken", "An account with this email already exists.")
	default:
		httpx.Error(c, http.StatusInternalServerError, "auth_failed", "Sign-in did not finish. Try again.")
	}
}

type userResponse struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	PictureURL string   `json:"pictureUrl"`
	Phone      string   `json:"phone"`
	Area       string   `json:"area"`
	Roles      []string `json:"roles"`
}

func userJSON(user identity.User) userResponse {
	roles := user.Roles
	if roles == nil {
		roles = []string{}
	}
	return userResponse{
		ID:         user.ID.String(),
		Email:      user.Email,
		Name:       user.Name,
		PictureURL: user.PictureURL,
		Phone:      user.Phone,
		Area:       user.Area,
		Roles:      roles,
	}
}
