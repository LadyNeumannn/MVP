package kuhnyaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL          string
	HTTP             *http.Client
	OnboardingSecret string

	apiKey string
}

func New(baseURL, onboardingSecret string) *Client {
	return &Client{
		BaseURL:          baseURL,
		HTTP:             &http.Client{Timeout: 5 * time.Second},
		OnboardingSecret: onboardingSecret,
	}
}

type Restaurant struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsActive   bool   `json:"is_active"`
	WebhookURL string `json:"webhook_url"`
	APIKey     string `json:"api_key"`
}

func (c *Client) RegisterRestaurant(ctx context.Context, id, name, webhookURL string) (*Restaurant, error) {
	body := map[string]string{"id": id, "name": name, "webhook_url": webhookURL}
	var out Restaurant
	if err := c.doJSON(ctx, http.MethodPost, "/partner/v1/restaurants", body, &out, "Bearer "+c.OnboardingSecret); err != nil {
		return nil, fmt.Errorf("register restaurant: %w", err)
	}
	c.apiKey = out.APIKey
	return &out, nil
}

type MenuItemInput struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PriceCents  int64  `json:"price_cents"`
	Available   bool   `json:"available"`
}

func (c *Client) CreateMenuItem(ctx context.Context, restaurantID string, item MenuItemInput) error {
	path := fmt.Sprintf("/partner/v1/restaurants/%s/menu/items", restaurantID)
	return c.doJSON(ctx, http.MethodPost, path, item, nil, c.authHeader())
}

func (c *Client) UpdateOrderStatus(ctx context.Context, restaurantID, orderID, status string) error {
	path := fmt.Sprintf("/partner/v1/restaurants/%s/orders/%s/status", restaurantID, orderID)
	return c.doJSON(ctx, http.MethodPatch, path, map[string]string{"status": status}, nil, c.authHeader())
}

func (c *Client) authHeader() string {
	return "Bearer " + c.apiKey
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out interface{}, authHeader string) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s -> %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
