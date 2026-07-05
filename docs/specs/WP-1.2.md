# WP-1.2 Spec: Ledger v2 Schema + API

**Work package:** WP-1.2 (Phase 1 — Ledger v2), master-plan §9.
**Type:** Backend-only (DB migrations 009 + 012 + Go models/repos/handlers + `openapi.yaml`). Contract-first per §10.1. **No frontend changes** — see §9 for the FE-compile survival decision.
**Risk:** High (two migrations with data backfill + a change to balance semantics). Master-plan §11 flags WP-1.2 as Opus-implement or Sonnet-implement-with-strict-Opus-review; the review prompt must check money math (§5.1), the authz matrix (§5.5), migration up/down, and that the balance query no longer depends on the `chores` join.

**Depends on:**
- **WP-1.1 merged** (money to minor units): migration **008** is applied, so `chores.amount`, `ledger_entries.amount`, `settlements.amount` are already `BIGINT` (minor units) and all Go money fields are `int64`. Migrations here start at **009**. The `GetBalanceForGroup` query already casts aggregates to `::bigint` (WP-1.1 §2) — WP-1.2 rewrites that query but keeps the cast.
- **WP-1.0 merged** (design system) — irrelevant to this backend WP; listed only because WP-1.3 (the FE consumer of this contract) depends on it.

**Acceptance (master-plan §9 WP-1.2 row):** integration tests for balance math, member-can't-see-others, head-only settlement/adjustment, and `decided_by` recorded on approve/reject; `/settlements` + `/pending` removed; settlement rows migrated; `openapi.yaml` updated; `make -C backend test lint` green.

---

## 0. Goal

Turn `ledger_entries` into the **single unified ledger** of §5.1: every balance-affecting event (chore credit, settlement debit, and — in later WPs — allowance/EMI/adjustment) is one immutable row, typed by `entry_type` + `direction`, with an audit trail (`decided_by`/`decided_at`). Consolidate the two legacy payout mechanisms (the `settlements` table **and** the system-"Settlement"-chore ledger entries) into `entry_type='settlement'` rows, then drop the `settlements` table and its endpoints, plus the redundant `/pending` endpoint.

**Balance becomes `Σ approved credits − Σ approved debits`** computed purely from `direction`, with no dependency on the `chores` join.

### Guardrails (stay in this WP)
- Do **not** create the `allowances` table (WP-2.1 / migration 010) or `loans` table (WP-3.1 / migration 011). `loan_id` is added here as a **bare nullable column with no FK**; the FK is added by migration 011 (§1.4).
- Do **not** build the posting engine (`internal/posting`) — WP-2.1/3.1. This WP only creates the unique partial index the engine will rely on.
- Do **not** rebuild any FE screen or regenerate FE types — WP-1.3 (§9).
- `allowance` and `emi` entry types are defined in the enum now (the schema is forward-looking) but are **not creatable via the API** in this WP (machine-posted only; §5). The API accepts only `chore | settlement | adjustment` on create.

---

## 1. Migrations

### 1.0 Numbering decision (009 ledger_v2, 012 drop_settlements — gap 010/011 reserved)

The plan reserves **010 = allowances** (WP-2.1) and **011 = loans** (WP-3.1); **012 = drop_settlements**. WP-1.2 ships **009** and **012**, leaving **010/011 as a deliberate gap**.

**Decision: keep the plan's numbering (009 and 012, not 009 + 010).** `golang-migrate` applies files in strict numeric order and only requires contiguity per its own tracking — a gap is fine; later WPs slot 010/011 in without renumbering. Renumbering 012→010 now would force WP-2.1/WP-3.1 to renumber their already-specced files and would desync every reference in the master plan (§6.2 names the files by number). Keeping 012 with a reserved gap is the lower-churn, plan-faithful choice.

**FK-ordering consequence.** `ledger_entries.loan_id` must reference `loans`, which does not exist until 011. Migration **009 adds `loan_id UUID NULL` with *no* foreign-key constraint** (a plain placeholder column). Migration **011** (WP-3.1, out of scope here) will add the FK:
```sql
-- migration 011 (WP-3.1) — documented here only as the handoff contract:
ALTER TABLE ledger_entries
    ADD CONSTRAINT fk_ledger_entries_loan
    FOREIGN KEY (loan_id) REFERENCES loans(id) ON DELETE SET NULL;
```
This lets 009 run before `loans` exists while still giving the posting engine the column and the unique index it needs.

### 1.1 Enum handling in Postgres

Two **new** enum types (the existing `ledger_status` enum is untouched):
```sql
CREATE TYPE ledger_entry_type AS ENUM ('chore', 'allowance', 'emi', 'settlement', 'adjustment');
CREATE TYPE ledger_direction  AS ENUM ('credit', 'debit');
```
Notes on enum mechanics:
- New columns are added as these types directly (`ADD COLUMN entry_type ledger_entry_type`). Postgres enum comparison/`WHERE` works with string literals (`entry_type = 'settlement'`), so repo SQL stays readable.
- Down migration must `DROP TYPE` **after** the columns using them are dropped (a type in use cannot be dropped).
- pgx binds Go `string`/typed-string values to enum columns natively (the existing `models.LedgerStatus` string type already does this) — no driver config needed. The new Go types (§2) follow the same `type X string` pattern.

### 1.2 Migration `009_ledger_v2`

Adds the v2 columns, backfills existing rows, consolidates the approve/reject audit into `decided_by`/`decided_at`, and creates the posting unique index.

#### `009_ledger_v2.up.sql`
```sql
-- 1. New enum types.
CREATE TYPE ledger_entry_type AS ENUM ('chore', 'allowance', 'emi', 'settlement', 'adjustment');
CREATE TYPE ledger_direction  AS ENUM ('credit', 'debit');

-- 2. New columns (all nullable so the ALTER succeeds on existing rows).
ALTER TABLE ledger_entries
    ADD COLUMN entry_type  ledger_entry_type,
    ADD COLUMN direction   ledger_direction,
    ADD COLUMN loan_id      UUID,                 -- FK added in migration 011 (loans)
    ADD COLUMN period       CHAR(7),              -- 'YYYY-MM', set on allowance/emi only
    ADD COLUMN note         TEXT,
    ADD COLUMN decided_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN decided_at   TIMESTAMPTZ;

-- 3. chore_id becomes nullable (null for allowance/emi/adjustment, and for
--    the settlement rows we normalise in step 5).
ALTER TABLE ledger_entries ALTER COLUMN chore_id DROP NOT NULL;

-- 4. Backfill entry_type + direction from the existing chore linkage.
--    Existing rows are either regular-chore credits or system-"Settlement"-chore debits.
UPDATE ledger_entries le
SET entry_type = CASE WHEN c.is_system THEN 'settlement' ELSE 'chore' END::ledger_entry_type,
    direction  = CASE WHEN c.is_system THEN 'debit'      ELSE 'credit' END::ledger_direction
FROM chores c
WHERE le.chore_id = c.id;

-- 5. Normalise backfilled settlement rows to the v2 invariant (settlement entries
--    carry no chore_id — they are typed, not chore-linked). Safe because balance
--    now keys off `direction`, not the chore join.
UPDATE ledger_entries SET chore_id = NULL WHERE entry_type = 'settlement';

-- 6. Consolidate the approve/reject audit into decided_by/decided_at.
--    We have the "who" (approved_by/rejected_by) but never recorded the "when",
--    so decided_at stays NULL for historic rows (§6.2: "backfill nulls").
UPDATE ledger_entries
SET decided_by = COALESCE(approved_by_user_id, rejected_by_user_id);
ALTER TABLE ledger_entries
    DROP COLUMN approved_by_user_id,
    DROP COLUMN rejected_by_user_id;

-- 7. Enforce NOT NULL on the always-present typed columns now that they are backfilled.
ALTER TABLE ledger_entries
    ALTER COLUMN entry_type SET NOT NULL,
    ALTER COLUMN direction  SET NOT NULL;

-- 8. Unique partial index for idempotent machine posting (§5.1/§5.4).
--    Postgres 15 (pinned in docker-compose + CI): NULLS NOT DISTINCT makes the
--    NULL loan_id of allowance rows collide correctly on (group,user,type,period),
--    while emi rows (loan_id set) stay distinct per loan.
CREATE UNIQUE INDEX idx_ledger_entries_posting_unique
    ON ledger_entries (group_id, user_id, entry_type, period, loan_id)
    NULLS NOT DISTINCT
    WHERE period IS NOT NULL;

-- Helpful lookup index for the new type filter (§4 GET ?type=).
CREATE INDEX idx_ledger_entries_group_type ON ledger_entries (group_id, entry_type);
```

> **Pre-PG15 fallback (documented, not used here):** if the project ever targets Postgres < 15, replace the `NULLS NOT DISTINCT` index with an expression index that maps NULL `loan_id` to a sentinel:
> ```sql
> CREATE UNIQUE INDEX idx_ledger_entries_posting_unique
>   ON ledger_entries (group_id, user_id, entry_type, period,
>                      COALESCE(loan_id, '00000000-0000-0000-0000-000000000000'::uuid))
>   WHERE period IS NOT NULL;
> ```
> The project pins `postgres:15-alpine` (both `docker-compose.yml` and `backend/docker-compose.test.yml`, and the CI `services.postgres` image), so `NULLS NOT DISTINCT` is the primary form.

#### `009_ledger_v2.down.sql`
```sql
DROP INDEX IF EXISTS idx_ledger_entries_group_type;
DROP INDEX IF EXISTS idx_ledger_entries_posting_unique;

-- Restore the approve/reject columns and reconstruct them from decided_by + status.
ALTER TABLE ledger_entries
    ADD COLUMN approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN rejected_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
UPDATE ledger_entries SET approved_by_user_id = decided_by WHERE status = 'approved';
UPDATE ledger_entries SET rejected_by_user_id = decided_by WHERE status = 'rejected';

-- Re-point backfilled settlement rows at the group's system chore so NOT NULL can be restored.
UPDATE ledger_entries le
SET chore_id = (SELECT id FROM chores c WHERE c.group_id = le.group_id AND c.is_system = true LIMIT 1)
WHERE le.chore_id IS NULL AND le.entry_type = 'settlement';

ALTER TABLE ledger_entries
    DROP COLUMN decided_at,
    DROP COLUMN decided_by,
    DROP COLUMN note,
    DROP COLUMN period,
    DROP COLUMN loan_id,
    DROP COLUMN direction,
    DROP COLUMN entry_type;

-- Restore NOT NULL (succeeds on the round-trip test's fresh DB, and on any data
-- that predates allowance/emi/adjustment rows — see reversibility note below).
ALTER TABLE ledger_entries ALTER COLUMN chore_id SET NOT NULL;

DROP TYPE IF EXISTS ledger_direction;
DROP TYPE IF EXISTS ledger_entry_type;
```

**Reversibility note.** `009` down is a *schema* rollback and is exact on a fresh/empty DB (which is what `TestMigrations_UpAndDown` exercises) and on any dataset that predates v2-only rows. It is **not** a data-safe reverse once `allowance`/`emi`/`adjustment` rows exist, because step "restore NOT NULL on chore_id" fails for rows that legitimately have no chore. This is acceptable and expected: down migrations are a development/rollback tool, not a production data-preservation guarantee (§10.4 requires paired up/down that are idempotent to re-run **on a fresh DB**, which this satisfies).

### 1.3 Migration `012_drop_settlements`

Migrates every legacy `settlements` row into the ledger as an **approved settlement debit**, then drops the table.

#### `012_drop_settlements.up.sql`
```sql
-- Migrate legacy settlements into the unified ledger as approved settlement debits.
-- The settlements table has no created_by, so we attribute the payout to the group head
-- (settlements were always head-only actions). We set:
--   created_at = the payout DATE (so it groups under the month it happened, matching
--                the old settlements-by-date UI), decided_at = the original row's created_at.
INSERT INTO ledger_entries
    (id, group_id, user_id, chore_id, amount, status, entry_type, direction,
     note, created_by_user_id, decided_by, decided_at, created_at)
SELECT
    gen_random_uuid(),
    s.group_id,
    s.user_id,
    NULL,                 -- settlement entries carry no chore_id
    s.amount,             -- already BIGINT minor units (migration 008)
    'approved',
    'settlement'::ledger_entry_type,
    'debit'::ledger_direction,
    s.note,
    g.head_user_id,       -- created_by: the head (settlements were head-only)
    g.head_user_id,       -- decided_by: same
    s.created_at,         -- decided_at: when it was recorded
    s.date::timestamptz   -- created_at: the payout date (drives month grouping)
FROM settlements s
JOIN groups g ON g.id = s.group_id;

DROP INDEX IF EXISTS idx_settlements_user_id;
DROP INDEX IF EXISTS idx_settlements_group_id;
DROP TABLE IF EXISTS settlements;
```

#### `012_drop_settlements.down.sql`
```sql
-- Recreate the settlements table + indexes (mirror of migration 005) so the schema
-- round-trips. The up-migration's data move is NOT reversed (see note).
CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,          -- BIGINT: migration 008 already converted this column type
    date DATE NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_settlements_group_id ON settlements(group_id);
CREATE INDEX idx_settlements_user_id ON settlements(user_id);
```

**Reversibility note.** `012` up folds two originally-distinct sources into one `entry_type='settlement'` shape; after the fact, a migrated-from-table row is indistinguishable from a 009-backfilled system-chore settlement row. A perfect reverse is therefore impossible, so **down restores only the (empty) schema**. On the fresh-DB round-trip test there are zero settlement rows, so up is a no-op insert and down cleanly recreates the table — the test passes. In production 012 is effectively forward-only, which matches the plan's intent (the `settlements` table is being permanently retired). The recreated table uses `amount BIGINT` because migration 008 (a lower-numbered, already-applied migration) converted the column; a down past 012 must land on the post-008 shape, not the original `DECIMAL`.

### 1.4 Migration-runner / test-harness updates

`RunMigrationsDown` runs every down in reverse; no code change. Update the following so the integration suite reflects the new schema:

- **`backend/internal/db/migrate_test.go`** (`TestMigrations_UpAndDown`):
  - Remove `"settlements"` from the `tables` slice that is asserted to **exist after up** (012 drops it). Keep it out of the post-down assertion loop too.
  - Add `"ledger_entry_type"` and `"ledger_direction"` to the `types` existence check (alongside `member_role`, `ledger_status`).
- **`backend/testutil/db.go`**:
  - `ResetTestDB`: add `"ledger_entry_type"` and `"ledger_direction"` to the `types` drop list so a full reset clears the new enums.
  - `CleanupTestDB` / `ResetTestDB`: `settlements` may be left in the table lists (both use `IF EXISTS` / swallow errors) but should be removed for tidiness since the table no longer exists after a full migrate.

---

## 2. Go model / repo / handler changes (per file)

### `backend/internal/models/models.go`
- Add enum types mirroring the DB:
  ```go
  type LedgerEntryType string
  const ( EntryTypeChore LedgerEntryType = "chore"; EntryTypeAllowance = "allowance";
          EntryTypeEMI = "emi"; EntryTypeSettlement = "settlement"; EntryTypeAdjustment = "adjustment" )
  type LedgerDirection string
  const ( DirectionCredit LedgerDirection = "credit"; DirectionDebit LedgerDirection = "debit" )
  ```
- `LedgerEntry` struct:
  - `ChoreID uuid.UUID` → `ChoreID *uuid.UUID` (nullable).
  - Add `EntryType LedgerEntryType`, `Direction LedgerDirection`, `LoanID *uuid.UUID`, `Period *string`, `Note *string`, `DecidedBy *uuid.UUID`, `DecidedAt *time.Time`.
  - **Remove** `ApprovedByUserID` and `RejectedByUserID` (consolidated into `DecidedBy`/`DecidedAt`).
- Delete the `Settlement` struct (table gone).

### `backend/internal/db/ledger_repo.go`
- `Create` signature grows to accept the typed fields. Suggested shape:
  ```go
  func (r *LedgerRepo) Create(ctx, groupID, userID uuid.UUID, choreID *uuid.UUID,
      createdBy uuid.UUID, amount int64, entryType models.LedgerEntryType,
      direction models.LedgerDirection, status models.LedgerStatus, note *string,
      decidedBy *uuid.UUID, decidedAt *time.Time) (*models.LedgerEntry, error)
  ```
  INSERT lists the new columns; `period`/`loan_id` are always NULL on the API-create path (posting engine sets them later, WP-2.1/3.1).
- `GetByID`, `ListForGroupWithUser`, `UpdateStatus`: SELECT/scan the new column set (drop `approved_by`/`rejected_by`, add `entry_type, direction, chore_id (nullable), loan_id, period, note, decided_by, decided_at`).
- `ListForGroupWithUser`: add optional `entryType *models.LedgerEntryType` and `period *string` filters (append `AND entry_type = $n` / `AND period = $n`). Keep the existing `status` + `userID` filters.
- `UpdateStatus` → rename/repurpose to set the audit atomically:
  ```go
  func (r *LedgerRepo) SetDecision(ctx, id uuid.UUID, status models.LedgerStatus,
      decidedBy uuid.UUID, decidedAt time.Time) (*models.LedgerEntry, error)
  // SET status=$2, decided_by=$3, decided_at=$4 WHERE id=$1 RETURNING ...
  ```
- **`GetBalanceForGroup` — rewrite to use `direction` (critical).** The current query `INNER JOIN chores ON le.chore_id = c.id` would now **drop every settlement row** (their `chore_id` is NULL) and miscompute balances. New query has no chore join:
  ```sql
  WITH ledger_totals AS (
      SELECT le.user_id,
             COALESCE(SUM(CASE WHEN le.direction = 'credit' THEN le.amount ELSE 0 END), 0)::bigint AS credits,
             COALESCE(SUM(CASE WHEN le.direction = 'debit'  THEN le.amount ELSE 0 END), 0)::bigint AS debits
      FROM ledger_entries le
      WHERE le.group_id = $1 AND le.status = 'approved'
      GROUP BY le.user_id
  ),
  all_members AS (
      SELECT gm.user_id, u.name, gm.role
      FROM group_members gm JOIN users u ON gm.user_id = u.id
      WHERE gm.group_id = $1
  )
  SELECT am.user_id, am.name, am.role,
         (COALESCE(lt.credits, 0) - COALESCE(lt.debits, 0))::bigint AS balance
  FROM all_members am LEFT JOIN ledger_totals lt ON am.user_id = lt.user_id
  ORDER BY am.role DESC, am.name
  ```
  Keep the `::bigint` casts (WP-1.1: `SUM(bigint)` returns `numeric`). Head is still filtered out of the returned list, as today.

### `backend/internal/db/settlement_repo.go`
- **Delete the file** (table and repo gone).

### `backend/internal/handlers/ledger.go`
- `CreateLedgerRequest` becomes:
  ```go
  type CreateLedgerRequest struct {
      EntryType models.LedgerEntryType `json:"entry_type" binding:"required"`
      UserID    *uuid.UUID             `json:"user_id"`   // head may target a member
      ChoreID   *uuid.UUID             `json:"chore_id"`  // required iff entry_type=chore
      Amount    *int64                 `json:"amount"`    // required (>0) iff settlement/adjustment
      Direction *models.LedgerDirection `json:"direction"`// required iff entry_type=adjustment
      Note      *string                `json:"note"`
  }
  ```
- `CreateLedger` logic replaces the old `chore.IsSystem` branch with an `entry_type` switch (validation + authz table in §3):
  - Reject `entry_type` ∈ {`allowance`,`emi`} → `400 "allowance/emi entries are machine-posted only"`.
  - **chore:** `chore_id` required and must belong to the group and not be the system chore; amount = `chore.Amount` (member may **not** pass a custom amount); `direction='credit'`. Head → `approved` (may set `user_id`); member → `pending_approval`, self only.
  - **settlement:** head only; `user_id` required (target member); `amount` required `>0`; `chore_id`/`direction` ignored; `direction='debit'`, `status='approved'`, `decided_by=head`, `decided_at=now()`.
  - **adjustment:** head only; `user_id` required; `amount` required `>0`; `direction` required (`credit`|`debit`); `status='approved'`, `decided_by=head`, `decided_at=now()`. `note` recommended (the "why").
- `ListLedger`: add parsing of `type` and `period` query params → pass to `ListForGroupWithUser`. **Keep** the existing member-sees-own enforcement (member's `filterUserID` forced to self); validate `type` against the enum, `period` against `^\d{4}-\d{2}$`.
- `ApproveLedger` / `RejectLedger`: call `SetDecision(..., decidedAt=time.Now())`; response carries `decided_by`/`decided_at`.
- **Delete `ListPending`** (the `/pending` endpoint is removed; pending entries are fetched via `GET /ledger?status=pending_approval`).
- `LedgerResponse`: drop `approved_by_user_id`/`rejected_by_user_id`; add `entry_type`, `direction`, nullable `chore_id`, `loan_id`, `period`, `note`, `decided_by`, `decided_at`.

### `backend/internal/handlers/settlements.go`
- **Delete the file** (handler + request/response types).

### `backend/cmd/server/main.go`
- Remove `settlementRepo`, `settlementHandler`, and the two `/groups/:id/settlements` routes.
- Remove the `/groups/:id/pending` route.
- Update `NewLedgerHandler` call if its dependency set changes (it still needs `ledgerRepo, groupRepo, choreRepo` — chore lookup is still required for `entry_type=chore`).

---

## 3. Authorization & validation matrix (POST /groups/:id/ledger)

Mirrors master-plan §5.5. Enforced in the handler; membership is checked first (403 if not a member).

| entry_type | Who may create | user_id (target) | amount | chore_id | direction | Resulting status | decided_by/at |
|---|---|---|---|---|---|---|---|
| `chore` | head (any member) | head may set; member = self | from chore config (custom rejected) | required, group's, non-system | `credit` (server) | head→`approved`, member→`pending_approval` | head-create → head/now; member-create → null |
| `settlement` | **head only** | required | required, `>0` (custom) | must be null/absent | `debit` (server) | `approved` | head / now |
| `adjustment` | **head only** | required | required, `>0` (custom) | must be null/absent | **required** (`credit`\|`debit`) | `approved` | head / now |
| `allowance`, `emi` | nobody (API) | — | — | — | — | `400` machine-posted-only | — |

Read (`GET /groups/:id/ledger`): head sees all members (may filter `user_id`); **member sees own only** — the handler forces `filterUserID = self` regardless of a supplied `user_id` (unchanged from today, must be preserved). Approve/reject: **head only**, entry must be `pending_approval`.

Error shape stays `{"error": "..."}` with 400/401/403/404/409/500.

---

## 4. OpenAPI delta (`backend/openapi.yaml`)

Money fields are already `integer`/`int64` (WP-1.1). Changes:

**Paths — remove:**
- `/api/v1/groups/{id}/settlements` (GET + POST) and `SettlementResponse`/`CreateSettlementRequest` schemas.
- `/api/v1/groups/{id}/pending` (GET).

**`GET /api/v1/groups/{id}/ledger` — add query params:**
- `type` — `enum: [chore, allowance, emi, settlement, adjustment]`, optional.
- `period` — `type: string`, `pattern: '^\d{4}-\d{2}$'`, optional.
(`status`, `user_id` already present.)

**`CreateLedgerRequest` — restructure:**
- Add `entry_type` (`enum: [chore, settlement, adjustment]`, **required**).
- `chore_id` → `nullable: true`, **remove from `required`** (required only when `entry_type=chore`, enforced server-side; document in the field description).
- Add `direction` (`enum: [credit, debit]`, nullable — required only for `adjustment`).
- Add `note` (string, nullable).
- `amount` stays `integer/int64`, nullable (required for settlement/adjustment; `minimum: 1`).
- `required: [entry_type]`.

**`LedgerResponse` — restructure:**
- Add `entry_type` (enum, required), `direction` (enum, required).
- `chore_id` → `nullable: true`, remove from `required`.
- Add `loan_id` (uuid, nullable), `period` (string, nullable), `note` (string, nullable), `decided_by` (uuid, nullable), `decided_at` (date-time, nullable).
- **Remove** `approved_by_user_id`, `rejected_by_user_id`.
- `required: [id, group_id, user_id, amount, status, entry_type, direction, created_by_user_id, created_at]`.

**`BalanceResponse`:** update `balance` description to "sum of approved credits minus approved debits, in minor units (paise)".

**Tags/description:** drop `settlements` from the top-level `description` line.

Gate after editing: `grep -n "settlements\|/pending\|approved_by_user_id\|rejected_by_user_id" backend/openapi.yaml` → no path/field hits remain.

---

## 5. Frontend-compile survival strategy (decision + justification)

**Decision: WP-1.2 changes `openapi.yaml` (the source of truth) but does NOT regenerate `app/src/api-types.gen.ts` and touches NO FE file. FE reconciliation is handed entirely to WP-1.3.**

Rationale:
- **The scope says *remove* the endpoints** — so 410-tombstone stubs (keeping the routes alive) are rejected: they contradict "remove", leave dead handlers, and would still 500/410 at runtime on screens the plan already calls known-broken.
- **CI does not gate the FE.** `.github/workflows/ci.yml` runs only backend `lint`/`test`/`build` under `./backend`; there is no `codegen:check`, `tsc`, or `expo export` job. So regenerating types is **not** required to keep this PR's CI green, and skipping it introduces no CI failure.
- **Every consumer of the removed operations is deleted-and-rebuilt in WP-1.3.** `app/app/(app)/groups/[id]/ledger.tsx`, the `pending` screen, and the `settlements` screen all call the removed endpoints; `app/src/api.ts` has a `settlementsApi` object and a pending/ledger caller. Per master-plan §2/§9 the ledger tab is **known-broken by design until WP-1.3**, which "remove[s] Pending/Settlements screens". Regenerating types **now** would delete the `settlements`/`pending` operations from `api-types.gen.ts`, break `tsc` on those exact screens, and force WP-1.2 to either delete or patch FE files — i.e. do WP-1.3's deletion work early, violating "FE OUT of scope" and splitting one coherent FE rebuild across two WPs.
- **Net:** the smallest coherent change is backend-only. The spec↔types drift it creates is bounded (one WP), documented here, and closed by WP-1.3's very first step.

**Handoff contract to WP-1.3 (record, do not act on here):** WP-1.3 must, as its first step, run `npm run codegen`, then delete `settlementsApi` + the pending caller from `app/src/api.ts`, delete the pending/settlements screens and their route links, and rebuild the ledger screen on the new `LedgerResponse` shape (`entry_type`, `direction`, `decided_by`/`decided_at`, nullable `chore_id`). Because CI has no FE gate, nothing enforces this automatically — it is an explicit WP-1.3 acceptance item.

*(If a reviewer insists the committed types must never lag the spec, the fallback is to also run `npm run codegen` here and delete only the two orphaned FE callers/screens — but that pulls WP-1.3 deletion work forward for no CI benefit and is not recommended.)*

---

## 6. Integration test list (`//go:build integration`, `internal/handlers` + `internal/db`)

New/updated tests (Postgres via `docker-compose.test.yml`, existing `testutil`):

1. **Balance math (credits − debits).** Seed a group + member; create approved `chore` credits and an approved `settlement` debit (and an `adjustment` of each direction); assert `GET /balance` = `Σ credits − Σ debits` in exact minor units. Include a pending chore entry and assert it is **excluded**; a rejected entry excluded. Regression guard: this must pass **specifically because settlement rows have NULL chore_id** — proves the `direction`-based query counts them (the old chore-join query would drop them).
2. **Member can't see others.** Member A and B in a group; as A, `GET /ledger` (and `GET /ledger?user_id=B`) returns only A's entries. As head, `GET /ledger?user_id=B` returns B's entries.
3. **Head-only settlement/adjustment.** Member POST `entry_type=settlement` → 403; member POST `entry_type=adjustment` → 403. Head POST both → 201, `direction` debit/credit correct, `status=approved`, `decided_by`=head, `decided_at` set. Member POST `entry_type=chore` for self → 201 `pending_approval`; member POST with a custom `amount` → amount ignored (equals chore config).
4. **decided_by recorded on approve/reject.** Member creates pending chore entry; head approves → `status=approved`, `decided_by`=head, `decided_at` non-null. Fresh pending entry; head rejects → `status=rejected`, `decided_by`=head, `decided_at` non-null. Reject counts toward balance = 0.
5. **entry_type/period filters + validation.** `GET /ledger?type=settlement` returns only settlements; `?type=chore` only chores; invalid `type`/`period` → 400. `allowance`/`emi` on POST → 400 (machine-posted-only).
6. **Migration idempotency + round-trip.** Extend `TestMigrations_UpAndDown`/`_Idempotent`: full up (through 012) leaves `settlements` **absent** and `ledger_entry_type`/`ledger_direction` present; down→up re-run is clean. **009/012 data-migration re-run safety:** applying migrations twice on the same DB (the runner tracks `schema_migrations`, so 009/012 bodies run once) — assert no duplicate settlement rows and stable balances.
7. **009 backfill correctness (repo/DB test).** Pre-seed pre-009-shape rows (regular-chore entry + system-chore entry) *before* running 009 in a dedicated test, then assert post-009 `entry_type`/`direction` and that the system-chore entry became `settlement`+`debit` with `chore_id` NULL, and `decided_by` = old approver. *(If seeding pre-migration state is impractical in the harness, cover the equivalent invariants via the API-level tests 1–4 and assert the backfill SQL by code review — note this in the PR.)*
8. **012 settlement migration.** With the legacy `settlements` table still present at 008/009 (i.e., exercise via a seeded row before 012 in a migration-focused test, or assert the INSERT…SELECT shape), confirm each migrated row lands as approved `settlement` debit attributed to the head. Same practicality caveat as #7.

Keep existing auth/authz handler tests green (route removals: assert `GET /groups/:id/pending` and `/settlements` now 404 at the router).

---

## 7. Acceptance criteria

- Migrations **009** and **012** present, paired up/down, numbered with the 010/011 gap reserved; `make -C backend test-integration` (incl. `TestMigrations_UpAndDown`) passes — up runs through 012, down round-trips on a fresh DB.
- `009` adds `entry_type`, `direction`, nullable `chore_id`, bare `loan_id` (no FK), `period`, `note`, `decided_by`, `decided_at`; drops `approved_by_user_id`/`rejected_by_user_id`; backfills existing rows (system-chore→`settlement`/`debit`, else `chore`/`credit`; `decided_by`=old approver); creates the `NULLS NOT DISTINCT` partial unique index.
- `012` migrates all `settlements` rows into the ledger as approved `settlement` debits and drops the table; `/settlements` + `/pending` routes and their handlers/repo/model are deleted.
- Balance = `Σ approved credits − Σ approved debits` via `direction` (no `chores` join); settlement debits (NULL `chore_id`) are counted; pending/rejected excluded; integration-verified (test #1).
- `POST /ledger` enforces the §3 matrix: settlement/adjustment head-only with custom amount; member chore-entries self-only + pending + chore-config amount; `allowance`/`emi` rejected.
- `decided_by`/`decided_at` set on head-created approved entries and on approve/reject (tests #3/#4).
- `openapi.yaml` updated per §4; grep gate clean; **FE types not regenerated** (§5) — no FE file touched.
- `make -C backend test lint` passes (unit + `-race`, golangci-lint clean). Entries immutable — no edit/delete endpoints added.

## 8. Verification commands

Backend only (this is a backend WP — §5 justifies not running FE tooling):
```bash
make -C backend test lint            # unit (-race) + golangci-lint
make -C backend test-integration     # migrations 009/012 up-down + balance/authz integration (Postgres via docker-compose.test.yml)
grep -rn "settlement\|Settlement" backend/internal backend/cmd   # expect: no live settlement handler/repo/model refs (only migration 012 + ledger settlement entry_type)
grep -n "settlements\|/pending\|approved_by_user_id\|rejected_by_user_id" backend/openapi.yaml   # expect: none
```
**FE tooling (`npm run codegen`, `npx tsc --noEmit`) is intentionally NOT run in WP-1.2** (§5): CI has no FE gate, and WP-1.3 owns every consumer of the changed contract and will regenerate types as its first step. Running codegen here would only break `tsc` on screens WP-1.3 deletes.

**Definition of done:** migrations 009 (ledger v2 columns + enum types + backfill + `decided_by`/`decided_at` consolidation + partial unique index) and 012 (settlements→ledger migration + table/endpoint removal) paired and green; balance recomputed from `direction` with settlement debits counted; `POST/GET /ledger` implement the §3 authz/validation matrix and filters; `/settlements` and `/pending` gone; entries immutable; `openapi.yaml` updated with FE regeneration deferred to WP-1.3; `make -C backend test lint` + integration green. Commit/PR titled `WP-1.2 spec: ledger v2 schema + API` (spec) — implementation PR later titled `WP-1.2: ledger v2 schema + API`.
