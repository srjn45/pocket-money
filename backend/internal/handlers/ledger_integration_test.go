//go:build integration

package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

const testJWTSecret = "test-jwt-secret-for-ledger-integration"

// ledgerTestEnv holds all wired-up deps for a ledger handler integration test.
type ledgerTestEnv struct {
	router        *gin.Engine
	userRepo      *db.UserRepo
	groupRepo     *db.GroupRepo
	choreRepo     *db.ChoreRepo
	ledgerRepo    *db.LedgerRepo
	allowanceRepo *db.AllowanceRepo
	cleanup       func()
}

func setupLedgerTestEnv(t *testing.T) *ledgerTestEnv {
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

	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo, postingSvc, pool, db.NewAuditRepo(pool))

	router := gin.New()
	authMw := auth.AuthMiddleware(testJWTSecret)
	router.Use(authMw)
	router.GET("/groups/:id/ledger", lh.ListLedger)
	router.POST("/groups/:id/ledger", lh.CreateLedger)
	router.POST("/ledger/:id/approve", lh.ApproveLedger)
	router.POST("/ledger/:id/reject", lh.RejectLedger)
	router.PUT("/ledger/:id", lh.EditLedger)
	router.DELETE("/ledger/:id", lh.DeleteLedger)
	router.GET("/groups/:id/balance", lh.GetBalance)

	return &ledgerTestEnv{
		router:        router,
		userRepo:      userRepo,
		groupRepo:     groupRepo,
		choreRepo:     choreRepo,
		ledgerRepo:    ledgerRepo,
		allowanceRepo: allowanceRepo,
		cleanup: func() {
			testutil.CleanupTestDB(pool)
			pool.Close()
		},
	}
}

func bearerToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.IssueToken(userID.String(), testJWTSecret)
	require.NoError(t, err)
	return "Bearer " + tok
}

// money builds a Money request field for JSON request bodies in tests.
func money(currency string, value int64) map[string]interface{} {
	return map[string]interface{}{"currency": currency, "value": value}
}

// inr is money in INR — the default currency for handler integration test groups.
func inr(value int64) map[string]interface{} {
	return money(models.CurrencyINR, value)
}

func doRequest(router *gin.Engine, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// seedGroupWithMembers creates head + member users and a group, returning their IDs.
func (e *ledgerTestEnv) seedGroupWithMembers(t *testing.T, suffix string) (head, member *models.User, group *models.Group, chore *models.Chore) {
	t.Helper()
	ctx := t.Context()
	var err error
	head, err = e.userRepo.Create(ctx, fmt.Sprintf("head-%s@example.com", suffix), "hash", "Head", nil, nil)
	require.NoError(t, err)
	member, err = e.userRepo.Create(ctx, fmt.Sprintf("member-%s@example.com", suffix), "hash", "Member", nil, nil)
	require.NoError(t, err)
	group, err = e.groupRepo.Create(ctx, "Family "+suffix, head.ID, models.CurrencyINR)
	require.NoError(t, err)
	// GroupRepo.Create only inserts the group row; the head must be added to
	// group_members explicitly (the production CreateGroup handler does this).
	// Without it, head-authenticated requests 403 as "not a member".
	_, err = e.groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleAdmin)
	require.NoError(t, err)
	_, err = e.groupRepo.AddMember(ctx, group.ID, member.ID, models.RoleMember)
	require.NoError(t, err)
	chore, err = e.choreRepo.Create(ctx, group.ID, "Dishes", nil, 1000)
	require.NoError(t, err)
	return
}

// --- Test #2: Member can't see others ---

func TestLedger_MemberCannotSeeOthers(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, memberA, group, chore := env.seedGroupWithMembers(t, "vis")
	memberB, err := env.userRepo.Create(ctx, "memberb-vis@example.com", "hash", "MemberB", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, group.ID, memberB.ID, models.RoleMember)
	require.NoError(t, err)

	now := time.Now()
	// Create one approved entry for each member
	_, err = env.ledgerRepo.Create(ctx, group.ID, memberA.ID, &chore.ID, head.ID, 1000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)
	_, err = env.ledgerRepo.Create(ctx, group.ID, memberB.ID, &chore.ID, head.ID, 1000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	path := fmt.Sprintf("/groups/%s/ledger", group.ID)

	// D6: memberA explicitly requesting memberB's rows is a 403 (not a silent narrow).
	w := doRequest(env.router, http.MethodGet, path+"?user_id="+memberB.ID.String(), nil, bearerToken(t, memberA.ID))
	require.Equal(t, http.StatusForbidden, w.Code)
	var errResp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "members can only access their own data", errResp["error"])

	// memberA scoping to self (user_id=self) is allowed → own rows.
	var resp []handlers.LedgerResponse
	w = doRequest(env.router, http.MethodGet, path+"?user_id="+memberA.ID.String(), nil, bearerToken(t, memberA.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, memberA.ID, resp[0].UserID)

	// memberA with no user_id param → own rows only.
	w = doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, memberA.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, memberA.ID, resp[0].UserID)

	// admin can filter by memberB and see memberB's entries.
	w = doRequest(env.router, http.MethodGet, path+"?user_id="+memberB.ID.String(), nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, memberB.ID, resp[0].UserID)
}

// --- TestBalance_MemberScopedToOwn: D6 balance scoping ---

// A member GET /balance sees only their own balance row; an admin sees every
// member's row (the admin/payer is excluded by GetBalanceForGroup).
func TestBalance_MemberScopedToOwn(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, memberA, group, chore := env.seedGroupWithMembers(t, "bal")
	memberB, err := env.userRepo.Create(ctx, "memberb-bal@example.com", "hash", "MemberB", nil, nil)
	require.NoError(t, err)
	_, err = env.groupRepo.AddMember(ctx, group.ID, memberB.ID, models.RoleMember)
	require.NoError(t, err)

	now := time.Now()
	_, err = env.ledgerRepo.Create(ctx, group.ID, memberA.ID, &chore.ID, head.ID, 1000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)
	_, err = env.ledgerRepo.Create(ctx, group.ID, memberB.ID, &chore.ID, head.ID, 2000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	path := fmt.Sprintf("/groups/%s/balance", group.ID)

	// memberA sees only their own single balance row.
	w := doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, memberA.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, memberA.ID, resp[0].UserID)

	// admin sees both members' rows, and NOT the admin/payer.
	w = doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	ids := map[uuid.UUID]bool{}
	for _, b := range resp {
		ids[b.UserID] = true
	}
	assert.True(t, ids[memberA.ID], "admin should see memberA")
	assert.True(t, ids[memberB.ID], "admin should see memberB")
	assert.False(t, ids[head.ID], "admin/payer must be excluded from the balance list")
}

// --- Test #3: Head-only settlement/adjustment ---

func TestLedger_HeadOnlySettlementAdjustment(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	head, member, group, chore := env.seedGroupWithMembers(t, "authz")
	// D2 (V3-3.2): member chore submission defaults OFF; enable it so the member
	// self-submit branch below is functional (behavior change, not a regression).
	require.NoError(t, env.groupRepo.SetChoreSubmissionEnabled(t.Context(), group.ID, true))
	path := fmt.Sprintf("/groups/%s/ledger", group.ID)

	// Member tries settlement → 403
	w := doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "settlement",
		"user_id":    member.ID,
		"amount":     inr(500),
	}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Member tries adjustment → 403
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "adjustment",
		"user_id":    member.ID,
		"amount":     inr(500),
		"direction":  "credit",
	}, bearerToken(t, member.ID))
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Head creates settlement debit → 201
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "settlement",
		"user_id":    member.ID,
		"amount":     inr(300),
	}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var created handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, models.EntryTypeSettlement, created.EntryType)
	assert.Equal(t, models.DirectionDebit, created.Direction)
	assert.Equal(t, models.StatusApproved, created.Status)
	assert.NotNil(t, created.DecidedBy)
	assert.Equal(t, head.ID, *created.DecidedBy)
	assert.NotNil(t, created.DecidedAt)

	// Head creates credit adjustment → 201
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "adjustment",
		"user_id":    member.ID,
		"amount":     inr(200),
		"direction":  "credit",
		"note":       "bonus",
	}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, models.EntryTypeAdjustment, created.EntryType)
	assert.Equal(t, models.DirectionCredit, created.Direction)
	assert.Equal(t, models.StatusApproved, created.Status)

	// Member creates chore entry for self → 201, pending_approval
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "chore",
		"chore_id":   chore.ID,
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, models.StatusPendingApproval, created.Status)
	assert.Equal(t, member.ID, created.UserID)
	assert.Equal(t, chore.Amount, created.Amount.Value, "chore config amount must be used")

	// Member tries to pass custom amount → amount must equal chore config (ignored)
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "chore",
		"chore_id":   chore.ID,
		"amount":     inr(99999),
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, chore.Amount, created.Amount.Value, "custom amount must be ignored for chore entries")
}

// --- Test #4: decided_by recorded on approve/reject ---

func TestLedger_DecidedByOnApproveReject(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group, chore := env.seedGroupWithMembers(t, "dec")
	// D2 (V3-3.2): enable member chore submission (default OFF) so the member can
	// submit a pending chore for the admin to approve/reject below.
	require.NoError(t, env.groupRepo.SetChoreSubmissionEnabled(ctx, group.ID, true))

	path := fmt.Sprintf("/groups/%s/ledger", group.ID)

	// Member creates pending chore entry
	w := doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "chore",
		"chore_id":   chore.ID,
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var pending handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pending))
	require.Equal(t, models.StatusPendingApproval, pending.Status)
	assert.Nil(t, pending.DecidedBy)

	// Head approves → decided_by=head, decided_at set
	w = doRequest(env.router, http.MethodPost, fmt.Sprintf("/ledger/%s/approve", pending.ID), nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var approved handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &approved))
	assert.Equal(t, models.StatusApproved, approved.Status)
	require.NotNil(t, approved.DecidedBy)
	assert.Equal(t, head.ID, *approved.DecidedBy)
	assert.NotNil(t, approved.DecidedAt)

	// Approved entry must count toward balance; create another pending to reject
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "chore",
		"chore_id":   chore.ID,
	}, bearerToken(t, member.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var pending2 handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pending2))

	// Head rejects → decided_by=head
	w = doRequest(env.router, http.MethodPost, fmt.Sprintf("/ledger/%s/reject", pending2.ID), nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var rejected handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rejected))
	assert.Equal(t, models.StatusRejected, rejected.Status)
	require.NotNil(t, rejected.DecidedBy)
	assert.Equal(t, head.ID, *rejected.DecidedBy)

	// Balance = approved chore (1000) only; rejected entry excluded
	w = doRequest(env.router, http.MethodGet, fmt.Sprintf("/groups/%s/balance", group.ID), nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var balances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &balances))
	var memberBalance *handlers.BalanceResponse
	for i := range balances {
		if balances[i].UserID == member.ID {
			memberBalance = &balances[i]
			break
		}
	}
	require.NotNil(t, memberBalance)
	assert.Equal(t, chore.Amount, memberBalance.Balance.Value, "only approved entry should count toward balance")

	// Head-created chore entry is immediately approved with decided_by set
	w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{
		"entry_type": "chore",
		"chore_id":   chore.ID,
		"user_id":    member.ID,
	}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusCreated, w.Code)
	var headCreated handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &headCreated))
	assert.Equal(t, models.StatusApproved, headCreated.Status)
	require.NotNil(t, headCreated.DecidedBy)
	assert.Equal(t, head.ID, *headCreated.DecidedBy)
	assert.NotNil(t, headCreated.DecidedAt)
	_ = ctx
}

// --- Test #5: entry_type/period filters + validation ---

func TestLedger_TypeAndPeriodFiltersAndValidation(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	ctx := t.Context()
	head, member, group, chore := env.seedGroupWithMembers(t, "filt")

	now := time.Now()
	// Seed: one chore credit + one settlement debit (both approved)
	_, err := env.ledgerRepo.Create(ctx, group.ID, member.ID, &chore.ID, head.ID, 1000,
		models.EntryTypeChore, models.DirectionCredit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)
	_, err = env.ledgerRepo.Create(ctx, group.ID, member.ID, nil, head.ID, 400,
		models.EntryTypeSettlement, models.DirectionDebit, models.StatusApproved, nil, &head.ID, &now)
	require.NoError(t, err)

	basePath := fmt.Sprintf("/groups/%s/ledger", group.ID)

	// ?type=settlement returns only settlements
	w := doRequest(env.router, http.MethodGet, basePath+"?type=settlement", nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	var resp []handlers.LedgerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	for _, e := range resp {
		assert.Equal(t, models.EntryTypeSettlement, e.EntryType)
	}
	assert.Len(t, resp, 1)

	// ?type=chore returns only chore entries
	w = doRequest(env.router, http.MethodGet, basePath+"?type=chore", nil, bearerToken(t, head.ID))
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, models.EntryTypeChore, resp[0].EntryType)

	// invalid type → 400
	w = doRequest(env.router, http.MethodGet, basePath+"?type=invalid", nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// invalid period → 400
	w = doRequest(env.router, http.MethodGet, basePath+"?period=bad-period", nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// valid period format accepted (returns empty since no period-set entries exist)
	w = doRequest(env.router, http.MethodGet, basePath+"?period=2026-07", nil, bearerToken(t, head.ID))
	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp)

	// allowance/emi on POST → 400 machine-posted-only
	settlePath := fmt.Sprintf("/groups/%s/ledger", group.ID)
	for _, et := range []string{"allowance", "emi"} {
		w = doRequest(env.router, http.MethodPost, settlePath, map[string]interface{}{
			"entry_type": et,
		}, bearerToken(t, head.ID))
		assert.Equal(t, http.StatusBadRequest, w.Code, "entry_type=%s should be rejected", et)
	}
}

// --- Route removal: /pending and /settlements return 404 ---

func TestLedger_RemovedRoutesReturn404(t *testing.T) {
	env := setupLedgerTestEnv(t)
	defer env.cleanup()

	head, _, group, _ := env.seedGroupWithMembers(t, "routes")

	// These routes are no longer registered; Gin returns 404
	for _, path := range []string{
		fmt.Sprintf("/groups/%s/pending", group.ID),
		fmt.Sprintf("/groups/%s/settlements", group.ID),
	} {
		w := doRequest(env.router, http.MethodGet, path, nil, bearerToken(t, head.ID))
		assert.Equal(t, http.StatusNotFound, w.Code, "path %s should return 404", path)
		w = doRequest(env.router, http.MethodPost, path, map[string]interface{}{}, bearerToken(t, head.ID))
		assert.Equal(t, http.StatusNotFound, w.Code, "POST %s should return 404", path)
	}
}
