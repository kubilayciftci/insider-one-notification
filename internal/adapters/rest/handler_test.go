package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports/mocks"
	"github.com/kubilayciftci/insider-one-notification/internal/core/service"
	"github.com/kubilayciftci/insider-one-notification/internal/telemetry"
	"go.uber.org/mock/gomock"
)

func setupHandler(t *testing.T) (*Handler, *mocks.MockNotificationRepository, *mocks.MockMessageQueue) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	queue := mocks.NewMockMessageQueue(ctrl)
	svc := service.NewNotificationService(repo, queue, slog.Default())
	handler := NewHandler(svc, slog.Default(), NewWSHub(slog.Default()), telemetry.NewMetrics())
	return handler, repo, queue
}

func TestHandler_CreateNotification(t *testing.T) {
	tests := []struct {
		name       string
		body       CreateNotificationRequest
		setupMock  func(*mocks.MockNotificationRepository, *mocks.MockMessageQueue)
		wantStatus int
	}{
		{
			name: "success",
			body: CreateNotificationRequest{
				Recipient: "+905551234567",
				Channel:   "sms",
				Content:   "Hello",
				Priority:  "high",
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.StatusQueued).Return(nil)
				queue.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name: "invalid channel",
			body: CreateNotificationRequest{
				Recipient: "+905551234567",
				Channel:   "fax",
				Content:   "Hello",
			},
			setupMock:  func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, repo, queue := setupHandler(t)
			tt.setupMock(repo, queue)

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/notifications", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateNotification(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandler_GetNotification(t *testing.T) {
	handler, repo, _ := setupHandler(t)
	id := uuid.New()

	repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
		ID: id, Recipient: "+905551234567", Channel: domain.ChannelSMS,
		Content: "Hello", Status: domain.StatusPending, Priority: domain.PriorityNormal,
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/notifications/"+id.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetNotification(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_CancelNotification(t *testing.T) {
	handler, repo, _ := setupHandler(t)
	id := uuid.New()

	repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
		ID: id, Status: domain.StatusPending,
	}, nil)
	repo.EXPECT().Cancel(gomock.Any(), id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/notifications/"+id.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CancelNotification(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandler_ListNotifications(t *testing.T) {
	handler, repo, _ := setupHandler(t)

	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
		[]*domain.Notification{{}, {}}, int64(2), nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/notifications?status=pending&page=1&page_size=10", nil)
	w := httptest.NewRecorder()

	handler.ListNotifications(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp ListResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

func TestHandler_HealthCheck(t *testing.T) {
	handler, _, _ := setupHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
