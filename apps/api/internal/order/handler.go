package order

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/platform/httpx"
)

// Handler is the HTTP edge for customer orders.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c *gin.Context) {
	customer, ok := customerID(c)
	if !ok {
		return
	}
	var body struct {
		IdempotencyKey string `json:"idempotencyKey"`
		VendorID       string `json:"vendorId"`
		Fulfilment     string `json:"fulfilment"`
		Address        string `json:"address"`
		Phone          string `json:"phone"`
		Items          []struct {
			ItemID   string `json:"itemId"`
			Quantity int    `json:"quantity"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "The order is missing details.")
		return
	}
	vendorID, err := uuid.Parse(body.VendorID)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That kitchen is not valid.")
		return
	}
	lines := make([]Line, 0, len(body.Items))
	for _, item := range body.Items {
		itemID, err := uuid.Parse(item.ItemID)
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "That menu item is not valid.")
			return
		}
		lines = append(lines, Line{ItemID: itemID, Quantity: item.Quantity})
	}
	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		CustomerID: customer, VendorID: vendorID, IdempotencyKey: body.IdempotencyKey,
		Fulfilment: body.Fulfilment, Address: body.Address, Phone: body.Phone, Lines: lines,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"order": orderJSON(created)})
}

func (h *Handler) List(c *gin.Context) {
	customer, ok := customerID(c)
	if !ok {
		return
	}
	orders, err := h.svc.List(c.Request.Context(), customer)
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

func (h *Handler) Get(c *gin.Context) {
	customer, ok := customerID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	found, err := h.svc.Get(c.Request.Context(), customer, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(found)})
}

func (h *Handler) Cancel(c *gin.Context) {
	customer, ok := customerID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	cancelled, err := h.svc.Cancel(c.Request.Context(), customer, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": orderJSON(cancelled)})
}

func customerID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That order is not valid.")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the order and try again.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", "That order was not found.")
	case errors.Is(err, ErrForbidden):
		httpx.Error(c, http.StatusForbidden, "forbidden", "This order can no longer be cancelled.")
	case errors.Is(err, ErrConflict):
		httpx.Error(c, http.StatusConflict, "idempotency_conflict", "That idempotency key was already used for a different order.")
	case errors.Is(err, ErrClosed):
		httpx.Error(c, http.StatusConflict, "vendor_unavailable", "This kitchen is not taking orders right now.")
	default:
		httpx.Error(c, http.StatusInternalServerError, "order_failed", "The order did not finish.")
	}
}

func orderJSON(order Order) gin.H {
	items := make([]gin.H, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, gin.H{
			"itemId": item.MenuItemID, "name": item.Name, "quantity": item.Quantity,
			"unitPriceUgx": item.UnitPriceUGX, "subtotalUgx": item.SubtotalUGX,
		})
	}
	body := gin.H{
		"id": order.ID, "vendorId": order.VendorID, "status": order.Status,
		"fulfilment": order.Fulfilment, "address": order.DeliveryAddress, "phone": order.CustomerPhone,
		"foodSubtotalUgx": order.FoodSubtotalUGX, "deliveryFeeUgx": order.DeliveryFeeUGX,
		"totalUgx": order.TotalUGX, "pickupCode": order.PickupCode, "cancelRequested": order.CancelRequested,
		"items": items,
	}
	return body
}
