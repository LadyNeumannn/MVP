package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/kuhnyaclient"
	"github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/menu"
	"github.com/ladyneumannn/avito-kuhnya/services/restaurant-example/internal/server"
)

func main() {
	logger := log.New(os.Stdout, "restaurant-example ", log.LstdFlags)

	port := getEnv("PORT", "8090")
	kuhnyaBaseURL := getEnv("KUHNYA_BASE_URL", "http://kuhnya:8080")
	ownBaseURL := getEnv("OWN_BASE_URL", "http://restaurant-example:8090")
	restaurantID := getEnv("RESTAURANT_ID", "6f2f8c1a-0000-4b1a-9c3a-000000000000")
	restaurantName := getEnv("RESTAURANT_NAME", "Пельменная у Ашота")
	onboardingSecret := getEnv("ONBOARDING_SECRET", "dev-onboarding-secret-change-me")
	webhookSharedSecret := getEnv("WEBHOOK_SHARED_SECRET", "webhook-secret-change-me")

	client := kuhnyaclient.New(kuhnyaBaseURL, onboardingSecret)

	waitForKuhnya(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	restaurant, err := client.RegisterRestaurant(ctx, restaurantID, restaurantName, ownBaseURL+"/webhooks/orders")
	cancel()
	if err != nil {
		logger.Fatalf("не удалось зарегистрироваться в Авито.Кухня: %v", err)
	}
	logger.Printf("получен api_key для %s: %s", restaurantID, restaurant.APIKey)

	for _, item := range menu.Starter() {
		itemCtx, itemCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.CreateMenuItem(itemCtx, restaurantID, item); err != nil {
			logger.Printf("не удалось синхронизировать позицию меню %q: %v", item.Name, err)
		}
		itemCancel()
	}
	logger.Printf("зарегистрированы как %q (id=%s), меню синхронизировано", restaurantName, restaurantID)

	srv := &server.Server{
		RestaurantID:        restaurantID,
		WebhookSharedSecret: webhookSharedSecret,
		Kuhnya:              client,
		Logger:              logger,
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func waitForKuhnya(client *kuhnyaclient.Client, logger *log.Logger) {
	for i := 1; i <= 15; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL+"/healthz", nil)
		resp, err := client.HTTP.Do(req)
		cancel()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		logger.Printf("Авито.Кухня ещё не готова (попытка %d/15)", i)
		time.Sleep(2 * time.Second)
	}
	logger.Println("Авито.Кухня не ответила вовремя, то пробуем зарегистрироваться всё равно")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
