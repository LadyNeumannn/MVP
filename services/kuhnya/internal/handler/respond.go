package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/storage"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, logger *log.Logger, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
	case errors.Is(err, storage.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
	case errors.Is(err, storage.ErrItemUnavailable):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "one or more menu items are unavailable"})
	case errors.Is(err, storage.ErrItemWrongRestaurant):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "one or more menu items belong to a different restaurant"})
	case errors.Is(err, storage.ErrRestaurantInactive):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "restaurant is not accepting orders right now"})
	case errors.Is(err, storage.ErrInvalidTransition):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "invalid order status transition"})
	default:
		if logger != nil {
			logger.Printf("internal error: %v", err)
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}

func decodeJSON(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
