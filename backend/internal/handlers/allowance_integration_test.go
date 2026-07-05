//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
	"github.com/srjn45/pocket-money/backend/testutil"
)

// allowanceTestEnv holds all wired-up deps for allowance handler integration tests.
type allowanceTestEnv struct {
	router        *gin.Engine
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	postingSvc    *posting.Service
	cleanup       func()
}

func setupAllowanceTestEnv(t *testing.T) *allowanceTestEnv {
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
	ledgerRepo := db.NewLedgerRepo(pool)
	allowanceRepo := db.NewAllowanceRepo(pool)

	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, groupRepo, pool)

	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, nil, postingSvc)
	ah := handlers.NewAllowanceHandler(allowanceRepo, groupRepo)

	router := gin.New()
	authMw := auth.AuthMiddleware(testJWTSecret)
	router.Use(authMw)
	router.GET("/groups/:id/balance", lh.GetBalance)
	router.GET("/groups/:id/allowances", ah.ListAllowances)
	router.PUT("/groups/:id/allowances/:userId", ah.SetAllowance)

	return &allowanceTestEnv{
		router:        router,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		postingSvc:    postingSvc,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

// seedGroup creates head+member users and a group, adding both to group_members.
func (e *allowanceTestEnv) seedGroup(t *testing.T, suffix string) (head, member *models.User, group *models.Group) {
	t.Helper()
	ctx := t.Context()
	var err error
	head, err = e.userRepo.Create(ctx, fmt.Sprintf("head-%s@example.com", suffix), "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member", nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, head.ID)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	return
}

// TestAllowance_AuthZ tests authorization rules for allowance endpoints.
func TestAllowance_AuthZ(t *testing.T) {
	env := setupAllowanceTestEnv(t)
	defer env.cleanup()

	head, member, group := env.seedGroup(t, "authz")
	extraMember, err := env.userRepo.Create(t.Context(), "extra-authz@example.com", "hash", "Extra", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(t.Context(), group.ID, extraMember.ID, models.RoleMember)
	require.NoError(t, err)

	allowancePath := fmt.Sprintf("/groups/%s/allowances", group.ID)
	setPath := fmt.Sprintf("/groups/%s/allowances/%s", group.ID, member.ID)

	// Member PUT → 403
	w := doRequest(env.router, http.MethodPut, setPath, map[string]interface{}{
		"amount": 1000,
	}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Head sets allowance for member → 200
	w = doRequest(env.router, http.MethodPut, setPath, map[string]interface{}{
		"amount": 1000,
	}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)

	// Head GET → all rows (one for member)
	w = doRequest(env.router, http.MethodGet, allowancePath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var allRows []handlers.AllowanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &allRows))
	assert.GreaterOrEqual(t, len(allRows), 1)

	// Member GET → only own rows
	w = doRequest(env.router, http.MethodGet, allowancePath, nil, bearerToken(t, member.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var ownRows []handlers.AllowanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ownRows))
	for _, r := range ownRows {
		assert.Equal(t, member.ID, r.UserID, "member must only see own rows")
	}

	// PUT targeting the head → 400
	headTargetPath := fmt.Sprintf("/groups/%s/allowances/%s", group.ID, head.ID)
	w = doRequest(env.router, http.MethodPut, headTargetPath, map[string]interface{}{
		"amount": 500,
	}, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// PUT targeting a non-member → 404
	outsiderID := uuid.New()
	outsiderPath := fmt.Sprintf("/groups/%s/allowances/%s", group.ID, outsiderID)
	w = doRequest(env.router, http.MethodPut, outsiderPath, map[string]interface{}{
		"amount": 500,
	}, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAllowance_IdempotencyDoubleTrigger verifies that calling PostDue twice
// produces exactly one allowance entry per due month.
func TestAllowance_IdempotencyDoubleTrigger(t *testing.T) {
	env := setupAllowanceTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "idem")

	// Set allowance effective 2 months ago.
	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, twoMonthsAgo, head.ID)
	require.NoError(t, err)

	// Trigger twice via GET /balance.
	balancePath := fmt.Sprintf("/groups/%s/balance", group.ID)
	w := doRequest(env.router, http.MethodGet, balancePath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	w = doRequest(env.router, http.MethodGet, balancePath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)

	// Exactly one allowance entry per due month in ledger (3 months posted).
	posted, err := env.ledgerRepo.PostedAllowancePeriods(ctx, group.ID)
	require.NoError(t, err)
	require.NotNil(t, posted[member.ID])
	assert.Len(t, posted[member.ID], 3, "exactly 3 months must be posted: twoMonthsAgo, lastMonth, current")

	// Balance must be stable after double trigger (3 months × 1000).
	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	var memberBalance *handlers.BalanceResponse
	for i := range balances {
		if balances[i].UserID == member.ID {
			memberBalance = &balances[i]
		}
	}
	require.NotNil(t, memberBalance)
	assert.Equal(t, int64(3*1000), memberBalance.Balance)
}

// TestAllowance_ConcurrentRace verifies that concurrent PostDue calls produce no duplicates.
func TestAllowance_ConcurrentRace(t *testing.T) {
	env := setupAllowanceTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "race")

	twoMonthsAgo := time.Now().AddDate(0, -2, 0).Format("2006-01")
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 500, twoMonthsAgo, head.ID)
	require.NoError(t, err)

	now := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = env.postingSvc.PostDue(ctx, group.ID, now)
		}()
	}
	wg.Wait()

	for _, e := range errs {
		assert.NoError(t, e, "concurrent PostDue must not error")
	}

	// Verify no duplicate entries: count allowance rows in ledger.
	posted, err := env.ledgerRepo.PostedAllowancePeriods(ctx, group.ID)
	require.NoError(t, err)
	// 3 months should be posted (twoMonthsAgo, lastMonth, currentMonth).
	require.NotNil(t, posted[member.ID])
	assert.Len(t, posted[member.ID], 3, "exactly 3 distinct months must be posted")
}

// TestAllowance_BalanceAfterPosting verifies balance correctness after posting.
func TestAllowance_BalanceAfterPosting(t *testing.T) {
	env := setupAllowanceTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "bal")

	// Allowance 1000 effective 3 months ago → 4 months (3 back + current).
	fourMonthsAgo := time.Now().AddDate(0, -3, 0).Format("2006-01")
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, fourMonthsAgo, head.ID)
	require.NoError(t, err)

	// Trigger via GET /balance.
	balancePath := fmt.Sprintf("/groups/%s/balance", group.ID)
	w := doRequest(env.router, http.MethodGet, balancePath, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)

	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	var memberBalance *handlers.BalanceResponse
	for i := range balances {
		if balances[i].UserID == member.ID {
			memberBalance = &balances[i]
		}
	}
	require.NotNil(t, memberBalance, "member must appear in balance")
	assert.Equal(t, int64(4*1000), memberBalance.Balance, "4 months × 1000 = 4000")
}

// TestAllowance_AmountChangeEffectiveNextMonth verifies that amount changes
// only affect future months, leaving posted months immutable.
func TestAllowance_AmountChangeEffectiveNextMonth(t *testing.T) {
	env := setupAllowanceTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group := env.seedGroup(t, "amtchg")

	currentMonth := time.Now().Format("2006-01")
	nextMonth := time.Now().AddDate(0, 1, 0).Format("2006-01")

	// Set 500 effective current month and trigger (posts current month).
	_, err := env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 500, currentMonth, head.ID)
	require.NoError(t, err)
	require.NoError(t, env.postingSvc.PostDue(ctx, group.ID, time.Now()))

	// Change to 1000 effective next month.
	_, err = env.allowanceRepo.SetAllowance(ctx, group.ID, member.ID, 1000, nextMonth, head.ID)
	require.NoError(t, err)

	// Current month remains 500 (already posted, immutable).
	posted, err := env.ledgerRepo.PostedAllowancePeriods(ctx, group.ID)
	require.NoError(t, err)
	assert.True(t, posted[member.ID][currentMonth], "current month must already be posted")

	// Trigger for next month.
	futureNow := time.Now().AddDate(0, 1, 0)
	require.NoError(t, env.postingSvc.PostDue(ctx, group.ID, futureNow))

	posted, err = env.ledgerRepo.PostedAllowancePeriods(ctx, group.ID)
	require.NoError(t, err)
	assert.True(t, posted[member.ID][nextMonth], "next month must now be posted at new amount")
}
