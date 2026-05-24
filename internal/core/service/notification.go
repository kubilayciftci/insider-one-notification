package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
)

// CreateRequest holds the parameters needed to create a single notification.
type CreateRequest struct {
	Recipient      string
	Channel        domain.Channel
	Content        string
	Priority       domain.Priority
	IdempotencyKey string
	ScheduledAt    *time.Time
	Payload        map[string]any
}

// NotificationService implements the core business logic for notification
// lifecycle management. It orchestrates between the repository (persistence)
// and message queue (async delivery) ports without importing any
// infrastructure packages.
type NotificationService struct {
	repo   ports.NotificationRepository
	queue  ports.MessageQueue
	logger *slog.Logger
}

// NewNotificationService constructs a NotificationService with the given
// repository, message queue, and structured logger.
func NewNotificationService(repo ports.NotificationRepository, queue ports.MessageQueue, logger *slog.Logger) *NotificationService {
	return &NotificationService{
		repo:   repo,
		queue:  queue,
		logger: logger,
	}
}

// Create validates, persists, and enqueues a single notification.
// The flow is: validate -> persist (pending) -> update status (queued) -> publish.
// If the queue publish fails, the status is rolled back to pending.
func (s *NotificationService) Create(ctx context.Context, req CreateRequest) (*domain.Notification, error) {
	n, err := domain.NewNotification(req.Recipient, req.Channel, req.Content, req.Priority)
	if err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	n.IdempotencyKey = req.IdempotencyKey
	if req.Payload != nil {
		n.Payload = req.Payload
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("persist notification: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	n.Status = domain.StatusQueued

	if err := s.queue.Publish(ctx, n); err != nil {
		s.logger.ErrorContext(ctx, "queue publish failed, rolling back status",
			slog.String("id", n.ID.String()),
			slog.Any("error", err),
		)
		_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusPending)
		n.Status = domain.StatusPending
		return nil, fmt.Errorf("enqueue notification: %w", err)
	}

	s.logger.InfoContext(ctx, "notification created and queued",
		slog.String("id", n.ID.String()),
		slog.String("channel", string(n.Channel)),
		slog.String("priority", string(n.Priority)),
	)

	return n, nil
}

// CreateBatch validates all requests, persists them as a batch, then enqueues
// each notification individually. Partial queue failures are tolerated --
// the successfully published notifications are marked as queued while failures
// are logged and left in pending status.
func (s *NotificationService) CreateBatch(ctx context.Context, requests []CreateRequest) (uuid.UUID, []*domain.Notification, error) {
	if len(requests) > domain.MaxBatchSize {
		return uuid.Nil, nil, domain.ErrBatchTooLarge
	}

	batchID := uuid.New()
	notifications := make([]*domain.Notification, 0, len(requests))

	for _, req := range requests {
		n, err := domain.NewNotification(req.Recipient, req.Channel, req.Content, req.Priority)
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("validation failed for recipient %s: %w", req.Recipient, err)
		}
		n.BatchID = batchID
		n.IdempotencyKey = req.IdempotencyKey
		if req.Payload != nil {
			n.Payload = req.Payload
		}
		notifications = append(notifications, n)
	}

	if err := s.repo.CreateBatch(ctx, notifications); err != nil {
		return uuid.Nil, nil, fmt.Errorf("persist batch: %w", err)
	}

	for _, n := range notifications {
		if err := s.queue.Publish(ctx, n); err != nil {
			s.logger.ErrorContext(ctx, "failed to enqueue notification from batch",
				slog.String("id", n.ID.String()),
				slog.Any("error", err),
			)
			continue
		}
		_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusQueued)
		n.Status = domain.StatusQueued
	}

	s.logger.InfoContext(ctx, "batch created",
		slog.String("batch_id", batchID.String()),
		slog.Int("count", len(notifications)),
	)

	return batchID, notifications, nil
}

// GetByID retrieves a single notification by its ID.
func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByBatchID retrieves all notifications belonging to a batch.
func (s *NotificationService) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	return s.repo.GetByBatchID(ctx, batchID)
}

// Cancel cancels a notification if it is in a cancellable state (pending or queued).
// Returns domain.ErrNotCancellable if the notification has already been delivered,
// failed, or is being processed.
func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !n.IsCancellable() {
		return domain.ErrNotCancellable
	}

	return s.repo.Cancel(ctx, id)
}

// List retrieves a paginated, filtered list of notifications.
func (s *NotificationService) List(ctx context.Context, filter ports.ListFilter) ([]*domain.Notification, int64, error) {
	return s.repo.List(ctx, filter)
}
