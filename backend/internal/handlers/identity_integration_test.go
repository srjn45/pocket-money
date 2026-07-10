//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

// identityTestEnv wires the add-by-email and register-claim handlers plus direct
// DB access so the notifications table (insert-only, no read API) can be asserted.
type identityTestEnv struct {
	router    *gin.Engine
	pool      *pgxpool.Pool
	userRepo  *db.UserRepo
	groupRepo *db.GroupRepo
	cleanup   func()
}

func setupIdentityTestEnv(t *testing.T) *identityTestEnv {
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
	inviteRepo := db.NewInviteRepo(pool)
	notificationRepo := db.NewNotificationRepo(pool)
	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	authH := handlers.NewAuthHandler(userRepo, groupRepo, notificationRepo, pool, testJWTSecret)
	gh := handlers.NewGroupHandler(groupRepo, inviteRepo, choreRepo, ledgerRepo, loanRepo, allowanceRepo, userRepo, notificationRepo, postingSvc, pool, "")

	router := gin.New()
	// register/login are public (a shadow claimant holds no token).
	router.POST("/auth/register", authH.Register)
	router.POST("/auth/login", authH.Login)
	protected := router.Group("")
	protected.Use(auth.AuthMiddleware(testJWTSecret))
	protected.POST("/groups/:id/members", gh.AddMemberByEmail)

	return &identityTestEnv{
		router:    router,
		pool:      pool,
		userRepo:  userRepo,
		groupRepo: groupRepo,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedHead creates a registered head user + group with the head in group_members
// as RoleHead (GroupRepo.Create does NOT add the creator — §9.10).
func (e *identityTestEnv) seedHead(t *testing.T, suffix string) (head *models.User, group *models.Group) {
	t.Helper()
	ctx := t.Context()
	var err error
	head, err = e.userRepo.Create(ctx, fmt.Sprintf("head-%s@example.com", suffix), "hash", "Head "+suffix, nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, head.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	return
}

// addByEmail POSTs an add-by-email request as the given caller.
func (e *identityTestEnv) addByEmail(t *testing.T, callerID, groupID uuid.UUID, email, name string) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(e.router, http.MethodPost, fmt.Sprintf("/groups/%s/members", groupID),
		map[string]interface{}{"email": email, "name": name}, bearerToken(t, callerID))
}

// notifPayload is one notification row (type + decoded payload).
type notifPayload struct {
	Type    string
	Payload map[string]string
}

// notificationsFor reads all notifications for a user directly (no read API).
func (e *identityTestEnv) notificationsFor(t *testing.T, userID uuid.UUID) []notifPayload {
	t.Helper()
	rows, err := e.pool.Query(t.Context(),
		`SELECT type, payload FROM notifications WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	require.NoError(t, err)
	defer rows.Close()

	var out []notifPayload
	for rows.Next() {
		var ntype string
		var raw []byte
		require.NoError(t, rows.Scan(&ntype, &raw))
		payload := map[string]string{}
		require.NoError(t, json.Unmarshal(raw, &payload))
		out = append(out, notifPayload{Type: ntype, Payload: payload})
	}
	require.NoError(t, rows.Err())
	return out
}

func decodeMember(t *testing.T, w *httptest.ResponseRecorder) handlers.MemberResponse {
	t.Helper()
	var m handlers.MemberResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

// --- A: registered email attaches + N-1 ---

func TestAddMember_RegisteredUser_AttachesAndNotifies(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, group := env.seedHead(t, "a")
	target, err := env.userRepo.Create(ctx, "target-a@example.com", "hash", "Target A", nil, nil)
	require.NoError(t, err)

	w := env.addByEmail(t, head.ID, group.ID, target.Email, "ignored")
	require.Equal(t, http.StatusCreated, w.Code)

	m := decodeMember(t, w)
	assert.Equal(t, target.ID, m.UserID)
	assert.Equal(t, models.RoleMember, m.Role)
	assert.Equal(t, models.UserStatusRegistered, m.Status)

	// Membership row exists.
	_, err = env.groupRepo.GetMember(ctx, group.ID, target.ID)
	require.NoError(t, err)

	// Exactly one N-1 for the target, payload names the group.
	notifs := env.notificationsFor(t, target.ID)
	require.Len(t, notifs, 1)
	assert.Equal(t, models.NotificationAddedToGroup, notifs[0].Type)
	assert.Equal(t, group.ID.String(), notifs[0].Payload["group_id"])
}

// --- B: unknown email → shadow, no notification ---

func TestAddMember_UnknownEmail_CreatesShadowNoNotif(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, group := env.seedHead(t, "b")

	w := env.addByEmail(t, head.ID, group.ID, "newbie-b@example.com", "Newbie B")
	require.Equal(t, http.StatusCreated, w.Code)

	m := decodeMember(t, w)
	assert.Equal(t, models.UserStatusShadow, m.Status)

	// The users row is a shadow: NULL password, status='shadow', name from request.
	u, err := env.userRepo.GetByEmail(ctx, "newbie-b@example.com")
	require.NoError(t, err)
	assert.Nil(t, u.PasswordHash)
	assert.Equal(t, models.UserStatusShadow, u.Status)
	assert.Equal(t, "Newbie B", u.Name)

	// No notifications anywhere for the shadow.
	assert.Empty(t, env.notificationsFor(t, u.ID))
}

// --- C: existing shadow attaches to a second group, no new row, no notification ---

func TestAddMember_ExistingShadow_AttachesOnly(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head1, g1 := env.seedHead(t, "c1")
	head2, g2 := env.seedHead(t, "c2")

	email := "shadow-c@example.com"

	// Create the shadow by adding to G1.
	w1 := env.addByEmail(t, head1.ID, g1.ID, email, "Shadow C")
	require.Equal(t, http.StatusCreated, w1.Code)
	id1 := decodeMember(t, w1).UserID

	// Add the SAME email to G2 — attaches the existing shadow.
	w2 := env.addByEmail(t, head2.ID, g2.ID, email, "ignored")
	require.Equal(t, http.StatusCreated, w2.Code)
	id2 := decodeMember(t, w2).UserID

	// Same user id in both memberships.
	assert.Equal(t, id1, id2)

	// Still exactly one users row for the email.
	var count int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count))
	assert.Equal(t, 1, count)

	// Membership in both groups references the same shadow id.
	_, err := env.groupRepo.GetMember(ctx, g1.ID, id1)
	require.NoError(t, err)
	_, err = env.groupRepo.GetMember(ctx, g2.ID, id1)
	require.NoError(t, err)

	// No notifications for the shadow.
	assert.Empty(t, env.notificationsFor(t, id1))
}

// --- D: duplicate add is idempotent (409) ---

func TestAddMember_Duplicate_Returns409(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, group := env.seedHead(t, "d")
	target, err := env.userRepo.Create(ctx, "target-d@example.com", "hash", "Target D", nil, nil)
	require.NoError(t, err)

	w1 := env.addByEmail(t, head.ID, group.ID, target.Email, "ignored")
	require.Equal(t, http.StatusCreated, w1.Code)

	// Second add of the same email to the same group → 409.
	w2 := env.addByEmail(t, head.ID, group.ID, target.Email, "ignored")
	assert.Equal(t, http.StatusConflict, w2.Code)

	// Exactly one membership row.
	var memberships int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM group_members WHERE group_id = $1 AND user_id = $2`,
		group.ID, target.ID).Scan(&memberships))
	assert.Equal(t, 1, memberships)

	// No SECOND N-1 — exactly one from the first successful add.
	notifs := env.notificationsFor(t, target.ID)
	assert.Len(t, notifs, 1)
}

// --- E: claim keeps user id + memberships ---

func TestClaim_RegisterMatchingShadow_KeepsIdAndMemberships(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, group := env.seedHead(t, "e")
	email := "claimant-e@example.com"

	// Create the shadow by add-by-email; capture its id + membership.
	wAdd := env.addByEmail(t, head.ID, group.ID, email, "Claimant E")
	require.Equal(t, http.StatusCreated, wAdd.Code)
	shadowID := decodeMember(t, wAdd).UserID

	// Register with the same email → claim in place.
	wReg := doRequest(env.router, http.MethodPost, "/auth/register", map[string]interface{}{
		"email":    email,
		"password": "password123",
		"name":     "Claimant E",
	}, "")
	require.Equal(t, http.StatusCreated, wReg.Code)

	var reg handlers.LoginResponse
	require.NoError(t, json.Unmarshal(wReg.Body.Bytes(), &reg))
	assert.NotEmpty(t, reg.Token)
	assert.Equal(t, models.UserStatusRegistered, reg.User.Status)
	assert.Equal(t, shadowID, reg.User.ID, "claim keeps the same user id")

	// The users row: same id, now registered, non-NULL password, claimed_at set.
	u, err := env.userRepo.GetByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, shadowID, u.ID)
	assert.Equal(t, models.UserStatusRegistered, u.Status)
	require.NotNil(t, u.PasswordHash)
	require.NotNil(t, u.ClaimedAt)

	// The membership row still references that id.
	_, err = env.groupRepo.GetMember(ctx, group.ID, shadowID)
	require.NoError(t, err)
}

// --- F: claim writes N-2 to each group's head (not the claimant) ---

func TestClaim_NotifiesGroupAdmins(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	head1, g1 := env.seedHead(t, "f1")
	head2, g2 := env.seedHead(t, "f2")
	email := "claimant-f@example.com"

	// The shadow belongs to two groups with distinct heads.
	require.Equal(t, http.StatusCreated, env.addByEmail(t, head1.ID, g1.ID, email, "Claimant F").Code)
	require.Equal(t, http.StatusCreated, env.addByEmail(t, head2.ID, g2.ID, email, "ignored").Code)

	wReg := doRequest(env.router, http.MethodPost, "/auth/register", map[string]interface{}{
		"email":    email,
		"password": "password123",
		"name":     "Claimant F",
	}, "")
	require.Equal(t, http.StatusCreated, wReg.Code)
	var reg handlers.LoginResponse
	require.NoError(t, json.Unmarshal(wReg.Body.Bytes(), &reg))
	claimantID := reg.User.ID

	// Each head gets exactly one N-2 naming its own group; the claimant gets none.
	n1 := env.notificationsFor(t, head1.ID)
	require.Len(t, n1, 1)
	assert.Equal(t, models.NotificationShadowClaimed, n1[0].Type)
	assert.Equal(t, g1.ID.String(), n1[0].Payload["group_id"])
	assert.Equal(t, claimantID.String(), n1[0].Payload["claimed_user_id"])

	n2 := env.notificationsFor(t, head2.ID)
	require.Len(t, n2, 1)
	assert.Equal(t, models.NotificationShadowClaimed, n2[0].Type)
	assert.Equal(t, g2.ID.String(), n2[0].Payload["group_id"])

	assert.Empty(t, env.notificationsFor(t, claimantID), "claimant is never notified about their own claim")
}

// --- G: registered duplicate email still rejected ---

func TestRegister_ExistingRegisteredEmail_Rejected(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	email := "dup-g@example.com"
	body := map[string]interface{}{"email": email, "password": "password123", "name": "Dup G"}

	require.Equal(t, http.StatusCreated,
		doRequest(env.router, http.MethodPost, "/auth/register", body, "").Code)

	// Second register of the same registered email → 400.
	w := doRequest(env.router, http.MethodPost, "/auth/register", body, "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "email already exists")

	// No claim side effects: the user has no shadow_claimed notifications.
	u, err := env.userRepo.GetByEmail(t.Context(), email)
	require.NoError(t, err)
	assert.Empty(t, env.notificationsFor(t, u.ID))
}

// --- H: shadow login rejected ---

func TestLogin_ShadowUser_Rejected(t *testing.T) {
	env := setupIdentityTestEnv(t)
	defer env.cleanup()

	head, group := env.seedHead(t, "h")
	email := "shadow-h@example.com"
	require.Equal(t, http.StatusCreated, env.addByEmail(t, head.ID, group.ID, email, "Shadow H").Code)

	// Login with the shadow's email and any password → 401 generic, no token.
	w := doRequest(env.router, http.MethodPost, "/auth/login", map[string]interface{}{
		"email":    email,
		"password": "anything",
	}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid email or password", resp["error"])
	assert.Empty(t, resp["token"])
}
