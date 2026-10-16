package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

func validOrderInput(restaurantID string, items []storage.RequestedItem, idempotencyKey *string) storage.CreateOrderInput {
	return storage.CreateOrderInput{
		RestaurantID:    restaurantID,
		CustomerName:    "Ариана",
		CustomerPhone:   "+79990000000",
		DeliveryAddress: "г. Нижний Новгород, ул. Тестовая, 1",
		Items:           items,
		IdempotencyKey:  idempotencyKey,
	}
}

func TestCreateOrder_ValidationErrors(t *testing.T) {
	svc := service.New(nil, nil)

	cases := []struct {
		name  string
		input storage.CreateOrderInput
	}{
		{
			"пустое имя покупателя",
			storage.CreateOrderInput{CustomerName: "", CustomerPhone: "+7999", DeliveryAddress: "адрес", Items: []storage.RequestedItem{{MenuItemID: "x", Quantity: 1}}},
		},
		{
			"пустой телефон",
			storage.CreateOrderInput{CustomerName: "Ариана", CustomerPhone: "", DeliveryAddress: "адрес", Items: []storage.RequestedItem{{MenuItemID: "x", Quantity: 1}}},
		},
		{
			"пустой адрес доставки",
			storage.CreateOrderInput{CustomerName: "Ариана", CustomerPhone: "+7999", DeliveryAddress: "", Items: []storage.RequestedItem{{MenuItemID: "x", Quantity: 1}}},
		},
		{
			"пустой список позиций",
			storage.CreateOrderInput{CustomerName: "Ариана", CustomerPhone: "+7999", DeliveryAddress: "адрес", Items: nil},
		},
		{
			"неположительное количество",
			storage.CreateOrderInput{CustomerName: "Ариана", CustomerPhone: "+7999", DeliveryAddress: "адрес", Items: []storage.RequestedItem{{MenuItemID: "x", Quantity: 0}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.RestaurantID = "rest-1"
			_, _, err := svc.CreateOrder(context.Background(), tc.input)
			if !errors.Is(err, service.ErrValidation) {
				t.Fatalf("ожидалась ErrValidation, получено: %v", err)
			}
		})
	}
}

func setupServiceWithMenu(t *testing.T) (*service.Service, *fakeStorage, domain.Restaurant, domain.MenuItem) {
	t.Helper()
	store := newFakeStorage()

	restaurant := domain.Restaurant{ID: "rest-1", Name: "Тест", IsActive: true}
	store.seedRestaurant(restaurant)

	item := domain.MenuItem{ID: "item-1", RestaurantID: "rest-1", Name: "Пельмени", PriceCents: 30000, Available: true}
	store.seedMenuItem(item)

	svc := service.New(store, nil)
	return svc, store, restaurant, item
}

func TestCreateOrder_Success(t *testing.T) {
	svc, _, _, item := setupServiceWithMenu(t)

	input := validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 2}}, nil)
	order, alreadyExisted, err := svc.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if alreadyExisted {
		t.Fatal("новый заказ не должен считаться уже существующим")
	}
	if order.Status != domain.OrderStatusCreated {
		t.Errorf("статус = %q, ожидался %q", order.Status, domain.OrderStatusCreated)
	}
	if order.DeliveryAddress != input.DeliveryAddress || order.CustomerPhone != input.CustomerPhone {
		t.Errorf("адрес/телефон не сохранились: %+v", order)
	}
	wantTotal := item.PriceCents * 2
	if order.TotalCents != wantTotal {
		t.Errorf("сумма = %d, ожидалась %d", order.TotalCents, wantTotal)
	}
	if len(order.Items) != 1 || order.Items[0].NameSnapshot != item.Name {
		t.Errorf("неверный снапшот позиций заказа: %+v", order.Items)
	}
}

func TestCreateOrder_ItemUnavailable(t *testing.T) {
	svc, store, _, item := setupServiceWithMenu(t)
	item.Available = false
	store.seedMenuItem(item)

	input := validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil)
	_, _, err := svc.CreateOrder(context.Background(), input)
	if !errors.Is(err, storage.ErrItemUnavailable) {
		t.Fatalf("ожидалась ErrItemUnavailable, получено: %v", err)
	}
}

func TestCreateOrder_RestaurantInactive(t *testing.T) {
	svc, store, restaurant, item := setupServiceWithMenu(t)
	restaurant.IsActive = false
	store.seedRestaurant(restaurant)

	input := validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil)
	_, _, err := svc.CreateOrder(context.Background(), input)
	if !errors.Is(err, storage.ErrRestaurantInactive) {
		t.Fatalf("ожидалась ErrRestaurantInactive, получено: %v", err)
	}
}

func TestCreateOrder_WrongRestaurant(t *testing.T) {
	svc, store, _, item := setupServiceWithMenu(t)
	store.seedRestaurant(domain.Restaurant{ID: "rest-2", Name: "Другое", IsActive: true})

	input := validOrderInput("rest-2", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil)
	_, _, err := svc.CreateOrder(context.Background(), input)
	if !errors.Is(err, storage.ErrItemWrongRestaurant) {
		t.Fatalf("ожидалась ErrItemWrongRestaurant, получено: %v", err)
	}
}

func TestCreateOrder_IdempotencyKeyReplaysOrder(t *testing.T) {
	svc, _, _, item := setupServiceWithMenu(t)
	key := "test-key-1"

	first, alreadyExisted1, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, &key))
	if err != nil {
		t.Fatalf("неожиданная ошибка при первом создании: %v", err)
	}
	if alreadyExisted1 {
		t.Fatal("первый вызов не должен считаться повтором")
	}

	second, alreadyExisted2, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 5}}, &key))
	if err != nil {
		t.Fatalf("неожиданная ошибка при повторном создании: %v", err)
	}
	if !alreadyExisted2 {
		t.Fatal("повторный вызов с тем же Idempotency-Key должен вернуть существующий заказ")
	}
	if second.ID != first.ID {
		t.Errorf("повторный вызов вернул другой заказ: %s != %s", second.ID, first.ID)
	}
}

func TestUpdateOrderStatus_InvalidTransition(t *testing.T) {
	svc, _, _, item := setupServiceWithMenu(t)

	order, _, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка при создании заказа: %v", err)
	}
	_, err = svc.UpdateOrderStatus(context.Background(), "rest-1", order.ID, domain.OrderStatusDelivered)
	if !errors.Is(err, storage.ErrInvalidTransition) {
		t.Fatalf("ожидалась ErrInvalidTransition, получено: %v", err)
	}
}

func TestUpdateOrderStatus_ValidTransition(t *testing.T) {
	svc, _, _, item := setupServiceWithMenu(t)

	order, _, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка при создании заказа: %v", err)
	}

	updated, err := svc.UpdateOrderStatus(context.Background(), "rest-1", order.ID, domain.OrderStatusConfirmed)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if updated.Status != domain.OrderStatusConfirmed {
		t.Errorf("статус = %q, ожидался %q", updated.Status, domain.OrderStatusConfirmed)
	}
}

func TestGetRestaurantStats(t *testing.T) {
	svc, _, _, item := setupServiceWithMenu(t)

	delivered, _, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 2}}, nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	for _, status := range []domain.OrderStatus{domain.OrderStatusConfirmed, domain.OrderStatusPreparing, domain.OrderStatusReady, domain.OrderStatusDelivered} {
		if _, err = svc.UpdateOrderStatus(context.Background(), "rest-1", delivered.ID, status); err != nil {
			t.Fatalf("неожиданная ошибка при переходе в %q: %v", status, err)
		}
	}

	if _, _, err = svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	stats, err := svc.GetRestaurantStats(context.Background(), "rest-1")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if stats.TotalOrders != 2 {
		t.Errorf("TotalOrders = %d, ожидалось 2", stats.TotalOrders)
	}
	wantRevenue := item.PriceCents * 2
	if stats.RevenueCents != wantRevenue {
		t.Errorf("RevenueCents = %d, ожидалось %d (только delivered)", stats.RevenueCents, wantRevenue)
	}
	if stats.AvgOrderValueCents != wantRevenue {
		t.Errorf("AvgOrderValueCents = %d, ожидалось %d (один delivered заказ)", stats.AvgOrderValueCents, wantRevenue)
	}
	if stats.OrdersByStatus[domain.OrderStatusCreated] != 1 || stats.OrdersByStatus[domain.OrderStatusDelivered] != 1 {
		t.Errorf("OrdersByStatus некорректен: %+v", stats.OrdersByStatus)
	}
}

func TestListActiveRestaurants_Search(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{ID: "rest-1", Name: "Пельменная у Ашота", IsActive: true})
	store.seedRestaurant(domain.Restaurant{ID: "rest-2", Name: "Суши-бар Токио", IsActive: true})
	svc := service.New(store, nil)

	found, err := svc.ListActiveRestaurants(context.Background(), "пельм")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(found) != 1 || found[0].ID != "rest-1" {
		t.Errorf("поиск по подстроке 'пельм' должен найти только rest-1, получено: %+v", found)
	}

	all, err := svc.ListActiveRestaurants(context.Background(), "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("без фильтра должны вернуться оба заведения, получено: %+v", all)
	}
}

func TestRegisterRestaurant_GeneratesAndPersistsAPIKey(t *testing.T) {
	store := newFakeStorage()
	svc := service.New(store, nil)

	first, created, err := svc.RegisterRestaurant(context.Background(), "rest-1", "Тест", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !created {
		t.Fatal("первая регистрация должна считаться созданием")
	}
	if first.APIKey == "" {
		t.Fatal("api_key не сгенерирован")
	}

	second, created2, err := svc.RegisterRestaurant(context.Background(), "rest-1", "Тест", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка при повторной регистрации: %v", err)
	}
	if created2 {
		t.Fatal("повторная регистрация не должна считаться созданием")
	}
	if second.APIKey != first.APIKey {
		t.Errorf("api_key изменился при повторной регистрации: %q != %q", second.APIKey, first.APIKey)
	}
}

func TestVerifyRestaurantAPIKey(t *testing.T) {
	store := newFakeStorage()
	svc := service.New(store, nil)

	restaurant, _, err := svc.RegisterRestaurant(context.Background(), "rest-1", "Тест", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	restaurant2, _, err := svc.RegisterRestaurant(context.Background(), "rest-2", "Другое", "")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	cases := []struct {
		name         string
		restaurantID string
		apiKey       string
		wantErr      bool
	}{
		{"верный ключ своего заведения", "rest-1", restaurant.APIKey, false},
		{"пустой ключ", "rest-1", "", true},
		{"неверный ключ", "rest-1", "совсем-не-тот-ключ", true},
		{"ключ другого заведения не подходит", "rest-1", restaurant2.APIKey, true},
		{"несуществующее заведение", "rest-does-not-exist", restaurant.APIKey, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.VerifyRestaurantAPIKey(context.Background(), tc.restaurantID, tc.apiKey)
			if tc.wantErr && !errors.Is(err, service.ErrUnauthorized) {
				t.Errorf("ожидалась ErrUnauthorized, получено: %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("неожиданная ошибка: %v", err)
			}
		})
	}
}

func TestCreateMenuItem_ValidationErrors(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{ID: "rest-1", Name: "Тест", IsActive: true})
	svc := service.New(store, nil)

	if _, err := svc.CreateMenuItem(context.Background(), "rest-1", "", "", "", 10000, true); !errors.Is(err, service.ErrValidation) {
		t.Errorf("пустое имя: ожидалась ErrValidation, получено: %v", err)
	}
	if _, err := svc.CreateMenuItem(context.Background(), "rest-1", "", "Позиция", "", -100, true); !errors.Is(err, service.ErrValidation) {
		t.Errorf("отрицательная цена: ожидалась ErrValidation, получено: %v", err)
	}
}
