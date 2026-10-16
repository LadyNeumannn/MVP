package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/config"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/handler"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

func main() {
	logger := log.New(os.Stdout, "kuhnya ", log.LstdFlags)
	cfg := config.Load()

	db, err := connectWithRetry(cfg.DBDSN, logger, 10, 2*time.Second)
	if err != nil {
		logger.Fatalf("could not connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Printf("close db: %v", err)
		}
	}()

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdle)

	store := storage.NewPostgresStorage(db)
	svc := service.New(store, logger)

	router := handler.NewRouter(svc, logger, cfg.PartnerOnboardingSecret)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	outboxWorker := &service.OutboxWorker{
		Store:    store,
		Notifier: service.NewHTTPNotifier(cfg.WebhookSharedSecret),
		Logger:   logger,
		Interval: cfg.OutboxPollInterval,
	}
	go outboxWorker.Run(workerCtx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Println("shutting down...")
	stopWorker()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
}

func connectWithRetry(dsn string, logger *log.Logger, attempts int, delay time.Duration) (*sql.DB, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				return db, nil
			}
			_ = db.Close()
		}
		lastErr = err
		logger.Printf("db not ready (attempt %d/%d): %v", i, attempts, err)
		time.Sleep(delay)
	}
	return nil, lastErr
}
