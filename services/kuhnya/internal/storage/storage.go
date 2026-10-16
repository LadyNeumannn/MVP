package storage

import (
	"context"
	"errors"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrItemUnavailable     = errors.New("menu item unavailable")
	ErrItemWrongRestaurant = errors.New("menu item belongs to another restaurant")
	ErrRestaurantInactive  = errors.New("restaurant is not active")
	ErrInvalidTransition   = errors.New("invalid order status transition")
)

type Storage interface {
	CreateRestaurant(ctx context.Context, r *domain.Restaurant) (created bool, err error)
	GetRestaurant(ctx context.Context, id string) (*domain.Restaurant, error)
	ListActiveRestaurants(ctx context.Context, query string) ([]domain.Restaurant, error)
	CreateMenuItem(ctx context.Context, item *domain.MenuItem) error
	UpdateMenuItem(ctx context.Context, restaurantID, itemID string, priceCents *int64, available *bool) (*domain.MenuItem, error)
	ListMenuItems(ctx context.Context, restaurantID, query string) ([]domain.MenuItem, error)
	GetMenuItem(ctx context.Context, id string) (*domain.MenuItem, error)
	CreateOrder(ctx context.Context, input CreateOrderInput) (order *domain.Order, alreadyExisted bool, err error)
	GetOrder(ctx context.Context, id string) (*domain.Order, error)
	ListRestaurantOrders(ctx context.Context, restaurantID string, status *domain.OrderStatus) ([]domain.Order, error)
	UpdateOrderStatus(ctx context.Context, restaurantID, orderID string, newStatus domain.OrderStatus) (*domain.Order, error)
	GetRestaurantStats(ctx context.Context, restaurantID string) (*domain.RestaurantStats, error)
	FetchPendingWebhooks(ctx context.Context, limit int) ([]WebhookOutboxItem, error)
	MarkWebhookDelivered(ctx context.Context, id string) error
	MarkWebhookFailed(ctx context.Context, id, errMsg string, nextAttemptAt time.Time) error

	Ping(ctx context.Context) error
}

type WebhookOutboxItem struct {
	ID         string
	OrderID    string
	WebhookURL string
	Payload    []byte
	Attempts   int
}

type RequestedItem struct {
	MenuItemID string
	Quantity   int
}

type CreateOrderInput struct {
	RestaurantID    string
	CustomerName    string
	CustomerPhone   string
	DeliveryAddress string
	Items           []RequestedItem
	IdempotencyKey  *string
}
