package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

type recordingNotifier struct {
	mu        sync.Mutex
	calls     []string
	failUntil int
}

func (n *recordingNotifier) NotifyNewOrder(ctx context.Context, webhookURL string, order *domain.Order) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, webhookURL)
	if len(n.calls) <= n.failUntil {
		return errors.New("имитация сбоя доставки")
	}
	return nil
}

func (n *recordingNotifier) callCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

func TestOutboxWorker_DeliversEnqueuedWebhook(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{ID: "rest-1", Name: "Тест", IsActive: true, WebhookURL: "http://restaurant.example/webhooks/orders"})
	item := domain.MenuItem{ID: "item-1", RestaurantID: "rest-1", Name: "Пельмени", PriceCents: 30000, Available: true}
	store.seedMenuItem(item)

	svc := service.New(store, nil)
	order, _, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil))
	if err != nil {
		t.Fatalf("неожиданная ошибка при создании заказа: %v", err)
	}

	notifier := &recordingNotifier{}
	worker := &service.OutboxWorker{Store: store, Notifier: notifier, Logger: nil}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	worker.Interval = 10 * time.Millisecond
	worker.Run(ctx)
	cancel()

	if notifier.callCount() == 0 {
		t.Fatal("воркер ни разу не попытался доставить webhook")
	}

	pending, err := store.FetchPendingWebhooks(context.Background(), 10)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	for _, p := range pending {
		if p.OrderID == order.ID {
			t.Errorf("событие для заказа %s всё ещё pending после успешной доставки", order.ID)
		}
	}
}

func TestOutboxWorker_RetriesOnFailure(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{ID: "rest-1", Name: "Тест", IsActive: true, WebhookURL: "http://restaurant.example/webhooks/orders"})
	item := domain.MenuItem{ID: "item-1", RestaurantID: "rest-1", Name: "Пельмени", PriceCents: 30000, Available: true}
	store.seedMenuItem(item)

	svc := service.New(store, nil)
	if _, _, err := svc.CreateOrder(context.Background(),
		validOrderInput("rest-1", []storage.RequestedItem{{MenuItemID: item.ID, Quantity: 1}}, nil)); err != nil {
		t.Fatalf("неожиданная ошибка при создании заказа: %v", err)
	}
	notifier := &recordingNotifier{failUntil: 1}
	worker := &service.OutboxWorker{Store: store, Notifier: notifier, Logger: nil, Interval: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	worker.Run(ctx)
	cancel()

	if notifier.callCount() != 1 {
		t.Fatalf("ожидался 1 вызов за это окно (второй попытки быть не должно — next_attempt_at отложен), получено: %d", notifier.callCount())
	}

	pendingRightAfter, err := store.FetchPendingWebhooks(context.Background(), 10)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(pendingRightAfter) != 0 {
		t.Fatalf("после неудачной попытки next_attempt_at должен быть отложен в будущее, а не сразу снова pending: %+v", pendingRightAfter)
	}
}
