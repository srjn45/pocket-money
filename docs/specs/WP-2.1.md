# WP-2.1 Spec: Backend Allowances + Posting Engine

**Work package:** WP-2.1 (Phase 2 — Allowances), master-plan §9.
**Type:** Backend-only. DB migration **010** + `internal/posting` (new package) + `allowance_repo.go` + a new ledger insert path + allowance handlers + lazy-trigger wiring + `openapi.yaml` delta. Contract-first per §10.1. **No frontend** — FE lives in WP-2.2 (see §11).
**Risk:** **HIGH.** Master-plan §11 flags WP-2.1 (posting engine + money-affecting inserts) as *Opus-implement, or Sonnet-implement with strict Opus review*; **posting-engine unit tests MUST exist before review passes.** The review prompt must check: posting **idempotency** (§5.4), month math (join-month edge, backfill, amount-in-force), balance correctness after posting (§5.1), migration up/down, `::bigint` casts on any new aggregate, and authz (head-only allowance management, §5.5).

**Depends on (all merged to master):**
- **WP-1.1** — money is minor units: `int64` in Go, `BIGINT` in DB, aggregate SUMs cast `::bigint`.
- **WP-1.2** — unified ledger v2 is live. Migrations **009** (enums `ledger_entry_type`/`ledger_direction`, nullable `chore_id`, bare `loan_id`, `period CHAR(7)`, `note`, `decided_by`/`decided_at`, the **`NULLS NOT DISTINCT` partial unique index `idx_ledger_entries_posting_unique` on `(group_id, user_id, entry_type, period, loan_id) WHERE period IS NOT NULL`**) and **012** (settlements→ledger, `/settlements` + `/pending` removed) are applied. `models.EntryTypeAllowance` and `DirectionCredit` already exist. `GetBalanceForGroup` already computes `Σ approved credits − Σ approved debits` from `direction` (no chores join) with `::bigint` casts. **The posting unique index and the `allowance` enum value already exist — this WP is their first real user.**

**Acceptance (master-plan §9 WP-2.1 row):** unit tests for idempotency (call twice → no dupes), mid-month join, amount change effective next `effective_from`, and server-off-for-3-months backfill; migration 010; allowance endpoints; `internal/posting` with allowance posting; lazy trigger wired; `make -C backend test lint` (+ integration in CI) green.

---

## 0. Goal & scope boundary

Add **recurring monthly pocket money** (§5.2). The head configures a per-member allowance (amount in minor units, taking effect from a `YYYY-MM` month). A **lazy posting engine** (§5.4) turns each due month into exactly one approved `allowance` **credit** ledger entry — correct even if the home server was off for months, and safe under concurrent triggers. Balance (already `credits − debits`) then reflects allowances automatically.

### In scope
- Migration **010_allowances** (table + indexes, paired up/down).
- `internal/posting` package: pure `PostDue(...)` with allowance posting only, unit-testable without HTTP/Postgres.
- `allowance_repo.go` (config CRUD) + a **new idempotent ledger insert path** that sets `period` (the existing `LedgerRepo.Create` deliberately never sets `period`/`loan_id` — §2.3).
- Handlers: `GET /groups/:id/allowances`, `PUT /groups/:id/allowances/:userId` (§4, §5).
- Wire the lazy trigger into `GET /groups/:id`, `GET /groups/:id/ledger`, `GET /groups/:id/balance` (§6).
- `openapi.yaml` delta (§7). **No FE codegen** — WP-2.2 (§11).
- Unit + integration tests (§9).

### Explicitly OUT of scope (do not build; note discoveries in `docs/notes/` per §10.2)
- **Loans / EMI / migration 011** — WP-3.1. `loan_id` stays a bare nullable column; **do not** add its FK, the `loans` table, or EMI posting. The posting engine handles **allowances only**; leave a clearly-marked seam (`// EMI posting: WP-3.1`) where loan iteration will slot in, but implement nothing for it.
- **`GET /groups/:id/members/:userId/summary`** (§6.1) — its payload includes `active_loans[]`, so it depends on loans; defer to the loans/member-detail WPs. Do not add it here.
- **Any manual "post now" / "preview" endpoint.** §5.4/§6.1 define posting as *lazy-triggered only*. There is no allowance trigger or preview endpoint in v2. Do not invent one.
- **Weekly/other cadences.** Monthly-only; `period = YYYY-MM` (§5.2). Do not widen the format.
- **All frontend** (screens, `api-types.gen.ts`, TanStack hooks) — WP-2.2.
- **Proration.** Join month gets the *full* allowance (§5.2). No partial-month math.

---

## 1. Migration `010_allowances`

### 1.0 Numbering — WP-2.1 owns 010 (reserved gap from WP-1.2)

Per master-plan §6.2 and WP-1.2 §1.0, the `010/011` gap was reserved: **010 = allowances (this WP)**, 011 = loans (WP-3.1). Ship **`010_allowances.up.sql` / `.down.sql`**. Do **not** renumber; do **not** touch 011/012.

> **⚠️ golang-migrate out-of-order gotcha (document + verify).** `golang-migrate`'s `m.Up()` applies only versions **greater than the DB's current version**. A DB already migrated to **012** (WP-1.2) will **silently NOT apply an inserted 010** — `Up()` sees nothing `> 12`. This is fine for **every path this WP is tested on** because they all start from a **fresh DB**: CI `ResetTestDB` → `RunMigrations` applies `008,009,010,012` in ascending order (010 before 012); `TestMigrations_UpAndDown` resets first. There is **no production data** yet (Phase 2, LAN dev). **Consequence for a developer/home-server whose dev DB is already at 012:** they must reset/recreate the DB (drop schema, re-migrate) to pick up 010 — a bare `Up()` will skip it. Call this out in the PR description. Do **not** attempt to "fix" golang-migrate ordering; the reserved-gap decision is the master plan's and only bites already-advanced dev DBs, which reset trivially.

### 1.1 `010_allowances.up.sql`

```sql
-- Recurring monthly pocket money (§5.2). One row per (member, effective_from);
-- a change is a NEW row with a later effective_from (history preserved, past
-- months never rewritten). amount is minor units (paise), >= 0; 0 = paused.
CREATE TABLE allowances (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id       UUID   NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id        UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    amount         BIGINT NOT NULL CHECK (amount >= 0),   -- minor units; 0 = paused
    effective_from CHAR(7) NOT NULL,                      -- 'YYYY-MM', monthly-only
    created_by     UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id, effective_from)            -- upsert key (§5)
);

-- Posting engine reads all allowance rows for a group's members.
CREATE INDEX idx_allowances_group_user ON allowances (group_id, user_id);
```

Notes:
- **`amount BIGINT`** — matches the post-008 money convention; never `DECIMAL`.
- **`effective_from CHAR(7)`** — same storage as `ledger_entries.period`; format validated in the handler (`^\d{4}-\d{2}$`), not by a DB constraint (keep the format check in one place, Go).
- The `UNIQUE (group_id, user_id, effective_from)` is the **upsert arbiter** for `PUT` (§5): setting the same month twice updates the amount rather than erroring.

### 1.2 `010_allowances.down.sql`

```sql
DROP INDEX IF EXISTS idx_allowances_group_user;
DROP TABLE IF EXISTS allowances;
```

**Reversibility note.** 010 only *adds* the `allowances` table — it never touches `ledger_entries`. Down is an exact schema reverse. Any already-posted `allowance` ledger rows (`period` set) are **not** removed by 010-down (they belong to migration 009's table); this is correct — down is a schema-rollback tool (§10.4), and those rows remain valid unified-ledger entries. `TestMigrations_UpAndDown` round-trips cleanly on a fresh DB.

### 1.3 Migration-runner / harness updates
- **`backend/internal/db/migrate_test.go`** (`TestMigrations_UpAndDown`): add `"allowances"` to the `tables` slice asserted to **exist after up** and **absent after down**.
- **`backend/testutil/db.go`**:
  - `CleanupTestDB`: add `"allowances"` to the `TRUNCATE` list (place it **before** `ledger_entries` / early — `CASCADE` makes order non-critical, but keep child-before-parent tidiness). `allowances` exists post-migration, so it belongs in the non-`IF EXISTS` truncate list.
  - `ResetTestDB`: add `"allowances"` to the `DROP TABLE IF EXISTS` list.

---

## 2. Data model & repo (`backend/internal/models`, `backend/internal/db`)

### 2.1 `models.go` — new `Allowance` struct

```go
// Allowance is a per-member recurring monthly pocket-money configuration (§5.2).
// A change is a new row with a later EffectiveFrom; history is preserved.
type Allowance struct {
    ID            uuid.UUID `json:"id"`
    GroupID       uuid.UUID `json:"group_id"`
    UserID        uuid.UUID `json:"user_id"`
    Amount        int64     `json:"amount"`         // minor units; 0 = paused
    EffectiveFrom string    `json:"effective_from"` // 'YYYY-MM'
    CreatedBy     uuid.UUID `json:"created_by"`
    CreatedAt     time.Time `json:"created_at"`
}
```

### 2.2 `allowance_repo.go` — config CRUD

```go
type AllowanceRepo struct { pool *pgxpool.Pool }
func NewAllowanceRepo(pool *pgxpool.Pool) *AllowanceRepo { ... }
```

Methods:

- **`SetAllowance(ctx, groupID, userID uuid.UUID, amount int64, effectiveFrom string, createdBy uuid.UUID) (*models.Allowance, error)`** — upsert on the unique key:
  ```sql
  INSERT INTO allowances (id, group_id, user_id, amount, effective_from, created_by)
  VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
  ON CONFLICT (group_id, user_id, effective_from)
  DO UPDATE SET amount = EXCLUDED.amount, created_by = EXCLUDED.created_by
  RETURNING id, group_id, user_id, amount, effective_from, created_by, created_at
  ```
  (Re-setting a month updates its amount — matches "a change is a new row … same month = correction".)

- **`ListForGroup(ctx, groupID uuid.UUID) ([]*models.Allowance, error)`** — all rows for the group (head view). `ORDER BY user_id, effective_from`.
- **`ListForUser(ctx, groupID, userID uuid.UUID) ([]*models.Allowance, error)`** — one member's rows (member view). `WHERE group_id=$1 AND user_id=$2 ORDER BY effective_from`.
- **`ListPostingInputs(ctx, groupID uuid.UUID) ([]models.AllowancePostingInput, error)`** — the posting engine's read (§3). Joins allowance rows to `group_members` so the engine has each member's **join month** and can filter to `role='member'`:
  ```sql
  SELECT a.user_id, a.amount, a.effective_from, gm.joined_at
  FROM allowances a
  JOIN group_members gm ON gm.group_id = a.group_id AND gm.user_id = a.user_id
  WHERE a.group_id = $1 AND gm.role = 'member'
  ORDER BY a.user_id, a.effective_from
  ```
  Return type (put in `models` or `posting`): `AllowancePostingInput{ UserID uuid.UUID; Amount int64; EffectiveFrom string; JoinedAt time.Time }`.

> The GET response returns **full history rows** (all `effective_from` per member); "current" = the row with the greatest `effective_from ≤ current month`. This preserves history for FE display and keeps the BE dumb. WP-2.2 derives "current" client-side.

### 2.3 `ledger_repo.go` — new idempotent posting insert (**do not reuse `Create`**)

`LedgerRepo.Create` (WP-1.2) hardcodes `period`/`loan_id` as absent and has no conflict handling — it is the **API-create path** and must stay that way. Add a **separate** method for machine posting that sets `period` and is idempotent against `idx_ledger_entries_posting_unique`:

```go
// InsertAllowancePosting inserts one approved allowance credit for (group,user,period),
// idempotently. Returns (inserted bool) so the engine/tests can assert exactly-once.
// created_by is the group head (ledger_entries.created_by_user_id is NOT NULL; machine
// posts have no human decider, so decided_by/decided_at stay NULL). loan_id stays NULL,
// so the (group,user,'allowance',period,NULL) tuple collides via NULLS NOT DISTINCT.
func (r *LedgerRepo) InsertAllowancePosting(ctx context.Context, q Querier,
    groupID, userID uuid.UUID, amount int64, period string, createdBy uuid.UUID) (bool, error) {

    tag, err := q.Exec(ctx, `
        INSERT INTO ledger_entries
            (id, group_id, user_id, chore_id, amount, status, entry_type, direction,
             loan_id, period, note, created_by_user_id, decided_by, decided_at)
        VALUES (gen_random_uuid(), $1, $2, NULL, $3, 'approved', 'allowance', 'credit',
                NULL, $4, NULL, $5, NULL, NULL)
        ON CONFLICT DO NOTHING`,
        groupID, userID, amount, period, createdBy)
    if err != nil {
        return false, fmt.Errorf("failed to insert allowance posting: %w", err)
    }
    return tag.RowsAffected() == 1, nil
}
```

**`ON CONFLICT DO NOTHING` (bare, no target) is deliberate and required.** The only unique arbiter an allowance insert can violate is the partial index `idx_ledger_entries_posting_unique`. The bare form catches it without the fragility of restating a `NULLS NOT DISTINCT` partial-index predicate in the conflict target. (Equivalent explicit form, documented not used: `ON CONFLICT (group_id, user_id, entry_type, period, loan_id) WHERE period IS NOT NULL DO NOTHING`.) The `Querier` param lets the engine pass either the pool or a `pgx.Tx` (§3.4) — define/reuse a small interface:
```go
type Querier interface { Exec(context.Context, string, ...any) (pgconn.CommandTag, error) }
```
(`*pgxpool.Pool` and `pgx.Tx` both satisfy it.)

---

## 3. Posting engine (`backend/internal/posting`) — **the crux**

Pure, unit-testable, HTTP-free (§6.3). Allowances only in this WP; EMI is a WP-3.1 seam.

### 3.1 The one-line contract

> `PostDue(ctx, store, groupID, now)` makes the ledger contain **exactly one** approved `allowance` credit per `(member, month)` for **every month from `max(effective_from, join-month)` through the month of `now`** (server-local time), at the **amount in force** that month, **skipping paused (amount 0) months**. Calling it any number of times, from any number of concurrent triggers, converges to that exact set and never errors on a duplicate.

### 3.2 Store interface (dependency inversion — mock in unit tests)

```go
package posting

type Store interface {
    // ListPostingInputs returns every member allowance row (+ that member's join
    // date) for the group, ordered by user then effective_from. (AllowanceRepo)
    ListPostingInputs(ctx context.Context, groupID uuid.UUID) ([]models.AllowancePostingInput, error)
    // PostedAllowancePeriods returns, per user, the set of periods already posted
    // as allowance entries in this group — the fast-path guard (§3.5).
    PostedAllowancePeriods(ctx context.Context, groupID uuid.UUID) (map[uuid.UUID]map[string]bool, error)
    // GroupHead returns the group's head user id (created_by for machine posts).
    GroupHead(ctx context.Context, groupID uuid.UUID) (uuid.UUID, error)
    // WithTx runs fn inside a single transaction (one tx per group, §5.4).
    WithTx(ctx context.Context, fn func(q db.Querier) error) error
    // InsertAllowancePosting: the idempotent insert (§2.3), bound to the tx's Querier.
    InsertAllowancePosting(ctx context.Context, q db.Querier,
        groupID, userID uuid.UUID, amount int64, period string, createdBy uuid.UUID) (bool, error)
}
```
The concrete `Store` is a thin adapter in `internal/db` (or wired in `main.go`) over `AllowanceRepo` + `LedgerRepo` + `GroupRepo` + `pool`. Unit tests supply a fake `Store` — no Postgres needed.

`PostedAllowancePeriods` SQL:
```sql
SELECT user_id, period FROM ledger_entries
WHERE group_id = $1 AND entry_type = 'allowance' AND period IS NOT NULL
```

### 3.3 Algorithm

```
PostDue(ctx, store, groupID, now):
    currentPeriod := formatPeriod(now)                 // "YYYY-MM", server-local (§10.7)
    inputs := store.ListPostingInputs(groupID)         // member allowance rows + joined_at
    if len(inputs) == 0 { return nil }                 // genuine no-op: no allowances configured
    posted := store.PostedAllowancePeriods(groupID)    // guard set (§3.5)
    head  := store.GroupHead(groupID)

    // Group inputs by user; each user's rows are sorted by effective_from asc.
    return store.WithTx(ctx, func(q) error {
      for each user U with rows R[]:
        joinMonth := formatPeriod(R[].JoinedAt)        // all rows share the member's joined_at
        firstEff  := R[0].EffectiveFrom                // earliest configured month
        start     := maxPeriod(firstEff, joinMonth)    // §5.2: M >= effective_from AND >= join month
        for P in monthsInclusive(start, currentPeriod):        // ascending
            if posted[U][P] { continue }               // already have it → skip (fast-path)
            amt := amountInForce(R, P)                 // latest row with effective_from <= P
            if amt == 0 { continue }                   // paused month → post nothing (§5.2)
            store.InsertAllowancePosting(q, groupID, U, amt, P, head)  // ON CONFLICT DO NOTHING
        // EMI posting: WP-3.1 slots loan iteration here (out of scope).
      return nil
    })
```

Pure helpers (unit-tested directly, no DB):
- `formatPeriod(t time.Time) string` → `t.Format("2006-01")` (**server-local**; `now` is passed in, so tests pin it).
- `monthsInclusive(start, end string) []string` — ascending list of `YYYY-MM`; empty if `start > end` (e.g. a future `effective_from` — nothing due yet). Implement by parsing year*12+month ints; **do not** loop on `time.AddDate` across DST/oddities — integer month arithmetic is exact.
- `amountInForce(rows []AllowancePostingInput, period string) int64` — amount of the row with the greatest `effective_from ≤ period` (rows pre-sorted asc; walk/scan). There is always such a row because `start ≥ firstEff`.
- `maxPeriod(a, b string)` / period compare — lexical string compare works for `YYYY-MM` (zero-padded), no parsing needed.

### 3.4 Idempotency & concurrency (**MANDATORY invariant — reviewer checks this**)

- **Single-trigger idempotency:** every insert is `ON CONFLICT DO NOTHING` against `idx_ledger_entries_posting_unique`. A second `PostDue` for the same `now` inserts nothing. The `PostedAllowancePeriods` guard (§3.5) makes the common re-run a pure read (no INSERT attempts at all).
- **Concurrent triggers (the reason the index exists):** two `PostDue` calls racing on the same group each try to `INSERT` the same `(group,user,'allowance',P,NULL)` row. Postgres serializes on the unique index: the first transaction to reach the row wins; the second **blocks** until the first commits, then its `ON CONFLICT DO NOTHING` finds the now-committed row and **no-ops** (`RowsAffected()==0`). **No duplicate, no error, no 500 — exactly-once.** The `PostedAllowancePeriods` guard is only a best-effort read taken *before* the tx, so it does **not** guarantee exclusivity by itself; the **unique index is the real guarantee** and must never be removed as an "optimization".
- **`NULLS NOT DISTINCT`** (PG15, pinned) is what makes the two `NULL` `loan_id`s of allowance rows collide on `(group,user,type,period)`. This is a hard dependency (WP-1.2 §1.1 documents the pre-PG15 fallback). Any allowance insert **must** leave `loan_id` NULL and `period` non-NULL, or the partial index won't apply and duplicates become possible.

### 3.5 "Cheap no-op when nothing is due" (§5.4)

The `PostedAllowancePeriods` guard means a fully-caught-up group issues **zero** INSERTs — just two SELECTs (`ListPostingInputs`, `PostedAllowancePeriods`) and, if the guard shows the current period already present for all members, the tx body does nothing. For family scale (a handful of members, a few months) this is negligible. Do **not** micro-optimize further (no caching, no "last run" table) — correctness first.

### 3.6 Machine-post field values (fixed — no variation)

| Field | Value | Why |
|---|---|---|
| `entry_type` | `allowance` | §5.1 |
| `direction` | `credit` | pocket money adds to balance |
| `status` | `approved` | machine-posted → approved (§5.1) |
| `amount` | amount-in-force for the period, `> 0` | paused (0) months are skipped, never inserted |
| `period` | the due month `YYYY-MM` | idempotency key; must be non-NULL |
| `chore_id` / `loan_id` / `note` / `decided_by` / `decided_at` | **NULL** | no chore, no loan, no human decider |
| `created_by_user_id` | **group head** | column is `NOT NULL`; head owns the allowance policy |
| `created_at` | `now()` (DB default) | when it was posted; `period` carries the logical month |

---

## 4. Handlers (`backend/internal/handlers/allowances.go`, new)

New `AllowanceHandler{ allowanceRepo, groupRepo }`. Wire in `main.go` (§6).

### 4.1 `GET /api/v1/groups/:id/allowances`
- Auth: must be a group member (else 403), same pattern as `ListLedger`.
- **Head** → return **all** allowance rows for the group (`AllowanceRepo.ListForGroup`).
- **Member** → return **own** rows only (`AllowanceRepo.ListForUser(groupID, self)`), regardless of any query param (mirror the ledger member-sees-own rule, §5.5).
- 200 → `[]AllowanceResponse` (empty array, not null, when none).

### 4.2 `PUT /api/v1/groups/:id/allowances/:userId`
- Auth: **head only** (403 for members — "only group head can manage allowances").
- Validate `:userId` is a **member** of the group (`GroupRepo.GetMember`): not found → 404 `"target user is not a member of this group"`.
- **Reject targeting the head** → 400 `"cannot set an allowance for the group head"` (the head has no balance; §5.1 excludes them). Determine head via `group.HeadUserID` or the target member's `role == head`.
- Body `SetAllowanceRequest{ amount int64 (required, >=0); effective_from *string (optional) }`:
  - `amount` required, `>= 0` (0 = pause). Reject negative → 400.
  - `effective_from` optional; **default = current month** (`time.Now().Format("2006-01")`, server-local). If provided, must match `^\d{4}-\d{2}$` (reuse `isValidPeriod` from `ledger.go`) → else 400.
- Upsert via `AllowanceRepo.SetAllowance(...)` with `createdBy = caller`.
- 200 → `AllowanceResponse` (the created/updated row). Setting the same `effective_from` twice updates the amount (correction) — still 200.
- **Do not** trigger posting here — the next lazy read (§6) posts it. (Documented deliberate choice; keeps writes cheap and posting in one place.)

`AllowanceResponse` = the `Allowance` struct fields (id, group_id, user_id, amount, effective_from, created_by, created_at). Add an `allowanceToResponse` mapper for symmetry with `entryToResponse`.

Error shape stays `{"error": "..."}` with 400/401/403/404/500.

### 4.3 Authorization matrix (mirror §5.5)

| Action | Head | Member |
|---|---|---|
| `GET /allowances` | all members' rows | own rows only (API-enforced) |
| `PUT /allowances/:userId` | ✅ (any member, not the head) | ❌ 403 |

---

## 5. Lazy posting trigger wiring (§5.4)

Introduce a `PostingService` (the concrete `posting.Store` + `posting.PostDue`) and inject it where balance-affecting reads happen. Add a shared helper so all three call sites are identical:

```go
// runPosting triggers due allowance posting for the group before a balance-sensitive read.
// On error it writes 500 and returns false (do not serve a stale/incorrect balance).
func runPosting(c *gin.Context, svc *posting.Service, groupID uuid.UUID) bool {
    if err := svc.PostDue(c.Request.Context(), groupID, time.Now()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to post due allowances"})
        return false
    }
    return true
}
```

Call it **after membership is verified, before the data read**, in:
- `LedgerHandler.ListLedger` (`GET /groups/:id/ledger`)
- `LedgerHandler.GetBalance` (`GET /groups/:id/balance`)
- `GroupHandler.GetGroup` (`GET /groups/:id`)

**Failure policy = fail the read (500), do not log-and-continue.** A `PostDue` error means a real DB fault; serving a balance/ledger/group view that silently omits due pocket money would violate the core money-correctness promise (§5.1). If `PostDue` errors, the handler's own queries would likely fail anyway. `PostDue` is a genuine no-op when caught up (§3.5), so this adds ~two SELECTs to the hot read path — acceptable at family scale.

Wiring: `GroupHandler` and `LedgerHandler` each gain a `*posting.Service` field (extend their constructors + `main.go`). The service is built once in `main.go` from the repos + pool.

> **Membership-check ordering (regression guard):** posting must run **after** the existing `GetMember` 403 check in each handler, so a non-member still gets 403 (not a side-effecting post). Keep the current auth blocks first.

---

## 6. `main.go` wiring

- `allowanceRepo := db.NewAllowanceRepo(pool)`.
- Build the posting service: `postingSvc := posting.NewService(allowanceRepo, ledgerRepo, groupRepo, pool)` (constructor assembles the `Store` adapter).
- `allowanceHandler := handlers.NewAllowanceHandler(allowanceRepo, groupRepo)`.
- Pass `postingSvc` into `NewGroupHandler(...)` and `NewLedgerHandler(...)` (extend signatures + the two integration-test wirings in `ledger_integration_test.go` / any group test).
- Routes (inside `protected`):
  ```go
  protected.GET("/groups/:id/allowances", allowanceHandler.ListAllowances)
  protected.PUT("/groups/:id/allowances/:userId", allowanceHandler.SetAllowance)
  ```

> Gin route note: `GET /groups/:id/allowances` and existing `GET /groups/:id/...` share the `:id` param name — consistent, no wildcard conflict. `:userId` on the PUT is a distinct segment; fine.

---

## 7. OpenAPI delta (`backend/openapi.yaml`)

Contract-first (§10.1). Money fields are already `integer`/`int64`. **FE type regeneration is deferred to WP-2.2** — do not run `npm run codegen`, do not touch any `app/` file (see §11). Add:

**Paths — add:**
- `GET /api/v1/groups/{id}/allowances` — params: `id` (path, uuid). Responses: `200` → `array` of `AllowanceResponse`; `401`/`403`/`500` → `ErrorResponse`. Description: "Head sees all members' allowance history; member sees only their own."
- `PUT /api/v1/groups/{id}/allowances/{userId}` — params: `id`, `userId` (path, uuid). `requestBody` → `SetAllowanceRequest`. Responses: `200` → `AllowanceResponse`; `400`/`401`/`403`/`404`/`500` → `ErrorResponse`. Description: "Head only. Sets/changes a member's monthly pocket money; a new effective_from is a new history row (past months not rewritten). Amount 0 pauses."

**Schemas — add:**
```yaml
SetAllowanceRequest:
  type: object
  properties:
    amount:
      type: integer
      format: int64
      minimum: 0
      description: Monthly pocket money in minor units (paise). 0 pauses the allowance.
    effective_from:
      type: string
      nullable: true
      pattern: '^\d{4}-\d{2}$'
      description: Month this amount takes effect (YYYY-MM). Defaults to the current month.
  required: [amount]

AllowanceResponse:
  type: object
  properties:
    id:            { type: string, format: uuid }
    group_id:      { type: string, format: uuid }
    user_id:       { type: string, format: uuid }
    amount:        { type: integer, format: int64, description: Minor units; 0 = paused }
    effective_from:{ type: string, description: 'YYYY-MM' }
    created_by:    { type: string, format: uuid }
    created_at:    { type: string, format: date-time }
  required: [id, group_id, user_id, amount, effective_from, created_by, created_at]
```

No change to `LedgerResponse` (allowance rows already fit: `entry_type=allowance`, `direction=credit`, `period` set, `chore_id` null). No change to the ledger POST (it already rejects `entry_type=allowance` as machine-posted, WP-1.2). Add an `Allowances` tag if the file tags paths.

Gate after editing: `grep -n "allowances\|AllowanceResponse\|SetAllowanceRequest" backend/openapi.yaml` → the two new paths + two schemas present; the file still parses.

---

## 8. Gotchas (carried from WP-1.x — read before coding)

1. **`::bigint` casts on new aggregates.** WP-1.1: `SUM(bigint)` returns `numeric` in pgx. `GetBalanceForGroup` already casts and this WP doesn't change it — but **if you add any new SUM/aggregate over allowance amounts** (e.g. a "monthly total" query), cast it `::bigint` or the scan into `int64` fails.
2. **No `chores` JOIN in balance/posting.** Allowance entries have `chore_id NULL`. Never join `chores` to count them (WP-1.2's bug). Balance is already `direction`-based — leave it.
3. **`created_by_user_id` is `NOT NULL`.** Machine posts must supply the **group head** (§3.6). Don't try to insert NULL.
4. **`gofmt` before every commit.** WP-1.2 review caught unformatted code in CI (commit `f6f2581`). Run `gofmt -l .` / `make lint` locally; the CI lint job fails on any diff.
5. **Integration-test helpers must add the head to `group_members`.** `GroupRepo.Create` inserts only the group row — it does **not** add the head. Any new test that seeds a group must `AddMember(ctx, group.ID, head.ID, RoleHead)` (and members with `RoleMember`), or head-authenticated requests 403 "not a member" and the posting `role='member'` join returns nothing. This exact bug broke WP-1.2 CI (commit `c60b491`). Reuse `seedGroupWithMembers` from `ledger_integration_test.go`, which already does this.
6. **Integration tests run in CI only — no local Docker.** The `//go:build integration` tests need Postgres via `docker-compose.test.yml`; the dev environment has no Docker. Do **not** claim to have run them locally. Verify via `make -C backend test lint` (unit + lint) locally; integration is validated by CI on the PR. State exactly this in the PR.
7. **`golangci-lint` local-version skew is pre-existing.** CI pins `version: latest`; a locally-installed older/newer `golangci-lint` may report different findings. Trust CI's lint result; don't chase local-only lint noise.
8. **`TestMigrations_Idempotent` is a known CI flake** (test-isolation, per project memory). If it fails, **re-run** before treating it as a real break; it is not caused by this WP.
9. **golang-migrate out-of-order (010 below 012).** See §1.0 — only affects a dev DB already at 012; fresh DBs and CI are fine. Note it in the PR.
10. **`period`/`loan_id` invariant for the index.** Allowance inserts must set `period` non-NULL and leave `loan_id` NULL, or `idx_ledger_entries_posting_unique` (partial, `WHERE period IS NOT NULL`, `NULLS NOT DISTINCT`) won't guard them → duplicate risk. This is enforced by using `InsertAllowancePosting` (§2.3) exclusively for machine posts, never `LedgerRepo.Create`.

---

## 9. Tests

### 9.1 Unit tests — `internal/posting` (**MANDATORY; must exist before review — §11**)
No Postgres; drive `PostDue` with a **fake `Store`** and pin `now`.
- **Idempotency:** call `PostDue` twice for the same `now`; assert the fake's insert-set has **no duplicate** `(user,period)` and the second call inserts nothing.
- **Backfill (server off 3 months):** member joined & allowance effective 3 months before `now`; assert exactly 4 entries (join month … current, inclusive) at the right amounts.
- **Mid-month join:** `joined_at` mid-month; join month still gets a **full** allowance (no proration).
- **`effective_from` after join:** `start = max(effective_from, joinMonth)`; months before `effective_from` get nothing.
- **Future `effective_from`:** `effective_from > currentPeriod` → **zero** entries (`monthsInclusive` empty).
- **Amount change:** two rows (e.g. `2026-01: 500`, `2026-04: 1000`); with `now=2026-05`, assert 2026-01..03 = 500, 2026-04..05 = 1000 (amount-in-force selection); **past not rewritten**.
- **Paused (amount 0):** a `0` row for a month → that month posts nothing; un-pausing a later month resumes.
- **No allowances configured:** `PostDue` is a no-op (no inserts, no error).
- Pure-helper tests: `monthsInclusive` (normal, single-month, empty when start>end, year boundary Dec→Jan), `amountInForce`, `formatPeriod`.

### 9.2 Integration tests — `//go:build integration`, CI-only (Postgres via `docker-compose.test.yml`)
Reuse `testutil` + a `seedGroupWithMembers`-style helper (**must add head to `group_members`**, §8.5).

- **Idempotency under double-trigger (MANDATORY):** set an allowance; call the real `PostDue` (or hit `GET /balance` which triggers it) **twice**; assert exactly one allowance entry per due month and a stable balance.
- **Concurrent double-trigger / unique-index race (MANDATORY):** run `PostDue` for the same group from **two goroutines concurrently** (`sync.WaitGroup`); assert **no error**, **no duplicate** allowance rows, exactly the expected count. This is the exactly-once-under-concurrency proof (§3.4).
- **Unique-index conflict path (MANDATORY):** call `InsertAllowancePosting` twice for the same `(group,user,period)`; assert the first returns `inserted=true`, the second `inserted=false` (ON CONFLICT DO NOTHING), and exactly one row exists.
- **Balance correctness after posting:** allowance 1000 effective 3 months ago; trigger; `GET /balance` = `1000 × months`. Add an approved chore credit and a settlement debit; assert balance = `Σcredits − Σdebits` including the posted allowances (regression guard that allowance credits — `chore_id NULL` — are counted).
- **Amount change effective next month:** post current month; `PUT` a new amount effective next month; advance `now`; assert the new month posts the new amount and the already-posted month is unchanged (immutable).
- **Authz:** member `PUT /allowances/:userId` → 403; member `GET /allowances` returns only own rows; head `GET` returns all; `PUT` targeting the head → 400; `PUT` for a non-member userId → 404.
- **Migration up/down (010):** extend `TestMigrations_UpAndDown` — `allowances` exists after up, absent after down; `_Idempotent` still green (re-run on flake, §8.8).

### 9.3 Keep green
All existing unit + integration tests. Route additions must not break existing routing tests.

---

## 10. Verification floor

Backend WP — FE tooling intentionally not run (§11).
```bash
make -C backend test lint          # unit (-race) + golangci-lint — MUST pass locally
gofmt -l backend/                  # MUST be empty before commit (§8.4)
# Integration (CI-only; no local Docker — §8.6). CI runs:
#   go test -race -p 1 -tags=integration ./...
grep -n "allowances\|AllowanceResponse\|SetAllowanceRequest" backend/openapi.yaml  # new paths+schemas present
grep -rn "loans\|emi\|011" backend/internal/posting                               # expect: only the WP-3.1 seam comment, no impl
```
State in the PR exactly what ran locally (unit+lint+gofmt) vs. deferred to CI (integration). Do **not** assert integration passed locally.

## 11. Frontend & out-of-scope (explicit)

- **No FE changes.** `openapi.yaml` is updated (source of truth) but `app/src/api-types.gen.ts` is **not** regenerated and **no `app/` file is touched** — same rationale as WP-1.2 §5 (CI has no FE gate; WP-2.2 owns the FE consumer and will `npm run codegen` as its first step). **Handoff to WP-2.2:** regenerate types, build the head "set/edit allowance" UI + member "allowance visible in summary" + allowance rows rendering in the ledger.
- **No loans / EMI / migration 011 / `loan_id` FK / `/summary` endpoint / preview or manual-post endpoint / weekly cadence / proration.** (§0.)

## 12. Definition of Done (checklist)

- [ ] Migration **010_allowances** up/down, paired, fresh-DB idempotent; 011/012 untouched; golang-migrate gap caveat noted in PR (§1).
- [ ] `allowances` table: `amount BIGINT >= 0`, `effective_from CHAR(7)`, `UNIQUE(group_id,user_id,effective_from)`, index; `testutil` + `migrate_test` updated (§1.3).
- [ ] `internal/posting` with pure `PostDue`: exactly-one approved `allowance` credit per due `(member,month)` from `max(effective_from, join-month)` → current month, amount-in-force, paused-skip; EMI left as a marked WP-3.1 seam (§3).
- [ ] **Idempotency + concurrency exactly-once** via `ON CONFLICT DO NOTHING` on the 009 partial unique index; `InsertAllowancePosting` used for all machine posts (never `Create`); `loan_id` NULL, `period` set (§2.3, §3.4).
- [ ] Machine-post fields fixed per §3.6 (`created_by`=head, `decided_by/at` NULL, approved, credit).
- [ ] `AllowanceRepo` (upsert `SetAllowance`, `ListForGroup`, `ListForUser`, `ListPostingInputs`) + `models.Allowance` (§2).
- [ ] `GET /allowances` (head=all, member=own) + `PUT /allowances/:userId` (head-only, target must be a member, not the head; amount≥0; effective_from default=current, validated) (§4).
- [ ] Lazy trigger wired into `GET /groups/:id`, `/ledger`, `/balance`, **after** the membership check; `PostDue` error → 500 (§5).
- [ ] `main.go` wiring: repo, posting service, handler, routes (§6).
- [ ] `openapi.yaml` updated (2 paths, 2 schemas); grep gate clean; **FE not regenerated / untouched** (§7, §11).
- [ ] Posting **unit tests exist** (idempotency, backfill, mid-month join, amount-change, paused, helpers) — required before review (§9.1, §11).
- [ ] Integration tests (CI): double-trigger idempotency, concurrent race, unique-conflict path, balance-after-posting, amount-change, authz, migration up/down — idempotency ones marked MANDATORY (§9.2).
- [ ] `make -C backend test lint` + `gofmt -l` clean locally; integration green in CI (§10).
- [ ] Entries immutable (no allowance edit/delete of ledger rows; changing allowance = a new `allowances` row + future posting).

Commit/PR title: **`WP-2.1 spec: backend allowances + posting engine`** (this spec) — implementation PR later titled **`WP-2.1: backend allowances + posting engine`**.
