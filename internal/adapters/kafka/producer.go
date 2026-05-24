package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	TopicPrefix       = "notifications-"
	RetryTopic        = "notifications-retry"
	DLQTopic          = "notifications-dlq"
	HeaderRetryCount  = "retry-count"
	HeaderRetryAfter  = "retry-after"
	HeaderOrigChannel = "original-channel"
	HeaderFailReason  = "failure-reason"
)

func TopicForChannel(ch domain.Channel) string {
	return TopicPrefix + string(ch)
}

func TopicForChannelPriority(ch domain.Channel, p domain.Priority) string {
	return TopicPrefix + string(ch) + "-" + string(p)
}

type Producer struct {
	writer      *kafka.Writer
	retryWriter *kafka.Writer
	dlqWriter   *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer:      newWriter(brokers),
		retryWriter: newTopicWriter(brokers, RetryTopic),
		dlqWriter:   newTopicWriter(brokers, DLQTopic),
	}
}

func newWriter(brokers []string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
	}
}

func newTopicWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireAll,
	}
}

func (p *Producer) Publish(ctx context.Context, notification *domain.Notification) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	headers := injectTraceContext(ctx)
	headers = append(headers, kafka.Header{Key: "priority", Value: []byte(string(notification.Priority))})

	msg := kafka.Message{
		Topic:   TopicForChannelPriority(notification.Channel, notification.Priority),
		Key:     []byte(notification.ID.String()),
		Value:   body,
		Headers: headers,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish to %s: %w", msg.Topic, err)
	}
	return nil
}

func (p *Producer) PublishRetry(ctx context.Context, notification *domain.Notification, delay time.Duration) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	retryAfter := time.Now().Add(delay)

	headers := injectTraceContext(ctx)
	headers = append(headers,
		kafka.Header{Key: HeaderRetryCount, Value: []byte(strconv.Itoa(notification.RetryCount))},
		kafka.Header{Key: HeaderRetryAfter, Value: []byte(retryAfter.Format(time.RFC3339))},
		kafka.Header{Key: HeaderOrigChannel, Value: []byte(string(notification.Channel))},
	)

	msg := kafka.Message{
		Key:     []byte(notification.ID.String()),
		Value:   body,
		Headers: headers,
	}

	if err := p.retryWriter.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish to retry: %w", err)
	}
	return nil
}

func (p *Producer) PublishDLQ(ctx context.Context, notification *domain.Notification, reason string) error {
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	headers := injectTraceContext(ctx)
	headers = append(headers,
		kafka.Header{Key: HeaderFailReason, Value: []byte(reason)},
		kafka.Header{Key: HeaderOrigChannel, Value: []byte(string(notification.Channel))},
		kafka.Header{Key: HeaderRetryCount, Value: []byte(strconv.Itoa(notification.RetryCount))},
	)

	msg := kafka.Message{
		Key:     []byte(notification.ID.String()),
		Value:   body,
		Headers: headers,
	}

	if err := p.dlqWriter.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish to dlq: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	var firstErr error
	if err := p.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := p.retryWriter.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := p.dlqWriter.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func injectTraceContext(ctx context.Context) []kafka.Header {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	var headers []kafka.Header
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	return headers
}
