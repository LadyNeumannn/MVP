package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан, значит пропускаем интеграционные тесты (см. README)")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()

	tables := []string{
		"webhook_outbox",
		"idempotency_keys",
		"order_items",
		"orders",
		"menu_items",
		"restaurants",
	}

	for _, tbl := range tables {
		if _, err := db.ExecContext(context.Background(), "DELETE FROM "+tbl); err != nil {
			t.Fatalf("clean table %s: %v", tbl, err)
		}
	}
}

func seedRestaurant(t *testing.T, store *storage.PostgresStorage, r *domain.Restaurant) {
	t.Helper()

	if _, err := store.CreateRestaurant(context.Background(), r); err != nil {
		t.Fatalf("seed restaurant: %v", err)
	}
}

func seedMenuItem(t *testing.T, store *storage.PostgresStorage, item *domain.MenuItem) {
	t.Helper()

	if err := store.CreateMenuItem(context.Background(), item); err != nil {
		t.Fatalf("seed menu item: %v", err)
	}
}

func TestPostgres_CreateOrder_Success(t *testing.T) {
	db := setupDB(t)
	cleanTables(t, db)

	store := storage.NewPostgresStorage(db)

	restaurant := &domain.Restaurant{
		ID:     "11111111-1111-1111-1111-111111111111",
		Name:   "Тест",
		APIKey: "key1",
	}
	seedRestaurant(t, store, restaurant)

	item := &domain.MenuItem{
		ID:           "22222222-2222-2222-2222-222222222222",
		RestaurantID: restaurant.ID,
		Name:         "Пельмени",
		PriceCents:   30000,
		Available:    true,
	}
	seedMenuItem(t, store, item)

	order, alreadyExisted, err := store.CreateOrder(
		context.Background(),
		storage.CreateOrderInput{
			RestaurantID:    restaurant.ID,
			CustomerName:    "Ариана",
			CustomerPhone:   "+7999",
			DeliveryAddress: "Адрес",
			Items: []storage.RequestedItem{
				{
					MenuItemID: item.ID,
					Quantity:   2,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if alreadyExisted {
		t.Fatal("новый заказ не должен считаться уже существующим")
	}

	if order.TotalCents != 60000 {
		t.Errorf("сумма = %d, ожидалось 60000", order.TotalCents)
	}

	fetched, err := store.GetOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if len(fetched.Items) != 1 || fetched.Items[0].Quantity != 2 {
		t.Errorf("позиции заказа не сохранились корректно: %+v", fetched.Items)
	}
}

func TestPostgres_CreateOrder_UnavailableItem(t *testing.T) {
	db := setupDB(t)
	cleanTables(t, db)

	store := storage.NewPostgresStorage(db)

	restaurant := &domain.Restaurant{
		ID:     "11111111-1111-1111-1111-111111111112",
		Name:   "Тест",
		APIKey: "key1",
	}
	seedRestaurant(t, store, restaurant)

	item := &domain.MenuItem{
		ID:           "22222222-2222-2222-2222-222222222223",
		RestaurantID: restaurant.ID,
		Name:         "Компот",
		PriceCents:   12000,
		Available:    false,
	}
	seedMenuItem(t, store, item)

	_, _, err := store.CreateOrder(
		context.Background(),
		storage.CreateOrderInput{
			RestaurantID:    restaurant.ID,
			CustomerName:    "Ариана",
			CustomerPhone:   "+7999",
			DeliveryAddress: "Адрес",
			Items: []storage.RequestedItem{
				{
					MenuItemID: item.ID,
					Quantity:   1,
				},
			},
		},
	)

	if !errors.Is(err, storage.ErrItemUnavailable) {
		t.Fatalf("ожидалась ErrItemUnavailable, получено: %v", err)
	}
}

func TestPostgres_CreateOrder_IdempotencyKey(t *testing.T) {
	db := setupDB(t)
	cleanTables(t, db)

	store := storage.NewPostgresStorage(db)

	restaurant := &domain.Restaurant{
		ID:     "11111111-1111-1111-1111-111111111113",
		Name:   "Тест",
		APIKey: "key1",
	}
	seedRestaurant(t, store, restaurant)

	item := &domain.MenuItem{
		ID:           "22222222-2222-2222-2222-222222222224",
		RestaurantID: restaurant.ID,
		Name:         "Пельмени",
		PriceCents:   30000,
		Available:    true,
	}
	seedMenuItem(t, store, item)

	key := "test-idem-key"

	input := storage.CreateOrderInput{
		RestaurantID:    restaurant.ID,
		CustomerName:    "Ариана",
		CustomerPhone:   "+7999",
		DeliveryAddress: "Адрес",
		Items: []storage.RequestedItem{
			{
				MenuItemID: item.ID,
				Quantity:   1,
			},
		},
		IdempotencyKey: &key,
	}

	first, alreadyExisted1, err := store.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("первое создание: %v", err)
	}

	if alreadyExisted1 {
		t.Fatal("первый вызов не должен считаться повтором")
	}

	second, alreadyExisted2, err := store.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("повторное создание: %v", err)
	}

	if !alreadyExisted2 {
		t.Fatal("повторный вызов с тем же Idempotency-Key должен вернуть существующий заказ")
	}

	if second.ID != first.ID {
		t.Errorf("повторный вызов вернул другой заказ: %s != %s", second.ID, first.ID)
	}
}

func TestPostgres_GetRestaurantStats(t *testing.T) {
	db := setupDB(t)
	cleanTables(t, db)

	store := storage.NewPostgresStorage(db)

	restaurant := &domain.Restaurant{
		ID:     "11111111-1111-1111-1111-111111111114",
		Name:   "Тест",
		APIKey: "key1",
	}
	seedRestaurant(t, store, restaurant)

	item := &domain.MenuItem{
		ID:           "22222222-2222-2222-2222-222222222225",
		RestaurantID: restaurant.ID,
		Name:         "Пельмени",
		PriceCents:   30000,
		Available:    true,
	}
	seedMenuItem(t, store, item)

	order, _, err := store.CreateOrder(
		context.Background(),
		storage.CreateOrderInput{
			RestaurantID:    restaurant.ID,
			CustomerName:    "Ариана",
			CustomerPhone:   "+7999",
			DeliveryAddress: "Адрес",
			Items: []storage.RequestedItem{
				{
					MenuItemID: item.ID,
					Quantity:   2,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	for _, status := range []domain.OrderStatus{
		domain.OrderStatusConfirmed,
		domain.OrderStatusPreparing,
		domain.OrderStatusReady,
		domain.OrderStatusDelivered,
	} {
		_, err = store.UpdateOrderStatus(
			context.Background(),
			restaurant.ID,
			order.ID,
			status,
		)
		if err != nil {
			t.Fatalf("update status to %s: %v", status, err)
		}
	}

	stats, err := store.GetRestaurantStats(context.Background(), restaurant.ID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}

	if stats.TotalOrders != 1 {
		t.Errorf("TotalOrders = %d, ожидалось 1", stats.TotalOrders)
	}

	if stats.RevenueCents != 60000 {
		t.Errorf("RevenueCents = %d, ожидалось 60000", stats.RevenueCents)
	}
}
