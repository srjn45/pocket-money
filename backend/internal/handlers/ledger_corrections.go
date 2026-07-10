package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// EditLedgerRequest is the body for PUT /api/v1/ledger/:id (V3-3.2 §1.3). amount
// is required (value >= 1, currency == group). direction is required for
// adjustment entries and must be absent for chore/settlement (their direction is
// fixed). note is full-replace: null or absent clears it to NULL.
type EditLedgerRequest struct {
	Amount    *models.Money           `json:"amount"`
	Direction *models.LedgerDirection `json:"direction"`
	Note      *string                 `json:"note"`
}

// loadEntryForCorrection performs the shared prelude for EditLedger/DeleteLedger
// (V3-3.2 §1.4 steps 1–6): auth, parse :id, load the entry, require the caller be
// the group admin (D6), and reject system-generated (allowance/emi) entries. On
// any failure it writes the error response and returns ok=false.
func (h *LedgerHandler) loadEntryForCorrection(c *gin.Context) (entry *models.LedgerEntry, userID uuid.UUID, ok bool) {
	userIDStr, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return nil, uuid.Nil, false
	}

	entryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry ID"})
		return nil, uuid.Nil, false
	}

	entry, err = h.ledgerRepo.GetByID(c.Request.Context(), entryID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return nil, uuid.Nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get entry"})
		return nil, uuid.Nil, false
	}

	member, err := h.groupRepo.GetMember(c.Request.Context(), entry.GroupID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
			return nil, uuid.Nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return nil, uuid.Nil, false
	}
	if member.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "only group admin can edit/delete entries"})
		return nil, uuid.Nil, false
	}

	// System-generated entries are not directly editable/deletable (any status).
	// The D2 flag does NOT gate corrections — it gates submission only.
	if entry.EntryType == models.EntryTypeAllowance || entry.EntryType == models.EntryTypeEMI {
		c.JSON(http.StatusForbidden, gin.H{"error": "system-generated entries cannot be edited or deleted; edit the base amount or loan instead"})
		return nil, uuid.Nil, false
	}

	return entry, userID, true
}

// EditLedger edits a manual ledger entry in place (V3-3.2 §1). Admin-only; manual
// types only (chore/adjustment/settlement). Writes an entry_audit row capturing the
// prior values, then applies the edit — both in one transaction.
// PUT /api/v1/ledger/:id
func (h *LedgerHandler) EditLedger(c *gin.Context) {
	entry, userID, ok := h.loadEntryForCorrection(c)
	if !ok {
		return
	}

	currency, err := h.groupRepo.GetCurrency(c.Request.Context(), entry.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get group currency"})
		return
	}

	var req EditLedgerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// amount: required, currency must match the group (D7), value >= 1.
	if req.Amount == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required"})
		return
	}
	if !checkMoneyCurrency(c, *req.Amount, currency) {
		return
	}
	if req.Amount.Value < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be >= 1"})
		return
	}

	// direction: required for adjustment (credit|debit); forbidden for chore/settlement.
	var direction *models.LedgerDirection
	switch entry.EntryType {
	case models.EntryTypeAdjustment:
		if req.Direction == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direction is required for adjustment entries"})
			return
		}
		d := *req.Direction
		if d != models.DirectionCredit && d != models.DirectionDebit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be credit or debit"})
			return
		}
		direction = &d
	default: // chore, settlement
		if req.Direction != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "direction cannot be set for chore or settlement entries"})
			return
		}
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Re-read FOR UPDATE: locks the row and is the audit snapshot source.
	preEntry, err := h.ledgerRepo.GetForUpdate(ctx, tx, entry.ID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lock entry"})
		return
	}

	oldRow, err := json.Marshal(preEntry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to snapshot entry"})
		return
	}
	if err := h.auditRepo.Insert(ctx, tx, entry.ID, oldRow, models.AuditActionEdit, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write audit"})
		return
	}

	updated, err := h.ledgerRepo.UpdateManualEntry(ctx, tx, entry.ID, req.Amount.Value, direction, req.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update entry"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	c.JSON(http.StatusOK, entryToResponse(updated, currency))
}

// DeleteLedger hard-deletes a manual ledger entry (V3-3.2 §1). Admin-only; manual
// types only. Writes an entry_audit row capturing the prior values, then deletes —
// both in one transaction. The audit row survives (entry_id FK is SET NULL).
// DELETE /api/v1/ledger/:id
func (h *LedgerHandler) DeleteLedger(c *gin.Context) {
	entry, userID, ok := h.loadEntryForCorrection(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Re-read FOR UPDATE: locks the row and is the audit snapshot source.
	preEntry, err := h.ledgerRepo.GetForUpdate(ctx, tx, entry.ID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lock entry"})
		return
	}

	oldRow, err := json.Marshal(preEntry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to snapshot entry"})
		return
	}
	if err := h.auditRepo.Insert(ctx, tx, entry.ID, oldRow, models.AuditActionDelete, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write audit"})
		return
	}

	if err := h.ledgerRepo.DeleteEntry(ctx, tx, entry.ID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete entry"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	c.Status(http.StatusNoContent)
}
