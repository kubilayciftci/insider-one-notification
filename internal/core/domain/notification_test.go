package domain

import (
	"strings"
	"testing"
)

func TestParseChannel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Channel
		wantErr error
	}{
		{name: "valid sms", input: "sms", want: ChannelSMS, wantErr: nil},
		{name: "valid email", input: "email", want: ChannelEmail, wantErr: nil},
		{name: "valid push", input: "push", want: ChannelPush, wantErr: nil},
		{name: "invalid channel", input: "whatsapp", want: "", wantErr: ErrInvalidChannel},
		{name: "empty string", input: "", want: "", wantErr: ErrInvalidChannel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChannel(tt.input)
			if err != tt.wantErr {
				t.Errorf("ParseChannel(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseChannel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Priority
		wantErr error
	}{
		{name: "high", input: "high", want: PriorityHigh, wantErr: nil},
		{name: "normal", input: "normal", want: PriorityNormal, wantErr: nil},
		{name: "low", input: "low", want: PriorityLow, wantErr: nil},
		{name: "invalid", input: "urgent", want: "", wantErr: ErrInvalidPriority},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePriority(tt.input)
			if err != tt.wantErr {
				t.Errorf("ParsePriority(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParsePriority(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewNotification(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		channel   Channel
		content   string
		priority  Priority
		wantErr   error
	}{
		{
			name:      "valid sms notification",
			recipient: "+905551234567",
			channel:   ChannelSMS,
			content:   "Hello world",
			priority:  PriorityNormal,
			wantErr:   nil,
		},
		{
			name:      "empty recipient",
			recipient: "",
			channel:   ChannelSMS,
			content:   "Hello",
			priority:  PriorityNormal,
			wantErr:   ErrEmptyRecipient,
		},
		{
			name:      "empty content",
			recipient: "+905551234567",
			channel:   ChannelSMS,
			content:   "",
			priority:  PriorityNormal,
			wantErr:   ErrEmptyContent,
		},
		{
			name:      "sms content too long",
			recipient: "+905551234567",
			channel:   ChannelSMS,
			content:   strings.Repeat("a", MaxContentLengthSMS+1),
			priority:  PriorityHigh,
			wantErr:   ErrContentTooLong,
		},
		{
			name:      "email content at max length",
			recipient: "user@example.com",
			channel:   ChannelEmail,
			content:   strings.Repeat("a", MaxContentLengthEmail),
			priority:  PriorityLow,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNotification(tt.recipient, tt.channel, tt.content, tt.priority)
			if err != tt.wantErr {
				t.Fatalf("NewNotification() error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if n.Status != StatusPending {
				t.Errorf("initial status = %v, want %v", n.Status, StatusPending)
			}
			if n.MaxRetries != DefaultMaxRetries {
				t.Errorf("max retries = %d, want %d", n.MaxRetries, DefaultMaxRetries)
			}
			if n.ID.String() == "" {
				t.Error("expected non-empty UUID")
			}
		})
	}
}

func TestNotification_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		maxRetries int
		want       bool
	}{
		{name: "can retry - 0 of 3", retryCount: 0, maxRetries: 3, want: true},
		{name: "can retry - 2 of 3", retryCount: 2, maxRetries: 3, want: true},
		{name: "cannot retry - at max", retryCount: 3, maxRetries: 3, want: false},
		{name: "cannot retry - over max", retryCount: 5, maxRetries: 3, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{RetryCount: tt.retryCount, MaxRetries: tt.maxRetries}
			if got := n.CanRetry(); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotification_IsCancellable(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "pending is cancellable", status: StatusPending, want: true},
		{name: "queued is cancellable", status: StatusQueued, want: true},
		{name: "processing is not cancellable", status: StatusProcessing, want: false},
		{name: "delivered is not cancellable", status: StatusDelivered, want: false},
		{name: "failed is not cancellable", status: StatusFailed, want: false},
		{name: "cancelled is not cancellable", status: StatusCancelled, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{Status: tt.status}
			if got := n.IsCancellable(); got != tt.want {
				t.Errorf("IsCancellable() = %v, want %v", got, tt.want)
			}
		})
	}
}
