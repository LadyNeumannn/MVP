package handler

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
)

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

func withOnboardingSecret(secret string, logger *log.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or missing onboarding secret"})
			return
		}
		next(w, r)
	}
}

func withRestaurantAuth(svc *service.Service, logger *log.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		restaurantID := r.PathValue("restaurantId")
		apiKey := bearerToken(r)
		if err := svc.VerifyRestaurantAPIKey(r.Context(), restaurantID, apiKey); err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or missing api key"})
			return
		}
		next(w, r)
	}
}
