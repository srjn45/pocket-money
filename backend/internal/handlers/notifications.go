package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
)

// NotificationHandler serves the notifications read API (V3-5.1).
type NotificationHandler struct {
	notificationRepo *db.NotificationRepo
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(notificationRepo *db.NotificationRepo) *NotificationHandler {
	return &NotificationHandler{notificationRepo: notificationRepo}
}

// notificationResponse is the per-item DTO matching the openapi Notification schema.
type notificationResponse struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"read_at"`
	CreatedAt time.Time       `json:"created_at"`
}

// notificationListResponse matches the openapi NotificationListResponse schema.
type notificationListResponse struct {
	Items      []notificationResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

// unreadCountResponse matches the openapi UnreadCountResponse schema.
type unreadCountResponse struct {
	Count int `json:"count"`
}

// markAllReadResponse matches the openapi MarkAllReadResponse schema.
type markAllReadResponse struct {
	Updated int64 `json:"updated"`
}

func toNotificationResponse(n *models.Notification) notificationResponse {
	return notificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Payload:   n.Payload,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

// encodeCursor encodes the last-item cursor as base64url(no-pad) of "<RFC3339Nano>|<uuid>".
func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes the cursor string; returns (createdAt, id, ok).
func decodeCursor(cursor string) (time.Time, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return t, id, true
}

// ListNotifications handles GET /api/v1/notifications
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	userIDStr, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	// Parse limit: default 20, clamp [1, 100].
	limit := 20
	if ls := c.Query("limit"); ls != "" {
		v, err := strconv.Atoi(ls)
		if err != nil || v < 1 || v > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer between 1 and 100"})
			return
		}
		limit = v
	}

	// Parse cursor (opaque base64url).
	var afterCreatedAt *time.Time
	var afterID *uuid.UUID
	if cs := c.Query("cursor"); cs != "" {
		t, id, ok := decodeCursor(cs)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		afterCreatedAt = &t
		afterID = &id
	}

	items, hasMore, err := h.notificationRepo.List(c.Request.Context(), userID, limit, afterCreatedAt, afterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notifications"})
		return
	}

	resp := notificationListResponse{
		Items:      make([]notificationResponse, 0, len(items)),
		NextCursor: nil,
	}
	for _, n := range items {
		resp.Items = append(resp.Items, toNotificationResponse(n))
	}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		s := encodeCursor(last.CreatedAt, last.ID)
		resp.NextCursor = &s
	}

	c.JSON(http.StatusOK, resp)
}

// GetUnreadCount handles GET /api/v1/notifications/unread_count
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userIDStr, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	count, err := h.notificationRepo.UnreadCount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get unread count"})
		return
	}

	c.JSON(http.StatusOK, unreadCountResponse{Count: count})
}

// MarkRead handles POST /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userIDStr, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	notifIDStr := c.Param("id")
	notifID, err := uuid.Parse(notifIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification ID"})
		return
	}

	found, err := h.notificationRepo.MarkRead(c.Request.Context(), notifID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark notification read"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// MarkAllRead handles POST /api/v1/notifications/read_all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userIDStr, ok := auth.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	updated, err := h.notificationRepo.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark all notifications read"})
		return
	}

	c.JSON(http.StatusOK, markAllReadResponse{Updated: updated})
}
