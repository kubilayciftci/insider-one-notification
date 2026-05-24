package ports

import (
	"context"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

type MessageQueue interface {
	Publish(ctx context.Context, notification *domain.Notification) error
	PublishRetry(ctx context.Context, notification *domain.Notification, delay time.Duration) error
	PublishDLQ(ctx context.Context, notification *domain.Notification, reason string) error
	Close() error
}
