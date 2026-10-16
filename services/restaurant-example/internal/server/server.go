package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/kuhnyaclient"
	"github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/menu"
)

type Server struct {
	RestaurantID        string
	WebhookSharedSecret string
	Kuhnya              *kuhnyaclient.Client
	Logger              *log.Logger
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /menu", s.handleGetMenu)
	mux.HandleFunc("POST /webhooks/orders", s.handleNewOrderWebhook)
	return mux
}

func (s *Server) handleGetMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(menu.Starter())
}

type orderPayload struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	CustomerName string `json:"customer_name"`
	Status       string `json:"status"`
	TotalCents   int64  `json:"total_cents"`
	Items        []struct {
		NameSnapshot       string `json:"name_snapshot"`
		PriceCentsSnapshot int64  `json:"price_cents_snapshot"`
		Quantity           int    `json:"quantity"`
	} `json:"items"`
}

func (s *Server) handleNewOrderWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.Logger.Printf("failed to read body: %v", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	if s.WebhookSharedSecret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if !verifySignature(body, signature, s.WebhookSharedSecret) {
			s.Logger.Printf("invalid webhook signature for order request")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var order orderPayload
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	s.Logger.Printf("новый заказ получен: id=%s сумма=%.2f₽ позиций=%d",
		order.ID, float64(order.TotalCents)/100, len(order.Items))
	w.WriteHeader(http.StatusAccepted)

	go s.fulfillOrder(order.ID)
}

func (s *Server) fulfillOrder(orderID string) {
	time.Sleep(randDuration(1, 3))

	const cancelChancePercent = 15
	if rand.Intn(100) < cancelChancePercent {
		if err := s.Kuhnya.UpdateOrderStatus(context.Background(), s.RestaurantID, orderID, "cancelled"); err != nil {
			s.Logger.Printf("не удалось отменить заказ %s: %v", orderID, err)
			return
		}
		s.Logger.Printf("заказ %s -> cancelled (не хватило ингредиентов)", orderID)
		return
	}

	steps := []struct {
		status string
		delay  time.Duration
	}{
		{"confirmed", 0},
		{"preparing", randDuration(3, 6)},
		{"ready", randDuration(3, 6)},
		{"delivered", randDuration(2, 4)},
	}

	for _, step := range steps {
		if step.delay > 0 {
			time.Sleep(step.delay)
		}
		if err := s.Kuhnya.UpdateOrderStatus(context.Background(), s.RestaurantID, orderID, step.status); err != nil {
			s.Logger.Printf("не удалось обновить статус заказа %s -> %s: %v", orderID, step.status, err)
			return
		}
		s.Logger.Printf("заказ %s -> %s", orderID, step.status)
	}
}

func randDuration(minSec, maxSec int) time.Duration {
	return time.Duration(minSec+rand.Intn(maxSec-minSec+1)) * time.Second
}

func verifySignature(payload []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}
