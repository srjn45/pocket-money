# WP-3.1 Spec: Backend Loans + EMI

**Work package:** WP-3.1 (Phase 3 — Loans & EMI), master-plan §9.
**Type:** Backend-only. DB migration **011** (loans table + enum + the deferred `ledger_entries.loan_id → loans` FK) + `loan_repo.go` + a new idempotent EMI ledger insert path + **EMI posting inside the existing `internal/posting` engine** (the marked WP-3.1 seam) + loan handlers + `openapi.yaml` delta. Contract-first per §10.1. **No frontend** — FE lives in WP-3.2 (loans tab) and WP-3.3 (member detail); see §12.
**Risk:** **HIGH.** Master-plan §11 flags WP-3.1 (migration + EMI math + posting engine + money-affecting, balance-can-go-negative inserts) as *Opus-implement, or Sonnet-implement with strict Opus review*; **posting-engine unit tests MUST exist before review passes.** The review prompt must check: EMI **idempotency per-loan-per-period** via the 009 partial unique index (§4.4), **exactly-once under concurrent triggers** (§4.4), **deterministic iteration / lock order preserved** (§4.2/§4.5), EMI rounding with last-installment fixup (§4.3), auto-close + early-payoff correctness and their race with lazy posting (§4.5/§5.3), balance correctness including **negative balances** (§5.1/§8), migration 011 up/down **including the FK against existing `loan_id` data**, `::bigint` casts on the new SUM aggregates (§10), and the loan authz matrix (§5.5).

**Depends on (all merged to master):**
- **WP-1.1** — money is minor units: `int64` in Go, `BIGINT` in DB, aggregate SUMs cast `::bigint`.
- **WP-1.2** — unified ledger v2 is live. Migration **009** added the bare nullable `loan_id UUID` column (**no FK** — deferred to 011, WP-1.2 §1.0), the `emi` enum value on `ledger_entry_type`, and the **`NULLS NOT DISTINCT` partial unique index `idx_ledger_entries_posting_unique` on `(group_id, user_id, entry_type, period, loan_id) WHERE period IS NOT NULL`**. `POST /ledger` already rejects `entry_type=emi` as machine-posted-only; `GET /ledger?type=emi` already parses. `GetBalanceForGroup` already computes `Σ approved credits − Σ approved debits` from `direction` (no chores join, `::bigint` casts) and does **not** clamp at zero.
- **WP-2.1** — `internal/posting` exists: `PostDue(ctx, store, groupID, now)` posts allowances, iterates users in deterministic SQL order to avoid concurrent-trigger deadlocks, and leaves a marked seam `// EMI posting: WP-3.1 slots loan iteration here (out of scope).` inside the per-user loop (`backend/internal/posting/engine.go`). `InsertAllowancePosting` (idempotent, bare `ON CONFLICT DO NOTHING`) is the template for the EMI insert. The lazy trigger already fires on `GET /groups/:id`, `/ledger`, `/balance` via `runPosting` (`internal/handlers/posting.go`) — **no new trigger wiring is needed**; extending `PostDue` is enough.

**Acceptance (master-plan §9 WP-3.1 row):** unit tests for ceil rounding with last-installment fixup (e.g. `1000/3 → 334, 334, 332`), idempotent periods, and close; migration 011; loan endpoints (§6.1); EMI posting in the engine; rounding rule; auto-close; early payoff; `make -C backend test lint` (+ integration in CI) green.

---

## 0. Goal & scope boundary

Add **loans repaid via monthly EMI deducted from pocket money** (§5.3). A member (or the head, pre-approved) creates a zero-interest loan of `principal` over `installments` months. The head hands over cash **outside the app** — disbursement posts **no ledger entry**. From `start_period` onward, the **existing lazy posting engine** posts one approved `emi` **debit** per due month, idempotent per-loan-per-period, until the principal is fully repaid, then the loan auto-closes. The head may reject a request, or close an active loan early (one final `emi` debit for the outstanding amount). Because EMIs post unconditionally, **a member's balance may go negative** (§5.3/§8) — it must not be clamped.

### In scope
- Migration **011_loans**: `loans` table + `loan_status` enum + the deferred `ledger_entries.loan_id → loans(id)` FK (§3).
- `models.Loan` + `models.LoanPostingInput` (§2).
- `loan_repo.go` (CRUD + lifecycle + posting reads) and a new **idempotent `InsertEMIPosting`** on `LedgerRepo` (§2.3).
- **EMI posting in `internal/posting`** at the marked seam: extend `PostDue`, `Store`, and the concrete `storeAdapter`; keep the allowance path untouched (§4).
- Loan handlers: `GET /groups/:id/loans`, `POST /groups/:id/loans`, `POST /loans/:id/approve`, `POST /loans/:id/reject`, `POST /loans/:id/close` (§5, §6).
- `main.go` + `NewService` wiring (§7); `openapi.yaml` delta (§9). **No FE codegen** — WP-3.2 (§12).
- Unit + integration tests (§11).

### Explicitly OUT of scope (do not build; note discoveries in `docs/notes/` per §10.2)
- **All frontend** — the loans tab (WP-3.2) and head member-detail page (WP-3.3). Do **not** touch any `app/` file or regenerate `app/src/api-types.gen.ts`. `openapi.yaml` is the source of truth; WP-3.2 runs `npm run codegen` as its first step (§12).
- **`GET /groups/:id/members/:userId/summary`** (§6.1) — its `active_loans[]` payload is a WP-3.3 concern. Do not add it here.
- **Interest / fees of any kind.** Loans are zero-interest (§5.3). No interest columns, no APR, nothing. (Interest-on-savings is backlog §8 #11 — not this WP.)
- **Any manual "post now" / EMI preview endpoint.** Posting is lazy-triggered only (§5.4). Do not invent one.
- **Loan editing/deletion after decision, or partial-amount member-initiated repayment.** The only lifecycle transitions are those in §5.3 (request→active/rejected, active→closed). Entries and decided loans are immutable except the specified status transitions.
- **Widening allowance behavior, weekly cadence, proration** — untouched.

---

## 1. Migration numbering — WP-3.1 owns 011 (the reserved gap)

Per master-plan §6.2, WP-1.2 §1.0, and WP-2.1 §1.0, the gap was reserved: 009 (ledger v2), **010 (allowances)**, **011 = loans (this WP)**, 012 (drop_settlements). On master the applied files are `008, 009, 010, 012` — **011 does not exist yet** (verified). Ship **`011_loans.up.sql` / `.down.sql`** into the gap **between 010 and 012**. Do **not** renumber; do **not** touch 009/010/012.

> **⚠️ golang-migrate out-of-order gotcha (document + verify) — identical to WP-2.1 §1.0.** `golang-migrate`'s `m.Up()` applies only versions **greater than the DB's current version**. A dev/home-server DB already migrated to **012** will **silently NOT apply an inserted 011** — `Up()` sees nothing `> 12`. This is fine for **every path this WP is tested on** because they all start from a **fresh DB**: CI/integration `ResetTestDB` → `RunMigrations` applies `008, 009, 010, 011, 012` in ascending order (**011 before 012**); `TestMigrations_UpAndDown` resets first. There is **no production data** yet (Phase 3, LAN dev). **Consequence for a developer whose dev DB is already at 012:** they must reset/recreate the DB (drop schema, re-migrate) to pick up 011 — a bare `Up()` will skip it. Call this out in the PR description. Do **not** "fix" golang-migrate ordering; the reserved-gap decision is the master plan's and only bites already-advanced dev DBs, which reset trivially. Note: because 011 adds a FK **to** `ledger_entries` and 012 only inserts settlement rows / drops the `settlements` table, 011-before-012 is a safe, dependency-free ordering on a fresh DB.

---

## 2. Data model & repo

### 2.1 `models.go` — `Loan` + `LoanPostingInput`

```go
// LoanStatus mirrors the DB loan_status enum.
type LoanStatus string

const (
    LoanStatusRequested LoanStatus = "requested"
    LoanStatusActive    LoanStatus = "active"
    LoanStatusRejected  LoanStatus = "rejected"
    LoanStatusClosed    LoanStatus = "closed"
)

// Loan is a zero-interest loan repaid via monthly EMI debits (§5.3).
type Loan struct {
    ID           uuid.UUID  `json:"id"`
    GroupID      uuid.UUID  `json:"group_id"`
    UserID       uuid.UUID  `json:"user_id"`       // borrower
    Principal    int64      `json:"principal"`     // minor units, > 0
    Installments int        `json:"installments"`  // > 0
    EMIAmount    int64      `json:"emi_amount"`    // ceil(principal/installments), > 0
    StartPeriod  *string    `json:"start_period,omitempty"` // 'YYYY-MM'; NULL until active
    Status       LoanStatus `json:"status"`
    Note         *string    `json:"note,omitempty"`
    RequestedAt  time.Time  `json:"requested_at"`
    DecidedBy    *uuid.UUID `json:"decided_by,omitempty"`
    DecidedAt    *time.Time `json:"decided_at,omitempty"`
}

// LoanPostingInput is the posting engine's read model for one active loan.
type LoanPostingInput struct {
    LoanID       uuid.UUID
    UserID       uuid.UUID
    Principal    int64
    Installments int
    EMIAmount    int64
    StartPeriod  string // active loans always have a start_period
}
```

### 2.2 `loan_repo.go` — CRUD, lifecycle, posting reads

`type LoanRepo struct { pool *pgxpool.Pool }` + `NewLoanRepo(pool)`, following the existing repo-per-entity style. Methods:

- **`Create(ctx, groupID, userID uuid.UUID, principal int64, installments int, emiAmount int64, status models.LoanStatus, startPeriod *string, note *string, decidedBy *uuid.UUID, decidedAt *time.Time) (*models.Loan, error)`** — inserts one loan row (`id gen_random_uuid()`, `requested_at` DB default). Used by both member-request (`status=requested`, `startPeriod=nil`, decided nil) and head pre-approved (`status=active`, `startPeriod` set, decided = head/now). `RETURNING` the full row.
- **`GetByID(ctx, id) (*models.Loan, error)`** → `ErrNotFound` on miss. Non-locking read (handlers use it for authz/404 before entering the lifecycle tx).
- **`ListForGroup(ctx, groupID uuid.UUID, userID *uuid.UUID, status *models.LoanStatus) (*[]LoanListRow …)`** — the GET query. Returns loans **plus computed** `installments_posted` and `outstanding` via a LEFT JOIN aggregate over posted EMI rows (§5.1 response). SQL:
  ```sql
  SELECT l.id, l.group_id, l.user_id, l.principal, l.installments, l.emi_amount,
         l.start_period, l.status, l.note, l.requested_at, l.decided_by, l.decided_at,
         COALESCE(e.posted_count, 0)                    AS installments_posted,
         (l.principal - COALESCE(e.paid, 0))::bigint    AS outstanding
  FROM loans l
  LEFT JOIN (
      SELECT loan_id, COUNT(*) AS posted_count, SUM(amount)::bigint AS paid
      FROM ledger_entries
      WHERE entry_type = 'emi' AND loan_id IS NOT NULL
      GROUP BY loan_id
  ) e ON e.loan_id = l.id
  WHERE l.group_id = $1
    -- append AND l.user_id = $n / AND l.status = $n when the filters are set
  ORDER BY l.requested_at DESC
  ```
  > **`::bigint` cast is mandatory** on `SUM(amount)` and on `outstanding` — `SUM(bigint)` returns `numeric` in pgx and scanning it into `int64` fails otherwise (WP-1.1 gotcha, §10). Scan `installments_posted` into `int` and `outstanding` into `int64`. Return a small `LoanWithProgress` struct (embed `Loan` + `InstallmentsPosted int` + `Outstanding int64`) — the handler maps it to `LoanResponse`.
- **`ListActiveLoans(ctx, groupID) ([]models.LoanPostingInput, error)`** — the engine's read. **Deterministic order is load-bearing** (§4.2):
  ```sql
  SELECT id, user_id, principal, installments, emi_amount, start_period
  FROM loans
  WHERE group_id = $1 AND status = 'active'
  ORDER BY user_id, start_period, id
  ```
- **`PostedEMIPeriods(ctx, groupID) (map[uuid.UUID]map[string]bool, error)`** — the fast-path guard, keyed **by loan_id** (§4.4):
  ```sql
  SELECT loan_id, period FROM ledger_entries
  WHERE group_id = $1 AND entry_type = 'emi' AND loan_id IS NOT NULL AND period IS NOT NULL
  ```
  Returns `map[loanID]map[period]bool`.
- **`LockActiveLoan(ctx, q db.Querier2, id uuid.UUID) (active bool, err error)`** — `SELECT status FROM loans WHERE id = $1 FOR UPDATE`; returns `status == 'active'`. Runs on the tx `Querier` so the row lock is held for the rest of the tx (§4.5). `pgx.ErrNoRows` → return `(false, nil)` (loan vanished — nothing to post). **Note:** `FOR UPDATE` needs `Query`/`QueryRow`, so the engine's tx boundary must expose a querier that can run `QueryRow`, not only `Exec` — see §2.4.
- **`CountPostedEMIs(ctx, q, loanID) (int, error)`** — `SELECT COUNT(*) FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi'`, run on the tx querier (authoritative post-insert count for auto-close, §4.5).
- **`SumPostedEMIs(ctx, q, loanID) (int64, error)`** — `SELECT COALESCE(SUM(amount), 0)::bigint FROM ledger_entries WHERE loan_id = $1 AND entry_type = 'emi'` (**`::bigint`**), for the close outstanding computation (§5.3).
- **`CloseLoan(ctx, q, loanID) error`** — `UPDATE loans SET status = 'closed' WHERE id = $1 AND status = 'active'` (idempotent; 0 rows affected is fine — already closed).
- **`Approve(ctx, q, id uuid.UUID, principal int64, installments int, emiAmount int64, startPeriod string, decidedBy uuid.UUID, decidedAt time.Time) (*models.Loan, error)`** — `UPDATE ... SET principal=$, installments=$, emi_amount=$, start_period=$, status='active', decided_by=$, decided_at=$ WHERE id=$ AND status='requested' RETURNING …`. `pgx.ErrNoRows` from the `WHERE status='requested'` guard → return a sentinel the handler maps to **409** (not `requested`).
- **`Reject(ctx, q, id, decidedBy, decidedAt) (*models.Loan, error)`** — `UPDATE ... SET status='rejected', decided_by=$, decided_at=$ WHERE id=$ AND status='requested' RETURNING …`; same 409-on-no-row contract.

### 2.3 `ledger_repo.go` — new idempotent `InsertEMIPosting` (**do not reuse `Create` or `InsertAllowancePosting`**)

Mirror `InsertAllowancePosting` (WP-2.1 §2.3) exactly, changing only the fixed field values. **`loan_id` is SET** (unlike allowance), `period` is set, `direction='debit'`, `entry_type='emi'`, `status='approved'`, `created_by`=head, `decided_by/at`=NULL. A `note` is accepted so the close path can label the final debit (schedule EMIs pass `nil`).

```go
// InsertEMIPosting inserts one approved emi debit for (group,user,loan,period), idempotently.
// Returns (inserted bool) so the engine/close path can assert exactly-once.
// loan_id AND period are both set, so the row collides on
// (group,user,'emi',period,loan_id) — distinct per loan, distinct from allowance
// rows (different entry_type) and from other loans' EMIs (different loan_id).
// The bare ON CONFLICT DO NOTHING catches the partial unique index without restating it.
func (r *LedgerRepo) InsertEMIPosting(ctx context.Context, q Querier,
    groupID, userID, loanID uuid.UUID, amount int64, period string,
    note *string, createdBy uuid.UUID) (bool, error) {

    tag, err := q.Exec(ctx, `
        INSERT INTO ledger_entries
            (id, group_id, user_id, chore_id, amount, status, entry_type, direction,
             loan_id, period, note, created_by_user_id, decided_by, decided_at)
        VALUES (gen_random_uuid(), $1, $2, NULL, $3, 'approved', 'emi', 'debit',
                $4, $5, $6, $7, NULL, NULL)
        ON CONFLICT DO NOTHING`,
        groupID, userID, amount, loanID, period, note, createdBy)
    if err != nil {
        return false, fmt.Errorf("failed to insert emi posting: %w", err)
    }
    return tag.RowsAffected() == 1, nil
}
```

> **Bare `ON CONFLICT DO NOTHING` is deliberate and required** (same rationale as allowances): the only unique arbiter an EMI insert can violate is the partial index `idx_ledger_entries_posting_unique`. The bare form catches it without restating the `NULLS NOT DISTINCT` partial predicate. **Invariant:** EMI inserts **must** set `loan_id` non-NULL and `period` non-NULL, or the index won't guard them → duplicate EMIs → double-charging. Enforced by using `InsertEMIPosting` exclusively for machine/close posts, never `LedgerRepo.Create`.

### 2.4 `db.Querier` gains `QueryRow` (or a second interface) for `FOR UPDATE`

The WP-2.1 `db.Querier` exposes only `Exec` (allowance posts are pure INSERTs). The EMI path needs `SELECT … FOR UPDATE` (`LockActiveLoan`) and `COUNT`/`SUM` reads **on the transaction** (`CountPostedEMIs`, `SumPostedEMIs`, `CloseLoan` is an `Exec`). Extend the querier abstraction so a `pgx.Tx` can run all of them. Recommended minimal change:

```go
// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type Querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

`*pgxpool.Pool` and `pgx.Tx` both already satisfy this (they have `QueryRow`). Adding `QueryRow` to `Querier` is backward-compatible with `InsertAllowancePosting` (still only calls `Exec`). Verify `InsertAllowancePosting`'s existing tests still compile (the fake `Store.WithTx` passes `nil` as the querier but never calls it for allowances — unaffected). Do **not** introduce a separate interface if one method addition suffices; keep it a single `Querier`.

---

## 3. Migration `011_loans`

### 3.1 `011_loans.up.sql`

```sql
-- Zero-interest loans repaid via monthly EMI debits (§5.3).
CREATE TYPE loan_status AS ENUM ('requested', 'active', 'rejected', 'closed');

CREATE TABLE loans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id      UUID   NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id       UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,  -- borrower
    principal     BIGINT NOT NULL CHECK (principal > 0),        -- minor units
    installments  INT    NOT NULL CHECK (installments > 0),
    emi_amount    BIGINT NOT NULL CHECK (emi_amount > 0),       -- ceil(principal/installments)
    start_period  CHAR(7),                                      -- 'YYYY-MM'; NULL until active
    status        loan_status NOT NULL DEFAULT 'requested',
    note          TEXT,
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_at    TIMESTAMPTZ
);

-- Engine reads active loans per group; handlers list per group (+ optional user/status).
CREATE INDEX idx_loans_group_status ON loans (group_id, status);
CREATE INDEX idx_loans_group_user   ON loans (group_id, user_id);

-- Add the FK deferred since migration 009 (WP-1.2 §1.0): ledger_entries.loan_id -> loans.
-- ON DELETE SET NULL: deleting a loan orphans its emi rows' loan_id rather than
-- cascade-deleting money history (the ledger is immutable audit; §5.1).
ALTER TABLE ledger_entries
    ADD CONSTRAINT fk_ledger_entries_loan
    FOREIGN KEY (loan_id) REFERENCES loans(id) ON DELETE SET NULL;
```

Notes:
- **`principal`/`emi_amount` `BIGINT`** — post-008 money convention; never `DECIMAL`.
- **`start_period CHAR(7)`** — same storage as `ledger_entries.period`; format produced/validated in Go (`^\d{4}-\d{2}$`), not by a DB constraint. NULL while `requested`; set when the loan goes `active`.
- **`emi_amount` is stored** (`ceil(principal/installments)`); the last-installment fixup is **derived** at posting time (§4.3), not stored.
- The FK is added **after** the `loans` table exists — this is the whole reason 009 left `loan_id` bare (WP-1.2 §1.0). At the time 011 runs on a fresh migrate, `ledger_entries` has **zero** `emi` rows with a non-NULL `loan_id` (allowance/settlement/chore rows all have `loan_id` NULL), so the FK validates trivially. `ON DELETE SET NULL` matches the column being nullable.

### 3.2 `011_loans.down.sql`

```sql
-- Drop the FK first (it references loans), then the table/enum. loan_id stays a
-- bare nullable column exactly as migration 009 created it.
ALTER TABLE ledger_entries DROP CONSTRAINT IF EXISTS fk_ledger_entries_loan;

DROP INDEX IF EXISTS idx_loans_group_user;
DROP INDEX IF EXISTS idx_loans_group_status;
DROP TABLE IF EXISTS loans;
DROP TYPE IF EXISTS loan_status;
```

**Reversibility note.** 011 down is a **schema** rollback (§10.4) and is exact on a fresh DB (what `TestMigrations_UpAndDown` exercises) — down removes the FK, table, indexes, and enum, restoring the post-010 schema (with `loan_id` still a bare nullable column, as 009 left it). It is **not** data-safe once real `emi` ledger rows reference a loan: dropping `loans` while `emi` rows hold a `loan_id` leaves those `loan_id`s dangling (the column is nullable and now unconstrained — no error, but referential meaning is lost). This is acceptable and expected for a down migration. On the round-trip test's fresh DB there are zero `emi` rows, so down is clean. **Do not** add a step that nulls out `emi.loan_id` on down — that would corrupt the ledger audit trail on any real DB, which is worse than a rollback tool's documented non-data-safety.

### 3.3 Migration-runner / harness updates
- **`backend/internal/db/migrate_test.go`** (`TestMigrations_UpAndDown`): add `"loans"` to the `tables` slice asserted to **exist after up** and **absent after down**; add `"loan_status"` to the `types` existence check (alongside `member_role`, `ledger_status`, `ledger_entry_type`, `ledger_direction`).
- **`backend/testutil/db.go`**:
  - `ResetTestDB`: add `"loans"` to the `DROP TABLE IF EXISTS` list (place it **before** `ledger_entries`, or rely on `CASCADE` — `DROP … CASCADE` drops the FK either way); add `"loan_status"` to the `types` drop list.
  - `CleanupTestDB`: add `"loans"` to the `TRUNCATE` list. **Ordering matters here:** `ledger_entries.loan_id` now references `loans`, so `TRUNCATE loans` requires `CASCADE` (already used) — but to keep it robust, place `"loans"` **before** `ledger_entries` in the list. `TRUNCATE loans CASCADE` will also truncate `ledger_entries` (it references `loans`); since `ledger_entries` is truncated anyway, the net effect is correct regardless of order. Note this in a comment.

---

## 4. EMI posting in the engine (`backend/internal/posting`) — **the crux**

Extend `PostDue` at the marked seam. **Allowance logic is untouched.** EMI posting rides the same lazy trigger and the same one-tx-per-group boundary.

### 4.1 The one-line contract

> For every `active` loan in the group, `PostDue` makes the ledger contain **exactly one** approved `emi` **debit** per due installment period — from `start_period` through the month of `now` (server-local), at the scheduled installment amount (last installment absorbs rounding) — and **auto-closes** the loan once all scheduled installments are posted. Calling it any number of times, from any number of concurrent triggers, converges to that exact set and never errors on a duplicate. Combined with the allowance path, one member can receive an `allowance` credit **and** one-or-more `emi` debits in the **same** period without collision.

### 4.2 Where it slots + iteration order (deterministic; **preserve the no-deadlock property**)

The seam is inside the existing per-user loop in `PostDue`. Group active loans by user (mirroring the allowance `groupByUser`) and iterate **each user's loans in `(start_period, id)` order**, immediately after that user's allowance months:

```
PostDue(ctx, store, groupID, now):
    currentPeriod := formatPeriod(now)
    inputs := store.ListPostingInputs(groupID)       // allowance rows (unchanged)
    loans  := store.ListActiveLoans(groupID)         // active loans, ORDER BY user_id,start_period,id
    if len(inputs) == 0 && len(loans) == 0 { return nil }   // genuine no-op
    postedAllow := store.PostedAllowancePeriods(groupID)
    postedEMI   := store.PostedEMIPeriods(groupID)   // map[loanID]map[period]bool
    head := store.GroupHead(groupID)

    byUser, userOrder := groupByUser(inputs)                 // allowance grouping (unchanged)
    loansByUser := groupLoansByUser(loans)                   // preserves SQL order per user

    return store.WithTx(ctx, func(q db.Querier) error {
        // Iterate the UNION of users who have allowances and/or loans, in a single
        // deterministic order (see below), so concurrent PostDue calls lock rows
        // in identical order.
        for _, userID := range mergedUserOrder(userOrder, loans) {
            // (a) allowance months for userID — unchanged WP-2.1 logic
            // (b) EMI: for each loan L of userID (SQL order):
            for _, L := range loansByUser[userID] {
                postLoanEMIs(ctx, store, q, groupID, L, currentPeriod, postedEMI[L.LoanID], head)
            }
        }
        return nil
    })
```

**Determinism is the invariant the reviewer checks (§4.5).** Both the allowance inserts (keyed on ledger rows) and the loan operations (keyed on `loans` rows via `FOR UPDATE`, then ledger rows via the unique index) must be attempted in an order that is **identical across concurrent `PostDue` calls**. Users are visited in a single deterministic order (the existing SQL `user_id` order for allowance users, extended to include loan-only users — e.g. sort the merged user set, or append loan-only users in `ListActiveLoans` order; **do not** iterate a Go map to derive user order — that randomizes and can deadlock, exactly the trap the WP-2.1 comment warns about). Within a user, loans are visited in `(start_period, id)` SQL order. **Do not reintroduce randomized iteration anywhere.** Keep/extend the explanatory comment already in `engine.go`.

> Implementation latitude: a clean equivalent is to keep the allowance loop exactly as today, then run a **second** loop over `loans` (already globally ordered by `user_id, start_period, id`) for EMIs — still one tx, still fully deterministic, and it disturbs the allowance code the least. Either structure is acceptable **provided iteration is deterministic and single-tx**; state which you chose in the PR.

### 4.3 EMI schedule & amounts (pure, unit-tested)

Given a loan's `principal`, `emi_amount` (`= ceil(principal/installments)`), and `installments`, the schedule is a pure function — **no dependency on what is already posted** (each installment's amount is fixed), which is what makes re-posting idempotent (same period ⇒ same amount):

```go
// emiSchedule returns the amount of each installment in order. The last real
// installment absorbs rounding so the sum equals principal exactly (§5.3).
// Robust to pathological tiny principals: it stops as soon as the loan is fully
// repaid, so no installment is ever <= 0.
func emiSchedule(principal, emiAmount int64, installments int) []int64 {
    out := make([]int64, 0, installments)
    var paid int64
    for i := 0; i < installments; i++ {
        remaining := principal - paid
        if remaining <= 0 {
            break // fully repaid before all installments (only for tiny principals)
        }
        amt := emiAmount
        if amt > remaining {
            amt = remaining // final installment: absorbs rounding / clamps to remaining
        }
        out = append(out, amt)
        paid += amt
    }
    return out
}
```

- `emiAmount = (principal + int64(installments) - 1) / int64(installments)` — integer ceil. Compute it in the handler (§5.2) and store it; the engine reads it back.
- Worked example `1000 / 3`: `emiAmount = ceil(1000/3) = 334`; schedule `= [334, 334, 332]`, `Σ = 1000`. ✓ (master-plan §9).
- `installments = 1`: `emiAmount = principal`; schedule `= [principal]`.
- Pathological `principal = 2, installments = 3`: `emiAmount = ceil(2/3) = 1`; schedule `= [1, 1]` (stops at 2 paid; **length 2, not 3**) — never emits a `0` installment. Auto-close keys off "all *scheduled* installments posted" = `len(emiSchedule)`, so such a loan closes after 2 EMIs. Unit-test this edge.

`scheduleLen := len(emiSchedule(...))` is the authoritative installment count for due-period and auto-close math (usually `== installments`, less for the tiny-principal edge).

Due periods: installment `i` (0-based) posts in period `addMonths(startPeriod, i)`. It is **due** iff `addMonths(startPeriod, i) <= currentPeriod` (lexical compare, zero-padded `YYYY-MM`).

Add pure helper `addMonths(period string, k int) string` using the existing integer month arithmetic (parse to `year*12 + (month-1)`, add `k`, reformat) — **do not** loop on `time.AddDate` (WP-2.1's DST caution). It is the natural sibling of `monthsInclusive`.

### 4.4 Idempotency & concurrency (**MANDATORY invariant — reviewer checks this**)

- **Per-loan-per-period idempotency:** every EMI insert is `ON CONFLICT DO NOTHING` against `idx_ledger_entries_posting_unique`, keyed on `(group_id, user_id, entry_type, period, loan_id)` with **`loan_id` set**. A second `PostDue` for the same `now` inserts nothing new. The `PostedEMIPeriods` guard makes the caught-up common case a pure read (no INSERT attempts).
- **Allowance ↔ EMI coexistence (same user, same period):** an allowance row has `entry_type='allowance', loan_id NULL`; an EMI row has `entry_type='emi', loan_id=<loan>`. They differ in the index tuple by **both** `entry_type` and `loan_id`, so they **coexist** — the same member gets pocket money **and** an EMI deduction in the same month. **Two different loans**' EMIs in the same period differ by `loan_id` → also coexist (a member repaying two loans gets two `emi` debits that month). This is the whole reason `loan_id` is in the index. Spell this out in a comment and prove it in an integration test (§11.2).
- **`NULLS NOT DISTINCT`** (PG15, pinned) governs only the allowance case (NULL `loan_id`s colliding). EMI rows have non-NULL `loan_id`, so plain distinctness applies to them; the same index serves both.
- **Concurrent triggers:** two `PostDue` calls racing on the same group serialize on the unique index per ledger row (first commits, second's `ON CONFLICT DO NOTHING` no-ops) **and** on the `loans` row via `LockActiveLoan`'s `FOR UPDATE` (§4.5). No duplicate EMI, no error, no 500 — exactly-once. The `PostedEMIPeriods` guard is best-effort (read before the tx); the **unique index + row lock are the real guarantees** and must never be removed as an "optimization".

### 4.5 Auto-close + `FOR UPDATE` serialization (engine ↔ early-payoff)

Per active loan, inside the tx:

```
postLoanEMIs(store, q, groupID, L, currentPeriod, postedForLoan, head):
    sched := emiSchedule(L.Principal, L.EMIAmount, L.Installments)   // pure
    // Fast path: if every due period is already in the guard, skip entirely
    // (no lock, no insert) — preserves the "cheap no-op when nothing is due".
    if allDuePostedAlready(sched, L.StartPeriod, currentPeriod, postedForLoan) { return }

    active := store.LockActiveLoan(q, L.LoanID)   // SELECT status ... FOR UPDATE
    if !active { return }                          // closed/rejected/gone concurrently
    for i, amt := range sched:
        p := addMonths(L.StartPeriod, i)
        if p > currentPeriod { break }             // not due yet
        if postedForLoan[p] { continue }           // fast-path within loan
        store.InsertEMIPosting(q, groupID, L.UserID, L.LoanID, amt, p, nil, head)  // ON CONFLICT DO NOTHING
    // Auto-close: recount committed EMIs for this loan (authoritative under the lock).
    if store.CountPostedEMIs(q, L.LoanID) >= len(sched) {
        store.CloseLoan(q, L.LoanID)               // idempotent UPDATE ... WHERE status='active'
    }
```

- **Why `FOR UPDATE`:** it serializes the lazy engine against the **early-payoff `POST /loans/:id/close`** endpoint (§5.3), which also locks the loan `FOR UPDATE`. Without it, the engine could post a normal EMI into the *next* period slot at the same time close tries to post the outstanding into that slot → one `ON CONFLICT`-no-ops → the member is under- or over-charged. With both taking the loan-row lock, they run strictly one-after-the-other and each recomputes committed state. Auto-close's `CountPostedEMIs` read is taken **under the lock, after the inserts**, so it sees the committed EMI it (or a peer) just wrote.
- **Deadlock-freedom is preserved** because loans are locked in the deterministic `(user_id, start_period, id)` order across all concurrent `PostDue` calls (§4.2), and `close` locks exactly one loan (no ordering cycle possible). This is the extension of WP-2.1's deterministic-iteration property to the loans table — **do not** lock loans in map/random order.
- **Cost:** one extra `SELECT … FOR UPDATE` + one `COUNT` per active loan that has a genuinely-due unposted period. The guard fast-path skips fully-caught-up loans without locking. Negligible at family scale; do not micro-optimize further.

### 4.6 Machine-post field values (fixed — no variation)

| Field | Value | Why |
|---|---|---|
| `entry_type` | `emi` | §5.1 |
| `direction` | `debit` | EMI reduces balance |
| `status` | `approved` | machine-posted → approved (§5.1) |
| `amount` | scheduled installment amount, `> 0` | last installment absorbs rounding (§4.3) |
| `loan_id` | the loan's id | **non-NULL** — idempotency key + FK; identifies the loan |
| `period` | the due installment month `YYYY-MM` | **non-NULL** — idempotency key |
| `chore_id` / `note` / `decided_by` / `decided_at` | **NULL** for schedule EMIs (`note` may be set only by close, §5.3) | no chore, no human decider |
| `created_by_user_id` | **group head** | column is `NOT NULL`; head owns loan policy (same as allowances) |
| `created_at` | `now()` (DB default) | `period` carries the logical month |

### 4.7 Store interface additions (`store.go`, `service.go`)

Add to `posting.Store` (concrete impl in `storeAdapter`, backed by `LoanRepo` + `LedgerRepo`):

```go
ListActiveLoans(ctx context.Context, groupID uuid.UUID) ([]models.LoanPostingInput, error)
PostedEMIPeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error)
LockActiveLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) (bool, error)
InsertEMIPosting(ctx context.Context, q db.Querier, groupID, userID, loanID uuid.UUID,
    amount int64, period string, note *string, createdBy uuid.UUID) (bool, error)
CountPostedEMIs(ctx context.Context, q db.Querier, loanID uuid.UUID) (int, error)
CloseLoan(ctx context.Context, q db.Querier, loanID uuid.UUID) error
```

`NewService` gains a `*db.LoanRepo` parameter; `storeAdapter` gains a `loanRepo` field and delegates the six methods. Unit tests supply a fake `Store` implementing these (extend the existing `fakeStore` in `engine_test.go`).

---

## 5. Loan handlers (`backend/internal/handlers/loans.go`, new)

New `LoanHandler{ loanRepo, groupRepo, pool }` (the `pool` — or a `WithTx` helper — is needed for approve/reject/close, which lock rows in a tx). Follow the `AllowanceHandler`/`LedgerHandler` patterns exactly: parse `auth.GetUserID`, parse UUIDs, `GetMember` for membership/authz, `{"error": "..."}` shape with 400/401/403/404/409/500.

### 5.1 Response shape

```go
type LoanResponse struct {
    ID                 uuid.UUID          `json:"id"`
    GroupID            uuid.UUID          `json:"group_id"`
    UserID             uuid.UUID          `json:"user_id"`
    Principal          int64              `json:"principal"`
    Installments       int                `json:"installments"`
    EMIAmount          int64              `json:"emi_amount"`
    StartPeriod        *string            `json:"start_period,omitempty"`
    Status             models.LoanStatus  `json:"status"`
    Note               *string            `json:"note,omitempty"`
    RequestedAt        time.Time          `json:"requested_at"`
    DecidedBy          *uuid.UUID         `json:"decided_by,omitempty"`
    DecidedAt          *time.Time         `json:"decided_at,omitempty"`
    InstallmentsPosted int                `json:"installments_posted"` // computed (§2.2)
    Outstanding        int64              `json:"outstanding"`         // principal − Σ posted EMIs
}
```
Add a `loanToResponse` mapper (from `LoanWithProgress`). `outstanding` is meaningful for `active`/`closed`; for `requested`/`rejected` it equals `principal` (no EMIs posted) and the FE decides what to show.

### 5.2 `GET /api/v1/groups/:id/loans?user_id=&status=`
- Auth: must be a group member (else 403), same pattern as `ListAllowances`/`ListLedger`.
- **Head** → all loans in the group; may filter by `user_id` and/or `status` (validate `status` against the enum → 400 on invalid; validate `user_id` UUID → 400).
- **Member** → own loans only (`user_id` forced to self regardless of the query param — mirror the ledger member-sees-own rule, §5.5); `status` filter still honored.
- 200 → `[]LoanResponse` (empty array, not null).

### 5.3 `POST /api/v1/groups/:id/loans`
- Auth: must be a group member.
- Body `CreateLoanRequest{ UserID *uuid.UUID; Principal int64; Installments int; Note *string }`.
- Validate: `Principal > 0` (else 400), `Installments > 0` (else 400). Compute `emiAmount = (principal + installments - 1) / installments`.
- **Member caller** → creates a **request**: `user_id = self` (reject a `user_id` naming someone else → 400 "members can only request loans for themselves"), `status='requested'`, `start_period=NULL`, `decided_by/at=NULL`. 201.
- **Head caller** → creates a **pre-approved active** loan: `user_id` **required** and must be a **member** of the group and **not the head** (400 "cannot create a loan for the group head", 404 if target not a member — mirror `SetAllowance`); `status='active'`, `start_period = nextPeriod(currentMonth)` (server-local next calendar month, §5.3), `decided_by=head`, `decided_at=now()`. 201.
- Returns `LoanResponse` (posted=0, outstanding=principal). **No ledger entry is created** — disbursement is not a debit; the first EMI posts lazily at `start_period` (§5.3).

`nextPeriod(p)` = `addMonths(p, 1)` from `internal/posting` — expose it or duplicate the 3-line integer-month helper in the handler; **do not** use `time.AddDate` for month math (WP-2.1 caution). "Next calendar month" is relative to `time.Now()` at the moment of approval/creation.

### 5.4 `POST /api/v1/loans/:id/approve` (head only) — `{principal?, installments?}`
- Load loan by id (404 if missing). Resolve its group; caller must be that group's **head** (403 otherwise) — check via `GetMember(loan.GroupID, caller)` + `role==head`.
- Run in a **transaction**: `LockActiveLoan`-style lock is not enough (we need the row regardless of status) — use `SELECT … FOR UPDATE` on the loan, require `status='requested'` else **409** "loan is not pending approval".
- Optional overrides: if `principal` and/or `installments` provided, validate (`>0`) and use them; recompute `emiAmount = ceil(principal/installments)` from the **effective** values.
- Set `status='active'`, `start_period = nextPeriod(currentMonth)`, `decided_by=head`, `decided_at=now()` via `LoanRepo.Approve`. 200 → `LoanResponse`.

### 5.5 `POST /api/v1/loans/:id/reject` (head only)
- Same load + head authz. In a tx, `UPDATE … SET status='rejected', decided_by, decided_at WHERE id=$ AND status='requested'`; 0 rows → **409** "loan is not pending approval". 200 → `LoanResponse`. No ledger entry.

### 5.6 `POST /api/v1/loans/:id/close` (head only — early payoff)
- Same load + head authz. In a **single transaction** (this is the race-critical path, §4.5):
  1. `LockActiveLoan(q, id)` — `SELECT status FROM loans WHERE id=$ FOR UPDATE`. If not `active` → **409** "loan is not active".
  2. `postedCount := CountPostedEMIs(q, id)`; `paid := SumPostedEMIs(q, id)`; `outstanding := loan.Principal - paid`.
  3. If `outstanding > 0`: `InsertEMIPosting(q, groupID, borrower, id, outstanding, addMonths(loan.StartPeriod, postedCount), &note, head)` where `note = "Early payoff — loan closed"`. The period `start_period + postedCount` is the **next un-posted installment slot**, guaranteed free (only `postedCount` installments posted so far occupy the first `postedCount` periods). Assert `inserted == true`; because the loan row is `FOR UPDATE`-locked, no concurrent engine EMI can occupy that slot (the engine also locks the loan before posting, §4.5), so a `false` here indicates a logic error — return 500 and roll back rather than silently under-charge.
  4. `CloseLoan(q, id)` (sets `status='closed'`).
  5. Commit. 200 → refreshed `LoanResponse` (`GetByID` + progress, or re-run the list query for this id) with `status=closed`, `outstanding=0`, `installments_posted = postedCount + (outstanding>0 ? 1 : 0)`.
- **Outstanding = principal − Σ posted EMIs** (from `SumPostedEMIs`, not `emi_amount × count`) so the final debit is exact even after a partial/last-installment history. Σ of all EMI debits for the loan then equals `principal` exactly.

### 5.7 Authorization matrix (mirror §5.5)

| Action | Head | Member |
|---|---|---|
| `GET /loans` | all loans (may filter `user_id`) | own only (API-enforced) |
| `POST /loans` | ✅ create pre-approved `active` (target member, not head) | ✅ request `requested` for self |
| `POST /loans/:id/approve` | ✅ (`requested`→`active`) | ❌ 403 |
| `POST /loans/:id/reject` | ✅ (`requested`→`rejected`) | ❌ 403 |
| `POST /loans/:id/close` | ✅ (`active`→`closed`) | ❌ 403 |

Error shape stays `{"error": "..."}` with 400/401/403/404/409/500.

---

## 6. Routes (`main.go`, inside `protected`)

```go
protected.GET("/groups/:id/loans", loanHandler.ListLoans)
protected.POST("/groups/:id/loans", loanHandler.CreateLoan)
protected.POST("/loans/:id/approve", loanHandler.ApproveLoan)
protected.POST("/loans/:id/reject", loanHandler.RejectLoan)
protected.POST("/loans/:id/close", loanHandler.CloseLoan)
```
`/groups/:id/loans` reuses the `:id` param name (consistent, no wildcard conflict); `/loans/:id/...` mirrors the existing `/ledger/:id/approve` shape. No new lazy-trigger wiring — EMI posting rides the already-wired `runPosting` on `GET /groups/:id`, `/ledger`, `/balance`.

---

## 7. `main.go` + `NewService` wiring

- `loanRepo := db.NewLoanRepo(pool)`.
- **`NewService` signature changes** to accept `loanRepo`: `posting.NewService(allowanceRepo, ledgerRepo, loanRepo, groupRepo, pool)`. **Update all three call sites** (verified): `backend/cmd/server/main.go:44`, `backend/internal/handlers/ledger_integration_test.go:58`, `backend/internal/handlers/allowance_integration_test.go:56` — each must pass a `loanRepo` (build one from the pool in the test setups).
- `loanHandler := handlers.NewLoanHandler(loanRepo, groupRepo, pool)`.
- Register the five routes (§6).
- `storeAdapter` gains `loanRepo *db.LoanRepo` and the six delegating methods (§4.7).

No change to `NewGroupHandler`/`NewLedgerHandler` (they already hold `*posting.Service`; the service internally grows the loan reads).

---

## 8. Negative balances (§5.3/§8 — do not clamp)

EMIs post unconditionally, so `Σ approved credits − Σ approved debits` can be **negative** (the member temporarily owes the head). `GetBalanceForGroup` already computes this without a floor and returns it as a signed `int64` — **do not** add any `GREATEST(…, 0)` clamp, and do not special-case loans in the balance query (EMI rows are ordinary `debit` rows). An integration test (§11.2) must assert a member's balance goes negative after EMIs exceed their credits. (FE renders `−₹…` in danger color — WP-3.2, out of scope here.)

---

## 9. OpenAPI delta (`backend/openapi.yaml`)

Contract-first (§10.1). The ledger schema is already EMI-ready: `LedgerResponse`/`CreateLedgerRequest` already list `emi` in the `entry_type` enum, `loan_id` exists, and `POST /ledger` already rejects `emi`. So **no ledger-schema change** is required. **FE type regeneration is deferred to WP-3.2** — do not run `npm run codegen`, do not touch any `app/` file (§12). Add:

**Tag — add:** `- name: Loans` / `description: Loan requests, approval, and EMI repayment`.

**Paths — add** (mirror the `Allowances` path style already in the file):
- `GET /api/v1/groups/{id}/loans` — params `id` (path uuid), `user_id` (query uuid, optional), `status` (query enum `[requested, active, rejected, closed]`, optional). Responses `200` → array of `LoanResponse`; `400`/`401`/`403`/`500` → `ErrorResponse`. Description: "Head sees all loans; member sees only their own."
- `POST /api/v1/groups/{id}/loans` — `requestBody` → `CreateLoanRequest`. Responses `201` → `LoanResponse`; `400`/`401`/`403`/`404`/`500`. Description: "Member creates a `requested` loan for themselves; head creates a pre-approved `active` loan for a member. Disbursement is not a ledger entry; repayment is via monthly EMI debits."
- `POST /api/v1/loans/{id}/approve` — params `id` (path uuid); `requestBody` → `ApproveLoanRequest` (optional overrides). Responses `200` → `LoanResponse`; `400`/`401`/`403`/`404`/`409`/`500`. Description: "Head only. `requested`→`active`; may override principal/installments; sets start_period to next calendar month."
- `POST /api/v1/loans/{id}/reject` — param `id`. Responses `200` → `LoanResponse`; `401`/`403`/`404`/`409`/`500`. Description: "Head only. `requested`→`rejected`."
- `POST /api/v1/loans/{id}/close` — param `id`. Responses `200` → `LoanResponse`; `401`/`403`/`404`/`409`/`500`. Description: "Head only. Early payoff: posts one final EMI debit for the outstanding amount and sets `closed`."

**Schemas — add:**
```yaml
CreateLoanRequest:
  type: object
  properties:
    user_id:
      type: string
      format: uuid
      nullable: true
      description: Borrower. Required when the head creates a pre-approved loan; ignored/self for member requests.
    principal:
      type: integer
      format: int64
      minimum: 1
      description: Loan principal in minor units (paise). > 0.
    installments:
      type: integer
      minimum: 1
      description: Number of monthly installments. > 0.
    note:
      type: string
      nullable: true
  required: [principal, installments]

ApproveLoanRequest:
  type: object
  properties:
    principal:    { type: integer, format: int64, minimum: 1, nullable: true }
    installments: { type: integer, minimum: 1, nullable: true }
  description: Optional overrides applied before approval; emi_amount is recomputed.

LoanResponse:
  type: object
  properties:
    id:                  { type: string, format: uuid }
    group_id:            { type: string, format: uuid }
    user_id:             { type: string, format: uuid }
    principal:           { type: integer, format: int64, description: Minor units }
    installments:        { type: integer }
    emi_amount:          { type: integer, format: int64, description: ceil(principal/installments), minor units }
    start_period:        { type: string, nullable: true, description: 'YYYY-MM; null until active' }
    status:              { type: string, enum: [requested, active, rejected, closed] }
    note:                { type: string, nullable: true }
    requested_at:        { type: string, format: date-time }
    decided_by:          { type: string, format: uuid, nullable: true }
    decided_at:          { type: string, format: date-time, nullable: true }
    installments_posted: { type: integer, description: Count of posted EMI entries }
    outstanding:         { type: integer, format: int64, description: principal − Σ posted EMIs, minor units }
  required: [id, group_id, user_id, principal, installments, emi_amount, status, requested_at, installments_posted, outstanding]
```

Gate after editing: `grep -n "loans\|LoanResponse\|CreateLoanRequest\|ApproveLoanRequest" backend/openapi.yaml` → the five new paths + three schemas + tag present; the file still parses (YAML valid).

---

## 10. Gotchas (carried from WP-1.x / WP-2.1 — read before coding)

1. **`::bigint` casts on the new aggregates.** `SUM(bigint)` returns `numeric` in pgx → scanning into `int64` fails. Cast in **three** new spots: the `paid` sub-aggregate and `outstanding` in the loans list query (§2.2), and `SumPostedEMIs` (§2.2). `installments_posted`/`CountPostedEMIs` are `COUNT(*)` → scan into `int` (no cast needed).
2. **`loan_id` + `period` invariant for the index.** EMI inserts must set **both** `loan_id` and `period` non-NULL, or `idx_ledger_entries_posting_unique` (partial, `WHERE period IS NOT NULL`) won't guard them → duplicate EMIs → double-charging. Enforced by using `InsertEMIPosting` exclusively (§2.3).
3. **Deterministic iteration / lock order.** Both concurrent `PostDue` calls must visit users, then loans (`user_id, start_period, id`), in identical order, and lock loans via `FOR UPDATE` in that order (§4.2/§4.5). **Never** iterate a Go map to derive posting order — it randomizes and can deadlock (the exact trap the existing `engine.go` comment warns about). Keep/extend that comment.
4. **`created_by_user_id` is `NOT NULL`.** Machine EMI posts (and the close final debit) must supply the **group head** (§4.6). Don't insert NULL.
5. **No `chores` join in balance/posting.** EMI entries have `chore_id NULL`; balance is already `direction`-based — leave it. Do not clamp negative balances (§8).
6. **`gofmt` before every commit.** WP-1.2 review caught unformatted code in CI. Run `gofmt -l backend/` / `make -C backend lint`; the CI lint job fails on any diff.
7. **Integration-test helpers must add the head to `group_members`** with `AddMember(ctx, group.ID, head.ID, RoleHead)` — `GroupRepo.Create` only inserts the group row (broke WP-1.2 CI, commit `c60b491`). Reuse the `seedGroupWithMembers`/`seedGroup` helpers (they already do this). Head-authed loan endpoints 403 "not a member" without it.
8. **EMI does NOT need `joined_at` backdating (but the coexistence test's allowance leg does).** Unlike allowances (floored at join month → tests `backdateJoin`), EMI due-periods are driven by `start_period`, **not** the member's join date. To exercise EMI backfill/catch-up, create an **active loan with a past `start_period`** (insert directly or approve then `UPDATE loans SET start_period=…`). For the **allowance+EMI coexistence** test, you still need `backdateJoin` for the allowance leg (per project memory: `PostDue` floors allowance backfill at join month; `AddMember` stamps `now()`).
9. **Integration tests run in CI only — no local Docker.** The `//go:build integration` tests need Postgres via `docker-compose.test.yml`; the dev environment has no Docker. Do **not** claim to have run them locally — verify `make -C backend test lint` (unit + lint) locally; integration is validated by CI on the PR. State exactly this in the PR.
10. **`golangci-lint` local-version skew is pre-existing.** CI pins `version: latest`; trust CI's lint result, don't chase local-only lint noise.
11. **`TestMigrations_Idempotent` is a known CI flake** (test-isolation, per project memory). If it fails, **re-run** before treating it as a real break; it is not caused by this WP.
12. **golang-migrate out-of-order (011 below 012).** See §1 — only affects a dev DB already at 012; fresh DBs and CI are fine. Note it in the PR.
13. **`Querier` gains `QueryRow`.** The EMI path needs `FOR UPDATE` + `COUNT`/`SUM` on the tx (§2.4). Confirm `pgx.Tx` and `*pgxpool.Pool` still satisfy the widened interface and that `InsertAllowancePosting` (Exec-only) and its tests still compile.

---

## 11. Tests

### 11.1 Unit tests — `internal/posting` (**MANDATORY; must exist before review — §13**)
No Postgres; extend the existing `fakeStore` in `engine_test.go` with loan inputs/inserts, pin `now`.
- **EMI schedule / rounding (`emiSchedule`):** `1000/3 → [334,334,332]` (Σ=1000); exact division `900/3 → [300,300,300]`; `installments=1 → [principal]`; **tiny-principal edge** `2/3 → [1,1]` (length 2, no zero installment).
- **`addMonths`:** normal, year boundary (`2026-12 +1 → 2027-01`), multi-year (`+13`), `+0`.
- **Due periods:** loan `start_period` 3 months before `now` → exactly the due installments (start … current, capped at `scheduleLen`) posted at the right amounts; future `start_period > currentPeriod` → **zero** EMIs.
- **Idempotency:** call `PostDue` twice for the same `now`; assert no duplicate `(loan,period)` inserts and the second call inserts nothing (guard fast-path).
- **Auto-close:** after the final scheduled installment is posted, the fake's `CountPostedEMIs >= len(sched)` path calls `CloseLoan`; assert it fires exactly when complete and not before.
- **Allowance + EMI coexistence:** one user with both an allowance row and an active loan due in the same period → the fake records **both** an allowance insert and an EMI insert for that period (distinct keys), no collision.
- **Deterministic order:** feed multiple users/loans; assert loan lock/insert order is `(user_id, start_period, id)` and stable across runs (guards the no-deadlock property).
- **No loans / no allowances:** `PostDue` with neither configured → no-op (no inserts, no error); with loans only (no allowances) → EMIs still post.

### 11.2 Integration tests — `//go:build integration`, CI-only (Postgres via `docker-compose.test.yml`)
Reuse `testutil` + a `seedGroup`-style helper (**must add head to `group_members`**, §10.7). Create a `loanTestEnv` mirroring `allowanceTestEnv` (wire `loanRepo`, updated `NewService`, loan routes + a balance route to trigger posting).

- **EMI idempotency under double-trigger (MANDATORY):** active loan, `start_period` a few months back; call the real `PostDue` (or hit `GET /balance` which triggers it) **twice**; assert exactly one `emi` row per due period and a stable balance.
- **Concurrent double-trigger / race (MANDATORY):** run `PostDue` for the same group from **two goroutines** (`sync.WaitGroup`); assert **no error**, **no duplicate** `emi` rows, exactly the expected count and balance. The exactly-once-under-concurrency proof (§4.4).
- **Unique-index conflict path (MANDATORY):** call `InsertEMIPosting` twice for the same `(group,user,loan,period)`; first `inserted=true`, second `inserted=false`; exactly one row.
- **Allowance + EMI coexistence, same user + same period (MANDATORY):** member with an allowance effective this month **and** an active loan with an EMI due this month (`backdateJoin` for the allowance leg, §10.8); trigger; assert **both** an `allowance` credit and an `emi` debit exist for that `(user, period)`, and `balance = allowanceCredit − emiDebit` (+ any other entries). Proves the `loan_id`/`entry_type` index discrimination (§4.4).
- **Balance across disbursement + repayments, incl. NEGATIVE (MANDATORY):** create/approve a loan (assert **no** ledger entry appears for the disbursement); backdate `start_period` and trigger so several EMIs post; assert `balance` reflects `Σcredits − ΣemiDebits` and **goes negative** when EMIs exceed credits (§8) — no clamp.
- **Final / partial EMI + auto-close:** loan `1000/3`; drive posting across 3 months; assert amounts `334,334,332` (Σ=1000) and that the loan's `status` becomes `closed` after the third EMI (and `GET /loans` shows `installments_posted=3`, `outstanding=0`).
- **Early payoff (close):** loan partway repaid; `POST /loans/:id/close`; assert one final `emi` debit for the exact outstanding posted, Σ of all EMIs == `principal`, `status=closed`, `outstanding=0`; a second close → **409**.
- **Lifecycle state guards:** `approve` a non-`requested` loan → 409; `reject` a non-`requested` loan → 409; `close` a non-`active` loan → 409.
- **Authz:** member `POST /loans` naming another user → 400; member `approve`/`reject`/`close` → 403; member `GET /loans` returns only own; head `GET` returns all; head `POST` creates `active` with `start_period` = next month; `POST` (head) targeting the head as borrower → 400; approve/reject/close by a non-head member of the group → 403; by a non-member → 403.
- **Migration up/down (011), incl. the FK against existing `loan_id` data (MANDATORY):** extend `TestMigrations_UpAndDown` — `loans` + `loan_status` exist after up, absent after down. Add a focused test that, on a fully-migrated DB, seeds a loan and an `emi` ledger row referencing it (`InsertEMIPosting`), asserts the FK holds (a bogus `loan_id` insert **fails** with an FK violation), and that `RunMigrationsDown` cleanly drops the FK/table/enum (leaving `loan_id` a bare nullable column). Keep `_Idempotent` green (re-run on flake, §10.11).

### 11.3 Keep green
All existing unit + integration tests (allowances, ledger, migrations). Route additions must not break routing tests; the widened `Querier` and `NewService` signature must not break allowance tests.

---

## 12. Frontend & out-of-scope (explicit)

- **No FE changes.** `openapi.yaml` is updated (source of truth) but `app/src/api-types.gen.ts` is **not** regenerated and **no `app/` file is touched** — same rationale as WP-2.1 §11 (CI has no FE gate). **Handoff to WP-3.2 (loans tab):** run `npm run codegen`; build member request flow + head approve(editable terms)/reject/close + loan card (progress `paid n/m`, outstanding) + EMI rows in the ledger (`entry_type='emi'`, `−₹`, "EMI k/n — <loan note>" derived from `loan_id` + period ordering). **WP-3.3** adds the head member-detail page. Negative balances render `−₹…` in danger color there.
- **No `GET /members/:userId/summary`, no interest, no manual-post/preview endpoint, no loan edit/delete.** (§0.)

## 13. Verification floor

Backend WP — FE tooling intentionally not run (§12).
```bash
make -C backend test lint          # unit (-race) + golangci-lint — MUST pass locally
gofmt -l backend/                  # MUST be empty before commit (§10.6)
# Integration (CI-only; no local Docker — §10.9). CI runs:
#   go test -race -p 1 -tags=integration ./...
grep -n "loans\|LoanResponse\|CreateLoanRequest\|ApproveLoanRequest" backend/openapi.yaml  # new paths+schemas+tag present
grep -rn "interest\|apr\|APR" backend/internal backend/migrations                          # expect: none (zero-interest)
```
State in the PR exactly what ran locally (unit+lint+gofmt) vs. deferred to CI (integration). Do **not** assert integration passed locally.

## 14. Definition of Done (checklist)

- [ ] Migration **011_loans** up/down, paired, fresh-DB idempotent; adds `loans` + `loan_status` enum + the deferred `ledger_entries.loan_id → loans` FK (`ON DELETE SET NULL`); 009/010/012 untouched; golang-migrate gap caveat noted in PR (§1, §3).
- [ ] `models.Loan` + `models.LoanPostingInput`; `loan_repo.go` (Create, GetByID, ListForGroup w/ computed `installments_posted`+`outstanding`, ListActiveLoans deterministic order, PostedEMIPeriods, LockActiveLoan, CountPostedEMIs, SumPostedEMIs, CloseLoan, Approve, Reject) (§2).
- [ ] `LedgerRepo.InsertEMIPosting` (idempotent, bare `ON CONFLICT DO NOTHING`, `loan_id`+`period` set, `emi`/`debit`/`approved`, head as `created_by`) used for **all** EMI posts (schedule + close); never `Create` (§2.3, §4.6).
- [ ] `Querier` widened with `QueryRow` for `FOR UPDATE`; allowance path + tests still compile (§2.4).
- [ ] EMI posting slotted at the engine seam: `PostDue` posts one `emi` debit per due installment period, schedule with last-installment fixup (`1000/3→334,334,332`), robust to tiny principals; auto-closes when all scheduled installments posted (§4.3, §4.5).
- [ ] **Idempotency + concurrency exactly-once** via the 009 partial unique index (per-loan-per-period) + `FOR UPDATE` loan-row serialization; deterministic `(user_id, start_period, id)` iteration/lock order preserved — no randomized iteration, no new deadlock (§4.2, §4.4, §4.5).
- [ ] Allowance ↔ EMI coexistence (same user+period distinguished by `entry_type`+`loan_id`); allowance path unchanged (§4.4).
- [ ] Loan endpoints (§5/§6): `GET /loans` (head=all, member=own, `user_id`/`status` filters), `POST /loans` (member→requested-self / head→active-for-member-not-head), `approve` (`requested`→`active`, optional overrides, start_period=next month, 409 guard), `reject` (409 guard), `close` (early payoff: final EMI = outstanding, `active`→`closed`, 409 guard).
- [ ] Disbursement posts **no** ledger entry; balance may go **negative** (no clamp), integration-verified (§8, §11.2).
- [ ] `main.go` + `NewService` wiring; **all 3 `NewService` call sites updated** (main + 2 integration tests) (§7).
- [ ] `openapi.yaml`: 5 paths + 3 schemas + `Loans` tag; grep gate clean; **FE not regenerated / untouched** (§9, §12).
- [ ] Posting **unit tests exist** (schedule/rounding incl. tiny-principal, addMonths, due periods, idempotency, auto-close, coexistence, deterministic order) — required before review (§11.1, §13).
- [ ] Integration tests (CI): EMI double-trigger idempotency, concurrent race, unique-conflict, allowance+EMI coexistence, negative-balance across disbursement+repayments, final/partial EMI + auto-close, early payoff close, lifecycle 409 guards, authz, migration 011 up/down + FK — MANDATORY ones marked (§11.2).
- [ ] `::bigint` casts on the three new SUM/aggregate spots (§10.1).
- [ ] `make -C backend test lint` + `gofmt -l` clean locally; integration green in CI (§13).
- [ ] Entries + decided loans immutable (only the specified status transitions; no loan edit/delete, no EMI edit).

Commit/PR title: **`WP-3.1 spec: backend loans + EMI`** (this spec) — implementation PR later titled **`WP-3.1: backend loans + EMI`**.
