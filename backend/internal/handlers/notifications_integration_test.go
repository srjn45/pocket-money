//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// notifTestEnv wires the notification handler + ledger handler for integration tests.
type notifTestEnv struct {
	router           *gin.Engine
	pool             *pgxpool.Pool
	userRepo         *db.UserRepo
	groupRepo        *db.GroupRepo
	choreRepo        *db.ChoreRepo
	notificationRepo *db.NotificationRepo
	ledgerRepo       *db.LedgerRepo
	cleanup          func()
}

func setupNotifTestEnv(t *testing.T) *notifTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool, err := testutil.NewTestPool()
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	require.NoError(t, testutil.ResetTestDB(pool))
	require.NoError(t, db.RunMigrations(testutil.GetTestDatabaseURL()))

	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	choreRepo := db.NewChoreRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)
	allowanceRepo := db.NewAllowanceRepo(pool)
	loanRepo := db.NewLoanRepo(pool)
	notificationRepo := db.NewNotificationRepo(pool)

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)
	notifH := handlers.NewNotificationHandler(notificationRepo)
	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo, postingSvc, pool, db.NewAuditRepo(pool), notificationRepo)

	router := gin.New()
	protected := router.Group("")
	protected.Use(auth.AuthMiddleware(testJWTSecret))
	protected.GET("/notifications", notifH.ListNotifications)
	protected.GET("/notifications/unread_count", notifH.GetUnreadCount)
	protected.POST("/notifications/:id/read", notifH.MarkRead)
	protected.POST("/notifications/read_all", notifH.MarkAllRead)
	protected.POST("/groups/:id/ledger", lh.CreateLedger)

	return &notifTestEnv{
		router:           router,
		pool:             pool,
		userRepo:         userRepo,
		groupRepo:        groupRepo,
		choreRepo:        choreRepo,
		notificationRepo: notificationRepo,
		ledgerRepo:       ledgerRepo,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedNotifUser creates a registered user (no group needed for read-API tests).
func (e *notifTestEnv) seedUser(t *testing.T, suffix string) *models.User {
	t.Helper()
	u, err := e.userRepo.Create(t.Context(), fmt.Sprintf("user-%s@example.com", suffix), "hash", "User "+suffix, nil, nil)
	require.NoError(t, err)
	return u
}

// seedGroup creates a group and explicitly adds the creator as RoleAdmin
// (GroupRepo.Create does NOT add the creator — project memory §9.10).
func (e *notifTestEnv) seedGroup(t *testing.T, adminID uuid.UUID, currency string) *models.Group {
	t.Helper()
	g, err := e.groupRepo.Create(t.Context(), "Group", adminID, currency)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(t.Context(), g.ID, adminID, models.RoleAdmin)
	require.NoError(t, err)
	return g
}

// insertNotif writes a notification directly to the DB (bypasses HTTP).
func (e *notifTestEnv) insertNotif(t *testing.T, userID uuid.UUID, ntype string, payload []byte) {
	t.Helper()
	err := e.notificationRepo.Insert(t.Context(), e.pool, userID, ntype, payload)
	require.NoError(t, err)
}

// directNotifs reads all notifications for a user from the DB, ordered by created_at ASC.
func (e *notifTestEnv) directNotifs(t *testing.T, userID uuid.UUID) []models.Notification {
	t.Helper()
	rows, err := e.pool.Query(t.Context(),
		`SELECT id, user_id, type, payload, read_at, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at ASC`,
		userID)
	require.NoError(t, err)
	defer rows.Close()

	var out []models.Notification
	for rows.Next() {
		var n models.Notification
		require.NoError(t, rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.ReadAt, &n.CreatedAt))
		out = append(out, n)
	}
	require.NoError(t, rows.Err())
	return out
}

// directReadAt reads the read_at for a single notification by ID.
func (e *notifTestEnv) directReadAt(t *testing.T, notifID uuid.UUID) *time.Time {
	t.Helper()
	var readAt *time.Time
	err := e.pool.QueryRow(t.Context(),
		`SELECT read_at FROM notifications WHERE id = $1`, notifID).Scan(&readAt)
	require.NoError(t, err)
	return readAt
}

// directNotifCount returns the total notification count for a user.
func (e *notifTestEnv) directNotifCount(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, e.pool.QueryRow(t.Context(),
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID).Scan(&count))
	return count
}

func notifListURL(cursor string, limit int) string {
	url := "/notifications"
	sep := "?"
	if limit > 0 {
		url += fmt.Sprintf("%slimit=%d", sep, limit)
		sep = "&"
	}
	if cursor != "" {
		url += fmt.Sprintf("%scursor=%s", sep, cursor)
	}
	return url
}

// --- Case 1: Scoping (read list + unread count) ---

func TestNotif_Scoping_List(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	userA := env.seedUser(t, "scope-a")
	userB := env.seedUser(t, "scope-b")

	payload := []byte(`{"msg":"hi"}`)
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, payload)
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, payload)
	env.insertNotif(t, userB.ID, models.NotificationAddedToGroup, payload)

	// A sees only A's 2 rows.
	w := doRequest(env.router, http.MethodGet, "/notifications", nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Items, 2)

	// Unread count for A excludes B's row.
	w2 := doRequest(env.router, http.MethodGet, "/notifications/unread_count", nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusOK, w2.Code)
	var cnt struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &cnt))
	assert.Equal(t, 2, cnt.Count)
}

// --- Case 2: Scoping (mark — foreign id → 404, own id → 204) ---

func TestNotif_Scoping_Mark(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	userA := env.seedUser(t, "mark-a")
	userB := env.seedUser(t, "mark-b")

	payload := []byte(`{"msg":"hi"}`)
	env.insertNotif(t, userB.ID, models.NotificationAddedToGroup, payload)

	bNotifs := env.directNotifs(t, userB.ID)
	require.Len(t, bNotifs, 1)
	bID := bNotifs[0].ID

	// A tries to mark B's notification → 404.
	w := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/notifications/%s/read", bID), nil, bearerToken(t, userA.ID))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// B's notification is still unread.
	assert.Nil(t, env.directReadAt(t, bID))

	// A marks their own notification → 204.
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, payload)
	aNotifs := env.directNotifs(t, userA.ID)
	require.Len(t, aNotifs, 1)
	aID := aNotifs[0].ID

	w2 := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/notifications/%s/read", aID), nil, bearerToken(t, userA.ID))
	assert.Equal(t, http.StatusNoContent, w2.Code)
	assert.NotNil(t, env.directReadAt(t, aID))
}

// --- Case 3: Idempotent mark-read (read_at unchanged on second call) ---

func TestNotif_MarkRead_Idempotent(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	userA := env.seedUser(t, "idem-a")
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, []byte(`{}`))

	notifs := env.directNotifs(t, userA.ID)
	require.Len(t, notifs, 1)
	nID := notifs[0].ID

	// First mark.
	w1 := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/notifications/%s/read", nID), nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusNoContent, w1.Code)
	readAt1 := env.directReadAt(t, nID)
	require.NotNil(t, readAt1)

	// Second mark — 204 again, read_at must not change.
	w2 := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/notifications/%s/read", nID), nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusNoContent, w2.Code)
	readAt2 := env.directReadAt(t, nID)
	require.NotNil(t, readAt2)
	assert.Equal(t, readAt1.UTC().Truncate(time.Microsecond), readAt2.UTC().Truncate(time.Microsecond))
}

// --- Case 4: Unread math (insert 3, mark 1, mark-all, second mark-all) ---

func TestNotif_UnreadMath(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	userA := env.seedUser(t, "math-a")
	p := []byte(`{}`)
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, p)
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, p)
	env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, p)

	unread := func() int {
		w := doRequest(env.router, http.MethodGet, "/notifications/unread_count", nil, bearerToken(t, userA.ID))
		require.Equal(t, http.StatusOK, w.Code)
		var r struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &r))
		return r.Count
	}

	assert.Equal(t, 3, unread())

	// Mark one read.
	notifs := env.directNotifs(t, userA.ID)
	w := doRequest(env.router, http.MethodPost,
		fmt.Sprintf("/notifications/%s/read", notifs[0].ID), nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 2, unread())

	// Mark all → updated=2.
	w2 := doRequest(env.router, http.MethodPost, "/notifications/read_all", nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusOK, w2.Code)
	var all struct {
		Updated int64 `json:"updated"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &all))
	assert.Equal(t, int64(2), all.Updated)
	assert.Equal(t, 0, unread())

	// Second mark-all → updated=0.
	w3 := doRequest(env.router, http.MethodPost, "/notifications/read_all", nil, bearerToken(t, userA.ID))
	require.Equal(t, http.StatusOK, w3.Code)
	var all2 struct {
		Updated int64 `json:"updated"`
	}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &all2))
	assert.Equal(t, int64(0), all2.Updated)
}

// --- Case 5: Pagination correctness ---

func TestNotif_Pagination(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	userA := env.seedUser(t, "page-a")

	// Insert 5 notifications with distinct created_at by spacing them out 1ms apart.
	ids := make([]uuid.UUID, 5)
	for i := 0; i < 5; i++ {
		env.insertNotif(t, userA.ID, models.NotificationAddedToGroup, []byte(fmt.Sprintf(`{"i":%d}`, i)))
		// Space inserts so created_at is distinct (Postgres timestamptz resolution = 1µs).
		time.Sleep(2 * time.Millisecond)
		notifs := env.directNotifs(t, userA.ID)
		ids[i] = notifs[len(notifs)-1].ID
	}

	// Order from DB is ASC; page API returns DESC (newest first).
	// After 5 inserts, ids[4] is newest, ids[0] is oldest.

	tok := bearerToken(t, userA.ID)

	// Page 1: limit=2 → 2 items + next_cursor.
	w := doRequest(env.router, http.MethodGet, notifListURL("", 2), nil, tok)
	require.Equal(t, http.StatusOK, w.Code)
	var page1 struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page1))
	assert.Len(t, page1.Items, 2)
	require.NotNil(t, page1.NextCursor)
	assert.Equal(t, ids[4], page1.Items[0].ID) // newest first
	assert.Equal(t, ids[3], page1.Items[1].ID)

	// Page 2: follow cursor → 2 items.
	w2 := doRequest(env.router, http.MethodGet, notifListURL(*page1.NextCursor, 2), nil, tok)
	require.Equal(t, http.StatusOK, w2.Code)
	var page2 struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &page2))
	assert.Len(t, page2.Items, 2)
	require.NotNil(t, page2.NextCursor)
	assert.Equal(t, ids[2], page2.Items[0].ID)
	assert.Equal(t, ids[1], page2.Items[1].ID)

	// Page 3: follow cursor → 1 item, no next_cursor.
	w3 := doRequest(env.router, http.MethodGet, notifListURL(*page2.NextCursor, 2), nil, tok)
	require.Equal(t, http.StatusOK, w3.Code)
	var page3 struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &page3))
	assert.Len(t, page3.Items, 1)
	assert.Equal(t, ids[0], page3.Items[0].ID)
	assert.Nil(t, page3.NextCursor)

	// Bad cursor → 400.
	wBad := doRequest(env.router, http.MethodGet, notifListURL("not-valid-base64!!!", 2), nil, tok)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)

	// limit=0 → 400.
	wL0 := doRequest(env.router, http.MethodGet, notifListURL("", 0), nil, tok)
	assert.Equal(t, http.StatusBadRequest, wL0.Code)

	// limit=101 → 400.
	wL101 := doRequest(env.router, http.MethodGet, "/notifications?limit=101", nil, tok)
	assert.Equal(t, http.StatusBadRequest, wL101.Code)
}

// --- Case 6: N-3 on settlement — happy path ---

func TestNotif_N3_SettlementWritesNotification(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin := env.seedUser(t, "n3-admin")
	member := env.seedUser(t, "n3-member")
	group := env.seedGroup(t, admin.ID, models.CurrencyINR)
	_, err := env.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	// Admin posts a settlement for the member.
	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{
			"entry_type": "settlement",
			"user_id":    member.ID,
			"amount":     inr(40000),
		}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusCreated, w.Code)

	// Member gets exactly one payment_recorded notification.
	notifs := env.directNotifs(t, member.ID)
	require.Len(t, notifs, 1)
	assert.Equal(t, models.NotificationPaymentRecorded, notifs[0].Type)

	var payload struct {
		GroupID string `json:"group_id"`
		Amount  struct {
			Currency string `json:"currency"`
			Value    int64  `json:"value"`
		} `json:"amount"`
	}
	require.NoError(t, json.Unmarshal(notifs[0].Payload, &payload))
	assert.Equal(t, group.ID.String(), payload.GroupID)
	assert.Equal(t, models.CurrencyINR, payload.Amount.Currency)
	assert.Equal(t, int64(40000), payload.Amount.Value)

	// Admin got no N-3.
	assert.Empty(t, env.directNotifs(t, admin.ID))
}

// --- Case 7: N-3 self-payment skip ---

func TestNotif_N3_SelfPaymentSkipped(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin := env.seedUser(t, "self-admin")
	group := env.seedGroup(t, admin.ID, models.CurrencyINR)

	// Admin also appears as a member (admin pays themselves).
	// The group was created above; admin is already RoleAdmin.
	// We need to also add them as a target of the settlement — but they're already in the group.
	_ = ctx

	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{
			"entry_type": "settlement",
			"user_id":    admin.ID,
			"amount":     inr(10000),
		}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusCreated, w.Code)

	// No N-3 for the admin (self-payment skip).
	assert.Empty(t, env.directNotifs(t, admin.ID))
}

// --- Case 8: N-3 fires ONLY for settlement (not chore, not adjustment) ---

func TestNotif_N3_OnlyForSettlement(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin := env.seedUser(t, "n3type-admin")
	member := env.seedUser(t, "n3type-member")
	group := env.seedGroup(t, admin.ID, models.CurrencyINR)
	_, err := env.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	chore, err := env.choreRepo.Create(ctx, group.ID, "Test Chore", nil, 500)
	require.NoError(t, err)

	// Admin posts a chore entry for the member.
	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{
			"entry_type": "chore",
			"user_id":    member.ID,
			"chore_id":   chore.ID,
		}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusCreated, w.Code)

	// Admin posts an adjustment for the member.
	w2 := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{
			"entry_type": "adjustment",
			"user_id":    member.ID,
			"amount":     inr(200),
			"direction":  "credit",
		}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusCreated, w2.Code)

	// Zero payment_recorded notifications for the member.
	for _, n := range env.directNotifs(t, member.ID) {
		assert.NotEqual(t, models.NotificationPaymentRecorded, n.Type)
	}
}

// --- Case 9: EUR currency case (guards against hardcoded INR) ---

func TestNotif_N3_EURCurrency(t *testing.T) {
	env := setupNotifTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	admin := env.seedUser(t, "eur-admin")
	member := env.seedUser(t, "eur-member")
	group := env.seedGroup(t, admin.ID, models.CurrencyEUR)
	_, err := env.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)

	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/ledger", group.ID),
		map[string]interface{}{
			"entry_type": "settlement",
			"user_id":    member.ID,
			"amount":     map[string]interface{}{"currency": "EUR", "value": int64(5000)},
		}, bearerToken(t, admin.ID))
	require.Equal(t, http.StatusCreated, w.Code)

	notifs := env.directNotifs(t, member.ID)
	require.Len(t, notifs, 1)
	assert.Equal(t, models.NotificationPaymentRecorded, notifs[0].Type)

	var payload struct {
		Amount struct {
			Currency string `json:"currency"`
			Value    int64  `json:"value"`
		} `json:"amount"`
	}
	require.NoError(t, json.Unmarshal(notifs[0].Payload, &payload))
	assert.Equal(t, models.CurrencyEUR, payload.Amount.Currency)
	assert.Equal(t, int64(5000), payload.Amount.Value)
}
