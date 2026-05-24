//go:build integration

package postgres

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	_, thisFile, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")

	m, err := migrate.New("file://"+migrationsPath, connStr)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	m.Close()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	repo := NewRepository(pool)
	cleanup := func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}

	return repo, cleanup
}

func TestRepository_CreateAndGetByID(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	n, _ := domain.NewNotification("+905551234567", domain.ChannelSMS, "Hello", domain.PriorityNormal)

	if err := repo.Create(ctx, n); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Recipient != n.Recipient {
		t.Errorf("recipient = %v, want %v", got.Recipient, n.Recipient)
	}
	if got.Channel != n.Channel {
		t.Errorf("channel = %v, want %v", got.Channel, n.Channel)
	}
	if got.Status != domain.StatusPending {
		t.Errorf("status = %v, want %v", got.Status, domain.StatusPending)
	}
}

func TestRepository_CreateBatch(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	batchID := uuid.New()
	notifications := make([]*domain.Notification, 3)
	for i := range notifications {
		n, _ := domain.NewNotification("+9055512345"+string(rune('0'+i)), domain.ChannelSMS, "Batch msg", domain.PriorityHigh)
		n.BatchID = batchID
		notifications[i] = n
	}

	if err := repo.CreateBatch(ctx, notifications); err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}

	got, err := repo.GetByBatchID(ctx, batchID)
	if err != nil {
		t.Fatalf("GetByBatchID() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d notifications, want 3", len(got))
	}
}

func TestRepository_IdempotencyKey(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	n1, _ := domain.NewNotification("+905551234567", domain.ChannelSMS, "Hello", domain.PriorityNormal)
	n1.IdempotencyKey = "unique-key-123"

	if err := repo.Create(ctx, n1); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	n2, _ := domain.NewNotification("+905559999999", domain.ChannelSMS, "Duplicate", domain.PriorityNormal)
	n2.IdempotencyKey = "unique-key-123"

	err := repo.Create(ctx, n2)
	if err != domain.ErrDuplicateKey {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}
}

func TestRepository_UpdateStatusAndCancel(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	n, _ := domain.NewNotification("+905551234567", domain.ChannelSMS, "Hello", domain.PriorityNormal)
	_ = repo.Create(ctx, n)

	if err := repo.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	got, _ := repo.GetByID(ctx, n.ID)
	if got.Status != domain.StatusQueued {
		t.Errorf("status = %v, want queued", got.Status)
	}

	if err := repo.Cancel(ctx, n.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	got, _ = repo.GetByID(ctx, n.ID)
	if got.Status != domain.StatusCancelled {
		t.Errorf("status = %v, want cancelled", got.Status)
	}
}

func TestRepository_List(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		n, _ := domain.NewNotification("+9055500000"+string(rune('0'+i)), domain.ChannelSMS, "Hello", domain.PriorityNormal)
		_ = repo.Create(ctx, n)
	}
	for i := 0; i < 3; i++ {
		n, _ := domain.NewNotification("user@example.com", domain.ChannelEmail, "Email body", domain.PriorityHigh)
		_ = repo.Create(ctx, n)
	}

	tests := []struct {
		name      string
		filter    ports.ListFilter
		wantCount int
		wantTotal int64
	}{
		{
			name:      "all notifications page 1",
			filter:    ports.ListFilter{Page: 1, PageSize: 10},
			wantCount: 8,
			wantTotal: 8,
		},
		{
			name:      "filter by channel sms",
			filter:    ports.ListFilter{Channel: channelPtr(domain.ChannelSMS), Page: 1, PageSize: 10},
			wantCount: 5,
			wantTotal: 5,
		},
		{
			name:      "pagination",
			filter:    ports.ListFilter{Page: 1, PageSize: 3},
			wantCount: 3,
			wantTotal: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifications, total, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(notifications) != tt.wantCount {
				t.Errorf("got %d notifications, want %d", len(notifications), tt.wantCount)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

func TestRepository_IncrementRetry(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	n, _ := domain.NewNotification("+905551234567", domain.ChannelSMS, "Hello", domain.PriorityNormal)
	_ = repo.Create(ctx, n)

	_ = repo.IncrementRetry(ctx, n.ID)
	_ = repo.IncrementRetry(ctx, n.ID)

	got, _ := repo.GetByID(ctx, n.ID)
	if got.RetryCount != 2 {
		t.Errorf("retry_count = %d, want 2", got.RetryCount)
	}
}

func channelPtr(c domain.Channel) *domain.Channel { return &c }
