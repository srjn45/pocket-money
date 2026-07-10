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

// groupsTestEnv holds wired-up deps for groups handler integration tests.
type groupsTestEnv struct {
	router        *gin.Engine
	pool          *pgxpool.Pool
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	choreRepo     *db.ChoreRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	loanRepo      *db.LoanRepo
	postingSvc    *posting.Service
	cleanup       func()
}

func setupGroupsTestEnv(t *testing.T) *groupsTestEnv {
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

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)

	inviteRepo := db.NewInviteRepo(pool)
	notificationRepo := db.NewNotificationRepo(pool)
	// appBaseURL is only consumed by CreateInvite (WP-4.3); these tests exercise
	// GET /groups only, so the empty fall-back value is fine.
	gh := handlers.NewGroupHandler(groupRepo, inviteRepo, choreRepo, ledgerRepo, loanRepo, allowanceRepo, userRepo, notificationRepo, postingSvc, pool, "")

	router := gin.New()
	authMw := auth.AuthMiddleware(testJWTSecret)
	router.Use(authMw)
	router.GET("/groups", gh.ListGroups)

	return &groupsTestEnv{
		router:        router,
		pool:          pool,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		choreRepo:     choreRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		loanRepo:      loanRepo,
		postingSvc:    postingSvc,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedGroupHead creates a head user + group with the head in group_members.
func (e *groupsTestEnv) seedGroupHead(t *testing.T, suffix string) (head *models.User, group *models.Group) {
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

// addMember adds a new member user to an existing group.
func (e *groupsTestEnv) addMember(t *testing.T, groupID uuid.UUID, suffix string) *models.User {
	t.Helper()
	ctx := t.Context()
	u, err := e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member "+suffix, nil, nil)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, groupID, u.ID, models.RoleMember)
	require.NoError(t, err)
	return u
}

// insertLedger inserts a ledger entry directly without going through the handler.
func (e *groupsTestEnv) insertLedger(t *testing.T, groupID, userID uuid.UUID, createdBy uuid.UUID,
	amount int64, direction models.LedgerDirection, status models.LedgerStatus) {
	t.Helper()
	ctx := t.Context()
	now := time.Now()
	var decidedBy *uuid.UUID
	var decidedAt *time.Time
	if status == models.StatusApproved {
		decidedBy = &createdBy
		decidedAt = &now
	}
	_, err := e.ledgerRepo.Create(ctx, groupID, userID, nil, createdBy, amount,
		models.EntryTypeChore, direction, status, nil, decidedBy, decidedAt)
	require.NoError(t, err)
}

// listGroups hits GET /groups as the given user and returns parsed response.
func listGroups(t *testing.T, env *groupsTestEnv, callerID uuid.UUID) []handlers.GroupSummaryResponse {
	t.Helper()
	w := doRequest(env.router, http.MethodGet, "/groups", nil, bearerToken(t, callerID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp []handlers.GroupSummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// findGroup returns the summary row for groupID from a list response, failing the test if missing.
func findGroup(t *testing.T, rows []handlers.GroupSummaryResponse, groupID uuid.UUID) handlers.GroupSummaryResponse {
	t.Helper()
	for _, r := range rows {
		if r.ID == groupID {
			return r
		}
	}
	t.Fatalf("group %s not found in response", groupID)
	return handlers.GroupSummaryResponse{}
}

// --- Test 1: Head total = sum of member balances ---

func TestGroups_HeadTotalSumOfMemberBalances(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t1")
	memberA := env.addMember(t, group.ID, "t1a")
	memberB := env.addMember(t, group.ID, "t1b")

	// memberA: +2000 credit, -500 debit → balance = +1500
	env.insertLedger(t, group.ID, memberA.ID, head.ID, 2000, models.DirectionCredit, models.StatusApproved)
	env.insertLedger(t, group.ID, memberA.ID, head.ID, 500, models.DirectionDebit, models.StatusApproved)
	// memberB: +300 credit, -800 debit → balance = -500
	env.insertLedger(t, group.ID, memberB.ID, head.ID, 300, models.DirectionCredit, models.StatusApproved)
	env.insertLedger(t, group.ID, memberB.ID, head.ID, 800, models.DirectionDebit, models.StatusApproved)

	rows := listGroups(t, env, head.ID)
	row := findGroup(t, rows, group.ID)

	assert.Equal(t, models.RoleHead, row.Role)
	assert.Equal(t, 3, row.MemberCount)                    // head + 2 members
	assert.Equal(t, int64(1000), row.SummaryBalance.Value) // 1500 + (-500) = 1000
}

// --- Test 2: Member sees own balance only ---

func TestGroups_MemberSeesOwnBalanceOnly(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t2")
	memberA := env.addMember(t, group.ID, "t2a")
	env.addMember(t, group.ID, "t2b") // memberB — balance not relevant to memberA's view

	// memberA: +1500 (from test 1 scenario)
	env.insertLedger(t, group.ID, memberA.ID, head.ID, 2000, models.DirectionCredit, models.StatusApproved)
	env.insertLedger(t, group.ID, memberA.ID, head.ID, 500, models.DirectionDebit, models.StatusApproved)

	rows := listGroups(t, env, memberA.ID)
	row := findGroup(t, rows, group.ID)

	assert.Equal(t, models.RoleMember, row.Role)
	assert.Equal(t, int64(1500), row.SummaryBalance.Value, "memberA sees own balance")
	// no way to check B's balance isn't leaked — verified structurally in test 3
}

// --- Test 3: Member privacy (cannot see others' totals) ---

func TestGroups_MemberPrivacy(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t3")
	memberA := env.addMember(t, group.ID, "t3a")
	memberB := env.addMember(t, group.ID, "t3b")

	// memberA: +300
	env.insertLedger(t, group.ID, memberA.ID, head.ID, 300, models.DirectionCredit, models.StatusApproved)
	// memberB: large credit that should NOT affect memberA's row
	env.insertLedger(t, group.ID, memberB.ID, head.ID, 999999, models.DirectionCredit, models.StatusApproved)

	rows := listGroups(t, env, memberA.ID)
	row := findGroup(t, rows, group.ID)

	assert.Equal(t, models.RoleMember, row.Role)
	assert.Equal(t, int64(300), row.SummaryBalance.Value,
		"adding large credit to memberB must not change memberA's summary_balance")
}

// --- Test 4: Empty group (head only, no members) ---

func TestGroups_EmptyGroup(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t4")
	// no members added beyond head

	rows := listGroups(t, env, head.ID)
	row := findGroup(t, rows, group.ID)

	assert.Equal(t, models.RoleHead, row.Role)
	assert.Equal(t, 1, row.MemberCount, "only head in group")
	assert.Equal(t, int64(0), row.SummaryBalance.Value, "no non-head members to owe")
}

// --- Test 5: Group with members but no ledger entries ---

func TestGroups_MembersNoLedger(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t5")
	memberA := env.addMember(t, group.ID, "t5a")
	memberB := env.addMember(t, group.ID, "t5b")

	// head view: no ledger entries → COALESCE path
	rowsHead := listGroups(t, env, head.ID)
	rowHead := findGroup(t, rowsHead, group.ID)
	assert.Equal(t, 3, rowHead.MemberCount)
	assert.Equal(t, int64(0), rowHead.SummaryBalance.Value)

	// member view: no ledger entries → COALESCE path
	rowsMember := listGroups(t, env, memberA.ID)
	rowMember := findGroup(t, rowsMember, group.ID)
	assert.Equal(t, int64(0), rowMember.SummaryBalance.Value)

	_ = memberB
}

// --- Test 6: Multi-group user, mixed roles ---

func TestGroups_MultiGroupMixedRoles(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	// user is head of G1 and member of G2
	user, err := env.userRepo.Create(ctx, "multi-t6@example.com", "hash", "Multi", nil, nil)
	require.NoError(t, err)

	// G1: user is head
	g1, err := env.groupRepo.Create(ctx, "G1 t6", user.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, g1.ID, user.ID, models.RoleHead)
	require.NoError(t, err)
	memberG1 := env.addMember(t, g1.ID, "t6g1m")
	env.insertLedger(t, g1.ID, memberG1.ID, user.ID, 800, models.DirectionCredit, models.StatusApproved)

	// G2: user is member, head is someone else
	head2, err := env.userRepo.Create(ctx, "head2-t6@example.com", "hash", "Head2", nil, nil)
	require.NoError(t, err)
	g2, err := env.groupRepo.Create(ctx, "G2 t6", head2.ID, models.CurrencyINR)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, g2.ID, head2.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, g2.ID, user.ID, models.RoleMember)
	require.NoError(t, err)
	env.insertLedger(t, g2.ID, user.ID, head2.ID, 500, models.DirectionCredit, models.StatusApproved)

	rows := listGroups(t, env, user.ID)
	require.Len(t, rows, 2)

	rowG1 := findGroup(t, rows, g1.ID)
	assert.Equal(t, models.RoleHead, rowG1.Role)
	assert.Equal(t, int64(800), rowG1.SummaryBalance.Value, "G1: total owed to non-head members")

	rowG2 := findGroup(t, rows, g2.ID)
	assert.Equal(t, models.RoleMember, rowG2.Role)
	assert.Equal(t, int64(500), rowG2.SummaryBalance.Value, "G2: user's own balance")
}

// --- Test 7: Rejected/pending entries excluded ---

func TestGroups_RejectedPendingExcluded(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	head, group := env.seedGroupHead(t, "t7")
	member := env.addMember(t, group.ID, "t7m")

	// approved credit: +200
	env.insertLedger(t, group.ID, member.ID, head.ID, 200, models.DirectionCredit, models.StatusApproved)
	// pending credit: should NOT count
	env.insertLedger(t, group.ID, member.ID, member.ID, 500, models.DirectionCredit, models.StatusPendingApproval)
	// rejected credit: should NOT count
	env.insertLedger(t, group.ID, member.ID, member.ID, 1000, models.DirectionCredit, models.StatusRejected)

	// member view
	rows := listGroups(t, env, member.ID)
	row := findGroup(t, rows, group.ID)
	assert.Equal(t, int64(200), row.SummaryBalance.Value, "only approved entry counts")

	// head view
	rowsH := listGroups(t, env, head.ID)
	rowH := findGroup(t, rowsH, group.ID)
	assert.Equal(t, int64(200), rowH.SummaryBalance.Value, "head total reflects only approved")
}

// --- Test 8: No posting side-effect (D1) ---

func TestGroups_NoPostingSideEffect(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, group := env.seedGroupHead(t, "t8")
	member := env.addMember(t, group.ID, "t8m")

	// Set an allowance for the member effective a past month (would trigger posting if called).
	pastMonth := time.Now().AddDate(0, -2, 0).Format("2006-01")
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, pastMonth, head.ID)
	require.NoError(t, err)
	// Backdate joined_at so the engine would post multiple months if triggered.
	_, err = env.pool.Exec(ctx,
		`UPDATE group_members SET joined_at = $1::timestamptz WHERE group_id = $2 AND user_id = $3`,
		pastMonth+"-01T00:00:00Z", group.ID, member.ID)
	require.NoError(t, err)

	// Count allowance ledger rows BEFORE the GET /groups call.
	var beforeCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id = $1 AND entry_type = 'allowance'`,
		group.ID).Scan(&beforeCount)
	require.NoError(t, err)
	assert.Equal(t, 0, beforeCount, "no allowance entries before the GET /groups call")

	// Hit GET /groups — must NOT trigger PostDue.
	rows := listGroups(t, env, head.ID)
	_ = findGroup(t, rows, group.ID) // endpoint must succeed

	// Count again — must be unchanged.
	var afterCount int
	err = env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ledger_entries WHERE group_id = $1 AND entry_type = 'allowance'`,
		group.ID).Scan(&afterCount)
	require.NoError(t, err)
	assert.Equal(t, 0, afterCount, "GET /groups must not trigger allowance posting (D1)")
}

// --- Test: order is created_at DESC ---

func TestGroups_OrderByCreatedAtDesc(t *testing.T) {
	env := setupGroupsTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, err := env.userRepo.Create(ctx, "head-order@example.com", "hash", "Head", nil, nil)
	require.NoError(t, err)

	// Create three groups; DB RETURNING created_at gives stable ordering.
	var groupIDs []uuid.UUID
	for i := 0; i < 3; i++ {
		g, err := env.groupRepo.Create(ctx, fmt.Sprintf("Group %d", i), head.ID, models.CurrencyINR)
		require.NoError(t, err)
		_, err = env.groupRepo.AddMember(ctx, g.ID, head.ID, models.RoleHead)
		require.NoError(t, err)
		groupIDs = append(groupIDs, g.ID)
	}

	rows := listGroups(t, env, head.ID)
	require.Len(t, rows, 3)

	// Rows should be in created_at DESC — last created first.
	assert.Equal(t, groupIDs[2], rows[0].ID)
	assert.Equal(t, groupIDs[1], rows[1].ID)
	assert.Equal(t, groupIDs[0], rows[2].ID)
}
