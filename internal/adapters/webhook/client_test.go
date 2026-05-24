package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

func TestClient_Send(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   map[string]string
		wantErr    bool
	}{
		{
			name:       "successful delivery",
			statusCode: http.StatusAccepted,
			response: map[string]string{
				"messageId": "ext-123",
				"status":    "accepted",
				"timestamp": "2026-05-24T10:00:00Z",
			},
			wantErr: false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"error": "internal"},
			wantErr:    true,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			response:   map[string]string{"error": "rate limited"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("content-type = %s, want application/json", r.Header.Get("Content-Type"))
				}
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL, 10)
			n, _ := domain.NewNotification("+905551234567", domain.ChannelSMS, "Test", domain.PriorityNormal)

			result, err := client.Send(context.Background(), n)
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if result.ExternalID != "ext-123" {
					t.Errorf("ExternalID = %v, want ext-123", result.ExternalID)
				}
			}
		})
	}
}
