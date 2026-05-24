package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	kgo "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type MessageHandler func(ctx context.Context, notification *domain.Notification) error

type Consumer struct {
	reader *kgo.Reader
	logger *slog.Logger
}

func NewConsumer(brokers []string, topic, groupID string, logger *slog.Logger) *Consumer {
	reader := kgo.NewReader(kgo.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
	})

	return &Consumer{
		reader: reader,
		logger: logger,
	}
}

func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		spanCtx := extractTraceContext(ctx, msg.Headers)

		var notification domain.Notification
		if err := json.Unmarshal(msg.Value, &notification); err != nil {
			c.logger.ErrorContext(spanCtx, "unmarshal notification failed",
				slog.Any("error", err),
				slog.String("topic", msg.Topic),
			)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.ErrorContext(spanCtx, "commit failed", slog.Any("error", err))
			}
			continue
		}

		if err := handler(spanCtx, &notification); err != nil {
			c.logger.ErrorContext(spanCtx, "handle notification failed",
				slog.String("id", notification.ID.String()),
				slog.Any("error", err),
			)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.ErrorContext(spanCtx, "commit offset failed",
				slog.String("id", notification.ID.String()),
				slog.Any("error", err),
			)
		}
	}
}

func (c *Consumer) ConsumeWithDelay(ctx context.Context, handler MessageHandler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		spanCtx := extractTraceContext(ctx, msg.Headers)

		retryAfter := extractHeader(msg.Headers, HeaderRetryAfter)
		if retryAfter != "" {
			t, parseErr := time.Parse(time.RFC3339, retryAfter)
			if parseErr == nil && time.Now().Before(t) {
				select {
				case <-time.After(time.Until(t)):
				case <-ctx.Done():
					return nil
				}
			}
		}

		var notification domain.Notification
		if err := json.Unmarshal(msg.Value, &notification); err != nil {
			c.logger.ErrorContext(spanCtx, "unmarshal notification failed",
				slog.Any("error", err), slog.String("topic", msg.Topic))
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err := handler(spanCtx, &notification); err != nil {
			c.logger.ErrorContext(spanCtx, "handle notification failed",
				slog.String("id", notification.ID.String()), slog.Any("error", err))
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.ErrorContext(spanCtx, "commit offset failed",
				slog.String("id", notification.ID.String()), slog.Any("error", err))
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func extractHeader(headers []kgo.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func extractTraceContext(ctx context.Context, headers []kgo.Header) context.Context {
	carrier := propagation.MapCarrier{}
	for _, h := range headers {
		carrier.Set(h.Key, string(h.Value))
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
