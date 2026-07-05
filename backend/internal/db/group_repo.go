package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// GroupSummary is a dashboard-listing row: a group plus the caller's role, member count,
// and a role-dependent summary balance (minor units). See GET /groups.
type GroupSummary struct {
	ID             uuid.UUID
	Name           string
	HeadUserID     uuid.UUID
	CreatedAt      time.Time
	Role           models.MemberRole
	MemberCount    int
	SummaryBalance int64
}

// GroupRepo handles database operations for groups
type GroupRepo struct {
	pool *pgxpool.Pool
}

// NewGroupRepo creates a new GroupRepo
func NewGroupRepo(pool *pgxpool.Pool) *GroupRepo {
	return &GroupRepo{pool: pool}
}

// Create inserts a new group into the database
func (r *GroupRepo) Create(ctx context.Context, name string, headUserID uuid.UUID) (*models.Group, error) {
	group := &models.Group{
		ID:         uuid.New(),
		Name:       name,
		HeadUserID: headUserID,
	}

	query := `
		INSERT INTO groups (id, name, head_user_id)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`

	err := r.pool.QueryRow(ctx, query, group.ID, name, headUserID).Scan(&group.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

// GetByID retrieves a group by ID
func (r *GroupRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	group := &models.Group{}

	query := `
		SELECT id, name, head_user_id, created_at
		FROM groups
		WHERE id = $1
	`

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&group.ID,
		&group.Name,
		&group.HeadUserID,
		&group.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get group by id: %w", err)
	}

	return group, nil
}

// ListForUserWithSummary returns every group the user belongs to, each enriched with the caller's
// role, the member count, and a summary balance: for head groups the sum of all non-head members'
// balances (total owed); for member groups the caller's own balance. Balances come from currently
// approved ledger entries only (no posting is triggered — see WP-4.2 §0.2).
func (r *GroupRepo) ListForUserWithSummary(ctx context.Context, userID uuid.UUID) ([]*GroupSummary, error) {
	query := `
		WITH my_groups AS (
			SELECT g.id, g.name, g.head_user_id, g.created_at, gm.role
			FROM groups g
			JOIN group_members gm ON gm.group_id = g.id
			WHERE gm.user_id = $1
		),
		member_counts AS (
			SELECT group_id, COUNT(*)::int AS member_count
			FROM group_members
			WHERE group_id IN (SELECT id FROM my_groups)
			GROUP BY group_id
		),
		member_balances AS (
			SELECT le.group_id, le.user_id,
				(COALESCE(SUM(CASE WHEN le.direction = 'credit' THEN le.amount ELSE 0 END), 0)
			   - COALESCE(SUM(CASE WHEN le.direction = 'debit'  THEN le.amount ELSE 0 END), 0))::bigint AS balance
			FROM ledger_entries le
			WHERE le.status = 'approved'
			  AND le.group_id IN (SELECT id FROM my_groups)
			GROUP BY le.group_id, le.user_id
		),
		head_totals AS (
			SELECT mb.group_id, COALESCE(SUM(mb.balance), 0)::bigint AS total_owed
			FROM member_balances mb
			JOIN group_members gm ON gm.group_id = mb.group_id AND gm.user_id = mb.user_id
			WHERE gm.role <> 'head'
			GROUP BY mb.group_id
		)
		SELECT mg.id, mg.name, mg.head_user_id, mg.created_at, mg.role,
			COALESCE(mc.member_count, 0) AS member_count,
			CASE WHEN mg.role = 'head'
				 THEN COALESCE(ht.total_owed, 0)
				 ELSE COALESCE(ob.balance, 0)
			END::bigint AS summary_balance
		FROM my_groups mg
		LEFT JOIN member_counts mc ON mc.group_id = mg.id
		LEFT JOIN head_totals   ht ON ht.group_id = mg.id
		LEFT JOIN member_balances ob ON ob.group_id = mg.id AND ob.user_id = $1
		ORDER BY mg.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list group summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]*GroupSummary, 0)
	for rows.Next() {
		s := &GroupSummary{}
		if err := rows.Scan(&s.ID, &s.Name, &s.HeadUserID, &s.CreatedAt,
			&s.Role, &s.MemberCount, &s.SummaryBalance); err != nil {
			return nil, fmt.Errorf("failed to scan group summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// AddMember adds a user to a group
func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID uuid.UUID, role models.MemberRole) (*models.GroupMember, error) {
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  userID,
		Role:    role,
	}

	query := `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING joined_at
	`

	err := r.pool.QueryRow(ctx, query, groupID, userID, role).Scan(&member.JoinedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("user is already a member of this group")
		}
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	return member, nil
}

// GetMember retrieves a member from a group
func (r *GroupRepo) GetMember(ctx context.Context, groupID, userID uuid.UUID) (*models.GroupMember, error) {
	member := &models.GroupMember{}

	query := `
		SELECT group_id, user_id, role, joined_at
		FROM group_members
		WHERE group_id = $1 AND user_id = $2
	`

	err := r.pool.QueryRow(ctx, query, groupID, userID).Scan(
		&member.GroupID,
		&member.UserID,
		&member.Role,
		&member.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	return member, nil
}

// ListMembers retrieves all members of a group with user details
func (r *GroupRepo) ListMembers(ctx context.Context, groupID uuid.UUID) ([]*models.MemberWithUser, error) {
	query := `
		SELECT gm.group_id, gm.user_id, gm.role, gm.joined_at, u.name, u.email
		FROM group_members gm
		INNER JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC
	`

	rows, err := r.pool.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	defer rows.Close()

	var members []*models.MemberWithUser
	for rows.Next() {
		member := &models.MemberWithUser{}
		if err := rows.Scan(
			&member.GroupID,
			&member.UserID,
			&member.Role,
			&member.JoinedAt,
			&member.Name,
			&member.Email,
		); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, member)
	}

	return members, nil
}

// CountChores returns the number of chores in a group
func (r *GroupRepo) CountChores(ctx context.Context, groupID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM chores WHERE group_id = $1`
	err := r.pool.QueryRow(ctx, query, groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count chores: %w", err)
	}
	return count, nil
}
