package ops

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/order"
	"beta-go/api/internal/platform/httpx"
)

// Handler is the HTTP edge for paid-order operations.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListVendor(c *gin.Context) {
	userID, ok := actorID(c)
	if !ok {
		return
	}
	orders, err := h.svc.ListVendor(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	out := make([]gin.H, 0, len(orders))
	for _, item := range orders {
		out = append(out, orderJSON(item))
	}
	c.JSON(http.StatusOK, gin.H{"orders": out})
}

func (h *Handler) GetVendor(c *gin.Context) {
	userID, orderID, ok := actorOrder(c)
	if !ok {
		return
	}
	found, err := h.svc.GetVendor(c.Request.Context(), userID, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func (h *Handler) Accept(c *gin.Context) {
	h.vendorAct(c, func(ctx userOrder) (order.Order, error) {
		return h.svc.Accept(c.Request.Context(), ctx.UserID, ctx.OrderID)
	})
}

func (h *Handler) Reject(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	h.vendorAct(c, func(ctx userOrder) (order.Order, error) {
		return h.svc.Reject(c.Request.Context(), ctx.UserID, ctx.OrderID, body.Reason)
	})
}

func (h *Handler) Preparing(c *gin.Context) {
	h.advance(c, order.StatusPreparing)
}

func (h *Handler) Ready(c *gin.Context) {
	h.advance(c, order.StatusReady)
}

func (h *Handler) Fulfilled(c *gin.Context) {
	h.advance(c, order.StatusFulfilled)
}

func (h *Handler) VendorCancel(c *gin.Context) {
	h.vendorAct(c, func(ctx userOrder) (order.Order, error) {
		return h.svc.VendorCancel(c.Request.Context(), ctx.UserID, ctx.OrderID)
	})
}

func (h *Handler) ApproveCancel(c *gin.Context) {
	h.vendorAct(c, func(ctx userOrder) (order.Order, error) {
		return h.svc.ApproveCustomerCancel(c.Request.Context(), ctx.UserID, ctx.OrderID)
	})
}

func (h *Handler) CustomerCancel(c *gin.Context) {
	userID, orderID, ok := actorOrder(c)
	if !ok {
		return
	}
	found, err := h.svc.CustomerCancel(c.Request.Context(), userID, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func (h *Handler) Confirm(c *gin.Context) {
	userID, orderID, ok := actorOrder(c)
	if !ok {
		return
	}
	found, err := h.svc.Confirm(c.Request.Context(), userID, orderID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func (h *Handler) OpenDispute(c *gin.Context) {
	userID, orderID, ok := actorOrder(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "A dispute needs a reason.")
		return
	}
	dispute, found, err := h.svc.OpenDispute(c.Request.Context(), userID, orderID, body.Reason)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"dispute": gin.H{"id": dispute.ID, "status": dispute.Status, "reason": dispute.Reason}, "order": orderJSON(found)})
}

func (h *Handler) ResolveDispute(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That order is not valid.")
		return
	}
	var body struct {
		Outcome    string `json:"outcome"`
		Resolution string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "A resolution needs an outcome.")
		return
	}
	found, err := h.svc.ResolveDispute(c.Request.Context(), actor, orderID, body.Outcome, body.Resolution)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func (h *Handler) Reconcile(c *gin.Context) {
	if err := h.svc.Reconcile(c.Request.Context()); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type userOrder struct {
	UserID  uuid.UUID
	OrderID uuid.UUID
}

func (h *Handler) advance(c *gin.Context, next string) {
	h.vendorAct(c, func(ctx userOrder) (order.Order, error) {
		return h.svc.Advance(c.Request.Context(), ctx.UserID, ctx.OrderID, next)
	})
}

func (h *Handler) vendorAct(c *gin.Context, fn func(userOrder) (order.Order, error)) {
	userID, orderID, ok := actorOrder(c)
	if !ok {
		return
	}
	found, err := fn(userOrder{UserID: userID, OrderID: orderID})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func actorID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func actorOrder(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := actorID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That order is not valid.")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, orderID, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the order and try again.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", "That order was not found.")
	case errors.Is(err, ErrForbidden):
		httpx.Error(c, http.StatusForbidden, "forbidden", "This order can no longer be changed that way.")
	case errors.Is(err, ErrClosed):
		httpx.Error(c, http.StatusConflict, "decision_expired", "The kitchen did not answer in time.")
	case errors.Is(err, ErrConflict):
		httpx.Error(c, http.StatusConflict, "order_conflict", "That order is not in the right state.")
	default:
		httpx.Error(c, http.StatusInternalServerError, "order_failed", "The order update did not finish.")
	}
}

func orderJSON(item order.Order) gin.H {
	return gin.H{
		"id": item.ID, "vendorId": item.VendorID, "customerId": item.CustomerID, "status": item.Status,
		"fulfilment": item.Fulfilment, "totalUgx": item.TotalUGX, "pickupCode": item.PickupCode,
		"cancelRequested": item.CancelRequested,
	}
}
