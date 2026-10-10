package vendor

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/httpx"
)

// SessionStarter issues a fresh session after a role change.
type SessionStarter interface {
	StartSession(c *gin.Context, userID uuid.UUID) error
}

type authStarter struct {
	svc *auth.Service
	cfg config.Config
}

func (a authStarter) StartSession(c *gin.Context, userID uuid.UUID) error {
	signedIn, err := a.svc.StartSession(c.Request.Context(), userID, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		return err
	}
	auth.WriteSessionCookies(c, a.cfg, signedIn)
	return nil
}

// Handler is the HTTP edge for vendor onboarding.
type Handler struct {
	svc     *Service
	starter SessionStarter
}

func NewHandler(svc *Service, sessions *auth.Service, cfg config.Config) *Handler {
	return &Handler{svc: svc, starter: authStarter{svc: sessions, cfg: cfg}}
}

func (h *Handler) PublicList(c *gin.Context) {
	vendors, err := h.svc.ListVisible(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	out := make([]gin.H, 0, len(vendors))
	for _, item := range vendors {
		payload := publicVendorJSON(item)
		shop, shopErr := h.svc.PublicShop(c.Request.Context(), item.ID)
		dishes := []gin.H{}
		if shopErr == nil {
			dishes = featuredDishes(shop.Items, 5)
		}
		payload["dishes"] = dishes
		out = append(out, payload)
	}
	c.JSON(http.StatusOK, gin.H{"vendors": out})
}

func (h *Handler) PublicGet(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	shop, err := h.svc.PublicShop(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, publicShopJSON(shop))
}

func (h *Handler) Create(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	var body struct {
		BusinessName string `json:"businessName"`
		ContactName  string `json:"contactName"`
		ContactEmail string `json:"contactEmail"`
		Phone        string `json:"phone"`
		Address      string `json:"address"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Vendor details are missing.")
		return
	}
	created, err := h.svc.Create(c.Request.Context(), actor, CreateInput{
		BusinessName: body.BusinessName,
		ContactName:  body.ContactName,
		ContactEmail: body.ContactEmail,
		Phone:        body.Phone,
		Address:      body.Address,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"vendor": vendorJSON(created)})
}

func (h *Handler) List(c *gin.Context) {
	vendors, err := h.svc.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	out := make([]vendorResponse, 0, len(vendors))
	for _, item := range vendors {
		out = append(out, vendorJSON(item))
	}
	c.JSON(http.StatusOK, gin.H{"vendors": out})
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	vendor, destination, hasDestination, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	payload := gin.H{"vendor": vendorJSON(vendor)}
	if hasDestination {
		payload["destination"] = destinationJSON(destination)
	}
	if gaps, err := h.svc.GoLiveGaps(c.Request.Context(), id); err == nil {
		payload["missing"] = gaps
	}
	c.JSON(http.StatusOK, payload)
}

func (h *Handler) Update(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body struct {
		BusinessName string `json:"businessName"`
		ContactName  string `json:"contactName"`
		Phone        string `json:"phone"`
		Address      string `json:"address"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Vendor details are missing.")
		return
	}
	updated, err := h.svc.UpdateIdentity(c.Request.Context(), actor, id, body.BusinessName, body.ContactName, body.Phone, body.Address)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vendor": vendorJSON(updated)})
}

func (h *Handler) Invite(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	token, expires, err := h.svc.Invite(c.Request.Context(), actor, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "expiresAt": expires})
}

func (h *Handler) SetDestination(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		Account  string `json:"account"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Payment destination is missing.")
		return
	}
	destination, err := h.svc.SetDestination(c.Request.Context(), actor, id, body.Provider, body.Account)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"destination": destinationJSON(destination)})
}

func (h *Handler) VerifyDestination(c *gin.Context) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	destination, err := h.svc.VerifyDestination(c.Request.Context(), actor, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"destination": destinationJSON(destination)})
}

func (h *Handler) Approve(c *gin.Context) {
	h.status(c, func(actor, id uuid.UUID) (Vendor, error) {
		return h.svc.Approve(c.Request.Context(), actor, id)
	})
}

func (h *Handler) Reject(c *gin.Context) {
	h.statusWithReason(c, func(actor, id uuid.UUID, reason string) (Vendor, error) {
		return h.svc.Reject(c.Request.Context(), actor, id, reason)
	})
}

func (h *Handler) Pause(c *gin.Context) {
	h.statusWithReason(c, func(actor, id uuid.UUID, reason string) (Vendor, error) {
		return h.svc.Pause(c.Request.Context(), actor, id, reason)
	})
}

func (h *Handler) Suspend(c *gin.Context) {
	h.statusWithReason(c, func(actor, id uuid.UUID, reason string) (Vendor, error) {
		return h.svc.Suspend(c.Request.Context(), actor, id, reason)
	})
}

func (h *Handler) Reactivate(c *gin.Context) {
	h.status(c, func(actor, id uuid.UUID) (Vendor, error) {
		return h.svc.Reactivate(c.Request.Context(), actor, id)
	})
}

func (h *Handler) AcceptInvitation(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Token == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Invitation token is missing.")
		return
	}
	if err := h.svc.Accept(c.Request.Context(), principal.UserID, body.Token); err != nil {
		writeError(c, err)
		return
	}
	if err := h.starter.StartSession(c, principal.UserID); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accepted": true})
}

func (h *Handler) GetShop(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	shop, err := h.svc.Shop(c.Request.Context(), principal.UserID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, shopJSON(shop))
}

func (h *Handler) UpdateShop(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	var body struct {
		Description     string `json:"description"`
		LogoURL         string `json:"logoUrl"`
		ShopPhotoURL    string `json:"shopPhotoUrl"`
		DeliveryFeeUGX  int    `json:"deliveryFeeUgx"`
		IsOpen          bool   `json:"isOpen"`
		AcceptingOrders bool   `json:"acceptingOrders"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Shop details are missing.")
		return
	}
	updated, err := h.svc.UpdateShop(c.Request.Context(), principal.UserID, body.Description, body.LogoURL, body.ShopPhotoURL, body.DeliveryFeeUGX, body.IsOpen, body.AcceptingOrders)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vendor": vendorJSON(updated)})
}

func (h *Handler) ReplaceHours(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	var body struct {
		Hours []struct {
			Weekday  int    `json:"weekday"`
			OpensAt  string `json:"opensAt"`
			ClosesAt string `json:"closesAt"`
			Closed   bool   `json:"closed"`
		} `json:"hours"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Opening hours are missing.")
		return
	}
	hours := make([]Hours, 0, len(body.Hours))
	for _, day := range body.Hours {
		hours = append(hours, Hours{Weekday: day.Weekday, OpensAt: day.OpensAt, ClosesAt: day.ClosesAt, Closed: day.Closed})
	}
	if err := h.svc.ReplaceHours(c.Request.Context(), principal.UserID, hours); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) AddCategory(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Category name is missing.")
		return
	}
	category, err := h.svc.AddCategory(c.Request.Context(), principal.UserID, body.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": gin.H{"id": category.ID, "name": category.Name}})
}

func (h *Handler) AddItem(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	var body struct {
		Name        string    `json:"name"`
		Description string    `json:"description"`
		PriceUGX    int       `json:"priceUgx"`
		CategoryID  uuid.UUID `json:"categoryId"`
		PhotoURL    string    `json:"photoUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Menu item is missing.")
		return
	}
	item, err := h.svc.AddItem(c.Request.Context(), principal.UserID, body.Name, body.Description, body.PriceUGX, body.CategoryID, body.PhotoURL)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": itemJSON(item)})
}

func (h *Handler) UpdateItem(c *gin.Context) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceUGX    int    `json:"priceUgx"`
		Available   bool   `json:"available"`
		PhotoURL    string `json:"photoUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Menu item is missing.")
		return
	}
	item, err := h.svc.UpdateItem(c.Request.Context(), principal.UserID, id, body.Name, body.Description, body.PriceUGX, body.Available, body.PhotoURL)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": itemJSON(item)})
}

func (h *Handler) status(c *gin.Context, fn func(uuid.UUID, uuid.UUID) (Vendor, error)) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	vendor, err := fn(actor, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vendor": vendorJSON(vendor)})
}

func (h *Handler) statusWithReason(c *gin.Context, fn func(contextActor, id uuid.UUID, reason string) (Vendor, error)) {
	actor, ok := actorID(c)
	if !ok {
		return
	}
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	vendor, err := fn(actor, id, body.Reason)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"vendor": vendorJSON(vendor)})
}

func actorID(c *gin.Context) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFrom(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Sign in to continue.")
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func pathID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "That id is not valid.")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBadInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Check the details and try again.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(c, http.StatusNotFound, "not_found", "That record was not found.")
	case errors.Is(err, ErrForbidden):
		httpx.Error(c, http.StatusForbidden, "forbidden", "That change is not allowed from the current state.")
	case errors.Is(err, ErrInvitation):
		httpx.Error(c, http.StatusForbidden, "invitation_invalid", "This invitation cannot be used.")
	case errors.Is(err, ErrInviteEmail):
		httpx.Error(c, http.StatusForbidden, "invitation_email", "Sign in with the Google email this kitchen was created for.")
	case errors.Is(err, ErrInviteGone):
		httpx.Error(c, http.StatusForbidden, "invitation_used", "This invitation was already used or has expired. Create a new one.")
	default:
		var notReady NotReadyError
		if errors.As(err, &notReady) {
			httpx.Error(c, http.StatusConflict, "not_ready", err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "vendor_failed", "The vendor action did not finish.")
	}
}

type vendorResponse struct {
	ID              string `json:"id"`
	BusinessName    string `json:"businessName"`
	ContactName     string `json:"contactName"`
	ContactEmail    string `json:"contactEmail"`
	Phone           string `json:"phone"`
	Address         string `json:"address"`
	Description     string `json:"description"`
	LogoURL         string `json:"logoUrl"`
	ShopPhotoURL    string `json:"shopPhotoUrl"`
	Status          string `json:"status"`
	IsOpen          bool   `json:"isOpen"`
	AcceptingOrders bool   `json:"acceptingOrders"`
	DeliveryFeeUGX  int    `json:"deliveryFeeUgx"`
}

func vendorJSON(vendor Vendor) vendorResponse {
	return vendorResponse{
		ID: vendor.ID.String(), BusinessName: vendor.BusinessName, ContactName: vendor.ContactName,
		ContactEmail: vendor.ContactEmail, Phone: vendor.Phone, Address: vendor.Address,
		Description: vendor.Description, LogoURL: vendor.LogoURL, ShopPhotoURL: vendor.ShopPhotoURL,
		Status: vendor.Status, IsOpen: vendor.IsOpen, AcceptingOrders: vendor.AcceptingOrders,
		DeliveryFeeUGX: vendor.DeliveryFeeUGX,
	}
}

func destinationJSON(destination Destination) gin.H {
	return gin.H{"provider": destination.Provider, "account": destination.Account, "status": destination.Status}
}

func dishMatches(name, key string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if key == "chips" {
		return n == "chips" || strings.HasPrefix(n, "chips ")
	}
	return strings.Contains(n, key)
}

func featuredDishes(items []Item, limit int) []gin.H {
	keys := []string{"chips", "fried chicken", "pork", "burger", "pizza"}
	used := map[uuid.UUID]bool{}
	out := make([]gin.H, 0, limit)
	pick := func(item Item) {
		if !item.Available || item.PhotoURL == "" || used[item.ID] || len(out) >= limit {
			return
		}
		used[item.ID] = true
		out = append(out, itemJSON(item))
	}
	for _, key := range keys {
		for _, item := range items {
			if !dishMatches(item.Name, key) || !item.Available || item.PhotoURL == "" {
				continue
			}
			pick(item)
			break
		}
	}
	for _, item := range items {
		pick(item)
	}
	return out
}

func itemJSON(item Item) gin.H {
	return gin.H{
		"id": item.ID, "name": item.Name, "description": item.Description,
		"priceUgx": item.PriceUGX, "available": item.Available, "photoUrl": item.PhotoURL,
	}
}

func publicVendorJSON(vendor Vendor) gin.H {
	return gin.H{
		"id": vendor.ID, "businessName": vendor.BusinessName, "description": vendor.Description,
		"address": vendor.Address, "shopPhotoUrl": vendor.ShopPhotoURL, "logoUrl": vendor.LogoURL,
		"deliveryFeeUgx": vendor.DeliveryFeeUGX, "isOpen": vendor.IsOpen, "acceptingOrders": vendor.AcceptingOrders,
	}
}

func publicShopJSON(shop Shop) gin.H {
	items := make([]gin.H, 0, len(shop.Items))
	for _, item := range shop.Items {
		items = append(items, gin.H{
			"id": item.ID, "name": item.Name, "description": item.Description,
			"priceUgx": item.PriceUGX, "available": item.Available, "photoUrl": item.PhotoURL,
		})
	}
	hours := make([]gin.H, 0, len(shop.Hours))
	for _, day := range shop.Hours {
		hours = append(hours, gin.H{"weekday": day.Weekday, "opensAt": day.OpensAt, "closesAt": day.ClosesAt, "closed": day.Closed})
	}
	payload := publicVendorJSON(shop.Vendor)
	return gin.H{"vendor": payload, "hours": hours, "items": items}
}

func shopJSON(shop Shop) gin.H {
	items := make([]gin.H, 0, len(shop.Items))
	for _, item := range shop.Items {
		items = append(items, itemJSON(item))
	}
	hours := make([]gin.H, 0, len(shop.Hours))
	for _, day := range shop.Hours {
		hours = append(hours, gin.H{"weekday": day.Weekday, "opensAt": day.OpensAt, "closesAt": day.ClosesAt, "closed": day.Closed})
	}
	categories := make([]gin.H, 0, len(shop.Categories))
	for _, category := range shop.Categories {
		categories = append(categories, gin.H{"id": category.ID, "name": category.Name})
	}
	return gin.H{"vendor": vendorJSON(shop.Vendor), "hours": hours, "categories": categories, "items": items}
}
