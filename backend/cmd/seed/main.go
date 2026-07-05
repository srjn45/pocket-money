// Seed builds the demo family into the database and writes a machine-readable
// summary. It is DESTRUCTIVE: it TRUNCATEs the demo table set and rebuilds
// from scratch. Guard: the target DB name must be in the allow-list OR the
// --reset flag must be passed.
//
// Usage:
//
//	DATABASE_URL=... JWT_SECRET=... go run ./cmd/seed [--reset] [--out path]
//
// DESTROYS all data in the target DB. Demo/dev/CI only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/srjn45/pocket-money/backend/internal/config"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/models"
	"github.com/srjn45/pocket-money/backend/internal/posting"
)

// Demo credentials (exported constants mirrored in e2e/fixtures/seed.ts).
const (
	HeadEmail  = "head@demo.test"
	HeadName   = "Priya Sharma"
	AaravEmail = "aarav@demo.test"
	AaravName  = "Aarav Sharma"
	DiyaEmail  = "diya@demo.test"
	DiyaName   = "Diya Sharma"
	DemoPass   = "demo1234"
	GroupName  = "Sharma Family"
)

// SummaryUser is one user's entry in the JSON output.
type SummaryUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// SummaryGroup is the group section of the JSON output.
type SummaryGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SeedSummary is written to --out (default ../e2e/fixtures/seed.summary.json).
type SeedSummary struct {
	CurrentPeriod    string                 `json:"current_period"`
	Users            map[string]SummaryUser `json:"users"`
	Group            SummaryGroup           `json:"group"`
	ExpectedBalances map[string]int64       `json:"expected_balances"`
	LoanOutstanding  int64                  `json:"loan_outstanding"`
}

func main() {
	resetFlag := flag.Bool("reset", false, "Truncate the demo tables (DESTROYS ALL DATA). Required for unrecognized DB names.")
	outPath := flag.String("out", "../e2e/fixtures/seed.summary.json", "Write JSON summary to this path; '-' prints to stdout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: seed [--reset] [--out path]\n\n")
		fmt.Fprintf(os.Stderr, "WARNING: --reset DESTROYS ALL DATA in the target database.\n")
		fmt.Fprintf(os.Stderr, "Only run against demo/dev/CI databases.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := run(ctx, cfg, *resetFlag, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, resetFlag bool, outPath string) error {
	pool, err := db.NewPool(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if err := checkMigrations(ctx, pool); err != nil {
		return err
	}

	dbName, err := getDatabaseName(ctx, pool)
	if err != nil {
		return err
	}
	allowedDBs := map[string]bool{
		"pocket_money": true, "pocket_money_test": true, "pocket_money_dev": true,
	}
	if !allowedDBs[dbName] && !resetFlag {
		return fmt.Errorf("refusing to truncate unknown database %q; pass --reset to force", dbName)
	}

	fmt.Printf("Seeding demo family into database %q...\n", dbName)

	if err := truncateTables(ctx, pool); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	summary, err := buildDemoFamily(ctx, pool)
	if err != nil {
		return fmt.Errorf("build demo family: %w", err)
	}

	printHumanSummary(summary)

	if outPath == "" {
		return nil
	}
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if outPath == "-" {
		fmt.Println(string(jsonBytes))
		return nil
	}
	if err := os.WriteFile(outPath, jsonBytes, 0o644); err != nil {
		// If the directory doesn't exist yet, skip silently (local run without e2e/).
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: --out %q: directory not found, skipping summary write\n", outPath)
			return nil
		}
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("Summary written to %s\n", outPath)
	return nil
}

func checkMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	if err != nil {
		return fmt.Errorf("schema_migrations not found — run the server (or migrations) first: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no migrations applied — run the server (or migrations) first")
	}
	return nil
}

func getDatabaseName(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var name string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		return "", fmt.Errorf("current_database: %w", err)
	}
	return name, nil
}

func truncateTables(ctx context.Context, pool *pgxpool.Pool) error {
	// Same table set as testutil.CleanupTestDB; loans first (FK ledger_entries.loan_id → loans).
	// TRUNCATE ... CASCADE clears dependents in one statement.
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			loans, allowances, invite_tokens, ledger_entries,
			chores, group_members, groups, users
		CASCADE
	`)
	if err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}
	return nil
}

func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// ceilDiv returns ceil(a / b) for positive int64 values.
func ceilDiv(a, b int64) int64 {
	return int64(math.Ceil(float64(a) / float64(b)))
}

// parseYM parses "YYYY-MM" into (year, month).
func parseYM(p string) (int, time.Month) {
	var y, m int
	_, _ = fmt.Sscanf(p, "%d-%d", &y, &m)
	return y, time.Month(m)
}

// firstOfPeriod returns midnight UTC on the first day of the YYYY-MM period.
func firstOfPeriod(p string) time.Time {
	y, m := parseYM(p)
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// midOfPeriod returns noon UTC on the 15th of the YYYY-MM period.
func midOfPeriod(p string) time.Time {
	y, m := parseYM(p)
	return time.Date(y, m, 15, 12, 0, 0, 0, time.UTC)
}

// insertLedgerEntry inserts a ledger row with an explicit created_at timestamp.
// All invariant fields (type/direction) are validated before insert.
func insertLedgerEntry(
	ctx context.Context, pool *pgxpool.Pool,
	groupID, userID, createdBy uuid.UUID,
	choreID *uuid.UUID,
	amount int64,
	entryType models.LedgerEntryType,
	direction models.LedgerDirection,
	status models.LedgerStatus,
	note *string,
	decidedBy *uuid.UUID, decidedAt *time.Time,
	createdAt time.Time,
) error {
	// Validate direction invariants (§5.1).
	expectedDir := expectedDirection(entryType)
	if expectedDir != "" && expectedDir != direction {
		return fmt.Errorf("entry type %q expects direction %q, got %q", entryType, expectedDir, direction)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO ledger_entries
			(id, group_id, user_id, chore_id, amount, status, entry_type, direction,
			 note, created_by_user_id, decided_by, decided_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		uuid.New(), groupID, userID, choreID, amount, status, entryType, direction,
		note, createdBy, decidedBy, decidedAt, createdAt,
	)
	return err
}

// expectedDirection returns the canonical direction for a given entry type, or ""
// if the type allows both (e.g. adjustment).
func expectedDirection(t models.LedgerEntryType) models.LedgerDirection {
	switch t {
	case models.EntryTypeChore, models.EntryTypeAllowance:
		return models.DirectionCredit
	case models.EntryTypeEMI, models.EntryTypeSettlement:
		return models.DirectionDebit
	default:
		return "" // adjustment: direction determined by caller
	}
}

func buildDemoFamily(ctx context.Context, pool *pgxpool.Pool) (*SeedSummary, error) {
	now := time.Now().UTC()
	p0 := now.Format("2006-01")
	pm1 := posting.AddMonths(p0, -1)
	pm2 := posting.AddMonths(p0, -2)
	pm3 := posting.AddMonths(p0, -3)

	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	choreRepo := db.NewChoreRepo(pool)
	allowanceRepo := db.NewAllowanceRepo(pool)
	loanRepo := db.NewLoanRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)

	// ── Create users ──────────────────────────────────────────────────────────
	hashPw := func() string {
		h, err := hashPassword(DemoPass)
		if err != nil {
			panic(err)
		}
		return h
	}

	head, err := userRepo.Create(ctx, HeadEmail, hashPw(), HeadName, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create head: %w", err)
	}
	aarav, err := userRepo.Create(ctx, AaravEmail, hashPw(), AaravName, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create aarav: %w", err)
	}
	diya, err := userRepo.Create(ctx, DiyaEmail, hashPw(), DiyaName, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create diya: %w", err)
	}

	// ── Create group ──────────────────────────────────────────────────────────
	group, err := groupRepo.Create(ctx, GroupName, head.ID)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	// ── Add members (GroupRepo.Create does NOT add the head — project memory) ──
	if _, err := groupRepo.AddMember(ctx, group.ID, head.ID, models.RoleHead); err != nil {
		return nil, fmt.Errorf("add head: %w", err)
	}
	if _, err := groupRepo.AddMember(ctx, group.ID, aarav.ID, models.RoleMember); err != nil {
		return nil, fmt.Errorf("add aarav: %w", err)
	}
	if _, err := groupRepo.AddMember(ctx, group.ID, diya.ID, models.RoleMember); err != nil {
		return nil, fmt.Errorf("add diya: %w", err)
	}

	// ── Backdate joined_at to P-3 for all three (AddMember stamps now()) ──────
	// PostDue floors allowance backfill at join month; project memory confirms
	// backfill tests must UPDATE joined_at, not just effective_from.
	joinedAt := firstOfPeriod(pm3)
	if _, err := pool.Exec(ctx,
		`UPDATE group_members SET joined_at = $1 WHERE group_id = $2`,
		joinedAt, group.ID,
	); err != nil {
		return nil, fmt.Errorf("backdate joined_at: %w", err)
	}

	// ── Create chores ─────────────────────────────────────────────────────────
	settlement, err := choreRepo.CreateWithSystem(ctx, group.ID, "Settlement", nil, 0, true)
	if err != nil {
		return nil, fmt.Errorf("create settlement chore: %w", err)
	}
	washDishes, err := choreRepo.Create(ctx, group.ID, "Wash dishes", nil, 2000)
	if err != nil {
		return nil, fmt.Errorf("create wash-dishes chore: %w", err)
	}
	walkDog, err := choreRepo.Create(ctx, group.ID, "Walk the dog", nil, 5000)
	if err != nil {
		return nil, fmt.Errorf("create walk-the-dog chore: %w", err)
	}

	// ── Configure allowances ──────────────────────────────────────────────────
	if _, err := allowanceRepo.SetAllowance(ctx, group.ID, aarav.ID, 50000, pm3, head.ID); err != nil {
		return nil, fmt.Errorf("set aarav allowance: %w", err)
	}
	if _, err := allowanceRepo.SetAllowance(ctx, group.ID, diya.ID, 30000, pm3, head.ID); err != nil {
		return nil, fmt.Errorf("set diya allowance p-3: %w", err)
	}
	if _, err := allowanceRepo.SetAllowance(ctx, group.ID, diya.ID, 40000, pm1, head.ID); err != nil {
		return nil, fmt.Errorf("set diya allowance p-1: %w", err)
	}

	// ── Aarav's active loan (6 installments so it stays active at seed time) ──
	aaravPrincipal := int64(120000)
	aaravInstallments := 6
	aaravEMI := ceilDiv(aaravPrincipal, int64(aaravInstallments)) // 20000
	decidedNow := now
	aaravLoan, err := loanRepo.Create(ctx,
		group.ID, aarav.ID,
		aaravPrincipal, aaravInstallments, aaravEMI,
		models.LoanStatusActive, &pm2, nil,
		&head.ID, &decidedNow,
	)
	if err != nil {
		return nil, fmt.Errorf("create aarav loan: %w", err)
	}

	// ── Diya's rejected loan ──────────────────────────────────────────────────
	diyaPrincipal := int64(500000)
	diyaInstallments := 6
	diyaEMI := ceilDiv(diyaPrincipal, int64(diyaInstallments))
	if _, err := loanRepo.Create(ctx,
		group.ID, diya.ID,
		diyaPrincipal, diyaInstallments, diyaEMI,
		models.LoanStatusRejected, nil, nil,
		&head.ID, &decidedNow,
	); err != nil {
		return nil, fmt.Errorf("create diya rejected loan: %w", err)
	}

	// ── Run PostDue to generate machine allowance/EMI postings ───────────────
	// This is the load-bearing step: it posts all due allowance credits and EMI
	// debits through the real engine (idempotent, honors join-month floor, EMI
	// rounding, and auto-close). G3: 6 installments keeps Aarav's loan active.
	postingSvc := posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)
	if err := postingSvc.PostDue(ctx, group.ID, now); err != nil {
		return nil, fmt.Errorf("PostDue: %w", err)
	}

	// ── Manual ledger entries with backdated created_at ───────────────────────
	p1Mid := midOfPeriod(pm1)
	recentDate := now.AddDate(0, 0, -5)

	// Manual credit — adjustment: head gives Aarav ₹100 bonus in P-1.
	note := func(s string) *string { return &s }
	if err := insertLedgerEntry(ctx, pool,
		group.ID, aarav.ID, head.ID,
		nil, 10000,
		models.EntryTypeAdjustment, models.DirectionCredit,
		models.StatusApproved,
		note("Good grades bonus"),
		&head.ID, &p1Mid, p1Mid,
	); err != nil {
		return nil, fmt.Errorf("insert adjustment: %w", err)
	}

	// Manual debit — settlement: head pays out Diya ₹200 in P-1.
	choreID := &settlement.ID
	if err := insertLedgerEntry(ctx, pool,
		group.ID, diya.ID, head.ID,
		choreID, 20000,
		models.EntryTypeSettlement, models.DirectionDebit,
		models.StatusApproved,
		nil, &head.ID, &p1Mid, p1Mid,
	); err != nil {
		return nil, fmt.Errorf("insert settlement: %w", err)
	}

	// Member chore — approved: Aarav submitted "Wash dishes" in P-1.
	washID := &washDishes.ID
	if err := insertLedgerEntry(ctx, pool,
		group.ID, aarav.ID, aarav.ID,
		washID, 2000,
		models.EntryTypeChore, models.DirectionCredit,
		models.StatusApproved,
		nil, &head.ID, &p1Mid, p1Mid,
	); err != nil {
		return nil, fmt.Errorf("insert approved chore: %w", err)
	}

	// Member chore — pending: Diya submitted "Walk the dog".
	// chore_id MUST be set: the API requires chore_id for chore entries
	// (ledger.go: "chore_id is required for chore entries"), so a NULL chore_id
	// here would be a state the API can never produce.
	walkDogID := &walkDog.ID
	if err := insertLedgerEntry(ctx, pool,
		group.ID, diya.ID, diya.ID,
		walkDogID, 5000,
		models.EntryTypeChore, models.DirectionCredit,
		models.StatusPendingApproval,
		nil, nil, nil, recentDate,
	); err != nil {
		return nil, fmt.Errorf("insert pending chore: %w", err)
	}

	// Member chore — rejected: Aarav submitted "Walk the dog".
	rejectedAt := recentDate.AddDate(0, 0, -1)
	if err := insertLedgerEntry(ctx, pool,
		group.ID, aarav.ID, aarav.ID,
		walkDogID, 5000,
		models.EntryTypeChore, models.DirectionCredit,
		models.StatusRejected,
		nil, &head.ID, &rejectedAt, rejectedAt,
	); err != nil {
		return nil, fmt.Errorf("insert rejected chore: %w", err)
	}

	// ── Compute expected balances by querying the DB ─────────────────────────
	balances, err := ledgerRepo.GetBalanceForGroup(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}
	expectedBalances := make(map[string]int64, len(balances))
	for _, b := range balances {
		expectedBalances[b.UserID.String()] = b.Balance
	}

	// ── Loan outstanding (Aarav's active loan) ────────────────────────────────
	aaravLoans, err := loanRepo.ListForGroup(ctx, group.ID, &aarav.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("list aarav loans: %w", err)
	}
	var loanOutstanding int64
	for _, l := range aaravLoans {
		if l.ID == aaravLoan.ID {
			loanOutstanding = l.Outstanding
			break
		}
	}

	return &SeedSummary{
		CurrentPeriod: p0,
		Users: map[string]SummaryUser{
			"head":  {ID: head.ID.String(), Email: HeadEmail, Name: HeadName},
			"aarav": {ID: aarav.ID.String(), Email: AaravEmail, Name: AaravName},
			"diya":  {ID: diya.ID.String(), Email: DiyaEmail, Name: DiyaName},
		},
		Group: SummaryGroup{
			ID:   group.ID.String(),
			Name: GroupName,
		},
		ExpectedBalances: expectedBalances,
		LoanOutstanding:  loanOutstanding,
	}, nil
}

func printHumanSummary(s *SeedSummary) {
	fmt.Printf("\nDemo family %q seeded (period: %s)\n", s.Group.Name, s.CurrentPeriod)
	fmt.Printf("  HEAD    %-20s %-26s / %s\n", s.Users["head"].Name, s.Users["head"].Email, DemoPass)
	fmt.Printf("  MEMBER  %-20s %-26s / %s\n", s.Users["aarav"].Name, s.Users["aarav"].Email, DemoPass)
	fmt.Printf("  MEMBER  %-20s %-26s / %s\n", s.Users["diya"].Name, s.Users["diya"].Email, DemoPass)
	fmt.Printf("\nExpected balances (approved entries only):\n")
	for uid, bal := range s.ExpectedBalances {
		fmt.Printf("  %s: %d paise\n", uid, bal)
	}
	fmt.Printf("  Loan outstanding: %d paise\n\n", s.LoanOutstanding)
}
