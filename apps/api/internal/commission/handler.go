package commission

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/platform/httpx"
)

// Handler is the HTTP edge for the prepaid pool.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetMine(c *gin.Context) {
	userID, ok := actorID(c)
	if !ok {
		return
	}
	account, err := h.svc.AccountForVendor(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"account": accountJSON(account)})
}

func (h *Handler) LedgerMine(c *gin.Context) {
	userID, ok := actorID(c)
	if !ok {
		return
	}
	entries, err := h.svc.LedgerForVendor(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	out := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entryJSON(entry))
	}
	c.JSON(http.StatusOK, gin.H{"entries": out})
}

func (h *Handler) StartTopUp(c *gin.Context) {
	userID, ok := actorID(c)
	if !ok {
		return
	}
	var body struct {
		AmountUGX      int    `json:"amountUgx"`
		Provider       string `json:"provider"`
		Phone          string `json:"phone"`
		IdempotencyKey string `json:"idempotencyKey"`
		ScreenshotURL  string `json:"screenshotUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the top-up and try again.")
		return
	}
	_ = body.ScreenshotURL
	saved, first, err := h.svc.StartTopUp(c.Request.Context(), userID, body.AmountUGX, body.Provider, body.Phone, body.IdempotencyKey)
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusOK
	if first {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"topUp": topUpJSON(saved)})
}

func (h *Handler) GetTopUp(c *gin.Context) {
	userID, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	saved, err := h.svc.GetTopUp(c.Request.Context(), userID, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"topUp": topUpJSON(saved)})
}

func (h *Handler) TopUpWebhook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<16)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "The top-up notice could not be read.")
		return
	}
	saved, err := h.svc.AcceptTopUpWebhook(c.Request.Context(), body, c.GetHeader("X-Payment-Signature"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"topUp": topUpJSON(saved)})
}

func (h *Handler) GetAdmin(c *gin.Context) {
	vendorID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	account, err := h.svc.AccountForAdmin(c.Request.Context(), vendorID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"account": accountJSON(account)})
}

func (h *Handler) RequestAdjustment(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	vendorID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var body struct {
		AmountUGX int    `json:"amountUgx"`
		Reason    string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "An adjustment needs an amount and a reason.")
		return
	}
	adjustment, account, err := h.svc.RequestAdjustment(c.Request.Context(), actor, vendorID, body.AmountUGX, body.Reason)
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusCreated
	if adjustment.Status == AdjApplied {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"adjustment": adjustmentJSON(adjustment), "account": accountJSON(account)})
}

func (h *Handler) ApproveAdjustment(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	adjustment, account, err := h.svc.ApproveAdjustment(c.Request.Context(), actor, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"adjustment": adjustmentJSON(adjustment), "account": accountJSON(account)})
}

func actorID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That record is not valid.")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotConfigured):
		httpx.Error(c, http.StatusServiceUnavailable, "payment_not_configured", "Pool top-ups are not switched on yet.")
	case errors.Is(err, ErrBadSignature):
		httpx.Error(c, http.StatusUnauthorized, "invalid_signature", "The top-up notice could not be verified.")
	case errors.Is(err, ErrBadInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the pool details and try again.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", "That pool record was not found.")
	case errors.Is(err, ErrForbidden):
		httpx.Error(c, http.StatusForbidden, "forbidden", "A second person has to approve this adjustment.")
	case errors.Is(err, ErrInsufficient):
		httpx.Error(c, http.StatusConflict, "insufficient_pool", "The pool does not have enough available funds.")
	case errors.Is(err, ErrConflict):
		httpx.Error(c, http.StatusConflict, "commission_conflict", "That pool movement was already used a different way.")
	default:
		httpx.Error(c, http.StatusInternalServerError, "commission_failed", "The pool update did not finish.")
	}
}

func accountJSON(account Account) gin.H {
	return gin.H{
		"vendorId":     account.VendorID,
		"availableUgx": account.Available,
		"reservedUgx":  account.Reserved,
		"vendorStatus": account.VendorStatus,
		"lowBalance":   account.LowBalance,
	}
}

func entryJSON(entry Entry) gin.H {
	body := gin.H{
		"id": entry.ID, "stream": entry.Stream, "direction": entry.Direction,
		"amountUgx": entry.AmountUGX, "operation": entry.Operation,
	}
	if entry.OrderID != uuid.Nil {
		body["orderId"] = entry.OrderID
	}
	return body
}

func topUpJSON(topUp TopUp) gin.H {
	return gin.H{
		"id": topUp.ID, "amountUgx": topUp.AmountUGX, "provider": topUp.Provider,
		"phone": topUp.Phone, "status": topUp.Status, "providerReference": topUp.Reference,
	}
}

func adjustmentJSON(adjustment Adjustment) gin.H {
	return gin.H{
		"id": adjustment.ID, "amountUgx": adjustment.AmountUGX, "reason": adjustment.Reason,
		"status": adjustment.Status,
	}
}
