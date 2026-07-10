package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/srjn45/pocket-money/backend/internal/models"
)

// checkMoneyCurrency verifies that a request Money's currency matches the owning
// group's currency (D7). On mismatch it writes a human-readable 400 and returns
// false; the caller must return immediately. Storage/math stay int64 minor units
// — only the JSON boundary carries the currency.
func checkMoneyCurrency(c *gin.Context, money models.Money, groupCurrency string) bool {
	if money.Currency != groupCurrency {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("amount currency %s does not match group currency %s",
				money.Currency, groupCurrency),
		})
		return false
	}
	return true
}
