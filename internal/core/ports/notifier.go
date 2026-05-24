package ports

import (
	"context"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

type DeliveryResult struct {
	ExternalID string
	Status     string
	Timestamp  string
}

type Notifier interface {
	Send(ctx context.Context, notification *domain.Notification) (*DeliveryResult, error)
}
