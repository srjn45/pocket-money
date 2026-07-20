# Pocket Money — Master Plan v3 (Product Reframe)

Status: ✅ COMPLETE (2026-07-20) — all §8 WPs (Phases 1–6) merged; CI fully
green including the single-image compose smoke and e2e golden path. Approved
product decisions D1–D9, roadmap in §8, agent conventions in §9–§10.
Supersedes the *product framing* of `master-plan.md` v2.1. The v2.1 build (all
phases merged, e2e green at `3a920bd`) remains the technical foundation; this
document redefined what the product is and specified the delta.

Final SHA: `c16af96` — the `autopilot/integration` head this WP lands on. The
true final SHA is the V3-6.3 merge commit (unknown at implement time); the owner
fast-forwards `master` to the landed integration head, and the brain confirms
the merge SHA post-land.

---

## 1. What this product is

**Family payroll.** The paying member of a household (the **admin**) keeps books
on the pocket money they give family members. Each member's month:

```
payable(month) = base pocket money
              + chore earnings
              + one-off adjustments
              − loan EMI deduction
```

The app answers one question first: **"How much do I transfer to each person
this month, and is it settled?"** — with an auditable trail behind the number.

Corollaries that drive every design decision below:

- The **admin is the bookkeeper**; members are beneficiaries with a read-only
  view of their own money. All money movement happens outside the app (cash /
  bank transfer); the app only records it.
- The **month is the unit of account**. Running balance (arrears) is the
  carryover between months, not the headline number.
- **Membership is admin-driven.** The admin adds people by email; consent /
  registration by the member is optional and can come later.
- A **group = one payer's ledger book + its chore catalog**. One person may run
  several groups (kids vs spouse — different chores, different rates) and may
  simultaneously be a member of someone else's group.

## 2. Decisions locked (2026-07-10)

| # | Decision |
|---|---|
| D1 | Unpaid remainder **carries over** to next month (running-balance model). |
| D2 | Member chore submission is **hidden behind a group config flag**, default OFF. UI and endpoints inert when off. |
| D3 | Corrections are **edit-in-place** for *manual* entries only (chore credit, adjustment, clearing), with an invisible audit table recording prior values. System-generated entries (base credit, EMI debit) are never directly editable — edit the base amount / loan instead and the engine recomputes. Revisit append-only reversals only if multi-admin ever lands. |
| D4 | Notifications are **in-app only** for MVP (bell + unread badge). No email/push. |
| D5 | **Loan is a separate entity**, not a ledger entry type. Disbursement is off-ledger (money already changed hands in the real world). Only monthly EMI debits appear in the ledger. The loan record keeps its own repayment bookkeeping: principal, EMI, repaid, outstanding, schedule. Final installment = min(EMI, outstanding); loan auto-settles at zero. |
| D6 | **Visibility:** admin sees the whole group ledger (1-n). A member sees only their own rows (1-1) — enforced in the **backend**, not just the UI. Members never see other members' amounts or group totals. |
| D7 | **Currency (2026-07-10):** every group has a designated currency — `EUR`, `USD`, or `INR` — chosen at group creation and immutable. Every amount in the group (chores, base amounts, ledger, loans, statements) is in that currency. **No conversions, ever**; totals are never summed across groups. Amounts travel through the API as an object `{ "currency": "INR", "value": <int minor units> }`. |
| D8 | Invite-link **sharing is hidden for MVP, not removed** — backend endpoints and frontend code stay; UI entry points are hidden behind a build-level flag. Add-by-email is the sole visible membership mechanism. |
| D9 | User-facing word for a clearing is **"payment"** ("Record payment", "₹400 paid"). `clearing` remains the internal/API type name. |

## 3. Domain model

### 3.1 Users — registered and shadow

```
users
  id            uuid PK
  email         citext UNIQUE NOT NULL
  name          text NOT NULL            -- admin-supplied for shadows
  password_hash text NULL                -- NULL ⇔ shadow
  status        text NOT NULL            -- 'shadow' | 'registered'
  claimed_at    timestamptz NULL
```

- **Add-by-email lifecycle** (replaces the v2 invite-link acceptance flow):
  1. Admin enters an email (+ display name).
  2. Registered user exists with that email → attach to group, notify them
     (N-1 below).
  3. No user → create a **shadow user** (no password). Admin bookkeeps against
     them immediately: base amount, ledger, loans — all normal.
  4. Existing shadow (added earlier, possibly to another group) → just attach.
- **Claiming:** registration with a matching email upgrades the shadow row in
  place — set password, `status='registered'`, `claimed_at=now()`. Because the
  user id never changes, every membership, balance, loan, and passbook is
  already theirs. Admins of that user's groups are notified (N-2).
- **Security stance (MVP):** email is not verified, so claiming is trust-based.
  Mitigation: the claim notification to admins (N-2) + this is a family app.
  Real email verification is on the post-MVP roadmap; revisit before any
  public/multi-tenant deployment.

### 3.2 Groups & membership

```
groups
  id, name, created_at
  currency                         char(3) NOT NULL              -- D7: 'EUR'|'USD'|'INR', immutable
  member_chore_submission_enabled  bool NOT NULL DEFAULT false   -- D2

group_members
  group_id, user_id, role ('admin' | 'member'), joined_at
```

- Creator becomes `admin` (rename of v2 `head` — API + UI copy).
- No global roles; role is per-membership (a user can be admin here, member
  there).
- MVP keeps single-admin per group; schema doesn't forbid multiple (future).

### 3.3 Base pocket money

Reuses the v2 **allowances** subsystem unchanged in mechanics — it already has
exactly what D3/bookkeeping needs: per-member amount history
(`effective_from`), lazy monthly posting via `PostDue`, join-month flooring.

- Rename at the API/UI level: "allowance" → **"base amount"** (concept name:
  base pocket money). Table rename optional; do not churn migrations just for
  naming.
- Changing the base = new history row effective from month X. Never edits
  posted ledger rows retroactively (D3).

### 3.4 Chores catalog

```
chores: id, group_id, title, description (NEW), amount, is_system, active
```

- Admin-managed reference data. Per-group (different catalogs for kids vs
  spouse — see §1).
- The v2 "Settlement" system chore remains the internal vehicle for clearings.

### 3.5 Ledger

One append-ordered ledger per group; every row belongs to exactly one member.
Entry types:

| Type | Sign | Origin | Editable (D3) |
|---|---|---|---|
| `base_credit` | + | system (monthly posting) | no — edit base amount |
| `chore_credit` | + | admin picks member + chore | yes |
| `adjustment` | ± | admin one-off (bonus / deduction) | yes |
| `emi_debit` | − | system (monthly posting from loan) | no — edit loan |
| `clearing` | − | admin records real-world payment | yes |

- `entry_audit` table (id, entry_id, old_row jsonb, action, actor, at) written
  on every edit/delete of a manual entry. No UI in MVP.
- Members' ledger reads are filtered server-side to their own rows (D6).

### 3.6 Loans (separate entity — D5)

Keeps the v2 loans engine: principal, `emi_amount`, `start_period`, status;
monthly `emi_debit` posted lazily; final installment `min(emi, outstanding)`;
auto-settle at zero; early close. Additions:

- Loan detail exposes its own repayment record: schedule of posted EMIs,
  repaid-to-date, outstanding, projected end month.
- Disbursement never appears in the ledger (already true in v2 — confirmed
  correct behavior, keep).

### 3.7 Notifications (in-app, D4)

```
notifications: id, user_id, type, payload jsonb, read_at, created_at
```

MVP types:
- **N-1** `added_to_group` — to a registered user when an admin adds them.
- **N-2** `shadow_claimed` — to group admin(s) when a shadow member registers.
- **N-3** `payment_recorded` — to member when a clearing is recorded for them.
  (Cheap and high-value: "₹400 paid to you"; cut if scope demands.)

API: list + unread count + mark-read. UI: bell in the app header.

### 3.8 Money & currency (D7)

- **Storage is unchanged:** all amounts remain `int64` in **minor units**
  (paise / cents; all three supported currencies have exponent 2). The v2
  money core, `::bigint` casts, and posting arithmetic carry over as-is —
  only the *label* generalizes from "paise" to "minor units".
- **Single source of truth:** `groups.currency`. No per-chore / per-entry /
  per-loan currency column — everything under a group is definitionally in the
  group's currency, which is exactly the invariant D7 asks for.
- **Immutability:** currency is picked at group creation and can never change
  (no conversion support, so a change would corrupt the books).
- **API shape (breaking change):** every amount field becomes
  `{ "currency": "INR", "value": 12345 }` (value = minor units, never a
  float). Requests within a group may omit/`must-match` the currency — the
  server rejects any request whose currency differs from the group's.
  `openapi.yaml` + generated types updated in lockstep (codegen, never
  hand-edited).
- **No aggregation across currencies:** the dashboard shows per-group
  headlines each in their own currency; there is no combined total anywhere.
- **Formatting:** UI formats by currency (₹1.520,00 / €15.20 / $15.20 per
  locale-appropriate rules), replacing the hardcoded ₹ in `AmountText`.

### 3.9 Monthly statement (derived — the centerpiece)

No new tables. Per (group, member, month M), computed from the ledger after
`PostDue` materializes system entries:

```
opening_balance   = Σ all entries strictly before M        (D1 carryover)
earned(M)         = base_credit + chore_credits + adjustments − emi_debit in M
total_due(M)      = opening_balance + earned(M)
cleared(M)        = Σ clearings in M
closing(M)        = total_due(M) − cleared(M)               → next month's opening
```

Endpoint sketch: `GET /groups/{id}/statement?period=2026-07` → per-member rows
(admin) or the caller's single row (member), plus a group total for admins.

## 4. Permissions matrix

| Capability | Admin | Member | Shadow |
|---|---|---|---|
| Create group / add & remove members | ✅ | ❌ | — (no login) |
| Manage chores catalog, base amounts, loans | ✅ | ❌ | — |
| Add/edit/delete manual ledger entries | ✅ | ❌ | — |
| Record clearing (payment) | ✅ | ❌ | — |
| View statement / ledger | whole group | **own rows only** | — |
| View own loans & repayment record | ✅ (all members') | ✅ (own) | — |
| Submit chores | — | only if group flag ON (D2) | — |
| Profile, password, notifications | ✅ | ✅ | — |

## 5. UI — redesigned around the monthly question

Carries forward the shell fixes already diagnosed (2026-07-10 screenshots):
flatten the nested tab navigators (no double bottom bars), one meaningful
header per screen, responsive centered ~700px content column on web, theme.ts
spacing/typography pass, no clipped tab labels.

**All screens are one expo-router codebase shipping to three targets — web,
Android, iOS — and every UI WP owes all three.** Platform-adaptive rules:
bottom tab bar on native with safe-area insets (notch/home-indicator), top
tabs/segmented control inside a group everywhere; wide web gets the centered
column, native is full-bleed; touch targets ≥44pt; keyboard-avoiding forms on
native; no web-only APIs outside `Platform.select`/`.web.tsx` splits.
Automated verification of native is structurally limited (no emulator in CI),
so native acceptance = expo-doctor clean + the §8 Phase-6 device QA script;
web e2e remains the behavioral gate since the component tree is shared.

### Screen map

- **Dashboard** — "Groups you manage" and "Groups you're in" sections; per-group
  headline = this month's remaining-to-pay (admin) / to-receive (member), each
  in the group's own currency, never summed across groups (D7).
- **Create group** — name + **currency picker (EUR / USD / INR)** with copy
  making clear it's permanent (D7).
- **Group home (admin) = Statement.** Month switcher; one row per member:
  `base + chores − EMI = payable · cleared · remaining`, with a **Record
  payment** button pre-filled with the remaining amount; group total on top.
  Past months = the audit archive for free.
- **Member detail** — passbook (their ledger), base-amount history, loan cards
  with EMI progress.
- **Chores** — catalog management, styled as settings-like reference data, not
  a primary tab.
- **Loans** — list + detail with repayment schedule (D5).
- **Add entry** — one entry point, type picker: chore / adjustment / clearing /
  new loan. (Loan creates the entity; others create ledger rows.)
- **Group home (member)** — the same statement + passbook, own rows only,
  read-only; "You'll receive ₹X this month."
- **Notifications** — bell + list, mark read.
- **Profile** — name, password (existing hygiene work carries over).

## 6. Delta from the v2.1 build

| Area | Change |
|---|---|
| Users/auth | Nullable password, `status`, claim-on-register flow. |
| Money/API | `groups.currency` (D7); every amount field across the API becomes the `{currency, value}` object — breaking change to `openapi.yaml`, all handlers, and the generated client; `AmountText` becomes currency-aware. Storage stays int64 minor units. |
| Invites | Invite-link **UI hidden behind a build flag, implementation kept** (D8); add-by-email becomes the sole visible membership mechanism. Registration auto-links by email. |
| Chore submission | Member-facing submit + admin approve/reject UI and endpoints gated behind group flag, default OFF (D2). |
| Ledger API | Server-side member scoping (D6); edit/delete for manual entries + audit table (D3). |
| Statement | New derived endpoint + centerpiece screens. |
| Notifications | New subsystem (§3.7). |
| Chores | `description` field. |
| Naming | `head` → `admin`; "allowance" → "base amount" (API/UI copy). |
| App shell | Navigation flattening + responsive/web overhaul (§5). |
| Unchanged | Money core (int64 minor units, derived balances), posting engine (`PostDue`, lock ordering), loans engine, settlements mechanics, backups, CI + e2e harness. |

## 7. Open items

1. ~~Invite links~~ — RESOLVED (D8): hide UI for MVP, keep implementation.
2. ~~"clearing" vs "payment"~~ — RESOLVED (D9): "payment".
3. ~~N-3 notification~~ — RESOLVED: in scope for Phase 5; it is a trivial
   insert at the payment handler and high-value. First scope cut if Phase 5
   runs long.
4. ~~Remove-member hygiene~~ — RESOLVED: v2.1 rules carry over unchanged
   (removal only at zero balance and no active loan; ledger history kept).

## 8. Roadmap — Phases & Work Packages

Rules (unchanged from v2.1): **one WP = one pipeline = one branch/PR.** Each WP
lists its contract. BE and FE are separate WPs. Phases run in order; WPs inside
a phase marked ∥ may run in parallel **only after verifying zero file overlap
against the sibling WP's spec**. Spec files are committed to
`docs/specs/V3-x.y.md` (the `V3-` prefix avoids colliding with v2's `WP-*`
specs) before implementation starts.

### Phase 1 — Money & currency (D7). Goes first: touches every later surface.

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-1.1** BE currency | Migration: `groups.currency char(3) NOT NULL` (existing rows → `'INR'`), CHECK in (`EUR`,`USD`,`INR`). `openapi.yaml`: introduce `Money {currency, value:int64 minor units}` schema; replace every bare integer amount field (requests + responses) with it. Handlers: serialize group currency into every Money; reject writes whose currency ≠ group's (400, human-readable). Group create takes currency (required). Seed: second demo group in EUR to prove isolation. §3.8 is the contract. | Integration tests: create group per currency; mismatched-currency write rejected; all existing money math tests still pass unchanged (storage untouched). `go build/vet` both tags, unit tests green. |
| **V3-1.2** FE currency | `npm run codegen`; `api.ts` types follow. `AmountText` formats by currency (₹/€/$, 2 decimals); input parsing "12.50"→1250 unchanged. Create-group screen: currency picker with "permanent" copy. Dashboard/group screens render Money objects. Update e2e amount assertions + seed summary consumption. | `tsc --noEmit`, `expo export --platform web` pass; e2e suite green in CI incl. one EUR-group assertion; no hardcoded ₹ outside `AmountText`. |

### Phase 2 — Membership & identity

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-2.1** BE identity | Migration: `users.password_hash` nullable, `status ('shadow'\|'registered')`, `claimed_at`; `notifications` table (§3.7) created now, **insert-only** (read API is Phase 5). `POST /groups/{id}/members {email, name}` per §3.1 lifecycle (attach registered / create shadow / attach existing shadow). Register claims a matching shadow in place (same user id) and inserts N-2; adding a registered user inserts N-1. Shadow users cannot authenticate. | Integration tests: full lifecycle incl. claim keeps user id and memberships; shadow login rejected; duplicate add is idempotent 409/200 per spec; N-1/N-2 rows written. |
| **V3-2.2** BE authz & rename | Member-scoped reads (D6): ledger/statement/balance endpoints return only caller's rows unless admin; loans likewise. Rename `head`→`admin` across API + openapi. Invite endpoints kept (D8), untouched. Remove-member hygiene rules unchanged (§7.4). | Integration tests: member cannot read another member's entries/loans/totals (403/filtered per spec); admin sees all; openapi has no `head` remaining. |
| **V3-2.3** FE membership | Add-member-by-email sheet (email + display name); members list shows shadow badge ("not registered yet"); invite UI hidden behind a build-level flag, code kept (D8); `head`→`admin` copy sweep; codegen. Rework e2e invite tests → add-by-email + claim flow. | e2e: admin adds unregistered email → bookkeeps against shadow → user registers with that email → sees their group and only their own rows. Web export + tsc pass. |

### Phase 3 — Statement engine & corrections (backend)

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-3.1** BE statement | `GET /groups/{id}/statement?period=YYYY-MM` per §3.9: triggers `PostDue`, returns per-member rows (admin) or caller's row (member): opening, base, chores, adjustments, emi, total_due, cleared (payments), closing; plus group totals for admin. Months before a member's join return no row. | Integration tests: closing(M) == opening(M+1) across a 3-month backfill (backdate `joined_at` — see §9.11); mid-month join flooring; loan final-partial-EMI month; member gets own row only. |
| **V3-3.2** BE corrections & flags | Edit/delete (PUT/DELETE) for **manual** entry types only (chore_credit, adjustment, clearing); system types 403. `entry_audit` table written on every edit/delete (D3). `groups.member_chore_submission_enabled` flag (D2), default false, gating the v2 submit endpoints (404/403 when off). `chores.description`. | Integration tests: edit recomputes statement; audit row captures prior values; system-entry edit rejected; submit endpoints inert when flag off, functional when on. |

### Phase 4 — UI overhaul (month-first, on the new APIs)

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-4.1** Navigation shell | Fix the 2026-07-10 findings: flatten nested Tabs (no double bottom bars, no clipped labels), one meaningful header per screen (kill leaked "groups" title), group sections as top tabs/segmented control, responsive centered ~700px column on web, theme.ts spacing/typography pass. **Platform-adaptive per §5**: safe-area insets on native, keyboard-avoiding forms, no web-only APIs outside platform splits. No new features; testIDs preserved. | Playwright screenshot pass at 390px and 1440px shows single nav bar, single header, bounded content column; `npx expo-doctor` clean; grep shows no unguarded web-only API use; existing e2e green with at most selector-path updates. |
| **V3-4.2** Statement & dashboard screens | Group home = statement (§5): month switcher, per-member `base + chores − EMI = payable · paid · remaining`, **Record payment** pre-filled with remaining; group total; member variant read-only own-row ("You'll receive …"). Dashboard v3: manage/in sections, per-currency headlines (D7). | e2e: admin runs a full month from the statement screen (add chore entry → check payable → record payment → remaining 0); member sees own statement only. |
| **V3-4.3** Passbook, entry flows, catalog | Member detail = passbook + base-amount history + loan cards with repayment schedule (D5). Add-entry type picker: chore / adjustment / payment / new loan (D9 copy). Chores catalog restyled as reference data with `description`. Edit/delete on manual entries with "edited" affordance (D3). | e2e: correction flow (edit a chore amount → statement updates); loan card shows repaid/pending/schedule; all flows pass on web export. |
| **V3-4.4** e2e suite v3 | Consolidate: rewrite the suite around the v3 happy path (add-by-email → shadow bookkeeping → claim → month statement → payment; EUR group case), prune dead v2 flows (invite links, member submission), update seed. This WP owns suite health after 4.1–4.3's incremental edits. | Full CI green including E2E job; zero flaky over 3 consecutive runs; suite documents the v3 golden path. |

### Phase 5 — Notifications

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-5.1** BE notifications API | List (paged), unread count, mark-read/mark-all-read over the Phase-2 table. Add N-3 insert at the payment handler (§7.3). | Integration tests: scoping (only own notifications), unread math, N-3 written on payment. |
| **V3-5.2** FE bell | Header bell + unread badge, notifications list screen, mark-read on open; N-1 deep-links to the group. All three platforms per §5. | e2e: add registered user to group → bell shows 1 → open → marked read. |

### Phase 6 — Packaging & launch (the plan is not DONE until this ships)

| WP | Scope | Acceptance criteria |
|---|---|---|
| **V3-6.1** Single-image server | Serve the product from one artifact: `go:embed` the `expo export --platform web` dist into the Go server with SPA fallback (`NoRoute` → index.html); migrations via `iofs` embed; web built with relative `EXPO_PUBLIC_API_URL=/api/v1` (same origin → CORS moot, invite/`APP_BASE_URL` defaults to request host). Multi-stage root `Dockerfile` (node export → go build → alpine); `docker-compose.yml` slims to `postgres` + `app`; README quick-start rewritten (`docker compose up` + one `JWT_SECRET`). | CI builds the image; a compose smoke job boots postgres+app and runs the e2e suite against the **single-image** server (not the dev servers); README verified from scratch. |
| **V3-6.2** Android build & device QA | Commit `eas.json` (preview profile, `EXPO_PUBLIC_API_URL` documented as build-time env) + a `docs/qa-device-checklist.md` covering the golden path on Android/iOS: install, safe areas, statement flow, add-by-email, payment, notifications, icons/splash. Local-build path (`npx expo run:android`) documented as the no-account alternative. **Running `eas build` needs the operator's Expo credentials — the WP delivers config + docs + doctor-clean, and flags the build itself as an operator step.** | `npx expo-doctor` clean; `eas.json` validates; checklist committed; operator can produce an installable APK by following the README section verbatim. |
| ✅ **V3-6.3** Final acceptance & closeout | Fresh-machine golden path executed by the review agent against the V3-6.1 image: register → create INR + EUR groups → add member by email (shadow) → chores/base/loan → month statement → record payment → claim by registering the member → member sees own statement → bell notification. Prune dead v2 docs, mark this plan COMPLETE with final SHA, update README screenshots. | The scripted golden path passes against the single image; plan marked COMPLETE; no red anywhere in CI. |

## 9. Conventions for Coding Agents (Definition of Done)

Carried from v2.1 (§10 there), amended for v3 — these are **binding** for every
implement agent:

1. **Contract first**: API changes start in `backend/openapi.yaml`, then BE,
   then `npm run codegen` in `app/`. Generated files are never hand-edited.
2. **Stay in your WP**; note discoveries in `docs/notes/` instead of fixing.
3. **Verify**: BE → `make -C backend test lint`, `go vet` both build tags,
   `gofmt`. FE floor → `npx tsc --noEmit` AND `npx expo export --platform web`
   (expo-doctor/tsc miss bundle-only breakage — proven in v2).
   **Integration tests and e2e run only in CI** (no Docker on the dev host) —
   never mark a WP done on local green alone; the PR's CI is the gate.
4. **Migrations**: paired up/down, sequential numbering, re-runnable on fresh DB.
5. **AuthZ in the API** (D6 matrix in §4), mirrored — not implemented — in UI.
6. **Money**: int64 minor units everywhere except UI formatting; amounts cross
   the API only as `Money {currency, value}`; never `parseFloat` money input;
   currency comes only from the group (D7).
7. **Periods**: `YYYY-MM` strings, server-local timezone, server is the clock.
8. **UI**: theme.ts tokens + `src/components` kit; every list has
   empty/loading/error states; no inline hex; hooks above early returns; no
   `Alert.alert` (web no-op) — use the kit's confirm/toast.
9. **Auth/nav invariants** (learned in v2, do not regress): 401 force-logout
   lives in `api.ts` (wrong-old-password must return 403, not 401); **AuthGate
   is the single post-auth navigator** — never add per-screen post-auth
   navigation; CORS `Allow-Methods` must include every verb the API uses.
10. **Integration-test invariants** (learned in v2): handler tests must
    `AddMember(admin, RoleAdmin)` explicitly — `GroupRepo.Create` does not;
    ledger rows must reference real chores (FK + API-impossible states);
    settled-member tests must not accidentally accrue current-month base.
11. **Backfill tests** must backdate `group_members.joined_at` (posting floors
    at join month; `AddMember` stamps now()).
12. **e2e**: navigate group tabs by full URL (expo-router web param gotcha);
    success assertions must assert a **state change** (sheet closes, row
    appears), never absence-of-failure; keep testIDs stable or update the
    suite in the same PR.
13. **Commits/PRs** reference the WP id (`V3-2.1: shadow users & claim flow`).

## 10. Model Selection per Pipeline Stage

Same policy as v2.1: strong model on spec and review; implementation is
mechanical once the contract exists.

| Stage | Default | Notes |
|---|---|---|
| Spec | Opus | Commit `docs/specs/V3-x.y.md` before implement starts. |
| Implement | Sonnet | Sufficient for well-specified WPs. |
| Review | Opus | Different model than implementer. Review against §9 + the D-decisions. |

Risk overrides — **Opus-grade implement or strict Opus review with invariants
in the prompt** for: V3-1.1 (API-wide breaking change), V3-2.1 (identity/claim
— security sensitive), V3-2.2 (authz), V3-3.1 (statement math). Low-risk FE
(V3-2.3, V3-5.2): Sonnet review acceptable. Never skip review.

## 11. Autopilot handover

**Goal statement** (give this to warden autopilot verbatim):

> Execute `docs/master-plan-v3.md` §8 in full — Phases 1 through 6, all WPs —
> until the Definition of DONE below is met. One WP = one spec(Opus) →
> implement(Sonnet) → review(Opus) pipeline; commit the spec to
> `docs/specs/V3-x.y.md` before implementation; merge only on full CI green
> including the E2E job; §9 conventions are binding and reviews must check
> them. Every UI WP delivers web + Android + iOS per §5. Phases strictly in
> order; ∥ WPs in parallel only after the spec agent confirms zero file
> overlap. Tear down each agent (worktree, local+remote branch, tmux session,
> daemon record) the moment its WP merges. Stop and surface to the operator
> on: red master CI that a fix-forward doesn't clear in one attempt, any
> scope question not answerable from the plan, any change to auth/money
> semantics not covered by D1–D9, and the two designated operator steps
> (Expo credentials for `eas build` in V3-6.2; on-device QA sign-off in
> V3-6.3).

Operational facts the fleet needs (institutional knowledge from the v2 run):

- **Environment**: no Docker on the dev host — integration tests (build tag
  `integration`) and e2e run only in CI. Local gates are build/vet/fmt/unit +
  tsc + web export.
- **Merge discipline**: master must stay green; fix-forward PRs are preferred
  over reverts when the cause is known (v2 precedent: WP-4.7/PR #38).
- **Teardown**: finished agents can linger as idle tmux sessions + stale
  records that MCP verbs 404 on — `tmux kill-session -t <id>` then CLI
  `warden delete <id> --hard`. Keep exactly one master worktree plus live
  agents' worktrees. Never touch sessions belonging to other projects.
- **Known flake**: `TestMigrations_Idempotent` is a known CI test-isolation
  flake — re-run once before treating as a real break.
- **Sequencing insight from v2**: WPs sharing `auth.go`/`openapi.yaml` must
  not run in parallel; openapi is touched by nearly every BE WP here, so
  within a phase, BE WPs are effectively serial unless their spec proves
  otherwise.

### Definition of DONE for the whole plan

All §8 WPs (Phases 1–6) merged; master CI fully green including e2e **and the
single-image compose smoke**; the v3 golden path (add-by-email → shadow →
claim → monthly statement → payment, in two currencies) covered by e2e and
executed against the V3-6.1 image; web app usable from a phone browser against
that image; Android build producible from the committed config by following
the README; `docs/master-plan-v3.md` marked COMPLETE with the final SHA.

Honest boundary: the fleet can automate everything except physically running
the app on a device — installable-build config, doctor checks, and the QA
script are deliverables; the final on-device tap-through is the operator's
sign-off (V3-6.2/6.3 flag it).
