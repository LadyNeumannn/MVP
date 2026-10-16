package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

var (
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("unauthorized")
)

const maxQuantity = 10000

type Service struct {
	Store  storage.Storage
	Logger *log.Logger
}

func New(store storage.Storage, logger *log.Logger) *Service {
	return &Service{Store: store, Logger: logger}
}

func (s *Service) HealthCheck(ctx context.Context) error {
	return s.Store.Ping(ctx)
}

func (s *Service) RegisterRestaurant(ctx context.Context, id, name, webhookURL string) (*domain.Restaurant, bool, error) {
	if strings.TrimSpace(name) == "" {
		return nil, false, fmt.Errorf("%w: name is required", ErrValidation)
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, false, fmt.Errorf("generate api key: %w", err)
	}
	r := &domain.Restaurant{ID: id, Name: name, WebhookURL: webhookURL, APIKey: apiKey}
	created, err := s.Store.CreateRestaurant(ctx, r)
	if err != nil {
		return nil, false, err
	}
	return r, created, nil
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) VerifyRestaurantAPIKey(ctx context.Context, restaurantID, apiKey string) error {
	if apiKey == "" {
		return ErrUnauthorized
	}
	r, err := s.Store.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return ErrUnauthorized
	}
	if subtle.ConstantTimeCompare([]byte(r.APIKey), []byte(apiKey)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) GetRestaurant(ctx context.Context, id string) (*domain.Restaurant, error) {
	return s.Store.GetRestaurant(ctx, id)
}

func (s *Service) ListActiveRestaurants(ctx context.Context, query string) ([]domain.Restaurant, error) {
	return s.Store.ListActiveRestaurants(ctx, query)
}

func (s *Service) CreateMenuItem(ctx context.Context, restaurantID, id, name, description string, priceCents int64, available bool) (*domain.MenuItem, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if priceCents < 0 {
		return nil, fmt.Errorf("%w: price_cents must be >= 0", ErrValidation)
	}
	if _, err := s.Store.GetRestaurant(ctx, restaurantID); err != nil {
		return nil, err
	}

	item := &domain.MenuItem{
		ID:           id,
		RestaurantID: restaurantID,
		Name:         name,
		Description:  description,
		PriceCents:   priceCents,
		Available:    available,
	}
	if err := s.Store.CreateMenuItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateMenuItem(ctx context.Context, restaurantID, itemID string, priceCents *int64, available *bool) (*domain.MenuItem, error) {
	if priceCents != nil && *priceCents < 0 {
		return nil, fmt.Errorf("%w: price_cents must be >= 0", ErrValidation)
	}
	return s.Store.UpdateMenuItem(ctx, restaurantID, itemID, priceCents, available)
}

func (s *Service) ListMenuItems(ctx context.Context, restaurantID, query string) ([]domain.MenuItem, error) {
	return s.Store.ListMenuItems(ctx, restaurantID, query)
}

func (s *Service) CreateOrder(ctx context.Context, input storage.CreateOrderInput) (order *domain.Order, alreadyExisted bool, err error) {
	if strings.TrimSpace(input.CustomerName) == "" {
		return nil, false, fmt.Errorf("%w: customer_name is required", ErrValidation)
	}
	if strings.TrimSpace(input.CustomerPhone) == "" {
		return nil, false, fmt.Errorf("%w: customer_phone is required", ErrValidation)
	}
	if strings.TrimSpace(input.DeliveryAddress) == "" {
		return nil, false, fmt.Errorf("%w: delivery_address is required", ErrValidation)
	}
	if len(input.Items) == 0 {
		return nil, false, fmt.Errorf("%w: order must contain at least one item", ErrValidation)
	}
	for _, it := range input.Items {
		if it.Quantity < 1 {
			return nil, false, fmt.Errorf("%w: quantity must be positive", ErrValidation)
		}
		if it.Quantity > maxQuantity {
			return nil, false, fmt.Errorf("%w: quantity too large (max %d)", ErrValidation, maxQuantity)
		}
	}

	order, alreadyExisted, err = s.Store.CreateOrder(ctx, input)
	if err != nil {
		return nil, false, err
	}
	return order, alreadyExisted, nil
}

func (s *Service) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	return s.Store.GetOrder(ctx, id)
}

func (s *Service) ListRestaurantOrders(ctx context.Context, restaurantID string, status *domain.OrderStatus) ([]domain.Order, error) {
	return s.Store.ListRestaurantOrders(ctx, restaurantID, status)
}

func (s *Service) UpdateOrderStatus(ctx context.Context, restaurantID, orderID string, newStatus domain.OrderStatus) (*domain.Order, error) {
	switch newStatus {
	case domain.OrderStatusConfirmed, domain.OrderStatusPreparing, domain.OrderStatusReady,
		domain.OrderStatusDelivered, domain.OrderStatusCancelled:
	default:
		return nil, fmt.Errorf("%w: unknown status %q", ErrValidation, newStatus)
	}
	return s.Store.UpdateOrderStatus(ctx, restaurantID, orderID, newStatus)
}

func (s *Service) GetRestaurantStats(ctx context.Context, restaurantID string) (*domain.RestaurantStats, error) {
	return s.Store.GetRestaurantStats(ctx, restaurantID)
}
