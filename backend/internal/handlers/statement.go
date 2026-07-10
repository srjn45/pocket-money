package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/statement"
)

// StatementTotals is the element-wise sum of the eight statement figures over the
// returned members. Present only on the unfiltered admin view (§3.4).
type StatementTotals struct {
	OpeningBalance models.Money `json:"opening_balance"`
	Base           models.Money `json:"base"`
	Chores         models.Money `json:"chores"`
	Adjustments    models.Money `json:"adjustments"`
	EMI            models.Money `json:"emi"`
	TotalDue       models.Money `json:"total_due"`
	Cleared        models.Money `json:"cleared"`
	ClosingBalance models.Money `json:"closing_balance"`
}

// MemberStatementResponse is one member's derived figures for the month plus that
// month's passbook (§3.6). Signed fields may be negative; magnitude fields ≥ 0.
type MemberStatementResponse struct {
	UserID         uuid.UUID        `json:"user_id"`
	Name           string           `json:"name"`
	OpeningBalance models.Money     `json:"opening_balance"`
	Base           models.Money     `json:"base"`
	Chores         models.Money     `json:"chores"`
	Adjustments    models.Money     `json:"adjustments"`
	EMI            models.Money     `json:"emi"`
	TotalDue       models.Money     `json:"total_due"`
	Cleared        models.Money     `json:"cleared"`
	ClosingBalance models.Money     `json:"closing_balance"`
	Entries        []LedgerResponse `json:"entries"`
}

// StatementResponse is the monthly statement for a group (§3.6). group_total is
// present only on the unfiltered admin view; omitted (null) otherwise.
type StatementResponse struct {
	GroupID    uuid.UUID                 `json:"group_id"`
	Period     string                    `json:"period"`
	Currency   string                    `json:"currency"`
	Members    []MemberStatementResponse `json:"members"`
	GroupTotal *StatementTotals          `json:"group_total,omitempty"`
}

// GetStatement returns the derived monthly statement for a group (§3.9).
// GET /api/v1/groups/:id/statement?period=YYYY-MM[&user_id=UUID]
func (h *LedgerHandler) GetStatement(c *gin.Context) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return
	}

	// Membership gate BEFORE PostDue (mirrors GetBalance/ListLedger, §3.3/§9.10).
	member, err := h.groupRepo.GetMember(c.Request.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	// period: defaults to the current server-local month when omitted (§3.2).
	period := c.Query("period")
	if period == "" {
		period = statement.FormatPeriodLocal(time.Now())
	} else if !isValidPeriod(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period format, use YYYY-MM"})
		return
	}

	// user_id: admin narrows to one member; a member may pass only their own id
	// (any other → 403), mirroring ListLedger (§3.2, D6).
	isAdmin := member.Role == models.RoleAdmin
	var filterUserID *uuid.UUID
	if uidStr := c.Query("user_id"); uidStr != "" {
		parsed, err := uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		if !isAdmin && parsed != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "members can only access their own data"})
			return
		}
		filterUserID = &parsed
	}
	if !isAdmin {
		// A member always sees only their own row, regardless of user_id.
		filterUserID = &userID
	}

	// Materialize due allowance/EMI entries before reading (balance-sensitive).
	if !runPosting(c, h.postingSvc, groupID) {
		return
	}

	currency, err := h.groupRepo.GetCurrency(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group currency"})
		return
	}

	members, err := h.groupRepo.ListMembers(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}

	approved := models.StatusApproved
	entries, err := h.ledgerRepo.ListForGroupWithUser(c.Request.Context(), groupID, &approved, filterUserID, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ledger entries"})
		return
	}

	// Group approved entries by member for per-member Compute.
	entriesByUser := make(map[uuid.UUID][]*models.LedgerEntry)
	for _, e := range entries {
		entriesByUser[e.UserID] = append(entriesByUser[e.UserID], e)
	}

	// Decide who is included (§3.4/§3.5):
	//   - members[] holds non-admin recipients only (mirrors GET /balance);
	//   - a member is included iff period >= their join month (join flooring);
	//   - an admin narrowing via user_id gets just that member (if included);
	//   - a member caller gets only their own row.
	resp := StatementResponse{
		GroupID:  groupID,
		Period:   period,
		Currency: currency,
		Members:  []MemberStatementResponse{},
	}

	includeGroupTotal := isAdmin && filterUserID == nil
	var total statement.MemberStatement

	for _, m := range members {
		if m.Role == models.RoleAdmin {
			continue // recipients only; admin's own row is never shown (§3.4)
		}
		if filterUserID != nil && m.UserID != *filterUserID {
			continue // narrowed to a single member (admin user_id, or member self)
		}
		if statement.FormatPeriodLocal(m.JoinedAt) > period {
			continue // pre-join month: member absent, no row (§3.5)
		}

		st := statement.Compute(entriesByUser[m.UserID], period)
		resp.Members = append(resp.Members, memberStatementToResponse(m.UserID, m.Name, st, currency))

		if includeGroupTotal {
			total.Opening += st.Opening
			total.Base += st.Base
			total.Chores += st.Chores
			total.Adjustments += st.Adjustments
			total.EMI += st.EMI
			total.TotalDue += st.TotalDue
			total.Cleared += st.Cleared
			total.Closing += st.Closing
		}
	}

	if includeGroupTotal {
		resp.GroupTotal = &StatementTotals{
			OpeningBalance: models.NewMoney(currency, total.Opening),
			Base:           models.NewMoney(currency, total.Base),
			Chores:         models.NewMoney(currency, total.Chores),
			Adjustments:    models.NewMoney(currency, total.Adjustments),
			EMI:            models.NewMoney(currency, total.EMI),
			TotalDue:       models.NewMoney(currency, total.TotalDue),
			Cleared:        models.NewMoney(currency, total.Cleared),
			ClosingBalance: models.NewMoney(currency, total.Closing),
		}
	}

	c.JSON(http.StatusOK, resp)
}

// memberStatementToResponse serializes a computed MemberStatement into the API
// shape, stamping the group currency on every Money (D7).
func memberStatementToResponse(userID uuid.UUID, name string, st statement.MemberStatement, currency string) MemberStatementResponse {
	entries := make([]LedgerResponse, 0, len(st.Entries))
	for _, e := range st.Entries {
		entries = append(entries, entryToResponse(e, currency))
	}
	return MemberStatementResponse{
		UserID:         userID,
		Name:           name,
		OpeningBalance: models.NewMoney(currency, st.Opening),
		Base:           models.NewMoney(currency, st.Base),
		Chores:         models.NewMoney(currency, st.Chores),
		Adjustments:    models.NewMoney(currency, st.Adjustments),
		EMI:            models.NewMoney(currency, st.EMI),
		TotalDue:       models.NewMoney(currency, st.TotalDue),
		Cleared:        models.NewMoney(currency, st.Cleared),
		ClosingBalance: models.NewMoney(currency, st.Closing),
		Entries:        entries,
	}
}
