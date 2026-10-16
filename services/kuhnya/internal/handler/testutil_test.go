package handler_test

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

type fakeStorage struct {
	mu          sync.Mutex
	restaurants map[string]domain.Restaurant
	menuItems   map[string]domain.MenuItem
	orders      map[string]domain.Order
	nextID      int
}

// Ping implements [storage.Storage].
func (s *fakeStorage) Ping(ctx context.Context) error {
	panic("unimplemented")
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		restaurants: map[string]domain.Restaurant{},
		menuItems:   map[string]domain.MenuItem{},
		orders:      map[string]domain.Order{},
	}
}

func (s *fakeStorage) genID(prefix string) string {
	s.nextID++
	return prefix + "-" + strconv.Itoa(s.nextID)
}

func (s *fakeStorage) seedRestaurant(r domain.Restaurant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restaurants[r.ID] = r
}

func (s *fakeStorage) seedMenuItem(item domain.MenuItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.menuItems[item.ID] = item
}

func (s *fakeStorage) CreateRestaurant(ctx context.Context, r *domain.Restaurant) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = s.genID("rest")
	}
	if existing, ok := s.restaurants[r.ID]; ok {
		*r = existing
		return false, nil
	}
	r.IsActive = true
	s.restaurants[r.ID] = *r
	return true, nil
}

func (s *fakeStorage) GetRestaurant(ctx context.Context, id string) (*domain.Restaurant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.restaurants[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &r, nil
}

func (s *fakeStorage) ListActiveRestaurants(ctx context.Context, query string) ([]domain.Restaurant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Restaurant
	for _, r := range s.restaurants {
		if !r.IsActive {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(r.Name), strings.ToLower(query)) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *fakeStorage) CreateMenuItem(ctx context.Context, item *domain.MenuItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" {
		item.ID = s.genID("item")
	}
	s.menuItems[item.ID] = *item
	return nil
}

func (s *fakeStorage) UpdateMenuItem(ctx context.Context, restaurantID, itemID string, priceCents *int64, available *bool) (*domain.MenuItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.menuItems[itemID]
	if !ok || item.RestaurantID != restaurantID {
		return nil, storage.ErrNotFound
	}
	if priceCents != nil {
		item.PriceCents = *priceCents
	}
	if available != nil {
		item.Available = *available
	}
	s.menuItems[itemID] = item
	return &item, nil
}

func (s *fakeStorage) ListMenuItems(ctx context.Context, restaurantID, query string) ([]domain.MenuItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.MenuItem
	for _, item := range s.menuItems {
		if item.RestaurantID != restaurantID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(query)) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *fakeStorage) GetMenuItem(ctx context.Context, id string) (*domain.MenuItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.menuItems[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &item, nil
}

func (s *fakeStorage) CreateOrder(ctx context.Context, input storage.CreateOrderInput) (*domain.Order, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rest, ok := s.restaurants[input.RestaurantID]
	if !ok {
		return nil, false, storage.ErrNotFound
	}
	if !rest.IsActive {
		return nil, false, storage.ErrRestaurantInactive
	}

	var (
		total int64
		items []domain.OrderItem
	)
	for _, ri := range input.Items {
		item, ok := s.menuItems[ri.MenuItemID]
		if !ok {
			return nil, false, storage.ErrNotFound
		}
		if item.RestaurantID != input.RestaurantID {
			return nil, false, storage.ErrItemWrongRestaurant
		}
		if !item.Available {
			return nil, false, storage.ErrItemUnavailable
		}
		total += item.PriceCents * int64(ri.Quantity)
		items = append(items, domain.OrderItem{
			ID: s.genID("oi"), MenuItemID: item.ID, NameSnapshot: item.Name,
			PriceCentsSnapshot: item.PriceCents, Quantity: ri.Quantity,
		})
	}

	order := domain.Order{
		ID: s.genID("order"), RestaurantID: input.RestaurantID, CustomerName: input.CustomerName,
		CustomerPhone: input.CustomerPhone, DeliveryAddress: input.DeliveryAddress,
		Status: domain.OrderStatusCreated, TotalCents: total, Items: items,
	}
	s.orders[order.ID] = order
	return &order, false, nil
}

func (s *fakeStorage) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &o, nil
}

func (s *fakeStorage) ListRestaurantOrders(ctx context.Context, restaurantID string, status *domain.OrderStatus) ([]domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Order
	for _, o := range s.orders {
		if o.RestaurantID == restaurantID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *fakeStorage) UpdateOrderStatus(ctx context.Context, restaurantID, orderID string, newStatus domain.OrderStatus) (*domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderID]
	if !ok || o.RestaurantID != restaurantID {
		return nil, storage.ErrNotFound
	}
	if !domain.CanTransition(o.Status, newStatus) {
		return nil, storage.ErrInvalidTransition
	}
	o.Status = newStatus
	s.orders[orderID] = o
	return &o, nil
}

func (s *fakeStorage) GetRestaurantStats(ctx context.Context, restaurantID string) (*domain.RestaurantStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.restaurants[restaurantID]; !ok {
		return nil, storage.ErrNotFound
	}
	return &domain.RestaurantStats{RestaurantID: restaurantID, OrdersByStatus: map[domain.OrderStatus]int{}}, nil
}

func (s *fakeStorage) FetchPendingWebhooks(ctx context.Context, limit int) ([]storage.WebhookOutboxItem, error) {
	return nil, nil
}

func (s *fakeStorage) MarkWebhookDelivered(ctx context.Context, id string) error { return nil }

func (s *fakeStorage) MarkWebhookFailed(ctx context.Context, id, errMsg string, nextAttemptAt time.Time) error {
	return nil
}
