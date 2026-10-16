package handler

import (
	"log"
	"net/http"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
)

type PartnerHandler struct {
	Service *service.Service
	Logger  *log.Logger
}

type registerRestaurantRequest struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	WebhookURL string `json:"webhook_url"`
}

type registerRestaurantResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsActive   bool   `json:"is_active"`
	WebhookURL string `json:"webhook_url,omitempty"`
	APIKey     string `json:"api_key"`
}

func (h *PartnerHandler) RegisterRestaurant(w http.ResponseWriter, r *http.Request) {
	var req registerRestaurantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	restaurant, created, err := h.Service.RegisterRestaurant(r.Context(), req.ID, req.Name, req.WebhookURL)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, registerRestaurantResponse{
		ID:         restaurant.ID,
		Name:       restaurant.Name,
		IsActive:   restaurant.IsActive,
		WebhookURL: restaurant.WebhookURL,
		APIKey:     restaurant.APIKey,
	})
}

type createMenuItemRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Available   *bool  `json:"available"`
}

func (h *PartnerHandler) CreateMenuItem(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")

	var req createMenuItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}
	available := true
	if req.Available != nil {
		available = *req.Available
	}

	item, err := h.Service.CreateMenuItem(r.Context(), restaurantID, req.ID, req.Name, req.Description, req.PriceCents, available)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *PartnerHandler) ListMenuItems(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")
	items, err := h.Service.ListMenuItems(r.Context(), restaurantID, "")
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	if items == nil {
		items = []domain.MenuItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

type updateMenuItemRequest struct {
	PriceCents *int64 `json:"price_cents"`
	Available  *bool  `json:"available"`
}

func (h *PartnerHandler) UpdateMenuItem(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")
	itemID := r.PathValue("itemId")

	var req updateMenuItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	item, err := h.Service.UpdateMenuItem(r.Context(), restaurantID, itemID, req.PriceCents, req.Available)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *PartnerHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")

	var statusFilter *domain.OrderStatus
	if raw := r.URL.Query().Get("status"); raw != "" {
		s := domain.OrderStatus(raw)
		statusFilter = &s
	}

	orders, err := h.Service.ListRestaurantOrders(r.Context(), restaurantID, statusFilter)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	if orders == nil {
		orders = []domain.Order{}
	}
	writeJSON(w, http.StatusOK, orders)
}

type updateOrderStatusRequest struct {
	Status string `json:"status"`
}

func (h *PartnerHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")
	orderID := r.PathValue("orderId")

	var req updateOrderStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	order, err := h.Service.UpdateOrderStatus(r.Context(), restaurantID, orderID, domain.OrderStatus(req.Status))
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (h *PartnerHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")
	stats, err := h.Service.GetRestaurantStats(r.Context(), restaurantID)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
