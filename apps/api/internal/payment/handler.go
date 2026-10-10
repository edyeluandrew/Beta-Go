package payment

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/platform/httpx"
)

// Handler is the HTTP edge for payments.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Initiate(c *gin.Context) {
	customer, orderID, ok := ownedOrder(c)
	if !ok {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the payment details and try again.")
		return
	}
	saved, first, err := h.svc.Initiate(c.Request.Context(), customer, orderID, body.Provider, body.Phone)
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusOK
	if first {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"payment": paymentJSON(saved)})
}

func (h *Handler) Get(c *gin.Context) {
	customer, orderID, ok := ownedOrder(c)
	if !ok {
		return
	}
	saved, err := h.svc.Get(c.Request.Context(), customer, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment": paymentJSON(saved)})
}

func (h *Handler) Webhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<16)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "The payment notice could not be read.")
		return
	}
	saved, err := h.svc.AcceptWebhook(c.Request.Context(), body, c.GetHeader("X-Payment-Signature"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"payment": paymentJSON(saved)})
}

func (h *Handler) Reconcile(c *gin.Context) {
	summary, err := h.svc.Reconcile(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"checked":       summary.Checked,
		"paid":          summary.Paid,
		"expired":       summary.Expired,
		"refundPending": summary.RefundPending,
		"unchanged":     summary.Unchanged,
	})
}

func ownedOrder(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, uuid.Nil, false
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That order is not valid.")
		return uuid.Nil, uuid.Nil, false
	}
	return principal.UserID, orderID, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotConfigured):
		httpx.Error(c, http.StatusServiceUnavailable, "payment_not_configured", "Payments are not switched on yet.")
	case errors.Is(err, ErrBadSignature):
		httpx.Error(c, http.StatusUnauthorized, "invalid_signature", "The payment notice could not be verified.")
	case errors.Is(err, ErrBadInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the payment details and try again.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", "That payment was not found.")
	case errors.Is(err, ErrConflict):
		httpx.Error(c, http.StatusConflict, "payment_conflict", "This order is not waiting for payment.")
	default:
		httpx.Error(c, http.StatusInternalServerError, "payment_failed", "The payment did not finish.")
	}
}

func paymentJSON(payment Payment) gin.H {
	body := gin.H{
		"id":                payment.ID,
		"orderId":           payment.OrderID,
		"status":            payment.Status,
		"amountUgx":         payment.AmountUGX,
		"provider":          payment.Provider,
		"phone":             payment.Phone,
		"providerReference": payment.Reference,
		"lateSuccess":       payment.LateSuccess,
	}
	if !payment.ExpiresAt.IsZero() {
		body["expiresAt"] = payment.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return body
}
