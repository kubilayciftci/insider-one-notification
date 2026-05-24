package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubilayciftci/insider-one-notification/internal/core/domain"
	"github.com/kubilayciftci/insider-one-notification/internal/core/ports"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, n *domain.Notification) error {
	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO notifications (id, batch_id, idempotency_key, recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at)
		 VALUES ($1, $2, NULLIF($3,''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		n.ID, nullUUID(n.BatchID), n.IdempotencyKey,
		n.Recipient, string(n.Channel), n.Content, string(n.Priority), string(n.Status),
		n.RetryCount, n.MaxRetries, n.ScheduledAt, payload, n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return domain.ErrDuplicateKey
		}
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *Repository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	batch := &pgx.Batch{}
	for _, n := range notifications {
		payload, err := json.Marshal(n.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		batch.Queue(
			`INSERT INTO notifications (id, batch_id, idempotency_key, recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at)
			 VALUES ($1, $2, NULLIF($3,''), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			n.ID, nullUUID(n.BatchID), n.IdempotencyKey,
			n.Recipient, string(n.Channel), n.Content, string(n.Priority), string(n.Status),
			n.RetryCount, n.MaxRetries, n.ScheduledAt, payload, n.CreatedAt, n.UpdatedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close() //nolint:errcheck

	for range notifications {
		if _, err := br.Exec(); err != nil {
			if isDuplicateKeyError(err) {
				return domain.ErrDuplicateKey
			}
			return fmt.Errorf("batch insert: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, batch_id, COALESCE(idempotency_key,''), recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at
		 FROM notifications WHERE id = $1`, id)

	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get by id: %w", err)
	}
	return n, nil
}

func (r *Repository) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, batch_id, COALESCE(idempotency_key,''), recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at
		 FROM notifications WHERE batch_id = $1 ORDER BY created_at`, batchID)
	if err != nil {
		return nil, fmt.Errorf("get by batch id: %w", err)
	}
	defer rows.Close()

	return collectNotifications(rows)
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET status = $1 WHERE id = $2`, string(status), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) Cancel(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET status = $1 WHERE id = $2 AND status IN ($3, $4)`,
		string(domain.StatusCancelled), id, string(domain.StatusPending), string(domain.StatusQueued))
	if err != nil {
		return fmt.Errorf("cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotCancellable
	}
	return nil
}

func (r *Repository) List(ctx context.Context, filter ports.ListFilter) ([]*domain.Notification, int64, error) {
	query := `SELECT id, batch_id, COALESCE(idempotency_key,''), recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at
			  FROM notifications WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM notifications WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filter.Status != nil {
		clause := fmt.Sprintf(" AND status = $%d", argIdx)
		query += clause
		countQuery += clause
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.Channel != nil {
		clause := fmt.Sprintf(" AND channel = $%d", argIdx)
		query += clause
		countQuery += clause
		args = append(args, string(*filter.Channel))
		argIdx++
	}
	if filter.FromDate != nil {
		clause := fmt.Sprintf(" AND created_at >= $%d", argIdx)
		query += clause
		countQuery += clause
		args = append(args, *filter.FromDate)
		argIdx++
	}
	if filter.ToDate != nil {
		clause := fmt.Sprintf(" AND created_at <= $%d", argIdx)
		query += clause
		countQuery += clause
		args = append(args, *filter.ToDate)
		argIdx++
	}

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.PageSize

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	notifications, err := collectNotifications(rows)
	if err != nil {
		return nil, 0, err
	}

	return notifications, total, nil
}

func (r *Repository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET retry_count = retry_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("increment retry: %w", err)
	}
	return nil
}

func (r *Repository) GetDueScheduled(ctx context.Context) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, batch_id, COALESCE(idempotency_key,''), recipient, channel, content, priority, status, retry_count, max_retries, scheduled_at, payload, created_at, updated_at
		 FROM notifications WHERE scheduled_at <= NOW() AND status = $1 ORDER BY scheduled_at`,
		string(domain.StatusPending))
	if err != nil {
		return nil, fmt.Errorf("get due scheduled: %w", err)
	}
	defer rows.Close()

	return collectNotifications(rows)
}

func scanNotification(row pgx.Row) (*domain.Notification, error) {
	var n domain.Notification
	var batchID *uuid.UUID
	var channel, priority, status string
	var payloadBytes []byte

	err := row.Scan(
		&n.ID, &batchID, &n.IdempotencyKey,
		&n.Recipient, &channel, &n.Content, &priority, &status,
		&n.RetryCount, &n.MaxRetries, &n.ScheduledAt, &payloadBytes,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if batchID != nil {
		n.BatchID = *batchID
	}
	n.Channel = domain.Channel(channel)
	n.Priority = domain.Priority(priority)
	n.Status = domain.Status(status)

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &n.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	return &n, nil
}

func collectNotifications(rows pgx.Rows) ([]*domain.Notification, error) {
	var result []*domain.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func nullUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
