package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ladyneumannn/avito-kuhnya/services/kuhnya/internal/domain"
)

type Notifier interface {
	NotifyNewOrder(ctx context.Context, webhookURL string, order *domain.Order) error
}

type HTTPNotifier struct {
	Client       *http.Client
	SharedSecret string
}

func NewHTTPNotifier(sharedSecret string) *HTTPNotifier {
	return &HTTPNotifier{
		Client:       &http.Client{Timeout: 5 * time.Second},
		SharedSecret: sharedSecret,
	}
}

func (n *HTTPNotifier) NotifyNewOrder(ctx context.Context, webhookURL string, order *domain.Order) error {
	body, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order for webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if n.SharedSecret != "" {
		signature := signPayload(body, n.SharedSecret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook responded with status %d", resp.StatusCode)
	}
	return nil
}

func signPayload(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
