package rest

import (
	"time"

	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

type NotificationResponse struct {
	ID             uuid.UUID      `json:"id"`
	BatchID        uuid.UUID      `json:"batch_id,omitempty"`
	Recipient      string         `json:"recipient"`
	Channel        string         `json:"channel"`
	Content        string         `json:"content"`
	Priority       string         `json:"priority"`
	Status         string         `json:"status"`
	RetryCount     int            `json:"retry_count"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type BatchResponse struct {
	BatchID       uuid.UUID              `json:"batch_id"`
	Notifications []NotificationResponse `json:"notifications"`
	Count         int                    `json:"count"`
}

type ListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	Total         int64                  `json:"total"`
	Page          int                    `json:"page"`
	PageSize      int                    `json:"page_size"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

func toNotificationResponse(n *domain.Notification) NotificationResponse {
	return NotificationResponse{
		ID:             n.ID,
		BatchID:        n.BatchID,
		Recipient:      n.Recipient,
		Channel:        string(n.Channel),
		Content:        n.Content,
		Priority:       string(n.Priority),
		Status:         string(n.Status),
		RetryCount:     n.RetryCount,
		IdempotencyKey: n.IdempotencyKey,
		Payload:        n.Payload,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
	}
}

func toNotificationResponses(notifications []*domain.Notification) []NotificationResponse {
	result := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		result[i] = toNotificationResponse(n)
	}
	return result
}
