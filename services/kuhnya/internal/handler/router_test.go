package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/handler"
	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/service"
)

const testOnboardingSecret = "test-onboarding-secret"

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func newTestRouter(store *fakeStorage) http.Handler {
	svc := service.New(store, testLogger())
	return handler.NewRouter(svc, testLogger(), testOnboardingSecret)
}

func doRequest(
	t *testing.T,
	router http.Handler,
	method, path string,
	body interface{},
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(buf)
	}

	//nolint:noctx // httptest.NewRequestWithContext требует Go 1.23+
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func TestListRestaurants_SearchFilter(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{
		ID:       "rest-1",
		Name:     "Пельменная у Ашота",
		IsActive: true,
	})
	store.seedRestaurant(domain.Restaurant{
		ID:       "rest-2",
		Name:     "Суши-бар Токио",
		IsActive: true,
	})
	router := newTestRouter(store)

	rec := doRequest(
		t,
		router,
		http.MethodGet,
		"/api/v1/restaurants?q=пельм",
		nil,
		nil,
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, тело: %s", rec.Code, rec.Body.String())
	}

	var restaurants []domain.Restaurant
	if err := json.Unmarshal(rec.Body.Bytes(), &restaurants); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(restaurants) != 1 || restaurants[0].ID != "rest-1" {
		t.Errorf("ожидался только rest-1, получено: %+v", restaurants)
	}
}

func TestCreateOrder_Success(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{
		ID:       "rest-1",
		Name:     "Тест",
		IsActive: true,
	})
	store.seedMenuItem(domain.MenuItem{
		ID:           "item-1",
		RestaurantID: "rest-1",
		Name:         "Пельмени",
		PriceCents:   30000,
		Available:    true,
	})
	router := newTestRouter(store)

	body := map[string]interface{}{
		"restaurant_id":    "rest-1",
		"customer_name":    "Ариана",
		"customer_phone":   "+79990000000",
		"delivery_address": "ул. Тестовая, 1",
		"items": []map[string]interface{}{
			{
				"menu_item_id": "item-1",
				"quantity":     2,
			},
		},
	}

	rec := doRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/orders",
		body,
		nil,
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf("статус = %d, тело: %s", rec.Code, rec.Body.String())
	}

	var order domain.Order
	if err := json.Unmarshal(rec.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if order.TotalCents != 60000 {
		t.Errorf("сумма = %d, ожидалось 60000", order.TotalCents)
	}
}

func TestCreateOrder_MissingDeliveryAddress_Returns422(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{
		ID:       "rest-1",
		Name:     "Тест",
		IsActive: true,
	})
	store.seedMenuItem(domain.MenuItem{
		ID:           "item-1",
		RestaurantID: "rest-1",
		Name:         "Пельмени",
		PriceCents:   30000,
		Available:    true,
	})
	router := newTestRouter(store)

	body := map[string]interface{}{
		"restaurant_id":  "rest-1",
		"customer_name":  "Ариана",
		"customer_phone": "+79990000000",
		"items": []map[string]interface{}{
			{
				"menu_item_id": "item-1",
				"quantity":     1,
			},
		},
	}

	rec := doRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/orders",
		body,
		nil,
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf(
			"статус = %d, ожидался 422. Тело: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestCreateOrder_UnavailableItem_Returns409(t *testing.T) {
	store := newFakeStorage()
	store.seedRestaurant(domain.Restaurant{
		ID:       "rest-1",
		Name:     "Тест",
		IsActive: true,
	})
	store.seedMenuItem(domain.MenuItem{
		ID:           "item-1",
		RestaurantID: "rest-1",
		Name:         "Компот",
		PriceCents:   12000,
		Available:    false,
	})
	router := newTestRouter(store)

	body := map[string]interface{}{
		"restaurant_id":    "rest-1",
		"customer_name":    "Ариана",
		"customer_phone":   "+7999",
		"delivery_address": "ул. Тестовая, 1",
		"items": []map[string]interface{}{
			{
				"menu_item_id": "item-1",
				"quantity":     1,
			},
		},
	}

	rec := doRequest(
		t,
		router,
		http.MethodPost,
		"/api/v1/orders",
		body,
		nil,
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf(
			"статус = %d, ожидался 409. Тело: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestGetOrder_NotFound_Returns404(t *testing.T) {
	router := newTestRouter(newFakeStorage())

	rec := doRequest(
		t,
		router,
		http.MethodGet,
		"/api/v1/orders/does-not-exist",
		nil,
		nil,
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("статус = %d, ожидался 404", rec.Code)
	}
}

func TestRegisterRestaurant_RequiresOnboardingSecret(t *testing.T) {
	router := newTestRouter(newFakeStorage())

	body := map[string]string{
		"id":   "rest-1",
		"name": "Тест",
	}

	rec := doRequest(
		t,
		router,
		http.MethodPost,
		"/partner/v1/restaurants",
		body,
		nil,
	)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без секрета: статус = %d, ожидался 401", rec.Code)
	}

	rec = doRequest(
		t,
		router,
		http.MethodPost,
		"/partner/v1/restaurants",
		body,
		map[string]string{
			"Authorization": "Bearer неверный-секрет",
		},
	)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("неверный секрет: статус = %d, ожидался 401", rec.Code)
	}

	rec = doRequest(
		t,
		router,
		http.MethodPost,
		"/partner/v1/restaurants",
		body,
		map[string]string{
			"Authorization": "Bearer " + testOnboardingSecret,
		},
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"верный секрет: статус = %d, ожидался 201. Тело: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if apiKey, _ := resp["api_key"].(string); apiKey == "" {
		t.Error("ответ регистрации должен содержать непустой api_key")
	}
}

func registerAndGetAPIKey(
	t *testing.T,
	router http.Handler,
	restaurantID,
	name string,
) string {
	t.Helper()

	rec := doRequest(
		t,
		router,
		http.MethodPost,
		"/partner/v1/restaurants",
		map[string]string{
			"id":   restaurantID,
			"name": name,
		},
		map[string]string{
			"Authorization": "Bearer " + testOnboardingSecret,
		},
	)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"регистрация не удалась: статус %d, тело %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	apiKey, _ := resp["api_key"].(string)
	if apiKey == "" {
		t.Fatal("пустой api_key в ответе регистрации")
	}

	return apiKey
}

func TestPartnerRoutes_RequireAPIKey(t *testing.T) {
	store := newFakeStorage()
	router := newTestRouter(store)

	apiKey := registerAndGetAPIKey(t, router, "rest-1", "Тест")

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			"без заголовка",
			"",
			http.StatusUnauthorized,
		},
		{
			"неверный ключ",
			"Bearer совсем-не-тот-ключ",
			http.StatusUnauthorized,
		},
		{
			"верный ключ",
			"Bearer " + apiKey,
			http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}

			if tc.authHeader != "" {
				headers["Authorization"] = tc.authHeader
			}

			rec := doRequest(
				t,
				router,
				http.MethodGet,
				"/partner/v1/restaurants/rest-1/stats",
				nil,
				headers,
			)

			if rec.Code != tc.wantStatus {
				t.Errorf(
					"статус = %d, ожидался %d. Тело: %s",
					rec.Code,
					tc.wantStatus,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestPartnerRoutes_APIKeyNotTransferableBetweenRestaurants(t *testing.T) {
	store := newFakeStorage()
	router := newTestRouter(store)

	registerAndGetAPIKey(t, router, "rest-1", "Заведение 1")
	apiKeyOfRest2 := registerAndGetAPIKey(t, router, "rest-2", "Заведение 2")

	rec := doRequest(
		t,
		router,
		http.MethodGet,
		"/partner/v1/restaurants/rest-1/stats",
		nil,
		map[string]string{
			"Authorization": "Bearer " + apiKeyOfRest2,
		},
	)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf(
			"статус = %d, ожидался 401 (ключ чужого заведения не должен подходить)",
			rec.Code,
		)
	}
}
