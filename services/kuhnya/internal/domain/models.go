package domain

import "time"

type Restaurant struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsActive   bool      `json:"is_active"`
	WebhookURL string    `json:"webhook_url,omitempty"`
	APIKey     string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type MenuItem struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	PriceCents   int64  `json:"price_cents"`
	Available    bool   `json:"available"`
}

type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "created"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

var nextAllowed = map[OrderStatus]map[OrderStatus]bool{
	OrderStatusCreated:   {OrderStatusConfirmed: true, OrderStatusCancelled: true},
	OrderStatusConfirmed: {OrderStatusPreparing: true, OrderStatusCancelled: true},
	OrderStatusPreparing: {OrderStatusReady: true, OrderStatusCancelled: true},
	OrderStatusReady:     {OrderStatusDelivered: true},
	OrderStatusDelivered: {},
	OrderStatusCancelled: {},
}

func CanTransition(from, to OrderStatus) bool {
	allowed, ok := nextAllowed[from]
	if !ok {
		return false
	}
	return allowed[to]
}

type OrderItem struct {
	ID                 string `json:"id"`
	OrderID            string `json:"order_id"`
	MenuItemID         string `json:"menu_item_id"`
	NameSnapshot       string `json:"name_snapshot"`
	PriceCentsSnapshot int64  `json:"price_cents_snapshot"`
	Quantity           int    `json:"quantity"`
}

type Order struct {
	ID              string      `json:"id"`
	RestaurantID    string      `json:"restaurant_id"`
	CustomerName    string      `json:"customer_name"`
	CustomerPhone   string      `json:"customer_phone"`
	DeliveryAddress string      `json:"delivery_address"`
	Status          OrderStatus `json:"status"`
	TotalCents      int64       `json:"total_cents"`
	Items           []OrderItem `json:"items"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type RestaurantStats struct {
	RestaurantID       string              `json:"restaurant_id"`
	TotalOrders        int                 `json:"total_orders"`
	OrdersByStatus     map[OrderStatus]int `json:"orders_by_status"`
	RevenueCents       int64               `json:"revenue_cents"`
	AvgOrderValueCents int64               `json:"avg_order_value_cents"`
}
