package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                    string
	DBDSN                   string
	DBMaxOpenConns          int
	DBMaxIdleConns          int
	DBConnMaxIdle           time.Duration
	ShutdownTimeout         time.Duration
	PartnerOnboardingSecret string
	OutboxPollInterval      time.Duration
	WebhookSharedSecret     string
}

func Load() Config {
	return Config{
		Port:                    getEnv("PORT", "8080"),
		DBDSN:                   getEnv("DB_DSN", "postgres://kuhnya:kuhnya@postgres:5432/kuhnya?sslmode=disable"),
		DBMaxOpenConns:          getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:          getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxIdle:           5 * time.Minute,
		ShutdownTimeout:         10 * time.Second,
		PartnerOnboardingSecret: getEnv("PARTNER_ONBOARDING_SECRET", "dev-onboarding-secret-change-me"),
		OutboxPollInterval:      2 * time.Second,
		WebhookSharedSecret:     getEnv("WEBHOOK_SHARED_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
