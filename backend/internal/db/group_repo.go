package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

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

// ListForUser retrieves all groups a user is a member of
func (r *GroupRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]*models.Group, error) {
	query := `
		SELECT g.id, g.name, g.head_user_id, g.created_at
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups for user: %w", err)
	}
	defer rows.Close()

	var groups []*models.Group
	for rows.Next() {
		group := &models.Group{}
		if err := rows.Scan(&group.ID, &group.Name, &group.HeadUserID, &group.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
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
