package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/adapters/kafka"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
	"go.opentelemetry.io/otel"
	"golang.org/x/time/rate"
)

var tracer = otel.Tracer("worker")

type Worker struct {
	repo            ports.NotificationRepository
	notifier        ports.Notifier
	queue           ports.MessageQueue
	logger          *slog.Logger
	rateLimitPerSec int
	maxRetries      int
	baseRetryDelay  time.Duration
	brokers         []string
}

func New(
	repo ports.NotificationRepository,
	notifier ports.Notifier,
	queue ports.MessageQueue,
	logger *slog.Logger,
	brokers []string,
	rateLimitPerSec int,
	maxRetries int,
	baseRetryDelay time.Duration,
) *Worker {
	return &Worker{
		repo:            repo,
		notifier:        notifier,
		queue:           queue,
		logger:          logger,
		rateLimitPerSec: rateLimitPerSec,
		maxRetries:      maxRetries,
		baseRetryDelay:  baseRetryDelay,
		brokers:         brokers,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	channels := []domain.Channel{domain.ChannelSMS, domain.ChannelEmail, domain.ChannelPush}
	priorities := []domain.Priority{domain.PriorityHigh, domain.PriorityNormal, domain.PriorityLow}
	errCh := make(chan error, len(channels)*len(priorities)+1)

	for _, ch := range channels {
		limiter := rate.NewLimiter(rate.Limit(w.rateLimitPerSec), w.rateLimitPerSec)

		for _, p := range priorities {
			topic := kafka.TopicForChannelPriority(ch, p)
			consumer := kafka.NewConsumer(w.brokers, topic, "notification-workers", w.logger)

			go func(ch domain.Channel, p domain.Priority, consumer *kafka.Consumer, limiter *rate.Limiter) {
				defer consumer.Close()
				w.logger.Info("starting worker",
					slog.String("channel", string(ch)),
					slog.String("priority", string(p)),
				)

				err := consumer.Consume(ctx, func(msgCtx context.Context, n *domain.Notification) error {
					return w.processNotification(msgCtx, n, limiter)
				})
				if err != nil {
					errCh <- fmt.Errorf("channel %s priority %s consumer: %w", ch, p, err)
				}
			}(ch, p, consumer, limiter)
		}
	}

	go func() {
		retryConsumer := kafka.NewConsumer(
			w.brokers,
			kafka.RetryTopic,
			"notification-retry-workers",
			w.logger,
		)
		defer retryConsumer.Close()
		w.logger.Info("starting retry worker")

		err := w.consumeRetry(ctx, retryConsumer)
		if err != nil {
			errCh <- fmt.Errorf("retry consumer: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (w *Worker) processNotification(ctx context.Context, n *domain.Notification, limiter *rate.Limiter) error {
	ctx, span := tracer.Start(ctx, "worker.processNotification")
	defer span.End()

	if err := limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	if err := w.repo.UpdateStatus(ctx, n.ID, domain.StatusProcessing); err != nil {
		w.logger.ErrorContext(ctx, "update status to processing failed",
			slog.String("id", n.ID.String()), slog.Any("error", err))
	}

	result, err := w.notifier.Send(ctx, n)
	if err != nil {
		return w.handleFailure(ctx, n, err)
	}

	if err := w.repo.UpdateStatus(ctx, n.ID, domain.StatusDelivered); err != nil {
		w.logger.ErrorContext(ctx, "update status to delivered failed",
			slog.String("id", n.ID.String()), slog.Any("error", err))
	}

	w.logger.InfoContext(ctx, "notification delivered",
		slog.String("id", n.ID.String()),
		slog.String("external_id", result.ExternalID),
		slog.String("channel", string(n.Channel)),
	)

	return nil
}

func (w *Worker) handleFailure(ctx context.Context, n *domain.Notification, sendErr error) error {
	if err := w.repo.IncrementRetry(ctx, n.ID); err != nil {
		w.logger.ErrorContext(ctx, "increment retry failed",
			slog.String("id", n.ID.String()), slog.Any("error", err))
	}
	n.RetryCount++

	if n.RetryCount >= w.maxRetries {
		w.logger.WarnContext(ctx, "max retries exceeded, routing to DLQ",
			slog.String("id", n.ID.String()),
			slog.Int("retry_count", n.RetryCount),
		)

		if err := w.repo.UpdateStatus(ctx, n.ID, domain.StatusFailed); err != nil {
			w.logger.ErrorContext(ctx, "update status to failed", slog.Any("error", err))
		}

		return w.queue.PublishDLQ(ctx, n, sendErr.Error())
	}

	delay := retryDelay(w.baseRetryDelay, n.RetryCount)

	w.logger.WarnContext(ctx, "transient failure, scheduling retry",
		slog.String("id", n.ID.String()),
		slog.Int("retry_count", n.RetryCount),
		slog.Duration("delay", delay),
	)

	return w.queue.PublishRetry(ctx, n, delay)
}

func (w *Worker) consumeRetry(ctx context.Context, consumer *kafka.Consumer) error {
	channelLimiters := map[domain.Channel]*rate.Limiter{
		domain.ChannelSMS:   rate.NewLimiter(rate.Limit(w.rateLimitPerSec), w.rateLimitPerSec),
		domain.ChannelEmail: rate.NewLimiter(rate.Limit(w.rateLimitPerSec), w.rateLimitPerSec),
		domain.ChannelPush:  rate.NewLimiter(rate.Limit(w.rateLimitPerSec), w.rateLimitPerSec),
	}

	return consumer.ConsumeWithDelay(ctx, func(msgCtx context.Context, n *domain.Notification) error {
		limiter := channelLimiters[n.Channel]
		if limiter != nil {
			if err := limiter.Wait(msgCtx); err != nil {
				return fmt.Errorf("rate limiter: %w", err)
			}
		}
		return w.processNotification(msgCtx, n, limiter)
	})
}

func retryDelay(base time.Duration, attempt int) time.Duration {
	backoff := float64(base) * math.Pow(2, float64(attempt-1))
	jitter := rand.Float64() * float64(base)
	return time.Duration(backoff + jitter)
}
