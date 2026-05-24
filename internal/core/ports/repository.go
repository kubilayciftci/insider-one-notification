package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

type ListFilter struct {
	Status   *domain.Status
	Channel  *domain.Channel
	FromDate *time.Time
	ToDate   *time.Time
	Page     int
	PageSize int
}

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	CreateBatch(ctx context.Context, notifications []*domain.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error
	Cancel(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*domain.Notification, int64, error)
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	GetDueScheduled(ctx context.Context) ([]*domain.Notification, error)
}
