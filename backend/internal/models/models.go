package models

import (
	"time"

	"github.com/google/uuid"
)

// MemberRole represents the role of a user in a group
type MemberRole string

const (
	RoleHead   MemberRole = "head"
	RoleMember MemberRole = "member"
)

// LedgerStatus represents the status of a ledger entry
type LedgerStatus string

const (
	StatusApproved        LedgerStatus = "approved"
	StatusPendingApproval LedgerStatus = "pending_approval"
	StatusRejected        LedgerStatus = "rejected"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never expose password hash
	Name         string     `json:"name"`
	DOB          *time.Time `json:"dob,omitempty"`
	Sex          *string    `json:"sex,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Group represents a family or group
type Group struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	HeadUserID uuid.UUID `json:"head_user_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// GroupMember represents a user's membership in a group
type GroupMember struct {
	GroupID  uuid.UUID  `json:"group_id"`
	UserID   uuid.UUID  `json:"user_id"`
	Role     MemberRole `json:"role"`
	JoinedAt time.Time  `json:"joined_at"`
}

// Chore represents a task/chore that can be completed for money
type Chore struct {
	ID          uuid.UUID `json:"id"`
	GroupID     uuid.UUID `json:"group_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Amount      float64   `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
}

// LedgerEntry represents a record of a completed chore
type LedgerEntry struct {
	ID               uuid.UUID    `json:"id"`
	GroupID          uuid.UUID    `json:"group_id"`
	UserID           uuid.UUID    `json:"user_id"`
	ChoreID          uuid.UUID    `json:"chore_id"`
	Amount           float64      `json:"amount"`
	Status           LedgerStatus `json:"status"`
	CreatedByUserID  uuid.UUID    `json:"created_by_user_id"`
	ApprovedByUserID *uuid.UUID   `json:"approved_by_user_id,omitempty"`
	RejectedByUserID *uuid.UUID   `json:"rejected_by_user_id,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// Settlement represents a cash payout to a member
type Settlement struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    float64   `json:"amount"`
	Date      time.Time `json:"date"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InviteToken represents an invitation to join a group
type InviteToken struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// MemberWithUser combines member info with user details
type MemberWithUser struct {
	GroupMember
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Balance represents a user's balance in a group
type Balance struct {
	UserID  uuid.UUID `json:"user_id"`
	Name    string    `json:"name"`
	Balance float64   `json:"balance"`
}
