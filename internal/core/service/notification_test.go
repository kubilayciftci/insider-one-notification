package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*NotificationService, *mocks.MockNotificationRepository, *mocks.MockMessageQueue) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	queue := mocks.NewMockMessageQueue(ctrl)
	logger := slog.Default()
	svc := NewNotificationService(repo, queue, logger)
	return svc, repo, queue
}

func TestNotificationService_Create(t *testing.T) {
	tests := []struct {
		name      string
		req       CreateRequest
		setupMock func(*mocks.MockNotificationRepository, *mocks.MockMessageQueue)
		wantErr   bool
	}{
		{
			name: "successful creation",
			req: CreateRequest{
				Recipient: "+905551234567",
				Channel:   domain.ChannelSMS,
				Content:   "Hello",
				Priority:  domain.PriorityNormal,
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.StatusQueued).Return(nil)
				queue.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "validation failure - empty recipient",
			req: CreateRequest{
				Recipient: "",
				Channel:   domain.ChannelSMS,
				Content:   "Hello",
				Priority:  domain.PriorityNormal,
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {},
			wantErr:   true,
		},
		{
			name: "validation failure - empty content",
			req: CreateRequest{
				Recipient: "+905551234567",
				Channel:   domain.ChannelSMS,
				Content:   "",
				Priority:  domain.PriorityNormal,
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {},
			wantErr:   true,
		},
		{
			name: "repository failure",
			req: CreateRequest{
				Recipient: "+905551234567",
				Channel:   domain.ChannelSMS,
				Content:   "Hello",
				Priority:  domain.PriorityNormal,
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "queue failure rolls back status",
			req: CreateRequest{
				Recipient: "+905551234567",
				Channel:   domain.ChannelSMS,
				Content:   "Hello",
				Priority:  domain.PriorityNormal,
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
				repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.StatusQueued).Return(nil)
				queue.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(errors.New("kafka down"))
				repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.StatusPending).Return(nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, queue := newTestService(t)
			tt.setupMock(repo, queue)

			n, err := svc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && n == nil {
				t.Error("Create() returned nil notification on success")
			}
		})
	}
}

func TestNotificationService_CreateBatch(t *testing.T) {
	tests := []struct {
		name      string
		requests  []CreateRequest
		setupMock func(*mocks.MockNotificationRepository, *mocks.MockMessageQueue)
		wantErr   bool
		wantCount int
	}{
		{
			name: "successful batch",
			requests: []CreateRequest{
				{Recipient: "+905551111111", Channel: domain.ChannelSMS, Content: "Msg1", Priority: domain.PriorityHigh},
				{Recipient: "+905552222222", Channel: domain.ChannelEmail, Content: "Msg2", Priority: domain.PriorityNormal},
			},
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {
				repo.EXPECT().CreateBatch(gomock.Any(), gomock.Len(2)).Return(nil)
				queue.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), domain.StatusQueued).Return(nil).Times(2)
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:      "batch too large",
			requests:  make([]CreateRequest, domain.MaxBatchSize+1),
			setupMock: func(repo *mocks.MockNotificationRepository, queue *mocks.MockMessageQueue) {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, queue := newTestService(t)
			tt.setupMock(repo, queue)

			batchID, notifications, err := svc.CreateBatch(context.Background(), tt.requests)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if batchID == uuid.Nil {
					t.Error("expected non-nil batch ID")
				}
				if len(notifications) != tt.wantCount {
					t.Errorf("got %d notifications, want %d", len(notifications), tt.wantCount)
				}
			}
		})
	}
}

func TestNotificationService_Cancel(t *testing.T) {
	tests := []struct {
		name      string
		status    domain.Status
		setupMock func(*mocks.MockNotificationRepository, uuid.UUID)
		wantErr   error
	}{
		{
			name:   "cancel pending notification",
			status: domain.StatusPending,
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
					ID: id, Status: domain.StatusPending,
				}, nil)
				repo.EXPECT().Cancel(gomock.Any(), id).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "cancel queued notification",
			status: domain.StatusQueued,
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
					ID: id, Status: domain.StatusQueued,
				}, nil)
				repo.EXPECT().Cancel(gomock.Any(), id).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "cannot cancel delivered notification",
			status: domain.StatusDelivered,
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
					ID: id, Status: domain.StatusDelivered,
				}, nil)
			},
			wantErr: domain.ErrNotCancellable,
		},
		{
			name:   "cannot cancel failed notification",
			status: domain.StatusFailed,
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
					ID: id, Status: domain.StatusFailed,
				}, nil)
			},
			wantErr: domain.ErrNotCancellable,
		},
		{
			name:   "notification not found",
			status: domain.StatusPending,
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, domain.ErrNotFound)
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newTestService(t)
			id := uuid.New()
			tt.setupMock(repo, id)

			err := svc.Cancel(context.Background(), id)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Cancel() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotificationService_GetByID(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockNotificationRepository, uuid.UUID)
		wantErr   bool
	}{
		{
			name: "successful retrieval",
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(&domain.Notification{
					ID: id, Recipient: "+905551234567",
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			setupMock: func(repo *mocks.MockNotificationRepository, id uuid.UUID) {
				repo.EXPECT().GetByID(gomock.Any(), id).Return(nil, domain.ErrNotFound)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newTestService(t)
			id := uuid.New()
			tt.setupMock(repo, id)

			got, err := svc.GetByID(context.Background(), id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.ID != id {
				t.Errorf("got ID %v, want %v", got.ID, id)
			}
		})
	}
}

func TestNotificationService_GetByBatchID(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mocks.MockNotificationRepository, uuid.UUID)
		wantErr   bool
		wantCount int
	}{
		{
			name: "successful batch retrieval",
			setupMock: func(repo *mocks.MockNotificationRepository, batchID uuid.UUID) {
				repo.EXPECT().GetByBatchID(gomock.Any(), batchID).Return([]*domain.Notification{
					{ID: uuid.New(), BatchID: batchID},
					{ID: uuid.New(), BatchID: batchID},
				}, nil)
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "empty batch",
			setupMock: func(repo *mocks.MockNotificationRepository, batchID uuid.UUID) {
				repo.EXPECT().GetByBatchID(gomock.Any(), batchID).Return(nil, nil)
			},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newTestService(t)
			batchID := uuid.New()
			tt.setupMock(repo, batchID)

			notifications, err := svc.GetByBatchID(context.Background(), batchID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByBatchID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(notifications) != tt.wantCount {
				t.Errorf("got %d notifications, want %d", len(notifications), tt.wantCount)
			}
		})
	}
}

func TestNotificationService_List(t *testing.T) {
	tests := []struct {
		name      string
		filter    ports.ListFilter
		setupMock func(*mocks.MockNotificationRepository)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name: "list with status filter",
			filter: func() ports.ListFilter {
				status := domain.StatusPending
				return ports.ListFilter{Status: &status, Page: 1, PageSize: 10}
			}(),
			setupMock: func(repo *mocks.MockNotificationRepository) {
				repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					[]*domain.Notification{{}, {}}, int64(2), nil,
				)
			},
			wantErr:   false,
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:   "list with no filters",
			filter: ports.ListFilter{Page: 1, PageSize: 20},
			setupMock: func(repo *mocks.MockNotificationRepository) {
				repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					[]*domain.Notification{{}, {}, {}}, int64(3), nil,
				)
			},
			wantErr:   false,
			wantCount: 3,
			wantTotal: 3,
		},
		{
			name:   "repository error",
			filter: ports.ListFilter{Page: 1, PageSize: 10},
			setupMock: func(repo *mocks.MockNotificationRepository) {
				repo.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					nil, int64(0), errors.New("db error"),
				)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newTestService(t)
			tt.setupMock(repo)

			notifications, total, err := svc.List(context.Background(), tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(notifications) != tt.wantCount {
					t.Errorf("got %d notifications, want %d", len(notifications), tt.wantCount)
				}
				if total != tt.wantTotal {
					t.Errorf("got total %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}
