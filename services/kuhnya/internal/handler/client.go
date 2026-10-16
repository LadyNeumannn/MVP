package handler

import (
	"log"
	"net/http"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

type ClientHandler struct {
	Service *service.Service
	Logger  *log.Logger
}

func (h *ClientHandler) ListRestaurants(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	restaurants, err := h.Service.ListActiveRestaurants(r.Context(), query)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	if restaurants == nil {
		restaurants = []domain.Restaurant{}
	}
	writeJSON(w, http.StatusOK, restaurants)
}

func (h *ClientHandler) GetMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID := r.PathValue("restaurantId")
	if _, err := h.Service.GetRestaurant(r.Context(), restaurantID); err != nil {
		writeError(w, h.Logger, err)
		return
	}

	query := r.URL.Query().Get("q")
	items, err := h.Service.ListMenuItems(r.Context(), restaurantID, query)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	if items == nil {
		items = []domain.MenuItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

type createOrderItemRequest struct {
	MenuItemID string `json:"menu_item_id"`
	Quantity   int    `json:"quantity"`
}

type createOrderRequest struct {
	RestaurantID    string                   `json:"restaurant_id"`
	CustomerName    string                   `json:"customer_name"`
	CustomerPhone   string                   `json:"customer_phone"`
	DeliveryAddress string                   `json:"delivery_address"`
	Items           []createOrderItemRequest `json:"items"`
}

func (h *ClientHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	items := make([]storage.RequestedItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, storage.RequestedItem{MenuItemID: it.MenuItemID, Quantity: it.Quantity})
	}

	var idempotencyKey *string
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		idempotencyKey = &key
	}

	input := storage.CreateOrderInput{
		RestaurantID:    req.RestaurantID,
		CustomerName:    req.CustomerName,
		CustomerPhone:   req.CustomerPhone,
		DeliveryAddress: req.DeliveryAddress,
		Items:           items,
		IdempotencyKey:  idempotencyKey,
	}

	order, alreadyExisted, err := h.Service.CreateOrder(r.Context(), input)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}

	status := http.StatusCreated
	if alreadyExisted {
		status = http.StatusOK
	}
	writeJSON(w, status, order)
}

func (h *ClientHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	order, err := h.Service.GetOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, h.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}
