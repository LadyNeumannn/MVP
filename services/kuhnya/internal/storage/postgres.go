package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) CreateRestaurant(ctx context.Context, r *domain.Restaurant) (bool, error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO restaurants (id, name, is_active, webhook_url, api_key, created_at)
		VALUES ($1, $2, TRUE, $3, $4, now())
		ON CONFLICT (id) DO NOTHING
		RETURNING created_at`,
		r.ID, r.Name, nullableString(r.WebhookURL), r.APIKey,
	)

	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			existing, getErr := s.GetRestaurant(ctx, r.ID)
			if getErr != nil {
				return false, getErr
			}
			*r = *existing
			return false, nil
		}
		return false, fmt.Errorf("create restaurant: %w", err)
	}

	r.IsActive = true
	r.CreatedAt = createdAt
	return true, nil
}

func (s *PostgresStorage) GetRestaurant(ctx context.Context, id string) (*domain.Restaurant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, is_active, COALESCE(webhook_url, ''), api_key, created_at
		FROM restaurants WHERE id = $1`, id)

	var r domain.Restaurant
	if err := row.Scan(&r.ID, &r.Name, &r.IsActive, &r.WebhookURL, &r.APIKey, &r.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get restaurant: %w", err)
	}
	return &r, nil
}

func (s *PostgresStorage) ListActiveRestaurants(ctx context.Context, query string) ([]domain.Restaurant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, is_active, COALESCE(webhook_url, ''), created_at
		FROM restaurants
		WHERE is_active = TRUE AND ($1 = '' OR name ILIKE '%' || $1 || '%')
		ORDER BY name`, query)
	if err != nil {
		return nil, fmt.Errorf("list restaurants: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []domain.Restaurant
	for rows.Next() {
		var r domain.Restaurant
		if err := rows.Scan(&r.ID, &r.Name, &r.IsActive, &r.WebhookURL, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan restaurant: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) CreateMenuItem(ctx context.Context, item *domain.MenuItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO menu_items (id, restaurant_id, name, description, price_cents, available, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (id) DO UPDATE SET
			name        = EXCLUDED.name,
			description = EXCLUDED.description,
			price_cents = EXCLUDED.price_cents,
			available   = EXCLUDED.available,
			updated_at  = now()`,
		item.ID, item.RestaurantID, item.Name, item.Description, item.PriceCents, item.Available,
	)
	if err != nil {
		return fmt.Errorf("create menu item: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateMenuItem(ctx context.Context, restaurantID, itemID string, priceCents *int64, available *bool) (*domain.MenuItem, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE menu_items SET
			price_cents = COALESCE($1, price_cents),
			available   = COALESCE($2, available),
			updated_at  = now()
		WHERE id = $3 AND restaurant_id = $4
		RETURNING id, restaurant_id, name, description, price_cents, available`,
		priceCents, available, itemID, restaurantID,
	)

	var item domain.MenuItem
	if err := row.Scan(&item.ID, &item.RestaurantID, &item.Name, &item.Description, &item.PriceCents, &item.Available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update menu item: %w", err)
	}
	return &item, nil
}

func (s *PostgresStorage) ListMenuItems(ctx context.Context, restaurantID, query string) ([]domain.MenuItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, restaurant_id, name, description, price_cents, available
		FROM menu_items
		WHERE restaurant_id = $1 AND ($2 = '' OR name ILIKE '%' || $2 || '%')
		ORDER BY name`, restaurantID, query)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []domain.MenuItem
	for rows.Next() {
		var m domain.MenuItem
		if err := rows.Scan(&m.ID, &m.RestaurantID, &m.Name, &m.Description, &m.PriceCents, &m.Available); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) GetMenuItem(ctx context.Context, id string) (*domain.MenuItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, restaurant_id, name, description, price_cents, available
		FROM menu_items WHERE id = $1`, id)

	var m domain.MenuItem
	if err := row.Scan(&m.ID, &m.RestaurantID, &m.Name, &m.Description, &m.PriceCents, &m.Available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get menu item: %w", err)
	}
	return &m, nil
}

func (s *PostgresStorage) CreateOrder(ctx context.Context, input CreateOrderInput) (*domain.Order, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	idempotencyKey := input.IdempotencyKey
	if idempotencyKey != nil && *idempotencyKey != "" {
		var existingOrderID string
		err = tx.QueryRowContext(ctx, `
			SELECT order_id FROM idempotency_keys WHERE key = $1`, *idempotencyKey,
		).Scan(&existingOrderID)
		switch {
		case err == nil:
			existing, getErr := s.GetOrder(ctx, existingOrderID)
			if getErr != nil {
				return nil, false, getErr
			}
			return existing, true, nil
		case errors.Is(err, sql.ErrNoRows):
		default:
			return nil, false, fmt.Errorf("check idempotency key: %w", err)
		}
	}

	var (
		isActive   bool
		webhookURL string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT is_active, COALESCE(webhook_url, '') FROM restaurants WHERE id = $1 FOR UPDATE`,
		input.RestaurantID,
	).Scan(&isActive, &webhookURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrNotFound
		}
		return nil, false, fmt.Errorf("lock restaurant: %w", err)
	}
	if !isActive {
		return nil, false, ErrRestaurantInactive
	}

	var (
		totalCents int64
		orderItems []domain.OrderItem
	)

	for _, ri := range input.Items {
		var (
			name       string
			priceCents int64
			available  bool
			itemRestID string
		)
		err = tx.QueryRowContext(ctx, `
			SELECT name, price_cents, available, restaurant_id
			FROM menu_items WHERE id = $1 FOR UPDATE`, ri.MenuItemID,
		).Scan(&name, &priceCents, &available, &itemRestID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, false, ErrNotFound
			}
			return nil, false, fmt.Errorf("lock menu item %s: %w", ri.MenuItemID, err)
		}
		if itemRestID != input.RestaurantID {
			return nil, false, ErrItemWrongRestaurant
		}
		if !available {
			return nil, false, ErrItemUnavailable
		}

		lineTotal := priceCents * int64(ri.Quantity)
		totalCents += lineTotal
		orderItems = append(orderItems, domain.OrderItem{
			ID:                 uuid.New().String(),
			MenuItemID:         ri.MenuItemID,
			NameSnapshot:       name,
			PriceCentsSnapshot: priceCents,
			Quantity:           ri.Quantity,
		})
	}

	order := domain.Order{
		ID:              uuid.New().String(),
		RestaurantID:    input.RestaurantID,
		CustomerName:    input.CustomerName,
		CustomerPhone:   input.CustomerPhone,
		DeliveryAddress: input.DeliveryAddress,
		Status:          domain.OrderStatusCreated,
		TotalCents:      totalCents,
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO orders (id, restaurant_id, customer_name, customer_phone, delivery_address, status, total_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
		RETURNING created_at, updated_at`,
		order.ID, order.RestaurantID, order.CustomerName, order.CustomerPhone, order.DeliveryAddress, order.Status, order.TotalCents,
	)
	if err = row.Scan(&order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, false, fmt.Errorf("insert order: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO order_items (id, order_id, menu_item_id, name_snapshot, price_cents_snapshot, quantity)
		VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return nil, false, fmt.Errorf("prepare order_items insert: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		if _, err := stmt.ExecContext(ctx,
			orderItems[i].ID, order.ID, orderItems[i].MenuItemID,
			orderItems[i].NameSnapshot, orderItems[i].PriceCentsSnapshot, orderItems[i].Quantity,
		); err != nil {
			return nil, false, fmt.Errorf("insert order item: %w", err)
		}
	}

	order.Items = orderItems
	if webhookURL != "" {
		payload, err := json.Marshal(order)
		if err != nil {
			return nil, false, fmt.Errorf("marshal order for outbox: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO webhook_outbox (id, order_id, webhook_url, payload)
			VALUES ($1, $2, $3, $4)`,
			uuid.New().String(), order.ID, webhookURL, payload,
		); err != nil {
			return nil, false, fmt.Errorf("enqueue webhook outbox: %w", err)
		}
	}

	if idempotencyKey != nil && *idempotencyKey != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO idempotency_keys (key, order_id) VALUES ($1, $2)`,
			*idempotencyKey, order.ID,
		); err != nil {
			return nil, false, fmt.Errorf("store idempotency key: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit order: %w", err)
	}

	return &order, false, nil
}

func (s *PostgresStorage) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, restaurant_id, customer_name, customer_phone, delivery_address, status, total_cents, created_at, updated_at
		FROM orders WHERE id = $1`, id)

	var o domain.Order
	if err := row.Scan(&o.ID, &o.RestaurantID, &o.CustomerName, &o.CustomerPhone, &o.DeliveryAddress, &o.Status, &o.TotalCents, &o.CreatedAt, &o.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, order_id, menu_item_id, name_snapshot, price_cents_snapshot, quantity
		FROM order_items WHERE order_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var oi domain.OrderItem
		if err := rows.Scan(&oi.ID, &oi.OrderID, &oi.MenuItemID, &oi.NameSnapshot, &oi.PriceCentsSnapshot, &oi.Quantity); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		o.Items = append(o.Items, oi)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &o, nil
}

func (s *PostgresStorage) ListRestaurantOrders(ctx context.Context, restaurantID string, status *domain.OrderStatus) ([]domain.Order, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if status != nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, restaurant_id, customer_name, customer_phone, delivery_address, status, total_cents, created_at, updated_at
			FROM orders WHERE restaurant_id = $1 AND status = $2 ORDER BY created_at DESC`,
			restaurantID, *status)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, restaurant_id, customer_name, customer_phone, delivery_address, status, total_cents, created_at, updated_at
			FROM orders WHERE restaurant_id = $1 ORDER BY created_at DESC`,
			restaurantID)
	}

	if err != nil {
		return nil, fmt.Errorf("list restaurant orders: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.RestaurantID, &o.CustomerName, &o.CustomerPhone, &o.DeliveryAddress, &o.Status, &o.TotalCents, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		result = append(result, o)
	}

	return result, rows.Err()
}

func (s *PostgresStorage) UpdateOrderStatus(ctx context.Context, restaurantID, orderID string, newStatus domain.OrderStatus) (*domain.Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var current domain.OrderStatus
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM orders WHERE id = $1 AND restaurant_id = $2 FOR UPDATE`,
		orderID, restaurantID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lock order: %w", err)
	}

	if !domain.CanTransition(current, newStatus) {
		return nil, ErrInvalidTransition
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`,
		newStatus, orderID,
	); err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit status update: %w", err)
	}

	return s.GetOrder(ctx, orderID)
}

func (s *PostgresStorage) GetRestaurantStats(ctx context.Context, restaurantID string) (*domain.RestaurantStats, error) {
	if _, err := s.GetRestaurant(ctx, restaurantID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*), COALESCE(SUM(total_cents), 0)
		FROM orders
		WHERE restaurant_id = $1
		GROUP BY status`, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("aggregate order stats: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	stats := &domain.RestaurantStats{
		RestaurantID:   restaurantID,
		OrdersByStatus: map[domain.OrderStatus]int{},
	}

	var deliveredRevenue int64
	var deliveredCount int

	for rows.Next() {
		var (
			status   domain.OrderStatus
			count    int
			sumCents int64
		)

		if err := rows.Scan(&status, &count, &sumCents); err != nil {
			return nil, fmt.Errorf("scan stats row: %w", err)
		}

		stats.OrdersByStatus[status] = count
		stats.TotalOrders += count

		if status == domain.OrderStatusDelivered {
			deliveredRevenue = sumCents
			deliveredCount = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	stats.RevenueCents = deliveredRevenue
	if deliveredCount > 0 {
		stats.AvgOrderValueCents = deliveredRevenue / int64(deliveredCount)
	}

	return stats, nil
}

func (s *PostgresStorage) FetchPendingWebhooks(ctx context.Context, limit int) ([]WebhookOutboxItem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, order_id, webhook_url, payload, attempts
		FROM webhook_outbox
		WHERE status = 'pending' AND next_attempt_at <= now()
		ORDER BY next_attempt_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending webhooks: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var (
		items []WebhookOutboxItem
		ids   []string
	)

	for rows.Next() {
		var item WebhookOutboxItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.WebhookURL, &item.Payload, &item.Attempts); err != nil {
			return nil, fmt.Errorf("scan webhook outbox item: %w", err)
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		leaseUntil := time.Now().Add(30 * time.Second)

		if _, err := tx.ExecContext(ctx, `
			UPDATE webhook_outbox SET next_attempt_at = $1, updated_at = now()
			WHERE id = ANY($2)`,
			leaseUntil, pq.Array(ids),
		); err != nil {
			return nil, fmt.Errorf("lease webhook outbox items: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit lease: %w", err)
	}

	return items, nil
}

func (s *PostgresStorage) MarkWebhookDelivered(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE webhook_outbox SET status = 'delivered', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark webhook delivered: %w", err)
	}
	return nil
}

func (s *PostgresStorage) MarkWebhookFailed(ctx context.Context, id, errMsg string, nextAttemptAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE webhook_outbox SET
			attempts = attempts + 1,
			last_error = $1,
			next_attempt_at = $2,
			updated_at = now()
		WHERE id = $3`,
		errMsg, nextAttemptAt, id,
	)
	if err != nil {
		return fmt.Errorf("mark webhook failed: %w", err)
	}
	return nil
}

func (s *PostgresStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
