# WP-1.1 Spec: Money to Minor Units

**Work package:** WP-1.1 (Phase 1 — Ledger v2), marked ∥ in master-plan §9.
**Type:** Full-stack (DB migration + Go + `openapi.yaml` + generated FE types + FE formatting). Contract-first per §10.1.
**Risk:** High (money semantics + a migration). Master-plan §11 flags this Opus-implement or Sonnet-implement-with-strict-Opus-review.
**Depends on:**
- **WP-0.2 merged** (`b935c6f`): `openapi.yaml` → `app/src/api-types.gen.ts` codegen (`npm run codegen`, `codegen:check`) and TanStack Query are in place. WP-1.1 regenerates the committed types.
- **WP-1.0 merged** (design-system core): ships `AmountText` (renders **minor units** → `₹x.yy`, signed + colored) and `theme.ts`. WP-1.0 lands while the API still returns float rupees, so it introduces **interim `×100` conversions** at call sites to feed `AmountText`. **WP-1.1 removes those** once the API returns integer minor units natively. If WP-1.0 is not actually merged when this WP starts, STOP and flag it — do not re-implement `AmountText` here.

**Acceptance (master-plan §9 WP-1.1 row):** all tests pass; `grep` shows no `float64` money in backend.

---

## 0. Goal

Store, transport, and compute all money as **integer minor units (paise)** end-to-end:

- **DB:** `amount` columns `DECIMAL(12,2)` → `BIGINT` (migration 008).
- **Go:** every money field `float64` → `int64`.
- **API/JSON/TS:** `amount`/`balance` fields `number (double)` → `integer` (minor units). Field names stay `amount`/`balance`.
- **FE:** display via `AmountText` (minor units in, `₹x.yy` out); parse money **input** to minor units with a dedicated string parser — **never `parseFloat`** (§10.6).

This closes the master-plan §2 "Money as floats" gap and is the invariant every later WP (ledger v2, allowances, EMI math) builds on: no float money anywhere except UI formatting.

Also folded in: a **CI flake fix** (§7) for the integration-test race on the shared test DB.

**Guardrail:** stay in this WP. Do **not** add `entry_type`/`direction`/`note`/`decided_by` (that is WP-1.2 / migration 009), do **not** drop `settlements` (WP-1.2 / migration 012), do **not** rebuild the ledger screen (WP-1.3). The `settlements` table/handlers still exist here and are migrated to `BIGINT` alongside the rest.

---

## 1. Migration 008 — `008_money_to_minor_units`

Two paired files in `backend/migrations/`, following the existing numbering (007 is the latest):

### `008_money_to_minor_units.up.sql`

```sql
-- Convert all money columns from DECIMAL(12,2) rupees to BIGINT minor units (paise).
-- round(amount*100) is exact for DECIMAL(12,2) input (at most 2 fractional digits).
ALTER TABLE chores
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);

ALTER TABLE settlements
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);
```

### `008_money_to_minor_units.down.sql`

```sql
-- Revert BIGINT minor units back to DECIMAL(12,2) rupees.
ALTER TABLE settlements
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;

ALTER TABLE chores
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;
```

Notes:
- **Paired up/down, sequential, idempotent on a fresh DB** (§10.4): on a fresh DB, 003/004/005 create the columns as `DECIMAL(12,2)` and 008 alters them; the up/down round-trips cleanly. The `USING` clause makes the type change data-preserving.
- `round(amount * 100)` on `DECIMAL(12,2)` is lossless — the source has at most 2 fractional digits, so no rounding error is introduced (the `round` guards against any binary-float artifact and yields an integer for the `BIGINT` cast).
- Down migration divides by `100.0` (float/numeric division) to restore the 2-decimal value; this is exact for values that originated as `DECIMAL(12,2)`.
- `BIGINT` max (~9.2×10^18) is far beyond any family-scale money value in paise — no overflow concern.
- Column stays named `amount`; only the type changes. No index/constraint changes needed.

---

## 2. Backend changes (exact files + types)

All money `float64` → `int64`. `grep -rn "float64" backend/` must return **zero** money hits afterward (there are no non-money `float64` uses today, so it should return nothing).

### `backend/internal/models/models.go`
- `Chore.Amount` `float64` → `int64` (line ~59)
- `LedgerEntry.Amount` `float64` → `int64` (line ~71)
- `Settlement.Amount` `float64` → `int64` (line ~84)
- `Balance.Balance` `float64` → `int64` (line ~110)

### `backend/internal/handlers/chores.go`
- `CreateChoreRequest.Amount` `float64` → `int64`, keep binding `binding:"required,gt=0"` (valid on int64; enforces ≥ 1 paise).
- `UpdateChoreRequest.Amount` `*float64` → `*int64`.
- `ChoreResponse.Amount` `float64` → `int64`.

### `backend/internal/handlers/ledger.go`
- `CreateLedgerRequest.Amount` `*float64` → `*int64` (custom amount for system/Settlement chore).
- `LedgerResponse.Amount` `float64` → `int64`.
- `BalanceResponse.Balance` `float64` → `int64`.
- Local `var amount float64` (line ~202) → `int64`; the `*req.Amount <= 0` guard (line ~209) stays valid.

### `backend/internal/handlers/settlements.go`
- `CreateSettlementRequest.Amount` `float64` → `int64`, keep `binding:"required,gt=0"`.
- `SettlementResponse.Amount` `float64` → `int64`.

### `backend/internal/db/chore_repo.go`
- `Create(... amount float64 ...)` → `int64`.
- `CreateWithSystem(... amount float64 ...)` → `int64` (system Settlement chore is created with amount `0`; that stays `0`).
- `Update(... amount *float64 ...)` → `*int64`.
- All `Scan(&chore.Amount)` sites now scan a `BIGINT` into `int64` — pgx handles this natively, no code change beyond the struct-field type.

### `backend/internal/db/ledger_repo.go`
- `Create(... amount float64 ...)` → `int64`.
- **`GetBalanceForGroup` SQL gotcha (must fix):** the balance query does `COALESCE(SUM(CASE WHEN ... THEN le.amount ELSE 0 END), 0)`. In Postgres, **`SUM(bigint)` returns `numeric`**, not `bigint` — scanning `numeric` straight into `int64` will fail at runtime. Cast the aggregated columns back to `bigint`:

  ```sql
  COALESCE(SUM(CASE WHEN c.is_system = false THEN le.amount ELSE 0 END), 0)::bigint AS earned,
  COALESCE(SUM(CASE WHEN c.is_system = true  THEN le.amount ELSE 0 END), 0)::bigint AS settled,
  ...
  (COALESCE(lt.earned, 0) - COALESCE(lt.settled, 0))::bigint AS balance
  ```

  Then `rows.Scan(&balance.UserID, &balance.Name, &role, &balance.Balance)` scans `balance` as `int64`. This is the single most likely runtime break; the integration balance test (§8) must cover it.

### `backend/internal/db/settlement_repo.go`
- `Create(... amount float64 ...)` → `int64`; `Scan(&settlement.Amount)` scans `BIGINT` → `int64`.

### `backend/testutil/db.go`
- No change required. `CleanupTestDB`/`ResetTestDB` operate on table/type names, not money types. (The CI-flake fix in §7 lives in `ci.yml`, not here.)

---

## 3. openapi.yaml delta

Every money field flips from `type: number, format: double` to `type: integer, format: int64`, with a description that states the unit is minor units (paise). This is the source of truth; FE types are regenerated from it (§4).

| Schema | Field | Before | After |
|---|---|---|---|
| `CreateChoreRequest` | `amount` | `number`/`double`, `minimum: 0.01` | `integer`/`int64`, `minimum: 1` |
| `UpdateChoreRequest` | `amount` | `number`/`double`, `nullable` | `integer`/`int64`, `nullable` |
| `ChoreResponse` | `amount` | `number`/`double` | `integer`/`int64` |
| `CreateLedgerRequest` | `amount` | `number`/`double`, `nullable` | `integer`/`int64`, `nullable`, `minimum: 1` |
| `LedgerResponse` | `amount` | `number`/`double` | `integer`/`int64` |
| `BalanceResponse` | `balance` | `number`/`double` | `integer`/`int64` |
| `CreateSettlementRequest` | `amount` | `number`/`double`, `minimum: 0.01` | `integer`/`int64`, `minimum: 1` |
| `SettlementResponse` | `amount` | `number`/`double` | `integer`/`int64` |

Description convention (apply to each): append **"in minor units (paise); e.g. ₹12.50 = 1250"**. Update `BalanceResponse.balance`'s prose ("sum of approved ledger entries minus settlements") to note it is minor units. Leave `required` lists, field names, and every non-money property untouched. Error shape unchanged.

`grep -nE "double|number" backend/openapi.yaml` afterward should show no money field still typed as a float. (There are no non-money numeric fields in the spec today, so this grep is a clean gate.)

---

## 4. FE changes

### 4a. Regenerate committed types
Run `npm run codegen` from `app/`. `amount`/`balance` in `api-types.gen.ts` become `number` (integer at runtime; TS has no integer type — the contract's "these are minor units" is enforced by convention + `AmountText`, not the type system). Commit the regenerated file. `npm run codegen:check` must pass (proves committed spec ⇒ committed types, no drift).

### 4b. Display via `AmountText` and remove interim `×100`
- `app/src/api.ts`: the inline payload types already say `amount: number` (`choresApi.create/update`, `ledgerApi.create`, `settlementsApi.create`) — the value is now **minor units**. No signature change, but callers must stop treating it as rupees.
- **Remove WP-1.0's interim conversions.** Grep the app for the interim scaling WP-1.0 added to feed `AmountText` while the API still returned rupees, and delete the `×100`:
  ```
  grep -rnE "\* ?100|Math\.round\([^)]*100|amount ?\* ?100" app/src app/app
  ```
  After WP-1.1 the API value is already minor units, so `AmountText` receives `item.amount` / `balance` **directly**.
- **Sweep any remaining float-rupee formatting** on the screens this WP's unit change touches. These currently format the value as float rupees and will mis-render integer minor units (e.g. show `1250` or `1250.00`) unless fixed — route each through `AmountText` (or a shared `formatMinorUnits(minor): string` helper if a non-component context needs it):
  - `app/app/(app)/index.tsx` — dashboard `totalBalance` (`.toFixed(2)`, hardcoded `$`, lines ~119/137).
  - `app/app/(app)/groups/[id]/index.tsx` — member `balance` (`.toFixed(2)`, line ~99).
  - `app/app/(app)/groups/[id]/chores.tsx` — `item.amount.toFixed(2)` (line ~156). WP-1.0 re-skins this screen, so it likely already uses `AmountText`; just remove any `×100`.
  - `app/app/(app)/groups/[id]/ledger.tsx` — `item.amount.toFixed(2)` (line ~180), `memberBalance.toFixed(2)` (line ~252). This screen is **rebuilt in WP-1.3**; here only make it not display raw minor units and keep it type-clean — do not redesign it.

  There must be **no** remaining `amount.toFixed`/`balance.toFixed`/`` `$${...}` `` that treats a money value as float rupees.

### 4c. Money **input** parsing (see §5)
Every text input that collects a money amount must convert its string to minor units via the §5 helper before sending. Today these use `parseFloat` and must be replaced:
- `app/app/(app)/groups/[id]/chores.tsx` — `parseFloat(choreAmount)` (lines ~57, ~66) → `parseMoneyToMinorUnits`.
- `app/app/(app)/groups/[id]/ledger.tsx` — `parseFloat(customAmount)` (lines ~92, ~104) → `parseMoneyToMinorUnits`. (Ledger is rebuilt in WP-1.3, but the parse must be correct in the interim so entries aren't created with wrong amounts.)

---

## 5. Money-input parsing rule (§10.6)

**Never `parseFloat` user money input.** `parseFloat("12.50") * 100` risks binary-float artifacts (`0.1 + 0.2` class bugs) and silently accepts junk. Add a small, tested pure helper — put it in a shared FE util (e.g. `app/src/money.ts`) so every form and future WP reuses it:

```ts
// Parse a user-entered rupee string to integer minor units (paise).
// "12.50" -> 1250, "12" -> 1200, "0.05" -> 5, ".5" -> 50, "12.5" -> 1250.
// Returns null for invalid input (empty, non-numeric, negative, >2 decimals).
export function parseMoneyToMinorUnits(input: string): number | null {
  const s = input.trim();
  // optional leading digits, optional . with up to 2 decimal digits
  const m = /^(\d*)(?:\.(\d{0,2}))?$/.exec(s);
  if (!m || s === '' || s === '.') return null;
  const rupees = m[1] === '' ? 0 : parseInt(m[1], 10);
  const frac = (m[2] ?? '').padEnd(2, '0');       // "" -> "00", "5" -> "50"
  const paise = frac === '' ? 0 : parseInt(frac, 10);
  const minor = rupees * 100 + paise;
  return minor > 0 ? minor : null;                 // amounts must be > 0 (mirrors backend gt=0)
}

// Inverse, for display fallback where AmountText isn't used.
export function formatMinorUnits(minor: number): string {
  const sign = minor < 0 ? '-' : '';
  const abs = Math.abs(minor);
  return `${sign}₹${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, '0')}`;
}
```

Rules the parser enforces (all integer arithmetic — no float anywhere):
- Reject empty, `.`, non-numeric, negative, and more-than-2-decimal input → `null` → form shows a visible validation message and does not submit (reuse the existing inline-error pattern from WP-0.4).
- `"12"` → `1200`, `"12.5"` → `1250`, `"12.50"` → `1250`, `"0.99"` → `99`, `"0"`/`"0.00"` → `null` (must be > 0).
- Forms send the returned integer as `amount`. The Save button stays disabled while the parse is `null` (consistent with WP-0.4's validation-gating).

Add unit tests for `parseMoneyToMinorUnits` (Jest, or at minimum a `tsc`-checked table the implementer runs) covering the cases above; this is the one piece of genuinely new FE logic and is where money bugs hide.

---

## 6. Out of scope

- Ledger v2 columns (`entry_type`, `direction`, `note`, `decided_by`/`decided_at`, nullable `chore_id`, `loan_id`, `period`) — WP-1.2 / migration 009.
- Dropping `settlements` and removing `/settlements` + `/pending` — WP-1.2 / migration 012. They stay and are migrated to `BIGINT` here.
- Rebuilding the unified ledger screen — WP-1.3. WP-1.1 only makes the ledger screen not mis-format minor units and keeps it compiling.
- Allowances / loans / posting engine — Phases 2–3.
- Design system components (`AmountText`, `theme.ts`) — WP-1.0 (dependency, not built here).
- Any new endpoint or query param.

---

## 7. CI flake fix (folded in)

**Symptom (issue; observed on Actions run 28717027506):** integration step fails intermittently with `relation "public.schema_migrations" does not exist`.

**Root cause:** CI runs `go test -v -race -tags=integration ./...` (`.github/workflows/ci.yml:82`) with Go's default package-level parallelism against **one shared** `TEST_DB_*` database. `internal/db` migration tests (`TestMigrations_UpAndDown`, `TestMigrations_Idempotent`) call `RunMigrationsDown`, which drops all tables **including `schema_migrations`**, while `internal/handlers` integration tests run concurrently against the same DB and expect the schema to exist → race.

**Fix (chosen): add `-p 1` to the CI integration step**, so the package test binaries run serially:

```yaml
# .github/workflows/ci.yml, "Run integration tests" step
run: go test -v -race -p 1 -tags=integration ./...
```

**Why this over per-package databases:** the repo's own Makefile **already** serializes integration tests with `-p 1` (`test-all:` and `test-integration:` targets both run `go test -v -p 1 -tags=integration ./...`). CI simply drifted from that convention. This fix (a) makes CI match the Makefile that developers already use and that already passes, (b) is a one-token change with zero new code and zero flake surface, and (c) costs negligible wall-clock time given the small integration suite. Provisioning a unique schema/database per test package would fully isolate and preserve parallelism, but it adds real complexity to `testutil` (dynamic DB creation/teardown, per-package migration runs, cleanup on failure) — unjustified for a home-server-scale project when a proven one-line alignment exists. `-p 1` only serializes **package** test binaries; `-race` and in-package parallelism are unaffected. Leave the unit-test step (`ci.yml:79`) untouched — it needs no DB and benefits from parallelism.

Only `.github/workflows/ci.yml` changes for this fix.

---

## 8. Acceptance criteria

Backend / contract:
- Migration 008 up+down present, paired, sequential; `make -C backend test-integration` (which includes `TestMigrations_UpAndDown`) passes, proving up→down round-trips.
- `grep -rn "float64" backend/` returns **no money hits** (ideally nothing).
- `grep -nE "type: number|format: double" backend/openapi.yaml` returns nothing for money fields; all `amount`/`balance` are `integer`/`int64` with paise descriptions.
- Balance math correct with the `::bigint` cast: an integration test creates approved chore entries + a settlement entry and asserts `balance == Σ earned − Σ settled` in minor units (integer-exact, no rounding).
- `make -C backend test lint` passes (unit + `-race`, golangci-lint clean).

FE / contract:
- `npm run codegen` regenerates `api-types.gen.ts` with integer `amount`/`balance`; `npm run codegen:check` passes (no drift); regenerated file committed.
- No `parseFloat` on money input remains; all money inputs go through `parseMoneyToMinorUnits`; helper has passing unit tests for the §5 table.
- No interim `×100` conversions remain (`grep` from §4b is clean); money displays via `AmountText`/`formatMinorUnits` (minor units), not `.toFixed(2)` on a rupee float.
- `npx tsc --noEmit` clean; `npx expo export --platform web` succeeds (§10.3 FE verification floor).

CI:
- `ci.yml` integration step runs with `-p 1`; the `schema_migrations` race no longer occurs.

---

## 9. Verification commands

Backend (from repo root):
```bash
make -C backend test lint            # unit tests (-race) + golangci-lint
make -C backend test-integration     # migration up/down + balance/handler integration (Postgres via docker-compose.test.yml)
grep -rn "float64" backend/          # expect: no money hits
grep -nE "type: number|format: double" backend/openapi.yaml   # expect: no money fields
```

Frontend (from `app/`):
```bash
npm run codegen                      # regenerate api-types.gen.ts from openapi.yaml
npm run codegen:check                # expect exit 0 (no drift)
npx tsc --noEmit                     # expect zero errors
npm test                             # parseMoneyToMinorUnits unit tests (if Jest configured; else run the table manually)
npx expo export --platform web       # FE verification floor (§10.3) — must succeed
grep -rnE "\* ?100|parseFloat|\.toFixed\(2\)" app/src app/app   # expect: no money-float leftovers
```

Manual smoke (web, LAN backend up), per §10.3:
- Head creates a chore "Dishes" at `12.50` → stored as `1250`, list shows **₹12.50** (via `AmountText`).
- Log a Settlement entry with a custom amount `5.75` → stored `575`, balance moves by exactly ₹5.75.
- Try invalid inputs (`abc`, `12.999`, empty, `0`) → visible validation error, no submit.
- Dashboard + group overview balances render `₹x.yy` correctly (no raw integers, no `$`).

**Definition of done:** migration 008 paired/idempotent and green; all backend money types `int64` with zero `float64` money and no `number/double` money in `openapi.yaml`; balance query cast to `::bigint` and integration-verified; FE types regenerated + committed with `codegen:check` green; money input parsed via `parseMoneyToMinorUnits` (no `parseFloat`); display via `AmountText`/minor units with interim `×100` removed; `tsc`/lint/web-export clean; CI integration step serialized with `-p 1`. Commit/PR titled `WP-1.1: money to minor units`.
