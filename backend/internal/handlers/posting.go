package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/posting"
)

// runPosting triggers due allowance posting for the group before a balance-sensitive read.
// Must be called after the membership check so a non-member still gets 403.
// On error it writes 500 and returns false — do not serve a potentially stale balance.
func runPosting(c *gin.Context, svc *posting.Service, groupID uuid.UUID) bool {
	if err := svc.PostDue(c.Request.Context(), groupID, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to post due allowances"})
		return false
	}
	return true
}
