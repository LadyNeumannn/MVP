package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

type OutboxWorker struct {
	Store     storage.Storage
	Notifier  Notifier
	Logger    *log.Logger
	Interval  time.Duration
	BatchSize int
}

func (w *OutboxWorker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	batchSize := w.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx, batchSize)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context, batchSize int) {
	items, err := w.Store.FetchPendingWebhooks(ctx, batchSize)
	if err != nil {
		if w.Logger != nil {
			w.Logger.Printf("outbox: fetch failed: %v", err)
		}
		return
	}

	for _, item := range items {
		w.deliver(ctx, item)
	}
}

func (w *OutboxWorker) deliver(ctx context.Context, item storage.WebhookOutboxItem) {
	var order domain.Order
	if err := json.Unmarshal(item.Payload, &order); err != nil {
		if w.Logger != nil {
			w.Logger.Printf("outbox: bad payload for event %s (order %s): %v", item.ID, item.OrderID, err)
		}
		_ = w.Store.MarkWebhookFailed(ctx, item.ID, "unmarshal payload: "+err.Error(), time.Now().Add(24*time.Hour))
		return
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := w.Notifier.NotifyNewOrder(sendCtx, item.WebhookURL, &order)
	cancel()

	if err != nil {
		backoff := nextBackoff(item.Attempts)
		if markErr := w.Store.MarkWebhookFailed(ctx, item.ID, err.Error(), time.Now().Add(backoff)); markErr != nil && w.Logger != nil {
			w.Logger.Printf("outbox: mark failed error for %s: %v", item.ID, markErr)
		}
		if w.Logger != nil {
			w.Logger.Printf("outbox: delivery failed for order %s: %v (retry in %s)", item.OrderID, err, backoff)
		}
		return
	}

	if err := w.Store.MarkWebhookDelivered(ctx, item.ID); err != nil && w.Logger != nil {
		w.Logger.Printf("outbox: mark delivered failed for %s: %v", item.ID, err)
	}
}

func nextBackoff(attempts int) time.Duration {
	n := attempts + 1
	backoff := time.Duration(n*n) * time.Second
	const maxBackoff = 5 * time.Minute
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}
