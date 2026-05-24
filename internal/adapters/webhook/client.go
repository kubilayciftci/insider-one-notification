package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("webhook-client")

var _ ports.Notifier = (*Client)(nil)

type webhookRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

type webhookResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type Client struct {
	url        string
	httpClient *http.Client
}

func NewClient(url string, timeoutSec int) *Client {
	return &Client{
		url: url,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

func (c *Client) Send(ctx context.Context, notification *domain.Notification) (*ports.DeliveryResult, error) {
	ctx, span := tracer.Start(ctx, "webhook.Send")
	defer span.End()

	span.SetAttributes(
		attribute.String("notification.id", notification.ID.String()),
		attribute.String("notification.channel", string(notification.Channel)),
	)

	payload := webhookRequest{
		To:      notification.Recipient,
		Channel: string(notification.Channel),
		Content: notification.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var result webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &ports.DeliveryResult{
			ExternalID: fmt.Sprintf("ext-%s", notification.ID.String()),
			Status:     "accepted",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	return &ports.DeliveryResult{
		ExternalID: result.MessageID,
		Status:     result.Status,
		Timestamp:  result.Timestamp,
	}, nil
}
