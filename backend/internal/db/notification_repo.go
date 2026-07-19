package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// NotificationRepo handles database operations for in-app notifications (§3.7).
type NotificationRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationRepo creates a new NotificationRepo.
func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

// Insert writes one notification. q lets it join the caller's transaction so the
// notification is atomic with the membership/claim change. payload is a JSON
// object (marshaled by the caller).
func (r *NotificationRepo) Insert(ctx context.Context, q Querier,
	userID uuid.UUID, ntype string, payload []byte) error {
	_, err := q.Exec(ctx,
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, $2, $3)`,
		userID, ntype, payload)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}
	return nil
}

// List returns up to limit of userID's notifications, newest first, keyset-paged.
// afterCreatedAt/afterID are the previous page's last row (both nil = first page).
// Fetches limit+1 rows internally and returns hasMore so the handler can build the
// next_cursor without an extra round-trip.
func (r *NotificationRepo) List(ctx context.Context, userID uuid.UUID, limit int,
	afterCreatedAt *time.Time, afterID *uuid.UUID) ([]*models.Notification, bool, error) {

	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, type, payload, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		  AND ( $2::timestamptz IS NULL
		        OR (created_at, id) < ($2::timestamptz, $3::uuid) )
		ORDER BY created_at DESC, id DESC
		LIMIT $4`,
		userID, afterCreatedAt, afterID, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list notifications: %w", err)
	}
	defer rows.Close()

	var items []*models.Notification
	for rows.Next() {
		n := &models.Notification{}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("failed to scan notification: %w", err)
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("failed to iterate notifications: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

// UnreadCount returns the number of unread notifications for userID.
func (r *NotificationRepo) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`,
		userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead marks notification id read for userID. Idempotent (COALESCE preserves
// existing read_at). Returns found=false when the row is absent or not owned by
// userID — both cases map to 404 at the handler (D6: no existence leak).
func (r *NotificationRepo) MarkRead(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE notifications
		 SET read_at = COALESCE(read_at, now())
		 WHERE id = $1 AND user_id = $2
		 RETURNING id`,
		id, userID)
	if err != nil {
		return false, fmt.Errorf("failed to mark notification read: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("failed to scan mark-read result: %w", err)
	}
	return found, nil
}

// MarkAllRead marks every unread notification for userID read; returns rows affected.
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`,
		userID)
	if err != nil {
		return 0, fmt.Errorf("failed to mark all notifications read: %w", err)
	}
	return tag.RowsAffected(), nil
}
