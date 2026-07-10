//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// currencyTestEnv wires the full set of amount-carrying routes so the D7 currency
// behaviour (group currency, Money serialization, mismatch rejection) can be
// exercised end-to-end.
type currencyTestEnv struct {
	router    *gin.Engine
	pool      *pgxpool.Pool
	userRepo  *db.UserRepo
	groupRepo *db.GroupRepo
	cleanup   func()
}

func setupCurrencyTestEnv(t *testing.T) *currencyTestEnv {
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

	gh := handlers.NewGroupHandler(groupRepo, inviteRepo, choreRepo, ledgerRepo, loanRepo, allowanceRepo, userRepo, notificationRepo, postingSvc, pool, "")
	ch := handlers.NewChoreHandler(choreRepo, groupRepo)
	lh := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo, postingSvc, pool, db.NewAuditRepo(pool))
	ah := handlers.NewAllowanceHandler(allowanceRepo, groupRepo)
	loanH := handlers.NewLoanHandler(loanRepo, ledgerRepo, groupRepo, pool)

	router := gin.New()
	router.Use(auth.AuthMiddleware(testJWTSecret))
	router.POST("/groups", gh.CreateGroup)
	router.GET("/groups", gh.ListGroups)
	router.GET("/groups/:id", gh.GetGroup)
	router.GET("/groups/:id/chores", ch.ListChores)
	router.POST("/groups/:id/chores", ch.CreateChore)
	router.POST("/groups/:id/ledger", lh.CreateLedger)
	router.GET("/groups/:id/balance", lh.GetBalance)
	router.PUT("/groups/:id/allowances/:userId", ah.SetAllowance)
	router.POST("/groups/:id/loans", loanH.CreateLoan)

	return &currencyTestEnv{
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

// createUser creates a user and returns it.
func (e *currencyTestEnv) createUser(t *testing.T, suffix string) *models.User {
	t.Helper()
	u, err := e.userRepo.Create(t.Context(), fmt.Sprintf("cur-%s@example.com", suffix), "hash", "User "+suffix, nil, nil)
	require.NoError(t, err)
	return u
}

// createGroupAPI creates a group through the real handler (which also adds the
// head member) and returns the parsed response.
func (e *currencyTestEnv) createGroupAPI(t *testing.T, head *models.User, name, currency string) handlers.GroupResponse {
	t.Helper()
	w := doRequest(e.router, http.MethodPost, "/groups",
		map[string]interface{}{"name": name, "currency": currency}, bearerToken(t, head.ID))
	require.Equal(t, http.StatusCreated, w.Code, "create group %s/%s: %s", name, currency, w.Body.String())
	var resp handlers.GroupResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// addMember creates a member user and joins them to the group.
func (e *currencyTestEnv) addMember(t *testing.T, groupID uuid.UUID, suffix string) *models.User {
	t.Helper()
	u := e.createUser(t, suffix)
	_, err := e.groupRepo.AddMember(t.Context(), groupID, u.ID, models.RoleMember)
	require.NoError(t, err)
	return u
}

// --- Test 1: create group per supported currency ---

func TestCurrency_CreateGroupPerCurrency(t *testing.T) {
	env := setupCurrencyTestEnv(t)
	defer env.cleanup()

	for _, cur := range []string{models.CurrencyEUR, models.CurrencyUSD, models.CurrencyINR} {
		head := env.createUser(t, "mk"+cur)
		resp := env.createGroupAPI(t, head, "Grp "+cur, cur)
		assert.Equal(t, cur, resp.Currency, "response currency must echo the request")
	}
}

// --- Test 2: create group rejects bad/missing currency ---

func TestCurrency_CreateGroupRejectsBadCurrency(t *testing.T) {
	env := setupCurrencyTestEnv(t)
	defer env.cleanup()

	head := env.createUser(t, "bad")
	tok := bearerToken(t, head.ID)

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing", map[string]interface{}{"name": "G"}},
		{"unsupported GBP", map[string]interface{}{"name": "G", "currency": "GBP"}},
		{"wrong case eur", map[string]interface{}{"name": "G", "currency": "eur"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(env.router, http.MethodPost, "/groups", tc.body, tok)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%v resp=%s", tc.body, w.Body.String())
		})
	}
}

// --- Test 3: mismatched-currency writes rejected; matching accepted ---

func TestCurrency_MismatchedWritesRejected(t *testing.T) {
	env := setupCurrencyTestEnv(t)
	defer env.cleanup()

	head := env.createUser(t, "mmhead")
	group := env.createGroupAPI(t, head, "INR Group", models.CurrencyINR)
	member := env.addMember(t, group.ID, "mmmember")
	headTok := bearerToken(t, head.ID)

	eur := func(v int64) map[string]interface{} { return money(models.CurrencyEUR, v) }

	// Chore: EUR amount → 400; INR → 201.
	chorePath := fmt.Sprintf("/groups/%s/chores", group.ID)
	w := doRequest(env.router, http.MethodPost, chorePath,
		map[string]interface{}{"name": "Dishes", "amount": eur(1000)}, headTok)
	assert.Equal(t, http.StatusBadRequest, w.Code, "EUR chore in INR group must be rejected")
	w = doRequest(env.router, http.MethodPost, chorePath,
		map[string]interface{}{"name": "Dishes", "amount": inr(1000)}, headTok)
	require.Equal(t, http.StatusCreated, w.Code, "INR chore in INR group must succeed: %s", w.Body.String())

	// Ledger adjustment: EUR amount → 400; INR → 201.
	ledgerPath := fmt.Sprintf("/groups/%s/ledger", group.ID)
	w = doRequest(env.router, http.MethodPost, ledgerPath, map[string]interface{}{
		"entry_type": "adjustment", "user_id": member.ID, "amount": eur(500), "direction": "credit",
	}, headTok)
	assert.Equal(t, http.StatusBadRequest, w.Code, "EUR adjustment in INR group must be rejected")
	w = doRequest(env.router, http.MethodPost, ledgerPath, map[string]interface{}{
		"entry_type": "adjustment", "user_id": member.ID, "amount": inr(500), "direction": "credit",
	}, headTok)
	require.Equal(t, http.StatusCreated, w.Code, "INR adjustment must succeed: %s", w.Body.String())

	// Allowance: EUR amount → 400; INR → 200.
	allowPath := fmt.Sprintf("/groups/%s/allowances/%s", group.ID, member.ID)
	w = doRequest(env.router, http.MethodPut, allowPath, map[string]interface{}{"amount": eur(5000)}, headTok)
	assert.Equal(t, http.StatusBadRequest, w.Code, "EUR allowance in INR group must be rejected")
	w = doRequest(env.router, http.MethodPut, allowPath, map[string]interface{}{"amount": inr(5000)}, headTok)
	require.Equal(t, http.StatusOK, w.Code, "INR allowance must succeed: %s", w.Body.String())

	// Loan: EUR principal → 400; INR → 201.
	loanPath := fmt.Sprintf("/groups/%s/loans", group.ID)
	w = doRequest(env.router, http.MethodPost, loanPath,
		map[string]interface{}{"user_id": member.ID, "principal": eur(12000), "installments": 6}, headTok)
	assert.Equal(t, http.StatusBadRequest, w.Code, "EUR loan in INR group must be rejected")
	w = doRequest(env.router, http.MethodPost, loanPath,
		map[string]interface{}{"user_id": member.ID, "principal": inr(12000), "installments": 6}, headTok)
	require.Equal(t, http.StatusCreated, w.Code, "INR loan must succeed: %s", w.Body.String())
}

// --- Test 4 & 5: responses carry the group currency; groups are isolated ---

func TestCurrency_ResponsesCarryGroupCurrencyAndIsolation(t *testing.T) {
	env := setupCurrencyTestEnv(t)
	defer env.cleanup()

	// EUR group with a EUR chore.
	eurHead := env.createUser(t, "eurhead")
	eurGroup := env.createGroupAPI(t, eurHead, "EUR Group", models.CurrencyEUR)
	assert.Equal(t, models.CurrencyEUR, eurGroup.Currency)
	eurTok := bearerToken(t, eurHead.ID)

	w := doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/chores", eurGroup.ID),
		map[string]interface{}{"name": "Gelato", "amount": money(models.CurrencyEUR, 1250)}, eurTok)
	require.Equal(t, http.StatusCreated, w.Code, "%s", w.Body.String())
	var eurChore handlers.ChoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &eurChore))
	assert.Equal(t, models.CurrencyEUR, eurChore.Amount.Currency, "chore amount must carry group currency")
	assert.Equal(t, int64(1250), eurChore.Amount.Value)

	// EUR group balance response carries EUR.
	w = doRequest(env.router, http.MethodGet, fmt.Sprintf("/groups/%s/balance", eurGroup.ID), nil, eurTok)
	require.Equal(t, http.StatusOK, w.Code)
	var eurBalances []handlers.BalanceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &eurBalances))
	for _, b := range eurBalances {
		assert.Equal(t, models.CurrencyEUR, b.Balance.Currency)
	}

	// A separate INR group's chore carries INR — never the EUR group's currency.
	inrHead := env.createUser(t, "inrhead")
	inrGroup := env.createGroupAPI(t, inrHead, "INR Group", models.CurrencyINR)
	inrTok := bearerToken(t, inrHead.ID)
	w = doRequest(env.router, http.MethodPost, fmt.Sprintf("/groups/%s/chores", inrGroup.ID),
		map[string]interface{}{"name": "Dishes", "amount": inr(2000)}, inrTok)
	require.Equal(t, http.StatusCreated, w.Code, "%s", w.Body.String())
	var inrChore handlers.ChoreResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inrChore))
	assert.Equal(t, models.CurrencyINR, inrChore.Amount.Currency, "INR group's chore must report INR, not EUR")
	assert.Equal(t, int64(2000), inrChore.Amount.Value)

	// Dashboard listing: each group summary reports its own currency.
	w = doRequest(env.router, http.MethodGet, "/groups", nil, eurTok)
	require.Equal(t, http.StatusOK, w.Code)
	var summaries []handlers.GroupSummaryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &summaries))
	require.Len(t, summaries, 1, "eurHead belongs only to the EUR group")
	assert.Equal(t, models.CurrencyEUR, summaries[0].Currency)
	assert.Equal(t, models.CurrencyEUR, summaries[0].SummaryBalance.Currency,
		"summary_balance Money must carry the group currency")
}
