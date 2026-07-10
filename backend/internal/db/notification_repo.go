package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationRepo handles database operations for in-app notifications (§3.7).
// INSERT-ONLY in this WP; the list/unread/mark-read READ API is Phase 5 (V3-5.1).
type NotificationRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationRepo creates a new NotificationRepo.
func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

// Insert writes one notification. q lets it join the caller's transaction so the
// notification is atomic with the membership/claim change. payload is a JSON
// object (marshaled by the caller). INSERT-ONLY — no read methods in this WP.
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
