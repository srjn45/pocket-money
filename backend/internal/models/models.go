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

// LedgerEntryType classifies what balance event a ledger entry represents
type LedgerEntryType string

const (
	EntryTypeChore      LedgerEntryType = "chore"
	EntryTypeAllowance  LedgerEntryType = "allowance"
	EntryTypeEMI        LedgerEntryType = "emi"
	EntryTypeSettlement LedgerEntryType = "settlement"
	EntryTypeAdjustment LedgerEntryType = "adjustment"
)

// LedgerDirection indicates whether an entry adds to or subtracts from a member's balance
type LedgerDirection string

const (
	DirectionCredit LedgerDirection = "credit"
	DirectionDebit  LedgerDirection = "debit"
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
	ID          uuid.UUID  `json:"id"`
	GroupID     uuid.UUID  `json:"group_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Amount      int64      `json:"amount"`
	IsSystem    bool       `json:"is_system"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// LedgerEntry represents an immutable balance event in the unified ledger
type LedgerEntry struct {
	ID              uuid.UUID       `json:"id"`
	GroupID         uuid.UUID       `json:"group_id"`
	UserID          uuid.UUID       `json:"user_id"`
	ChoreID         *uuid.UUID      `json:"chore_id,omitempty"` // null for settlement/adjustment/allowance/emi
	Amount          int64           `json:"amount"`
	Status          LedgerStatus    `json:"status"`
	EntryType       LedgerEntryType `json:"entry_type"`
	Direction       LedgerDirection `json:"direction"`
	LoanID          *uuid.UUID      `json:"loan_id,omitempty"`  // set on emi entries; FK added in migration 011
	Period          *string         `json:"period,omitempty"`   // YYYY-MM, set on allowance/emi
	Note            *string         `json:"note,omitempty"`
	CreatedByUserID uuid.UUID       `json:"created_by_user_id"`
	DecidedBy       *uuid.UUID      `json:"decided_by,omitempty"`  // who approved/rejected
	DecidedAt       *time.Time      `json:"decided_at,omitempty"`  // when approved/rejected
	CreatedAt       time.Time       `json:"created_at"`
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
	Balance int64     `json:"balance"`
}
