package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
)

func NewRouter(svc *service.Service, logger *log.Logger, onboardingSecret string) http.Handler {
	client := &ClientHandler{Service: svc, Logger: logger}
	partner := &PartnerHandler{Service: svc, Logger: logger}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := svc.HealthCheck(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /docs", ServeDocsUI)
	mux.HandleFunc("GET /docs/client-api.yaml", ServeOpenAPISpec("client-api.yaml"))
	mux.HandleFunc("GET /docs/partner-api.yaml", ServeOpenAPISpec("partner-api.yaml"))

	mux.HandleFunc("GET /api/v1/restaurants", client.ListRestaurants)
	mux.HandleFunc("GET /api/v1/restaurants/{restaurantId}/menu", client.GetMenu)
	mux.HandleFunc("POST /api/v1/orders", client.CreateOrder)
	mux.HandleFunc("GET /api/v1/orders/{orderId}", client.GetOrder)
	mux.HandleFunc("POST /partner/v1/restaurants",
		withOnboardingSecret(onboardingSecret, logger, partner.RegisterRestaurant))
	mux.HandleFunc("POST /partner/v1/restaurants/{restaurantId}/menu/items",
		withRestaurantAuth(svc, logger, partner.CreateMenuItem))
	mux.HandleFunc("GET /partner/v1/restaurants/{restaurantId}/menu/items",
		withRestaurantAuth(svc, logger, partner.ListMenuItems))
	mux.HandleFunc("PATCH /partner/v1/restaurants/{restaurantId}/menu/items/{itemId}",
		withRestaurantAuth(svc, logger, partner.UpdateMenuItem))
	mux.HandleFunc("GET /partner/v1/restaurants/{restaurantId}/orders",
		withRestaurantAuth(svc, logger, partner.ListOrders))
	mux.HandleFunc("PATCH /partner/v1/restaurants/{restaurantId}/orders/{orderId}/status",
		withRestaurantAuth(svc, logger, partner.UpdateOrderStatus))
	mux.HandleFunc("GET /partner/v1/restaurants/{restaurantId}/stats",
		withRestaurantAuth(svc, logger, partner.GetStats))

	return withCORS(withLogging(logger, withBodyLimit(mux)))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		if logger != nil {
			logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
		}
	})
}

func withBodyLimit(next http.Handler) http.Handler {
	const maxBodySize = 1 << 20
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
