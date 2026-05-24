package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
)

func (w *Worker) StartScheduler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.logger.Info("scheduler started", slog.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			w.processDueNotifications(ctx)
		}
	}
}

func (w *Worker) processDueNotifications(ctx context.Context) {
	notifications, err := w.repo.GetDueScheduled(ctx)
	if err != nil {
		w.logger.ErrorContext(ctx, "fetch due scheduled notifications", slog.Any("error", err))
		return
	}

	if len(notifications) == 0 {
		return
	}

	w.logger.InfoContext(ctx, "processing due scheduled notifications", slog.Int("count", len(notifications)))

	for _, n := range notifications {
		if err := w.queue.Publish(ctx, n); err != nil {
			w.logger.ErrorContext(ctx, "publish scheduled notification",
				slog.String("id", n.ID.String()), slog.Any("error", err))
			continue
		}
		if err := w.repo.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
			w.logger.ErrorContext(ctx, "update scheduled notification status",
				slog.String("id", n.ID.String()), slog.Any("error", err))
		}
	}
}
