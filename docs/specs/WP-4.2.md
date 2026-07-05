# WP-4.2 Spec: Dashboard v2 + `GET /groups` summary enrichment

**Work package:** WP-4.2 (Phase 4 — UX polish & launch readiness). Runs ∥ with 4.1/4.3 but has a
**hard ordering dependency on WP-4.1** (see §0.1).
**Type:** Frontend rebuild **+ a small backend addendum** (one enriched read endpoint, no new
table, **no migration**). Contract-first: `openapi.yaml` delta → BE handler/repo → `npm run codegen`
→ FE.
**Depends on:** WP-1.0 (design-system kit — `Card`, `ListRow`, `Avatar`, `Button`, `Sheet`,
`TextField`, `AmountText`, `EmptyState`, `ErrorMessage`, `LoadingSpinner`, `Toast`) and WP-1.2
(ledger v2 `direction`/`status`/`amount` int64 — the balance math this endpoint sums). Both landed.
**Acceptance (master plan §9, Phase 4 row 4.2):** "Sectioned dashboard per `docs/fe-flow.md` (head
groups w/ member count + total owed; member groups w/ own balance) — needs a `GET /groups` summary
enrichment (small BE addendum, spec first)."

> Roadmap refs: master-plan §5.1 (balance = Σ approved credits − Σ approved debits), §5.5 (authz —
> members read own only), §6.1 (API delta), §7.1 (dashboard = sectioned groups Head-of / Member-of),
> §7.3 (query keys + invalidation, `AmountText`), §7.4 (kit), §10 (Definition of Done: contract-first,
> money as minor units, authz in API, empty/loading/error on every list), §11 (low-risk FE WP:
> Sonnet spec → Sonnet/Haiku implement → Sonnet review).
>
> UX authority: `docs/fe-flow.md` → **Group Listing Screen** section (create/join buttons; two
> sections; head item = name + member count + total owed to all members; member item = name + own
> ledger sum; tap → Group Detail).

---

## 0. Scope, guardrails & the two decisions this WP makes

### In scope

1. **BE addendum** — enrich the `GET /api/v1/groups` response so each row carries `role`,
   `member_count`, and a role-dependent `summary_balance` (head → total owed to all members;
   member → caller's own balance). One new response schema, one new repo query, one changed
   handler. **No migration.** (§2–§4)
2. **FE dashboard v2** — rebuild `app/app/(app)/index.tsx` per `docs/fe-flow.md` on the WP-1.0 kit:
   two sections, per-item fields, preserved create/join flows, navigation, empty/loading/error,
   pull-to-refresh, money via `AmountText`. Collapses today's N+1 fetch into one enriched query.
   (§5)
3. **Delete the legacy Groups tab** — remove `app/app/(app)/groups/index.tsx` and hide the tab
   (§0.3, decision D2).

### Non-goals

- No new table, column, or migration (§3 proves none is needed — if you conclude otherwise, **STOP
  and flag loudly**, see §3).
- No change to `GET /groups/:id`, `/balance`, or the posting trigger set (§0.2, decision D1).
- No change to the group **detail** screens, chores, loans, ledger, member-detail — those are done.
- No change to `groupsApi.create` / `groupsApi.join` payloads or to `GroupResponse` (create/join
  keep the lean shape; only the **list** endpoint gets the new schema — §2).
- No new dependency; do not edit `theme.ts` or any kit component (frozen from WP-1.0).

### 0.1 Sequencing (READ FIRST)

**The implementer runs AFTER WP-4.1 merges.** WP-4.1 does a *minimal* re-skin of this exact file
(`app/app/(app)/index.tsx`) and deliberately left the layout for THIS WP to rebuild. If WP-4.2
branches before 4.1 merges, the two will collide on the whole dashboard file. **Branch WP-4.2 off
`master` only after WP-4.1 is merged**, and take WP-4.1's swept dashboard as the starting point (you
then largely replace its body, but you inherit its kit imports and the `Alert.alert → Toast`
conversion — don't regress those). BE work has no such dependency and may start immediately.

### 0.2 Decision D1 — the summary endpoint does **NOT** trigger `PostDue`

The lazy posting engine (`§5.4`) currently fires on `GET /groups/:id`, `.../ledger`, `.../balance`
(each a per-group transaction, via `runPosting`). The dashboard reads **all of the caller's groups
at once**; triggering `PostDue` per group would mean **N transactions on every dashboard load** (and
the dashboard refetches on window-focus and pull-to-refresh) — the wrong cost for a glanceable
summary on a home-LAN server.

**Decision: `GET /groups` serves the summary WITHOUT triggering `PostDue`.** Posting stays triggered
only on **group open** and the other balance-sensitive detail reads (unchanged). Consequence — a
**documented, bounded staleness**: if a month has rolled over and the user has not yet opened a
group (nor hit any balance-sensitive endpoint for it), the dashboard total omits that month's
not-yet-posted `allowance`/`emi` entries. It **self-heals** the moment the group is opened (posting
runs) and the dashboard refetches (on focus / pull-to-refresh). The authoritative figure is always
one tap away on the detail screen.

Tradeoff, stated plainly:
- **Chosen (no trigger):** dashboard is cheap and O(1 query); worst case it under-counts the current
  month's machine-posted entries until first group-open. Acceptable for a summary.
- **Rejected (trigger per group):** exact figures, but N write-transactions per dashboard load —
  write-amplification and latency that scale with group count, on every focus/refresh.

If product later demands exact dashboard figures, the cheap follow-up (a future WP, **not** this one)
is a single batched `PostDue` over the caller's groups in one transaction before the summary query —
not N separate triggers. Note that as a one-liner in `docs/notes/` if you touch it; do not build it
here.

Document this staleness in the openapi `description` of `summary_balance` (§2) so FE/consumers know
the number is a summary, not a live balance.

### 0.3 Decision D2 — delete the legacy Groups tab (`groups/index.tsx`)

`app/app/(app)/groups/index.tsx` is a pre-Query near-duplicate of the dashboard (raw `groupsApi.list()`
+ `useState`, its own create/join). WP-4.1 §G7 swept it but flagged it for deletion here. Dashboard
v2 fully subsumes it (list + create + join + navigate, now with richer summary data). Two divergent
group-list screens is dead surface and a "looks broken" hazard (§1 launch bar).

**Decision: delete it.** Mechanics — see §5.6. The only remaining consumers of `groupsApi.list()`
are `useGroups()` (dashboard) and this file (verified 2026-07-05); deleting it leaves `useGroups`
as the sole consumer, which this WP updates. `groups/create.tsx` and `groups/[id]/**` are reached via
`router.push` from the dashboard and are **kept** — only the redundant list `index.tsx` and its tab
entry go.

---

## 1. Data-flow overview (what changes end to end)

**Today (N+1):** dashboard calls `useGroups()` (`GET /groups`) then fans out `useQueries` over each
group for `groupsApi.get(id)` (member count) **and** `ledgerApi.getBalance(id)` (balances), then
enriches client-side. That's `1 + 2N` requests and duplicated balance math on the client.

**After WP-4.2:** `GET /groups` returns everything the dashboard needs in **one** request. The
dashboard drops the `useQueries` fans entirely and renders straight off the enriched list.

```
GET /api/v1/groups
  → [ { id, name, head_user_id, created_at, role, member_count, summary_balance }, … ]
       role='head'   → summary_balance = Σ balances of all NON-head members (total owed)
       role='member' → summary_balance = caller's OWN balance in that group
```

---

## 2. Contract delta — `backend/openapi.yaml` (do this FIRST)

Two edits. **Nothing else in the file changes.**

### 2.1 Change the `GET /api/v1/groups` 200 response to the new schema

At `paths` → `/api/v1/groups` → `get` → `responses` → `'200'` (currently line ~168), swap the array
`items` ref from `GroupResponse` to `GroupSummaryResponse`, and update the description:

```yaml
      responses:
        '200':
          description: List of the user's groups, each enriched with role, member count, and a
            role-dependent summary balance for the dashboard.
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/GroupSummaryResponse'
```

`POST /api/v1/groups` (create) and `POST /api/v1/groups/join` keep returning `GroupResponse`
unchanged — they don't need the enrichment and shouldn't pay for it.

### 2.2 Add the `GroupSummaryResponse` schema

Add under `components.schemas`, immediately after `GroupResponse` (line ~1506). It is `GroupResponse`
plus three fields:

```yaml
    GroupSummaryResponse:
      type: object
      description: A group the caller belongs to, enriched for the dashboard listing. The
        summary_balance is a glanceable figure computed from currently-posted approved ledger
        entries; it does NOT trigger allowance/EMI posting, so it may lag the authoritative
        per-group balance by the current month's machine-posted entries until the group is opened.
      properties:
        id:
          type: string
          format: uuid
          description: Group ID
        name:
          type: string
          description: Group name
        head_user_id:
          type: string
          format: uuid
          description: ID of the group head
        created_at:
          type: string
          format: date-time
          description: Group creation timestamp
        role:
          type: string
          enum: [head, member]
          description: The caller's role in this group.
        member_count:
          type: integer
          description: Number of members in the group (includes the head).
        summary_balance:
          type: integer
          format: int64
          description: >-
            Role-dependent summary in minor units (paise). For role=head, the sum of all NON-head
            members' balances (= total the head owes members; typically >= 0). For role=member, the
            caller's OWN balance in this group (positive = earned/owed-to-them, negative = they owe
            the head). Computed as Σ approved credits − Σ approved debits. A member never receives
            another member's figure.
      required:
        - id
        - name
        - head_user_id
        - created_at
        - role
        - member_count
        - summary_balance
```

`format: int64` matches the money convention already used by `BalanceResponse.balance` and every
`amount` (§10 rule 6). `member_count` is a plain `integer`.

---

## 3. Migration check — **NONE NEEDED** (verify, and flag loudly if wrong)

Every field is derivable from existing tables:
- `role` → `group_members.role` (exists).
- `member_count` → `COUNT(*)` over `group_members` (exists).
- `summary_balance` → SUM over `ledger_entries` (`direction`, `amount int64`, `status`) — the exact
  columns `GetBalanceForGroup` already reads (`ledger_repo.go:259`). No new column, index, or table.

**Therefore this WP adds NO migration file.** If during implementation you find yourself reaching for
`ALTER TABLE`/a new migration to satisfy any of the three fields, **STOP** — you have drifted from
this spec. The three fields are pure reads. Re-read §2/§4; do not invent schema.

---

## 4. Backend implementation

### 4.1 Repo — new method on `GroupRepo` (`backend/internal/db/group_repo.go`)

Add a single-round-trip query that returns all three fields per group, restricted to the caller's
groups, with the SUMs `::bigint`-cast (matching `GetBalanceForGroup`). No N+1, no per-group loop.

```go
// GroupSummary is a dashboard-listing row: a group plus the caller's role, member count,
// and a role-dependent summary balance (minor units). See GET /groups.
type GroupSummary struct {
    ID             uuid.UUID
    Name           string
    HeadUserID     uuid.UUID
    CreatedAt      time.Time
    Role           models.MemberRole
    MemberCount    int
    SummaryBalance int64
}

// ListForUserWithSummary returns every group the user belongs to, each enriched with the caller's
// role, the member count, and a summary balance: for head groups the sum of all non-head members'
// balances (total owed); for member groups the caller's own balance. Balances come from currently
// approved ledger entries only (no posting is triggered — see WP-4.2 §0.2).
func (r *GroupRepo) ListForUserWithSummary(ctx context.Context, userID uuid.UUID) ([]*GroupSummary, error) {
    query := `
        WITH my_groups AS (
            SELECT g.id, g.name, g.head_user_id, g.created_at, gm.role
            FROM groups g
            JOIN group_members gm ON gm.group_id = g.id
            WHERE gm.user_id = $1
        ),
        member_counts AS (
            SELECT group_id, COUNT(*)::int AS member_count
            FROM group_members
            WHERE group_id IN (SELECT id FROM my_groups)
            GROUP BY group_id
        ),
        member_balances AS (
            SELECT le.group_id, le.user_id,
                (COALESCE(SUM(CASE WHEN le.direction = 'credit' THEN le.amount ELSE 0 END), 0)
               - COALESCE(SUM(CASE WHEN le.direction = 'debit'  THEN le.amount ELSE 0 END), 0))::bigint AS balance
            FROM ledger_entries le
            WHERE le.status = 'approved'
              AND le.group_id IN (SELECT id FROM my_groups)
            GROUP BY le.group_id, le.user_id
        ),
        head_totals AS (
            SELECT mb.group_id, COALESCE(SUM(mb.balance), 0)::bigint AS total_owed
            FROM member_balances mb
            JOIN group_members gm ON gm.group_id = mb.group_id AND gm.user_id = mb.user_id
            WHERE gm.role <> 'head'
            GROUP BY mb.group_id
        )
        SELECT mg.id, mg.name, mg.head_user_id, mg.created_at, mg.role,
            COALESCE(mc.member_count, 0) AS member_count,
            CASE WHEN mg.role = 'head'
                 THEN COALESCE(ht.total_owed, 0)
                 ELSE COALESCE(ob.balance, 0)
            END::bigint AS summary_balance
        FROM my_groups mg
        LEFT JOIN member_counts mc ON mc.group_id = mg.id
        LEFT JOIN head_totals   ht ON ht.group_id = mg.id
        LEFT JOIN member_balances ob ON ob.group_id = mg.id AND ob.user_id = $1
        ORDER BY mg.created_at DESC
    `
    rows, err := r.pool.Query(ctx, query, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to list group summaries: %w", err)
    }
    defer rows.Close()

    summaries := make([]*GroupSummary, 0)
    for rows.Next() {
        s := &GroupSummary{}
        if err := rows.Scan(&s.ID, &s.Name, &s.HeadUserID, &s.CreatedAt,
            &s.Role, &s.MemberCount, &s.SummaryBalance); err != nil {
            return nil, fmt.Errorf("failed to scan group summary: %w", err)
        }
        summaries = append(summaries, s)
    }
    return summaries, rows.Err()
}
```

**Correctness notes the implementer must preserve:**
- `head_totals` sums only `role <> 'head'` members → the head's own (near-zero) balance is excluded,
  matching `GetBalanceForGroup`'s "drop head row" behavior. Empty group (head only) → no non-head
  rows → `COALESCE(..., 0)` → `summary_balance = 0`.
- **Member privacy is structural:** a `role='member'` row's `summary_balance` comes only from
  `ob` (`user_id = $1`). `head_totals` is joined but consumed **only** in the `role='head'` branch of
  the `CASE`. A member therefore can never receive another member's balance or the group total — the
  authz (§5.5: members read own only) is enforced in the query shape, not just the handler.
- `::bigint` on every SUM (Postgres `SUM(bigint)` returns `numeric`; the cast keeps it `int64` for
  the pgx scan into `int64`). Do not drop the casts.
- `WHERE ... group_id IN (SELECT id FROM my_groups)` keeps the balance scan bounded to the caller's
  groups (not the whole `ledger_entries` table).

### 4.2 Handler — `ListGroups` (`backend/internal/handlers/groups.go:124`)

Add a `GroupSummaryResponse` struct and rewrite `ListGroups` to use the new repo method. **Do NOT
call `runPosting`** (decision D1). Membership/authz is inherent (the query only returns groups the
caller is in).

```go
// GroupSummaryResponse is one dashboard-listing row (see openapi GroupSummaryResponse).
type GroupSummaryResponse struct {
    ID             uuid.UUID         `json:"id"`
    Name           string            `json:"name"`
    HeadUserID     uuid.UUID         `json:"head_user_id"`
    CreatedAt      time.Time         `json:"created_at"`
    Role           models.MemberRole `json:"role"`
    MemberCount    int               `json:"member_count"`
    SummaryBalance int64             `json:"summary_balance"`
}

// ListGroups returns the user's groups enriched for the dashboard. Does NOT trigger posting
// (WP-4.2 §0.2): summary_balance reflects currently-posted approved entries.
func (h *GroupHandler) ListGroups(c *gin.Context) {
    userIDStr, exists := auth.GetUserID(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
        return
    }
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
        return
    }

    summaries, err := h.groupRepo.ListForUserWithSummary(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list groups"})
        return
    }

    response := make([]GroupSummaryResponse, 0, len(summaries))
    for _, s := range summaries {
        response = append(response, GroupSummaryResponse{
            ID:             s.ID,
            Name:           s.Name,
            HeadUserID:     s.HeadUserID,
            CreatedAt:      s.CreatedAt,
            Role:           s.Role,
            MemberCount:    s.MemberCount,
            SummaryBalance: s.SummaryBalance,
        })
    }
    c.JSON(http.StatusOK, response)
}
```

The old `ListForUser` (`group_repo.go:74`) may still be used elsewhere — grep before removing it; if
nothing else calls it after this change, remove it in-scope to avoid dead code (note it in the PR).
Route registration (`main.go:80`) is unchanged (same path/verb).

### 4.3 Integration tests (`backend/internal/handlers/groups_integration_test.go` or repo test)

Run only in CI (Docker; §10). Follow the existing harness conventions — **`AddMember(head, RoleHead)`
explicitly** after `GroupRepo.Create` (Create does not add the head; see project memory / WP-1.2
fix), and backdate `group_members.joined_at` only if a test posts allowances (not needed here since
these tests write ledger entries directly).

Required cases (each asserts on the `GET /groups` response for the acting user):

1. **Head total = sum of member balances.** Head + 2 members; insert approved credits/debits so
   member A balance = +1500, member B = −500. Head's row: `role=head`, `member_count=3`,
   `summary_balance = 1000`. (Mix a `credit` and a `debit` so the direction math is exercised.)
2. **Member sees own balance only.** As member A (from case 1): their row has `role=member`,
   `summary_balance = 1500` (their own), and **no** field exposes B's −500 or the group total 1000.
3. **Member cannot see others' totals (privacy).** In a group where the caller is a member, assert
   the returned `summary_balance` equals the caller's own balance and is independent of other
   members' entries (add a large credit to another member → caller's row is unchanged).
4. **Empty group (head only).** Head creates a group, no members join: row has `member_count = 1`,
   `summary_balance = 0` (no non-head members to owe).
5. **Group with members but no ledger entries.** `summary_balance = 0` for both head and member rows
   (COALESCE path), `member_count` correct.
6. **Multi-group user, mixed roles.** User is head of G1 and member of G2; response contains both
   rows with correct per-row `role` and the correct role-dependent `summary_balance` for each.
7. **Rejected/pending excluded.** A `pending_approval` and a `rejected` entry for a member do **not**
   move `summary_balance` (only `approved` counts) — mirrors §5.1.
8. **No posting side-effect (D1).** Optional but recommended: a group with an in-force allowance and
   a rolled-over month, hit `GET /groups`, assert **no** new `allowance` ledger rows were created by
   the call (proves the endpoint doesn't trigger `PostDue`), and the summary reflects only
   pre-existing approved entries.

Also keep/extend a unit or repo-level assertion that the query returns rows ordered by `created_at
DESC` (stable list order for the FE).

---

## 5. Frontend — Dashboard v2 (`app/app/(app)/index.tsx`)

Rebuild the screen on the WP-1.0 kit per `docs/fe-flow.md` → Group Listing Screen. Start from the
WP-4.1-swept file (inherit its kit imports + Toast conversion); replace the body.

### 5.0 Codegen STOP-gate (do BEFORE writing any FE code)

After the openapi delta (§2) and BE build, run in `app/`:

```bash
npm run codegen         # regenerates app/src/api-types.gen.ts from openapi.yaml
```

Then **verify** `components['schemas']['GroupSummaryResponse']` exists with `role`, `member_count`,
`summary_balance` in `api-types.gen.ts`, and **commit the regenerated file**. Do **NOT** hand-write
or hand-edit the summary type (§10 rule 1). If the fields aren't there, the openapi edit is wrong —
fix §2, don't patch the generated file. `npm run codegen:check` must show no diff afterward.

### 5.1 Types & data layer

- `app/src/api.ts`: add `export type GroupSummary = Schemas['GroupSummaryResponse'];` and change
  `groupsApi.list` to `list: () => request<GroupSummary[]>('/groups')`. Keep `Group` /
  `groupsApi.create` / `groupsApi.join` on `GroupResponse` (unchanged).
- `app/src/hooks/useGroups.ts`: `useGroups()` now yields `GroupSummary[]` (no code change needed
  beyond the return type flowing through; keep `queryKey: qk.groups()`). `useCreateGroup` /
  `useJoinGroup` unchanged (they already invalidate `qk.groups()`).
- **Delete the dashboard's `useQueries` fans** over `groupsApi.get` and `ledgerApi.getBalance`, and
  the `enrichedGroups` client-side balance math — the server now supplies `role`, `member_count`,
  `summary_balance`. Remove now-unused imports (`useQueries`, `ledgerApi`, `formatMinorUnits`, the
  local `GroupWithDetails` interface).

### 5.2 Query keys + invalidation (§7.3)

- Read: single `useGroups()` on `qk.groups()`. Enable `refetchOnWindowFocus` (Query default; ensure
  the provider hasn't disabled it) so month-rollover self-heal (D1) lands on focus.
- Pull-to-refresh: `onRefresh` invalidates **only** `qk.groups()` now (no per-group balance/detail
  keys to invalidate from here). Wire it to `RefreshControl` via the list's `refreshing`/`onRefresh`.
- Join success: `useJoinGroup` already invalidates `qk.groups()` → the new group appears with its
  summary. No extra invalidation needed.

### 5.3 Layout & rendering (per fe-flow.md)

Keep a `SectionList` (or two `FlatList`s) with two sections, split by the server `role` field:

- **Header row:** greeting (keep "Welcome, {user.name}") + the two action buttons.
- **Actions:** `<Button variant="secondary" icon="add-circle" title="Create Group"
  onPress={() => router.push('/(app)/groups/create')} />` and `<Button variant="secondary"
  icon="enter" title="Join Group" onPress={() => setJoinVisible(true)} />` (two-across row).
- **Section 1 — "Groups You Manage" (role=head):** each item in a `<Card onPress={() =>
  router.push('/(app)/groups/${item.id}')}>` wrapping a `<ListRow>`:
  - `left`: `<Avatar name={item.name} id={item.id} />`
  - `title`: `item.name`
  - `subtitle`: `` `${item.member_count} ${item.member_count === 1 ? 'member' : 'members'}` ``
  - `right`: the **total owed** — `<AmountText minorUnits={item.summary_balance} variant="neutral"
    size="md" />` plus a small muted caption "owed to members". (The head total is a positive
    liability figure; `AmountText` has no warning/orange variant and the kit is frozen — render it
    **neutral** with the caption. Do not fake orange with an inline hex; if product wants warning
    color, that's a future kit change → note in `docs/notes/`, don't do it here.)
- **Section 2 — "Groups You're In" (role=member):** same `Card` + `ListRow`, but:
  - `subtitle`: omit member count (fe-flow's member item shows only name + own amount), or keep it
    subtle — fe-flow lists only name + own ledger sum, so **prefer name only**.
  - `right`: the caller's **own balance** via `AmountText` with sign-driven semantics:
    `variant={item.summary_balance < 0 ? 'debit' : 'credit'}`, `minorUnits={item.summary_balance}`.
    Positive → green "+₹…" (earned); negative → red "−₹…" (**owes** — this is where §5.3's
    negative-balance danger/owes treatment lands, which WP-4.1 explicitly deferred to WP-4.2). Zero →
    render `variant="credit"` (green "₹0.00") or `neutral`; pick `neutral` for a plain "₹0.00".
    `AmountText` takes raw minor units and formats ₹ itself — pass `item.summary_balance` directly,
    do not `Math.abs`/pre-format (kit gotcha G10 from WP-4.1).
- **Section header:** `theme`-styled text; only render a section whose data is non-empty
  (`.filter(s => s.data.length > 0)`), as today.

### 5.4 States

- **Loading:** `if (groupsQuery.isLoading) return <LoadingSpinner />;` (single query now — no N+1
  loading union).
- **Error:** `groupsQuery.isError` → `<ErrorMessage message={…} onRetry={() => groupsQuery.refetch()} />`
  as the page body (fetch error). Keep it a real, visible, recoverable error (§1 principle 5).
- **Empty:** no groups at all → `<EmptyState icon="people-outline" title="No groups yet"
  subtitle="Create a group or join one with an invite link" />` (teaches next step, §1 principle 4).
  Keep the Create/Join actions visible above the empty state so the user can act.
- **Pull-to-refresh:** `RefreshControl refreshing={groupsQuery.isRefetching} onRefresh={onRefresh}`.

### 5.5 Join-group Sheet (preserve behavior; don't regress WP-4.1's Toast)

Replace the legacy `<Modal>` with `<Sheet visible={joinVisible} onClose={closeJoin} title="Join
Group">` containing a `<TextField>` (invite link/token) + a primary `<Button title="Join"
loading={joinMutation.isPending} onPress={handleJoin} />` and a ghost/secondary Cancel.

- Keep the existing token-extraction logic (`token.match(/token=([^&]+)/)` to accept a full invite
  URL or a bare token).
- Feedback via **Toast** (WP-4.1 already converted these — keep them): empty input →
  `toast.show({ tone:'danger', message:'Please enter an invite token' })`; success →
  `toast.show({ tone:'success', message:'You have joined the group!' })` + close sheet + clear field;
  error → `toast.show({ tone:'danger', message: err instanceof Error ? err.message : 'Failed to join group' })`.
  **No `Alert.alert`** (dead on web).
- **Sheet field-reset gotcha (009d78c):** bottom-sheet forms must reset their fields on open/close.
  Clear `inviteToken` when the sheet closes (both Cancel and successful join) so a reopened sheet
  doesn't show stale input. Match the reset pattern established in 009d78c for the other sheets.

### 5.6 Delete the legacy Groups tab (decision D2 — exact mechanics)

1. **Delete** `app/app/(app)/groups/index.tsx`.
2. **`app/app/(app)/groups/_layout.tsx`:** remove the `<Stack.Screen name="index" … />` line (its
   route no longer exists). Leave `create` and `[id]`.
3. **`app/app/(app)/_layout.tsx`:** the `groups` folder is auto-registered as a tab by expo-router,
   so simply removing the `<Tabs.Screen name="groups">` block would **not** hide it. Instead set
   `options={{ href: null }}` on that screen so the tab is **hidden from the tab bar** while the
   nested `groups/create` and `groups/[id]/**` routes stay reachable via `router.push`:
   ```tsx
   <Tabs.Screen name="groups" options={{ href: null }} />
   ```
   Keep the `index` (Dashboard) and `profile` tabs. Net tab bar: **Dashboard | Profile**.
4. Verify nothing else navigates to bare `/(app)/groups` (grep: only `groups/create` and
   `groups/${id}` deep links remain — confirmed 2026-07-05).

### 5.7 FE gotchas checklist

- **Hooks above early returns:** all hooks (`useAuth`, `useQueryClient`, `useState`, `useCallback`,
  `useGroups`, `useJoinGroup`, `useToast`) must run before any `if (isLoading) return …`. Never put a
  hook after a conditional return.
- **No `Alert.alert`** anywhere under `app/app` (§10; use Toast). Drop `Alert`/`Modal`/`TextInput`/
  `ActivityIndicator` from the `react-native` import once replaced by kit components.
- **Money only via `AmountText`** — no `formatMinorUnits`/manual ₹ string on the dashboard. No
  `parseFloat` / `parseMoneyToMinorUnits` here (the dashboard takes no money input — the join field
  is a token), but the rule stands if you add any amount input: parse to minor units, never
  `parseFloat`.
- **No inline hex** — every color from `theme`; the WP-4.1 hex gate over `app/app` must stay clean.
- **`Avatar` tolerates empty id** but here `item.id` is always present; pass it for the id-hashed
  color.

---

## 6. Verification (Definition of Done, §10 rule 3)

### Backend (run from repo root or `backend/`)

```bash
gofmt -l backend/internal            # must print NOTHING (all files formatted)
go build ./...                       # compiles
go vet ./...                         # clean
go test -race ./...                  # unit tests pass
# Integration tests need Postgres (Docker) → they run in CI. Locally, at minimum COMPILE them:
go test -tags=integration -run xxxNONExxx ./backend/internal/handlers/... ./backend/internal/db/...
```

The `-tags=integration` compile-check (a run that matches no test) proves the integration files build
even though Docker isn't available locally (project convention — integration tests execute only in
CI). The new cases in §4.3 must pass in CI.

### Frontend (run from `app/`)

```bash
npm run codegen:check     # generated types are in sync with openapi (STOP-gate, §5.0)
npx tsc --noEmit          # zero type errors
npx expo export --platform web    # MANDATORY — must exit 0; tsc/expo-doctor miss bundle breakage (WP-0.1 precedent)
npm run lint              # no new issues
grep -rn "Alert\.alert" app/app --include='*.tsx'   # must print NOTHING
```

State all results in the PR. If the environment allows, boot `npm run web` and click through:
dashboard loads (both sections when applicable) → head row shows member count + total owed → member
row shows own balance (green +, red − for negative) → Create/Join buttons work → Join sheet joins +
Toast + new group appears → pull-to-refresh → tap a group → detail opens. Confirm the **Groups tab is
gone** and Dashboard/Profile remain.

---

## 7. Acceptance criteria (Definition of Done)

1. `backend/openapi.yaml`: `GET /groups` returns `GroupSummaryResponse[]`; the schema exists with
   `role`, `member_count`, `summary_balance` (int64) and the staleness note; create/join still return
   `GroupResponse`. **No migration file added.**
2. `ListForUserWithSummary` returns correct `role`, `member_count`, and role-dependent
   `summary_balance` (head = Σ non-head member balances; member = own), computed from `approved`
   entries with `::bigint`-cast SUMs, in **one** query. Members structurally cannot receive others'
   figures.
3. `ListGroups` uses the new method and does **NOT** trigger `PostDue` (D1). Integration cases §4.3
   pass in CI.
4. `npm run codegen` regenerated `api-types.gen.ts` (committed); `codegen:check` is clean; the FE
   summary type is generated, not hand-written.
5. Dashboard rebuilt per `docs/fe-flow.md`: two role-split sections; head item = name + member count
   + total owed; member item = name + own balance; money via `AmountText` (negative member balance in
   danger/owes treatment); Create + Join preserved (Join = Sheet + Toast, no `Alert.alert`);
   navigation to detail; empty/loading/error states; pull-to-refresh. N+1 fetch removed.
6. Legacy `groups/index.tsx` deleted; Groups tab hidden via `href: null`; nested group routes still
   reachable; tab bar = Dashboard | Profile.
7. Verification (§6) all green: gofmt/build/vet/test -race + integration compile-check; codegen:check
   + tsc + `expo export --platform web` exit 0; no `Alert.alert` under `app/app`; no inline hex.
8. Commit/PR reference the WP id (`WP-4.2: dashboard v2 + GET /groups summary enrichment`).

---

## 8. Gotchas & likely-slips (quick reference)

- **G1 — Sequence after WP-4.1.** Same dashboard file; branch after 4.1 merges (§0.1).
- **G2 — Do NOT trigger posting in `GET /groups`** (D1). N transactions per dashboard load is the
  bug this avoids; accept documented staleness instead.
- **G3 — No migration.** All three fields are reads (§3). Reaching for `ALTER TABLE` means you've
  drifted — stop.
- **G4 — Keep `GroupResponse` for create/join.** Only the list endpoint gets `GroupSummaryResponse`.
- **G5 — Member privacy is in the query shape.** `head_totals` must be consumed only in the
  `role='head'` CASE branch; member rows use `ob` (own) only. Test case §4.3.2/.3.
- **G6 — `::bigint` casts on SUMs.** Mirror `GetBalanceForGroup`; dropping them changes the scan type.
- **G7 — Codegen STOP-gate.** Regenerate + commit `api-types.gen.ts`; never hand-write the type.
- **G8 — `AmountText` takes raw minor units.** Pass `summary_balance` directly; don't `Math.abs` or
  pre-format (WP-4.1 G10). Head total = neutral + caption (no warning variant in the frozen kit);
  member negative = `variant="debit"` (danger/owes).
- **G9 — Groups tab: `href: null`, not deletion of the `<Tabs.Screen>`.** Removing the block leaves
  the auto-registered folder tab visible; `href: null` hides it while keeping nested routes.
- **G10 — Hooks above early returns; Toast not Alert; sheet field reset (009d78c).**
- **G11 — Integration tests: `AddMember(head, RoleHead)` explicitly** after `Create` (Create doesn't
  add the head); Docker/integration runs in CI only.
```
