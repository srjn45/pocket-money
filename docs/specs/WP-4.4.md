# WP-4.4 Spec: E2E Tests + Seed Data

**Work package:** WP-4.4 (Phase 4 — UX polish & launch readiness). It is the **FINAL** work package
of the master plan and its implementer runs **strictly LAST**, against the fully-assembled UI.
**Type:** **New tooling only** — a backend seed command, a new top-level `e2e/` Playwright project,
one CI job, and a **bounded, additive** sweep of `testID`/accessibility props onto existing screens.
**No product behavior changes**, no new API, no migration, no change to any runtime dependency of the
Expo app.
**Depends on (must be merged before this implementer starts):** the entire Phase 1–3 stack plus
**every Phase-4 sibling** — WP-4.1 (design sweep), WP-4.2 (dashboard v2 + `GET /groups` summary),
WP-4.3 (invite/share + pending-token round-trip), WP-4.5 (branding — merged, `f830aa4`), **WP-4.6
(onboarding: register auto-login + register-path pending-invite resume + smart empty states)** and
**WP-4.7 (change password + leave/remove member)**. See §0.1 — **this WP targets the app as it exists
AFTER 4.6 and 4.7 merge**, and its journeys exercise their exact flows.
**Acceptance (master plan §9, Phase 4 row 4.4):** "`make seed` (demo family), Maestro (mobile) or
Playwright (web) happy-path e2e; docker-compose runs full stack — one command brings up BE+DB+web with
seed data; e2e green in CI."

> Roadmap refs: master-plan §1 (the launch bar — the e2e is the automated proof of "register → group →
> invite → first chore → first approval"), §5.1 (balance = Σ approved credits − Σ approved debits, int64
> paise — the seed must reproduce this math), §5.2 (allowances, no proration, join-month floor), §5.3
> (loans + EMI, `emi = ceil(principal/installments)`, last installment absorbs rounding), §5.4 (lazy
> `PostDue` — the seed drives it, and the dashboard summary deliberately does **not**, §0.4/G4), §5.5
> (authz matrix — the e2e asserts head vs member surfaces), §10 (Definition of Done — rule 3 FE
> verification floor: `npx expo export --platform web` must pass; this WP makes that export the artifact
> the e2e runs against), §11 (this is a tooling WP: Sonnet implement + Sonnet/Opus review; correctness
> risk is in the seed's invariant fidelity and CI flake discipline, so the review prompt must check the
> seed's money/`PostDue`/`joined_at` handling and the testID-only app-code rule).
>
> UX authority for the journeys: `docs/fe-flow.md` (group listing, group detail head/member, join flow),
> `docs/prd.md` (F1–F17), and the sibling specs WP-4.6 §10 (the <3-min walkthrough — the e2e is its
> automation) and WP-4.7 §8 (change-password + leave/remove reviewer walkthrough).

---

## 0. Scope, guardrails & the decisions this WP makes

### In scope (exactly these four deliverables)

1. **SEED** — a deterministic `backend/cmd/seed` Go command, invoked by `make seed` (root) and
   `make -C backend seed`, that builds one demo family (head + 2 kids, one group) with a rich ledger
   history spanning **every entry type** — manual credit/adjustment, manual debit/settlement, machine
   `allowance` postings, member-submitted `chore` (approved, pending, rejected), and an active `loan`
   with posted `emi` rows. It reproduces the WP-4.2 dashboard math and the WP-3.x loan/EMI math, and it
   **honors every invariant** (money int64; `joined_at` backdated by explicit `UPDATE`; ledger written
   through the domain path or `PostDue`, never a hand-rolled balance). (§1)
2. **E2E** — a new **top-level `e2e/`** Playwright project (`@playwright/test`) that drives the
   **exported Expo web bundle** against a **seeded CI backend**, covering the launch-bar journeys end to
   end, split into **must-have** and **nice-to-have** (§2). Chromium only in CI (§2.4, D5).
3. **CI** — one new `e2e` job in `.github/workflows/ci.yml`: Postgres service → backend (auto-migrates
   on boot) → `make seed` → `expo export --platform web` → static-serve `dist/` → `playwright test`,
   with **traces + screenshots + videos uploaded as artifacts on failure**. Runtime budget ≤ ~10 min
   (§3).
4. **testIDs** — an **explicit, bounded** list of screens/components that gain `testID` (+ RN
   accessibility) props so selectors are stable. **This is the ONLY app-code change this WP is permitted
   to make**; it is additive and changes no behavior or layout (§5).

### Non-goals (explicit boundaries)

- **No product/behavior change.** No new endpoint, no `openapi.yaml` edit, no codegen, **no migration**
  (if you reach for `ALTER TABLE` or a new migration number, STOP — you have drifted). The seed uses only
  existing repos/tables; the e2e only reads/drives the running app.
- **No new runtime dependency in `app/`.** Playwright and its config live in a **separate `e2e/`
  package** with its own `package.json`/`node_modules` (decision **D3**, §0.3). Do **not** add
  `@playwright/test`, `serve`, or any test tool to `app/package.json` — that would ship test tooling in
  the Expo bundle and the app-store build.
- **No Maestro / native-device e2e in CI.** The master plan says "Maestro (mobile) **or** Playwright
  (web)"; we choose Playwright/web (decision **D1**, §0.2). Native deep-link and share-sheet flows are
  **device-only** and are listed as **manual reviewer guidance**, not automated (§2.3).
- **No change to `docker-compose.yml` service topology.** The acceptance line "docker-compose runs full
  stack" is satisfied by a documented **compose profile / one-command bring-up** that layers seed on the
  existing services (§6, D6) — not by rewriting the compose file's services.
- **No edit to `theme.ts` or any kit component's behavior.** Adding a `testID` passthrough to a kit
  component (if one is missing) is allowed **only** as an additive prop with a default of `undefined`
  and no render change (§5.2) — and only for the bounded list.
- **No flaky waits.** No `waitForTimeout`/arbitrary sleeps in tests; selectors are `getByTestID` /
  `getByRole` / `getByText` with Playwright auto-waiting (§4, D4).

### 0.1 Sequencing — this WP runs LAST, after 4.6 AND 4.7 (READ FIRST)

**Concurrency note.** WP-4.6's implementation "starts after WP-4.7" and both are Phase-4 siblings that
may still be **in flight** when this spec is written. **The WP-4.4 implementer MUST NOT start until both
WP-4.6 and WP-4.7 have merged to `master`.** Branch WP-4.4 off `master` only then, and take the
post-4.6/4.7 tree as ground truth. Concretely, the e2e depends on these behaviors that 4.6/4.7 introduce:

| Behavior the e2e drives | Owned by | The e2e assertion it enables |
|---|---|---|
| `POST /auth/register` returns `{token, user}` → client lands **in the app**, not back on login | **WP-4.6** D1 | T1 "register auto-logs-in" (no second login step) |
| Register-path pending-invite resume (`getPendingInviteToken` on the **register** success handler) | **WP-4.6** §5.3 | T3 "new member registers *via invite link* → lands INSIDE the group" |
| Smart empty-state copy (chores member bare-state fix E5, etc.) | **WP-4.6** §6 | Empty-state text assertions (nice-to-have) |
| `PUT /auth/password` — wrong current pw → **403, session preserved** (not 401/logout) | **WP-4.7** D1/G1 | T-CP "wrong current password shows inline error, user STAYS logged in" |
| `DELETE /groups/:id/members/:userId` — leave blocked by non-zero balance → **409 toast**; succeeds after settle | **WP-4.7** D2–D7 | T-LEAVE "leave blocked (409) → settle → leave succeeds" |
| Dashboard v2 sections (head total owed / member own balance), Groups tab hidden | **WP-4.2** | T2 dashboard structure assertions |
| Invite button + `/invite?token=` web route landing in group | **WP-4.3** | T3 invite generation + join |

**If, on your base, any of the above is absent** (e.g. register still returns `UserResponse`, or leave
returns 404 not 409): 4.6/4.7 did not fully land — **stop and re-verify the merge state** before writing
the affected journey. Do not "work around" a missing behavior in the test; the test is the acceptance
gate for that behavior.

**File-lane safety.** This WP touches: new files under `backend/cmd/seed/**` and `e2e/**`, the two
Makefiles, `.github/workflows/ci.yml`, and `testID`-only additive edits to a bounded list of `app/**`
files (§5). It writes **no** file that 4.1/4.2/4.3/4.6/4.7 own for behavior. Because it runs last, there
is no live concurrency on those app files — but keep the testID edits surgical so a late 4.6/4.7 hotfix
rebases cleanly.

### 0.2 Decision D1 — Playwright/web, not Maestro/mobile

The master plan offers "Maestro (mobile) **or** Playwright (web)". **Chosen: Playwright against the
exported Expo web bundle.** Rationale:

- **The app already exports web** and CI already runs `npx expo export --platform web` (the `fe-web-export`
  job, `ci.yml:144`). Reusing that exact artifact means the e2e tests the same bundle the FE-verification
  floor (§10 rule 3) already blesses — one artifact, one source of truth.
- **CI is Linux, Docker-capable, headless.** Playwright/Chromium runs first-class on `ubuntu-latest`
  with no emulator; Maestro needs an Android emulator (KVM, slow cold-boot, flakier) or a paid device
  cloud. For a home-LAN family app whose primary "share a link in a browser" onboarding path *is* the
  web, web e2e covers the launch-bar journey directly (the invitee opening `http://…/invite?token=` in a
  browser is literally WP-4.3 §4.2's universal path).
- **Native-specific surfaces** (OS share sheet, `pocketmoney://` deep link, SecureStore) are the
  minority and are covered as **manual device checks** (§2.3), consistent with how 4.3/4.6/4.7 already
  list device-only items as reviewer guidance.

Rejected: Maestro (emulator cost/flake, second toolchain); Detox (native build, heavier); Cypress
(Playwright's trace viewer + multi-context is better for the two-user invite journey).

### 0.3 Decision D3 — Playwright lives in a NEW top-level `e2e/`, not in `app/`

A **new top-level directory `e2e/`** with its **own `package.json`** (and its own `node_modules`, its own
lockfile). Rationale:

- **Keeps the Expo app's dependency graph clean.** `@playwright/test` (~large, with browser binaries) and
  a static file server must never enter `app/package.json` — Expo bundles/`expo export` would pull test
  tooling into consideration, `expo-doctor` could flag version skew, and the app-store build must not ship
  it. The monorepo already separates `backend/` and `app/`; `e2e/` is the natural third peer for
  cross-stack tests.
- **Independent Node/tooling version.** `e2e/` pins Node 22 + Playwright without touching the app's
  `devDependencies`.
- **Clear ownership.** Everything the e2e needs (config, fixtures, page objects, the static-serve dep)
  is under `e2e/`; nothing leaks.

Layout:
```
e2e/
  package.json            # @playwright/test + serve, pinned (§4.5)
  package-lock.json
  playwright.config.ts     # baseURL, webServer(serve dist), projects, reporter, trace/screenshot/video
  tsconfig.json
  .gitignore               # node_modules, test-results, playwright-report, blob-report
  fixtures/
    seed.ts                # the canonical seed credentials/ids as TS constants (mirrors the Go seed)
    users.ts               # freshly-registered test users (unique-per-run emails)
  support/
    api.ts                 # thin helper: register/login via REST for state setup (optional)
    pages.ts               # page-object helpers keyed on testIDs (login, dashboard, group, etc.)
  tests/
    onboarding.spec.ts     # T1–T3 (register auto-login, create group, invite, register-via-invite)
    ledger.spec.ts         # add entries, approve/reject, allowance visible
    loans.spec.ts          # request → approve → EMI visible, member detail
    account.spec.ts        # change password (403 path), leave-blocked-then-settle-then-leave
    seeded-readonly.spec.ts# assertions against the pre-seeded demo family (dashboard math, EMI rows)
```

### 0.4 Decision D4 — determinism & flake discipline are first-class

The single biggest risk in an e2e WP is flake. Non-negotiable rules, justified in §4:

- **Selectors:** `testID` (RN → `data-testid` on web) first; `getByRole`/`getByText` only for content
  assertions. **No CSS/nth-child/XPath.** (§5)
- **No arbitrary waits.** Rely on Playwright auto-waiting + web-first assertions (`await
  expect(locator).toBeVisible()`). The one legitimate "settle" wait is a web-first assertion on a network
  result, never `waitForTimeout`.
- **Fresh users get unique emails per run** (`e2e+<runid>-<n>@demo.test`) so re-runs never collide on the
  unique-email constraint; the **read-only seeded family** uses fixed credentials and is only ever
  *read/asserted*, never mutated by a mutating test (mutating journeys create their own users/groups).
- **Backend clock authority (§5.4/§10.7):** periods are `YYYY-MM` in the **server's** local time. The
  seed and the loan/allowance assertions compute expected periods from the **same clock the server uses**
  (the CI runner's `TZ`, pinned to `UTC` in the job env, §3) — never from the browser. A month-boundary
  flake (seeding on the 31st, asserting on the 1st) is avoided by computing "current period" once in the
  seed and exposing it (the seed prints a machine-readable summary, §1.6, that the read-only spec reads).

### 0.5 Decision — seed idempotency = **destructive reset**, guarded

`make seed` **truncates the demo tables and rebuilds from scratch every run** (reset semantics), **not**
an upsert/no-op. Rationale:

- **A demo/dev/CI database is disposable and must be deterministic.** Every seed run must yield the
  *exact same* balances, periods, and IDs-by-role so the read-only e2e assertions are stable. An
  incremental "insert if missing" seed drifts (re-running would double-post manual entries, or leave a
  half-seeded state after a mid-run failure) — the opposite of what a fixture needs.
- **The domain forbids in-place edits** (§5.1 entry immutability), so "update the existing demo to match"
  is not even expressible; reset is the only clean path.
- **Guard against nuking real data.** The reset only proceeds when it is safe: it refuses unless **either**
  the target DB name is in a dev/test allowlist (`pocket_money`, `pocket_money_test`, `pocket_money_dev`)
  **or** the operator passes `--reset` (env `SEED_RESET=1`). Absent both on an unrecognized DB name, the
  seed exits non-zero with a loud message ("refusing to truncate unknown database <name>; pass --reset to
  force"). CI passes `--reset` explicitly. This mirrors the root Makefile's `restore` FORCE guard.
- **Truncate, not DROP.** The schema is owned by migrations (auto-run on backend boot); the seed only
  clears **rows**. It `TRUNCATE`s the same table set `testutil.CleanupTestDB` uses
  (`loans, allowances, invite_tokens, ledger_entries, chores, group_members, groups, users`) with
  `CASCADE`, in one transaction. It never touches `schema_migrations`.

Consequence documented in the seed's `--help` and README: **`make seed` DESTROYS all data in the target
DB.** That is intended for demo/dev/CI; never point it at a family's live DB (they'd use `make backup`
first anyway — WP-1.4).

---

## 1. SEED — `backend/cmd/seed` (`make seed`)

### 1.1 Mechanism decision (Go command, reusing the domain layer)

**The seed is a Go program (`backend/cmd/seed/main.go`), not a `.sql` file and not a sequence of HTTP
calls.** Rationale (this is the master-plan's explicit concern — "Seed must NOT bypass invariants"):

- **A raw `.sql` seed would bypass every invariant**: it would hand-write balances, hand-pick `emi`
  rounding, and hand-set `joined_at` without the posting engine — exactly the drift the domain model
  guards against. Rejected.
- **An HTTP/API seed** (drive the real endpoints) honors authz/validation but **cannot backdate**:
  ledger `created_at` and membership `joined_at` are stamped `now()` by the API, so an API-only seed
  cannot produce *history across months* (needed for allowance/EMI rows in prior periods). It also can't
  set an `active` loan with a past `start_period` directly. Rejected as the sole mechanism.
- **A Go command reusing the repos + `posting.PostDue`** gets the best of both: it writes users/groups/
  members/chores/allowances/loans through the **same repo code the handlers use** (int64 money,
  validation-shaped inserts), it **runs the real `PostDue`** to generate `allowance`/`emi` history
  idempotently and correctly (join-month floor, EMI rounding, auto-close), and it uses **targeted direct
  SQL only** for the two things the domain intentionally stamps as `now()` — `group_members.joined_at`
  (backdated by `UPDATE`, per project memory: `AddMember` stamps `now()`, `PostDue` floors backfill at
  join month) and backdated `ledger_entries.created_at` for the manual/pending/rejected demo rows. Money
  stays int64 throughout. **Chosen.**

It imports `internal/config`, `internal/db`, `internal/posting`, `internal/models`, `internal/auth`
(for bcrypt via the same hashing the register path uses — reuse `auth` if it exposes a hash helper, else
`golang.org/x/crypto/bcrypt` directly, matching `auth.go`). It connects with `config.Load()` (so it reads
`DATABASE_URL`/`JWT_SECRET` from env exactly like the server) and it **does not run migrations** — it
assumes the schema exists (the backend binary auto-migrates on boot, `main.go:24`; CI boots the backend
before seeding, §3). If `schema_migrations` is empty/absent, the seed exits with "run the server (or
migrations) first".

### 1.2 Where credentials & base data live

**In the Go program, as exported constants**, and mirrored in `e2e/fixtures/seed.ts` (the single source
the read-only spec reads). Deterministic, printed to stdout on every run:

```
Demo family "Sharma Family"
  HEAD    Priya Sharma    head@demo.test    / demo1234
  MEMBER  Aarav Sharma    aarav@demo.test   / demo1234
  MEMBER  Diya Sharma     diya@demo.test    / demo1234
```

- Passwords are the literal `demo1234` (≥6, satisfies register/change-password `min=6`); bcrypt-hashed on
  insert via the same cost the app uses. **These are demo credentials only** — the seed refuses to run
  unless the DB-name/`--reset` guard passes (§0.5), so they never reach a real deployment.
- IDs are **not** hard-coded UUIDs (the schema generates them); instead the seed is deterministic **by
  role/email**, and both the seed's stdout summary and the machine-readable summary file (§1.6) expose the
  generated IDs for any consumer that needs them. The e2e keys on **email/name/role via testIDs**, not on
  raw UUIDs, so it stays stable across runs.

### 1.3 The demo ledger — what gets created (covers every entry type + the dashboard/loan math)

All amounts are **minor units (paise)**; the seed computes "current period" `P0 = YYYY-MM` from the server
clock **once** and derives prior periods `P-1, P-2, P-3` from it via `posting.AddMonths(P0, -k)`.

**Members & backdating.** Create the group (head = Priya), then `AddMember` Aarav & Diya as `member`.
Immediately `UPDATE group_members SET joined_at = <P-3 start>` for both kids (and the head), so allowance
backfill floors at 3 months ago, not `now()`. (Project memory: backfill tests must backdate `joined_at`,
not just `effective_from`.)

**Chores.** The group gets its system **"Settlement"** chore (via `ChoreRepo.CreateWithSystem(..., true)`
— matching how a real group is provisioned) plus two earnable chores: "Wash dishes" (₹20 = 2000) and
"Walk the dog" (₹50 = 5000).

**Allowances.** Set Aarav ₹500/mo (50000) `effective_from P-3`; Diya ₹300/mo (30000) `effective_from
P-3`, then a **change** to ₹400/mo (40000) `effective_from P-1` (exercises allowance history / "new row,
past months unaffected", §5.2). Head has no allowance.

**Loan.** Aarav has an **active** loan: principal ₹1200 (120000), 3 installments →
`emi = ceil(120000/3) = 40000`, `start_period = P-2`. Create it `status=active` (pre-approved by head)
via `LoanRepo.Create(..., LoanStatusActive, &Pminus2, ...)`. (Also create one **rejected** loan for Diya —
principal ₹5000, `status=rejected` — so the loans tab has a terminal row and the read-only assertions can
check rejected rendering.)

**Machine postings — run `posting.PostDue(group, now)`.** This is the load-bearing step: it posts, into
the ledger, all due `allowance` credits (Aarav: 500×? and Diya: 300 then 400 per the effective_from
schedule, from join month P-3 through P0) and all due `emi` debits for Aarav's loan (3 EMIs across
P-2..P0, then the engine marks the loan `closed` once all 3 are posted — **note:** with `start_period=P-2`
and 3 installments, periods P-2, P-1, P0 are all due at seed time, so the loan **closes**; if the demo
wants a *still-active* loan visible with outstanding > 0, set `installments = 6` so only 3 of 6 have
posted and outstanding = 120000 − 3×EMI remains — **the seed uses 6 installments so the loan stays
`active` with visible outstanding**, exercising the WP-3.2 progress card). EMI rounding with 6
installments: `ceil(120000/6)=20000`, last installment absorbs remainder (all equal here). PostDue is
idempotent (unique partial index), so re-running the seed after a reset re-posts cleanly.

**Manual / status-variety rows (direct SQL with backdated `created_at`, written through a small helper
that still sets `entry_type`/`direction`/`status`/`created_by` exactly like `LedgerRepo.Create` but
allows an explicit `created_at`):**
- **Manual credit — `adjustment` (credit):** head gives Aarav a ₹100 (10000) bonus, `note="Good grades
  bonus"`, `status=approved`, `created_at` in P-1. (Covers `adjustment` credit.)
- **Manual debit — `settlement`:** head pays out Diya ₹200 (20000) against the system chore,
  `entry_type=settlement`, `direction=debit`, `status=approved`, `created_at` in P-1. (Covers settlement
  and gives Diya a debit so her balance math is non-trivial.)
- **Member chore — approved:** Aarav submitted "Wash dishes" (2000), head **approved** (`decided_by=head`,
  `decided_at` set), `created_at` P-1.
- **Member chore — pending:** Diya submitted "Walk the dog" (5000), `status=pending_approval`
  (`decided_by` NULL) — so the head's approve/reject UI has a live pending row and the dashboard math
  excludes it.
- **Member chore — rejected:** Aarav submitted "Walk the dog" (5000), `status=rejected`
  (`decided_by=head`, `decided_at` set) — so the ledger renders a strikethrough row and it is excluded
  from balance.

**Balances that result (the read-only e2e asserts these):** the seed **does not hand-write balances** — it
computes the *expected* values by the §5.1 formula (Σ approved credit − Σ approved debit) from the rows it
inserted **for its own assertion/log only**, and prints them. The dashboard/`GET /groups` and
`/balance` endpoints must independently reproduce the same numbers when the e2e opens the group (proving
end-to-end math). The seed's printed expectations become the read-only spec's assertion constants
(pulled from the machine-readable summary, §1.6), so there is exactly one place the numbers live.

### 1.4 Invariants the seed MUST honor (review checklist)

1. **Money is int64 paise** at every layer — never a float, never a rupee string. (§10 rule 6)
2. **`joined_at` is backdated by explicit `UPDATE`** after `AddMember` (which stamps `now()`), so
   `PostDue`'s join-month floor produces the intended prior-month allowance rows. (Project memory:
   `project_backfill_tests_backdate_joined_at`.)
3. **The head is added as a member with `RoleHead`** — `GroupRepo.Create` does **not** add the head
   (project memory: `project_integration_test_head_membership`); the seed calls `AddMember(head,
   RoleHead)` explicitly, then the two kids as `RoleMember`.
4. **Machine entries (`allowance`/`emi`) come only from `PostDue`** — never hand-inserted — so the unique
   partial posting index, EMI schedule, and auto-close all behave exactly as in production and re-seed is
   idempotent.
5. **Entry immutability respected** — the seed only *inserts* rows; it never updates an existing ledger
   row's amount/type (status is set at insert; the pending/rejected rows are inserted in their terminal or
   pending state directly, which is a fresh insert, not a mutation).
6. **Direction rules (§5.1):** `chore`/`allowance`/positive `adjustment` → credit; `emi`/`settlement`/
   negative `adjustment` → debit; amounts stored positive. The seed's manual-row helper asserts the
   direction matches the type before inserting.
7. **No posting on `GET /groups`** — the seed itself calls `PostDue`; nothing in the seed relies on a read
   endpoint to post. (Mirrors WP-4.2 D1; the read-only e2e must **open a group** to trigger any newly-due
   posting, and the seed is authored so nothing is left un-posted at P0 — §0.4/G4.)

### 1.5 Reset & guard (per D5/§0.5)

`--reset` flag / `SEED_RESET=1`; DB-name allowlist. On proceed: one transaction — `TRUNCATE … CASCADE`
the demo table set (never `schema_migrations`), then build. On an unrecognized DB name without the flag:
exit non-zero with the loud refusal message. Idempotent by construction (reset every run → identical
state).

### 1.6 Machine-readable summary (so the e2e has one source of truth)

After a successful seed, write a JSON summary to a path given by `--out` (default
`e2e/fixtures/seed.summary.json`, git-ignored) containing: `current_period`, the demo credentials (email/
role/name), the generated group id + user ids, and the **expected balances per member** and **loan
outstanding**. The read-only spec loads this file instead of hard-coding numbers, so a deliberate change
to the demo (e.g. a different bonus amount) updates tests and fixture in one edit. Also `--out=-` prints
the JSON to stdout (for `make seed` logs). The human-readable table (§1.2) is always printed too.

### 1.7 Make targets

Root `Makefile` (add, next to the backup targets):
```make
seed: ## Seed the demo family into the compose DB (DESTRUCTIVE reset). Usage: make seed
	$(COMPOSE) exec -T $(PG_SERVICE) true 2>/dev/null || { echo "start the DB first: make -C backend dev-up"; exit 1; }
	cd backend && DATABASE_URL="$(SEED_DATABASE_URL)" JWT_SECRET="$(SEED_JWT_SECRET)" go run ./cmd/seed --reset
```
with `SEED_DATABASE_URL ?= postgres://pocket:pocket@localhost:5432/pocket_money?sslmode=disable` and
`SEED_JWT_SECRET ?= dev-secret-key` defaults (overridable). Backend `Makefile` gets a thin `seed:` target
(`go run ./cmd/seed`) for running against an already-exported env. CI calls the binary directly with its
own env (§3). Document in README that `make seed` is destructive and demo-only.

---

## 2. E2E — user journeys

Two Playwright browser **contexts** are used where a journey needs two accounts simultaneously (head H
and member M) — Playwright's isolated contexts model this cleanly without logout/login churn. Each
mutating spec **creates its own fresh users/group** (unique emails, §0.4) so specs are independent and
order-free; the read-only spec asserts against the **pre-seeded** family.

### 2.1 MUST-HAVE journeys (the launch-bar automation — these gate the WP)

**T1 — Register auto-login (WP-4.6 D1).**
Fresh user → open app (redirects to login) → tap **Register** → submit name/email/password →
**lands directly in the app on the Dashboard** (assert a dashboard testID is visible; assert the login
screen is NOT shown). No second login step. Dashboard shows the empty state "No groups yet".

**T2 — Head first-run: create group → invite.**
As the T1 user (now head): **Create Group** "Test Fam" → lands in group Overview (head view) → members
empty state visible → tap **Invite** → the app calls `POST /groups/:id/invite`; the test **captures the
token** by listening on that response (`page.waitForResponse`), not the clipboard (§4.3). Assert a success
toast/testID appears. Go to **Chores** tab → add a chore ("Wash dishes", ₹20) → it appears in the list.

**T3 — Register-VIA-INVITE new member (WP-4.6 §7 register-path resume — the headline journey).**
In a **second context** (logged out), navigate to `${webBase}/invite?token=<captured from T2>` →
redirected to login → tap **Register** → register a *new* member M → **lands INSIDE the joined group**
(member Overview), NOT on the dashboard, NOT stranded on login. This is the exact WP-4.6 §5.3 / §10-T3
assertion: the register success handler consumed the pending invite token and re-entered `/invite`. If M
lands on the dashboard instead, the WP-4.6 §12 G1 AuthGate race regressed — the test **fails** (do not
soften it).

**T4 — Ledger: add entries → approve/reject.**
- As M (member context): on member Overview tap **Log a chore** → pick the chore → submit → a **pending**
  entry appears in M's ledger.
- Add a second pending chore entry (so both approve and reject can be exercised).
- As H (head context): open the group → member detail for M (or Overview) → the pending entries show
  inline **Approve/Reject** → **Approve** the first (assert it flips to approved, M's balance updates) →
  **Reject** the second (assert strikethrough / rejected badge, balance unchanged).

**T5 — Allowance config visible.**
As H: on M's member detail, open the allowance sheet → set M's allowance (e.g. ₹300) → save → assert the
allowance value renders on the member summary. (Setting allowance is a config write; asserting the posted
allowance *ledger row* is nice-to-have since it depends on the current period — see T-RO.)

**T6 — Loan request → approve → EMI visible → member detail.**
- As M: Loans tab → **Request Loan** (principal, installments, note) → assert a `requested` loan card
  appears.
- As H: Loans tab → the requested loan shows **Approve** → approve → assert the loan becomes `active` with
  a progress/outstanding card (WP-3.2). Open the group Overview / M's ledger → assert the first **EMI**
  debit row is visible (approving sets `start_period`; opening the group triggers `PostDue`, which posts
  the due EMI — the test opens the group *after* approval so the posting runs).
- **Member detail:** as H, open `members/[userId]` for M → assert balance, the loan, and the ledger with
  the EMI row are all present (WP-3.3 surface).

**T-CP — Change password, 403 path does NOT log out (WP-4.7 D1/G1).**
As a fresh registered user on **Profile** → **Change password** sheet → submit with a **wrong current
password** → assert an **inline/toast error** ("current password is incorrect") **and that the user is
STILL logged in** (Profile still visible; NOT bounced to login — this is the 403-not-401 guarantee). Then
submit correctly → success toast, sheet closes. Then (optional within the spec) log out and log in with
the **new** password to prove it took.

**T-LEAVE — Leave blocked by nonzero balance (409 toast) → settle → leave succeeds (WP-4.7 D2–D7).**
Set up a member M with a **non-zero balance** (H approves a chore credit for M, or the test uses a member
who is owed money). As M: group Overview member view → **Leave group** → confirm → assert a **danger toast
with the 409 reason** ("…balance is not settled to zero…") **and** that M is **still in the group**
(Overview still shown). Then as H: **settle** M to ₹0 (record a settlement debit equal to the balance
via M's member detail "Add entry"). Back as M: **Leave group** → confirm → assert success toast and that
M lands on the **dashboard** with the group **gone from the list**. (Exercises the full WP-4.7 leave gate
+ the PostDue-before-check correctness only indirectly; the direct D3 case is a backend integration test
in WP-4.7 §6.14, not repeated here.)

### 2.2 NICE-TO-HAVE journeys (add if green & within the time budget; not blocking)

- **T-RO — Read-only seeded-family assertions** (`seeded-readonly.spec.ts`): log in as the **seeded** head
  → dashboard shows "Sharma Family" with the expected **total owed** (from `seed.summary.json`) and
  member count 3 → open the group → assert each member's balance matches the summary, the **active loan
  outstanding** matches, and the ledger shows rows of every type (allowance, emi, chore approved/pending/
  rejected, settlement, adjustment). Log in as the seeded member Aarav → dashboard shows **his own**
  balance only (privacy). This is the richest math check but depends on the seed running first in the job.
- **Empty-state copy assertions** (WP-4.6 §6): assert the chores **member** empty state is no longer bare
  (E5), and that the dashboard/overview empty states read their spec'd copy. Text-based, mildly brittle →
  nice-to-have.
- **Invite copy/toast on web** (WP-4.3): assert the success toast after Invite. (The clipboard *content*
  and the non-secure-http fallback Sheet are device/browser-context-specific — assert the toast, not the
  OS clipboard.)
- **Rejected-loan rendering** from the seed (Diya's rejected loan).

### 2.3 Explicitly NOT automated (manual reviewer guidance, list in PR)

- Native **OS share sheet** (RN `Share`) and **`pocketmoney://` deep link** (WP-4.3 §9 items 2–3) —
  device-only.
- Non-secure-**http** clipboard fallback Sheet (WP-4.3 G3) — browser-context/permission-specific.
- iOS/Android SecureStore token persistence — native-only.

These are covered by WP-4.3/4.6/4.7's own manual walkthroughs; WP-4.4 does not re-own them.

### 2.4 D5 — Chromium only in CI (WebKit/Firefox off by default)

CI runs **Chromium** (fastest install, representative of RN-Web + the primary browser onboarding path).
The Playwright config *defines* a `chromium` project; WebKit/Firefox projects are present but **commented/
guarded behind an env flag** so a maintainer can run cross-browser locally without paying the install/run
cost on every CI run. Rationale: the app is RN-Web (behaves consistently across engines for these flows);
the marginal flake/time of 3× browsers isn't worth it for a launch gate. Documented in the config.

---

## 3. CI wiring — new `e2e` job in `.github/workflows/ci.yml`

Add one job (does **not** replace `fe-web-export`; that stays as the fast bundle-smoke gate). It reuses
the existing Postgres-service pattern from the `test` job.

```yaml
  e2e:
    name: E2E (web)
    runs-on: ubuntu-latest
    needs: [build, fe-web-export]      # only run e2e once BE builds and the bundle smoke passes
    env:
      TZ: UTC                           # server clock authority (§0.4) — pin period math
    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_USER: pocket
          POSTGRES_PASSWORD: pocket
          POSTGRES_DB: pocket_money
        ports: ['5432:5432']
        options: >-
          --health-cmd pg_isready --health-interval 10s --health-timeout 5s --health-retries 5
    steps:
      - uses: actions/checkout@v4

      # --- Backend: build, boot (auto-migrates), seed ---
      - uses: actions/setup-go@v5
        with: { go-version: '1.24', cache-dependency-path: backend/go.sum }
      - name: Build backend
        working-directory: ./backend
        run: go build -o bin/server ./cmd/server
      - name: Start backend (auto-runs migrations on boot)
        working-directory: ./backend
        env:
          DATABASE_URL: postgres://pocket:pocket@localhost:5432/pocket_money?sslmode=disable
          JWT_SECRET: ci-e2e-secret
          PORT: '8080'
          CORS_ORIGINS: '*'
          APP_BASE_URL: http://localhost:8081     # invite_url points at the web origin (WP-4.3 §3)
        run: |
          ./bin/server & echo $! > /tmp/server.pid
          for i in $(seq 1 30); do curl -sf http://localhost:8080/health && break; sleep 1; done
          curl -sf http://localhost:8080/health || { echo "backend did not become healthy"; exit 1; }
      - name: Seed demo family
        working-directory: ./backend
        env:
          DATABASE_URL: postgres://pocket:pocket@localhost:5432/pocket_money?sslmode=disable
          JWT_SECRET: ci-e2e-secret
        run: go run ./cmd/seed --reset --out ../e2e/fixtures/seed.summary.json

      # --- Frontend: export the web bundle pointed at the CI backend ---
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: 'npm', cache-dependency-path: app/package-lock.json }
      - name: Install app deps
        working-directory: ./app
        run: npm ci
      - name: Export web bundle
        working-directory: ./app
        env:
          EXPO_PUBLIC_API_URL: http://localhost:8080/api/v1   # inlined at export time (api.ts BASE_URL)
        run: npx expo export --platform web   # → app/dist

      # --- E2E: install Playwright, serve dist, run tests ---
      - name: Install e2e deps
        working-directory: ./e2e
        run: npm ci
      - name: Install Playwright browsers (chromium)
        working-directory: ./e2e
        run: npx playwright install --with-deps chromium
      - name: Run Playwright
        working-directory: ./e2e
        env:
          E2E_WEB_BASE: http://localhost:8081
          E2E_API_BASE: http://localhost:8080/api/v1
        run: npx playwright test      # playwright.config webServer serves ../app/dist on :8081 (SPA)

      - name: Upload Playwright report & traces
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: |
            e2e/playwright-report
            e2e/test-results
          retention-days: 7

      - name: Stop backend
        if: always()
        run: kill "$(cat /tmp/server.pid)" 2>/dev/null || true
```

Notes:
- **Migrations** run automatically when the backend binary boots (`main.go:24` → `db.RunMigrations`), so
  no separate migrate step. The health-poll gates readiness.
- **The static server** for `app/dist` is launched by **Playwright's `webServer`** config (running `serve
  -s ../app/dist -l 8081`), so Playwright owns its lifecycle and waits for it. `-s` gives SPA fallback so
  `/invite?token=` resolves to the client router (the export uses `web.output: "single"`, app.json).
- **`EXPO_PUBLIC_API_URL`** is baked into the exported bundle so the browser calls the CI backend on
  `:8080`; `APP_BASE_URL` makes generated invite links point at the web origin `:8081` (so T3's
  `/invite?token=` opens the app, not the API).
- **Artifacts on failure only** (trace/screenshot/video via the config's `trace: 'on-first-retry'`,
  `screenshot: 'only-on-failure'`, `video: 'retain-on-failure'`) — keeps green runs cheap.

**Runtime budget.** Target **≤ ~10 min** wall-clock for the job: Go build ~1m (cached), Postgres ready
<30s, seed <10s, `npm ci` (app) ~1–2m (cached), export ~1–2m, Playwright browser install ~1m
(cached via `~/.cache/ms-playwright` — add a cache step if it proves slow), `npm ci` (e2e) ~30s, test run
~2–3m for the must-have set on one worker. It runs **after** `build`+`fe-web-export` (fast fail on
compile/bundle first), in parallel with nothing that blocks it. If the suite grows past budget, shard the
mutating specs across 2 workers (`--workers=2`) — they're independent by design (§0.4).

---

## 4. Flake discipline & Playwright config

### 4.1 Config (`e2e/playwright.config.ts`)
- `testDir: './tests'`, `fullyParallel: true` for the read-only spec but **mutating specs are independent**
  (own users) so parallel is safe; `retries: process.env.CI ? 1 : 0` (one retry masks genuine transient
  network flake without hiding real failures — a test that only passes on retry is surfaced in the report).
- `use: { baseURL: process.env.E2E_WEB_BASE, trace: 'on-first-retry', screenshot: 'only-on-failure',
  video: 'retain-on-failure' }`.
- `webServer: { command: 'npx serve -s ../app/dist -l 8081', url: 'http://localhost:8081', reuseExisting
  Server: !process.env.CI, timeout: 120_000 }`.
- `projects: [{ name: 'chromium', use: devices['Desktop Chrome'] }]` (+ commented webkit/firefox behind
  `E2E_ALL_BROWSERS`, D5).
- `reporter: [['html', { open: 'never' }], ['list']]`.

### 4.2 Selector rules (bind to §5 testIDs)
Page objects in `support/pages.ts` expose typed helpers (`login(email,pw)`, `register(...)`,
`createGroup(name)`, `logChore(name)`, `approveEntry(...)`, etc.) keyed **only** on `data-testid`
(from RN `testID`) and `getByRole`. No CSS selectors, no `nth`, no XPath, no text-matching for actions
(text only for content assertions, and prefer testIDs even there where the string is copy that 4.6 might
tweak).

### 4.3 Two-account journeys & token capture
- Use two `browser.newContext()` for H and M in the same spec (T3/T4/T6/T-LEAVE) — isolated storage, no
  login thrash.
- Capture the invite token via `const resp = await page.waitForResponse(r =>
  r.url().includes('/groups/') && r.url().endsWith('/invite') && r.request().method()==='POST'); const {
  token } = await resp.json();` — deterministic, no clipboard dependency (clipboard is browser-permission-
  gated and the whole point of WP-4.3's fallback is that it *isn't* reliable).

### 4.4 No sleeps
Every wait is a web-first assertion (`await expect(page.getByTestId('dashboard-root')).toBeVisible()`).
The only network-settle is asserting on the resulting DOM/toast, never `page.waitForTimeout`.

### 4.5 `e2e/package.json` (exact deps)
```json
{
  "name": "pocket-money-e2e",
  "private": true,
  "scripts": { "test": "playwright test", "report": "playwright show-report" },
  "devDependencies": {
    "@playwright/test": "1.49.1",
    "serve": "14.2.4",
    "typescript": "5.9.2"
  }
}
```
Pin exact versions (no `^`) for CI reproducibility. `@playwright/test` 1.49.x is current-stable and
Node-22 compatible; `serve` 14.x is the maintained static server with `-s` SPA fallback. Commit
`e2e/package-lock.json`. **Nothing here enters `app/` or `backend/`.**

---

## 5. testIDs — the ONLY app-code change (bounded, additive, behavior-neutral)

`grep -rn 'testID' app` currently returns **0** — no screen is instrumented. This WP adds `testID`
props (which RN-Web renders as `data-testid`) to the **minimum set** of elements the must-have journeys
touch. **This is the entire app-code footprint of WP-4.4.** Each addition is a single prop, no layout/
style/behavior change. RN maps `testID` → web `data-testid` automatically; for interactive controls also
pass `accessibilityLabel` where it aids `getByRole` (additive).

### 5.1 Bounded file-touch list (screens)

| # | File | testIDs to add (illustrative names — keep a consistent `screen-element` scheme) |
|---|---|---|
| 1 | `app/app/(auth)/login.tsx` | `login-email`, `login-password`, `login-submit`, `login-link-register` |
| 2 | `app/app/(auth)/register.tsx` | `register-name`, `register-email`, `register-password`, `register-confirm`, `register-submit` |
| 3 | `app/app/(app)/index.tsx` (dashboard) | `dashboard-root`, `dashboard-create-group`, `dashboard-join-group`, `group-card-<id>` (via a `testID={\`group-card-${item.id}\`}`), `dashboard-empty` |
| 4 | `app/app/(app)/groups/create.tsx` | `create-group-name`, `create-group-submit` |
| 5 | `app/app/(app)/groups/[id]/index.tsx` (overview) | `group-overview-root`, `group-invite-button`, `group-leave-button` (member branch, WP-4.7), `members-empty`, `member-card-<userId>` |
| 6 | `app/app/(app)/groups/[id]/chores.tsx` | `chores-root`, `chores-add-button`, `chores-empty`, `chore-row-<id>` |
| 7 | `app/app/(app)/groups/[id]/loans.tsx` | `loans-root`, `loans-request-button`, `loan-card-<id>`, `loan-approve-<id>`, `loans-empty` |
| 8 | `app/app/(app)/groups/[id]/members/[userId].tsx` | `member-detail-root`, `member-remove-button`, `member-add-entry-button`, `member-allowance-button` |
| 9 | `app/app/(app)/profile.tsx` | `profile-root`, `profile-change-password`, `profile-logout` |
| 10 | `app/app/invite.tsx` | `invite-loading`, `invite-error`, `invite-back` |

### 5.2 Bounded kit-component touch list (only if the prop doesn't already flow through)

Interactive kit components used by the journeys need to **forward** a `testID`. Add a `testID?: string`
prop that is passed to the underlying RN element **only if it isn't already accepted** — a pure
passthrough, default `undefined`, zero render change:

| Component | Element the journeys click/read |
|---|---|
| `src/components/Button.tsx` | the `Pressable`/`TouchableOpacity` — submit/approve/reject/confirm buttons |
| `src/components/TextField.tsx` | the `TextInput` — all form fields |
| `src/components/Sheet.tsx` | the sheet container + its primary action (change-password, add-entry, allowance, request-loan) |
| `src/components/Toast.tsx` | the toast container (`toast-root`) so success/error toasts are assertable |
| `src/components/LedgerRow.tsx` | the row + its Approve/Reject controls (`ledger-approve`, `ledger-reject`, status badge) |
| `src/components/StatusBadge.tsx` | badge text is already assertable via `getByText`; add `testID` only if needed |

Rule: prefer adding the `testID` at the **screen** call-site (pass `testID="login-submit"` to `<Button>`)
rather than hard-coding it inside the component — the component only needs to **accept & forward** the
prop. If a component already forwards `testID` (some RN wrappers do), no edit is needed — **verify per
component; do not add a duplicate prop.**

### 5.3 Discipline
- **Additive only:** every diff hunk in `app/**` is a new `testID=`/`accessibilityLabel=` prop or a
  one-line `testID?: string` prop + passthrough. **No logic, no JSX restructure, no style change.** A
  reviewer must be able to confirm "removes nothing, changes no behavior".
- **No `testID` on list-item text that carries money/copy** where a `getByText` on the value is the real
  assertion — instrument the **container** and assert the value with `getByText`/`toContainText`.
- Keep the naming scheme `kebab: screen-element` (and `screen-element-<id>` for list items) consistent so
  page objects stay predictable.

---

## 6. "docker-compose runs full stack with seed" (D6) — one-command bring-up

The acceptance line wants a single command to bring up BE + DB + web with seed data. **Do not restructure
`docker-compose.yml`'s services** (Postgres + backend already exist; the Expo web bundle is a static
artifact, not a long-running service worth adding to the family deployment compose). Instead provide a
documented **make target** that composes the existing pieces:

```make
demo-up: ## Bring up DB+backend (compose) and seed the demo family. Then run the web app separately.
	$(COMPOSE) up -d postgres backend
	@echo "waiting for backend health…"; for i in $$(seq 1 30); do curl -sf http://localhost:8080/health && break; sleep 1; done
	$(MAKE) seed
	@echo "Backend+DB up on :8080 and seeded. Start the web app with: (cd app && EXPO_PUBLIC_API_URL=http://localhost:8080/api/v1 npm run web)"
```

Rationale: the backend container already waits on Postgres health and auto-migrates on boot; `demo-up`
just adds the seed step and prints the web command. Running the Expo **web dev server** inside compose is
not worth a new service image for a demo (the dev host runs `npm run web`); the README documents both the
dev-server path (`npm run web`) and, for a fully-static demo, the `expo export` + `serve dist` path the CI
uses. **This keeps the compose file's production-shaped topology intact** while satisfying the
one-command-bring-up intent. If a reviewer insists on a single `docker compose --profile demo up`, that is
an optional follow-up (a `web` service running `serve` over a build step) — note it in `docs/notes/`, do
not expand this WP's compose surface.

---

## 7. Verification (Definition of Done, §10 rule 3)

Because this WP is docker-less locally (no Postgres — project constraint), the **full seed + e2e runs
only in CI**. Locally the implementer verifies everything that does not need Postgres/a browser, and
states results in the PR:

### Backend (seed compiles & is well-formed)
```bash
cd backend
gofmt -l ./cmd/seed ./internal      # prints NOTHING
go build ./...                       # seed compiles (incl. cmd/seed)
go vet ./...                          # clean
go test -race ./...                   # existing unit tests still pass (seed adds none required; a pure
                                      # helper — e.g. the manual-row direction check — MAY get a unit test)
# The seed's real exercise needs Postgres → CI. Locally, prove it builds and the --help/guard path works:
go run ./cmd/seed --help              # prints usage incl. the destructive-reset warning
```

### E2E project (typechecks & config loads; browser run is CI-only)
```bash
cd e2e
npm ci
npx tsc --noEmit                      # specs + config typecheck
npx playwright test --list            # lists all tests without running (proves config + specs load)
```

### Frontend (the testID sweep changed nothing behaviorally)
```bash
cd app
npx tsc --noEmit                      # zero type errors (testID props are typed)
npx expo export --platform web        # MANDATORY — must exit 0 (the e2e runs against THIS output)
npm run lint                          # no new issues
git diff --stat app/                  # every app/ hunk is an additive testID/accessibility prop (review it)
```

### CI (the real gate)
- The new `e2e` job is green: seed runs, bundle exports, Playwright must-have journeys pass.
- On a deliberately-broken run (local experiment), confirm the failure artifact (trace/screenshot/video)
  uploads — state in the PR that the artifact path is wired.

State all results in the PR. The must-have journeys (§2.1) are the acceptance evidence; the read-only and
nice-to-have specs are reported as "green" or "deferred with reason".

---

## 8. Acceptance criteria (Definition of Done)

1. **Seed:** `backend/cmd/seed` + `make seed` build a deterministic demo family (head + 2 members, one
   group, system+2 chores) with a ledger covering **every entry type** — manual `adjustment` credit,
   `settlement` debit, machine `allowance` (via `PostDue`), `emi` (via `PostDue`, loan still `active`
   with visible outstanding), and member `chore` entries in **approved / pending / rejected** states —
   plus a `rejected` loan. Money is **int64 paise** throughout; `joined_at` is backdated by explicit
   `UPDATE`; the head is added with `RoleHead`; machine entries come only from `PostDue`. Re-running
   `make seed` yields identical state (destructive-reset idempotency), guarded by the DB-name/`--reset`
   safety gate. A `seed.summary.json` records current period + credentials + ids + expected balances.
2. **E2E:** a top-level `e2e/` Playwright project (own `package.json`, exact-pinned `@playwright/test` +
   `serve`; **nothing added to `app/`**) drives the exported web bundle against the seeded CI backend and
   passes the **must-have** journeys T1–T6, T-CP, T-LEAVE (§2.1), including the WP-4.6 **register-via-
   invite** resume (T3) and the WP-4.7 **change-password-403-no-logout** (T-CP) and **leave-blocked-409 →
   settle → leave-succeeds** (T-LEAVE). Nice-to-have journeys are green or explicitly deferred.
3. **CI:** one new `e2e` job (Postgres service → backend boot+migrate → seed → export → serve → Playwright)
   is green; traces/screenshots/videos upload as artifacts **on failure**; the job stays within the
   ~10-min budget and runs after `build`+`fe-web-export`.
4. **testIDs:** the bounded list (§5) is added as **additive, behavior-neutral** props — the ONLY app-code
   change; `git diff app/` shows no logic/layout/style change; `tsc`, `lint`, and `expo export` all pass.
5. **No drift:** no `openapi.yaml` edit, no codegen, **no migration**, no new `app/` runtime dependency, no
   compose-service topology change. Selectors are testID/role-based; no `waitForTimeout`.
6. **Concurrency honored:** the implementer confirmed WP-4.6 and WP-4.7 are merged before starting, and the
   journeys assert their real post-merge behaviors (§0.1).
7. Commit/PR reference the WP id (`WP-4.4: e2e tests + seed data`).

---

## 9. Under-specified-by-the-master-plan decisions this WP resolves (no open questions left)

| Question the plan left open | Decision (with §) |
|---|---|
| Maestro or Playwright? | **Playwright/web** — reuses the existing export artifact, Linux-native, covers the browser onboarding path (D1, §0.2). Native flows = manual (§2.3). |
| Where does the e2e tooling live? | **New top-level `e2e/`** with its own package — keeps `app/` runtime deps clean (D3, §0.3). |
| Seed mechanism — SQL, API, or Go? | **Go command reusing repos + `PostDue`** — honors invariants a `.sql` seed would bypass and produces cross-month history an API seed cannot (§1.1). |
| Seed idempotency — reset or no-op? | **Destructive reset every run, guarded** — a fixture must be deterministic; entry immutability forbids upsert (D5, §0.5). |
| What runs against Postgres locally vs CI? | **Nothing needing Postgres runs locally** (no Docker on the dev host); seed + e2e execute **only in CI**; locally: build/typecheck/`--list`/export (§7). |
| How does the browser reach the backend / how do invite links point right? | **`EXPO_PUBLIC_API_URL`** baked at export → browser calls `:8080`; **`APP_BASE_URL`** → invite links point at web `:8081` (§3). |
| How is the two-user invite journey driven? | **Two Playwright contexts + `waitForResponse` token capture** — no clipboard dependency (§4.3). |
| Which browsers in CI? | **Chromium only**; cross-browser behind an env flag (D5, §2.4). |
| "docker-compose full stack with seed"? | **`make demo-up`** layering seed on the existing compose services — no service-topology change (D6, §6). |
| Clock/period flake? | **`TZ=UTC` in the job**, seed computes `current_period` once and publishes it to `seed.summary.json`; assertions read it (§0.4). |

---

## 10. Gotchas & likely-slips (quick reference)

- **G1 — Runs LAST.** Do not start until **both** WP-4.6 and WP-4.7 are merged; the headline journeys
  (T3 register-via-invite, T-CP 403-no-logout, T-LEAVE 409) *are* their behaviors. Verify the merge state
  first (§0.1).
- **G2 — Seed must NOT bypass invariants.** Machine `allowance`/`emi` rows come from `PostDue`, never
  hand-inserted; `joined_at` is backdated by `UPDATE` (AddMember stamps `now()`); head added `RoleHead`;
  money int64. This is the review's #1 check (§1.4).
- **G3 — Loan installments = 6 (not 3)** so the demo loan stays **`active` with outstanding > 0** after
  `PostDue` posts the 3 due EMIs — a 3-installment loan would auto-**close** and the WP-3.2 progress card
  wouldn't show an active loan (§1.3).
- **G4 — The dashboard summary does NOT post (WP-4.2 D1).** The read-only e2e must **open a group** to
  trigger any due posting before asserting a live balance; author the seed so nothing is left un-posted at
  P0 so the dashboard total already matches (§1.4.7 / §0.4).
- **G5 — testID is the ONLY app-code change**, additive and behavior-neutral. No logic/layout edits. Kit
  components only *forward* a `testID` prop if they don't already (§5.3). A reviewer confirms the app diff
  removes nothing.
- **G6 — Don't put Playwright/serve in `app/package.json`.** They live in `e2e/` (D3). Adding them to the
  app pollutes the Expo bundle/app-store build.
- **G7 — Capture the invite token from the response, not the clipboard.** Clipboard is permission-gated
  and its non-secure-http fallback is exactly why WP-4.3 has a fallback Sheet — unreliable for assertions
  (§4.3).
- **G8 — `web.output: "single"` needs SPA fallback.** Serve `dist` with `serve -s` (or the config's
  `webServer`) so `/invite?token=` resolves to the client router, not a 404 (§3/§4.1).
- **G9 — `EXPO_PUBLIC_API_URL` is inlined at export time**, not runtime — it must be set on the `expo
  export` step, not the serve step (`api.ts` reads it via `process.env` at bundle time) (§3).
- **G10 — Destructive-reset guard.** The seed refuses to truncate an unrecognized DB without `--reset`;
  CI passes `--reset`. Never document `make seed` without the "DESTROYS data" warning (§0.5).
- **G11 — `TZ=UTC` in the CI job** and compute periods from the server clock; a naïve month-boundary run
  otherwise flakes. Publish `current_period` from the seed; assertions read it (§0.4).
- **G12 — No `waitForTimeout`.** Web-first assertions only; `retries: 1` in CI masks transient network
  flake but a retry-only pass is surfaced in the report, not hidden (§4.1).
- **G13 — Fresh users per mutating spec** with unique emails; the seeded family is **read-only** — never
  mutate it in a test, or re-runs drift and the unique-email constraint bites (§0.4).
</content>
</invoke>
