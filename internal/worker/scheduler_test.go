package worker

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestWorker_ProcessDueNotifications(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	queue := mocks.NewMockMessageQueue(ctrl)
	notifier := mocks.NewMockNotifier(ctrl)

	w := New(repo, notifier, queue, slog.Default(), nil, 100, 3, 5*time.Second)

	pastTime := time.Now().Add(-1 * time.Hour)
	notifications := []*domain.Notification{
		{ID: uuid.New(), Recipient: "+905551234567", Channel: domain.ChannelSMS, Content: "Scheduled", Priority: domain.PriorityNormal, ScheduledAt: &pastTime},
	}

	repo.EXPECT().GetDueScheduled(gomock.Any()).Return(notifications, nil)
	queue.EXPECT().Publish(gomock.Any(), notifications[0]).Return(nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), notifications[0].ID, domain.StatusQueued).Return(nil)

	w.processDueNotifications(context.Background())
}
