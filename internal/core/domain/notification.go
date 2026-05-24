package domain

import (
	"time"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

func ParseChannel(s string) (Channel, error) {
	switch Channel(s) {
	case ChannelSMS, ChannelEmail, ChannelPush:
		return Channel(s), nil
	default:
		return "", ErrInvalidChannel
	}
}

type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

func ParsePriority(s string) (Priority, error) {
	switch Priority(s) {
	case PriorityHigh, PriorityNormal, PriorityLow:
		return Priority(s), nil
	default:
		return "", ErrInvalidPriority
	}
}

func (p Priority) Weight() int {
	switch p {
	case PriorityHigh:
		return 3
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

const (
	MaxContentLengthSMS   = 160
	MaxContentLengthEmail = 100_000
	MaxContentLengthPush  = 4096
	MaxBatchSize          = 1000
	DefaultMaxRetries     = 3
)

type Notification struct {
	ID             uuid.UUID
	BatchID        uuid.UUID
	IdempotencyKey string
	Recipient      string
	Channel        Channel
	Content        string
	Priority       Priority
	Status         Status
	RetryCount     int
	MaxRetries     int
	ScheduledAt    *time.Time
	Payload        map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewNotification(recipient string, channel Channel, content string, priority Priority) (*Notification, error) {
	n := &Notification{
		ID:         uuid.New(),
		Recipient:  recipient,
		Channel:    channel,
		Content:    content,
		Priority:   priority,
		Status:     StatusPending,
		MaxRetries: DefaultMaxRetries,
		Payload:    make(map[string]any),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *Notification) Validate() error {
	if n.Recipient == "" {
		return ErrEmptyRecipient
	}
	if n.Content == "" {
		return ErrEmptyContent
	}
	if len(n.Content) > maxContentLength(n.Channel) {
		return ErrContentTooLong
	}
	return nil
}

func (n *Notification) CanRetry() bool {
	return n.RetryCount < n.MaxRetries
}

func (n *Notification) IsCancellable() bool {
	return n.Status == StatusPending || n.Status == StatusQueued
}

func maxContentLength(ch Channel) int {
	switch ch {
	case ChannelSMS:
		return MaxContentLengthSMS
	case ChannelEmail:
		return MaxContentLengthEmail
	case ChannelPush:
		return MaxContentLengthPush
	default:
		return MaxContentLengthSMS
	}
}
