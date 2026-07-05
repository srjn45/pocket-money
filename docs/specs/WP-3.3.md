# WP-3.3 Spec: FE Member Detail Page

**Work package:** WP-3.3 (Phase 3 — Loans & EMI), master-plan §9. **Third and final WP of Phase 3** (backend is WP-3.1; loans tab is WP-3.2).
**Type:** Frontend-only. Promotes the interim WP-1.3 per-member ledger screen (`groups/[id]/ledger.tsx`) into the named **head member-detail page** `groups/[id]/members/[userId].tsx` of master-plan §7.1, and adds a **read-only active-loans section** for that member. It **consumes only already-shipped endpoints** — nothing new is added to `openapi.yaml`, so **no codegen, no backend, no migration changes.**
**Risk:** Low FE. No money math of its own (all money via the existing `AmountText`/`formatMinorUnits`/`parseMoneyToMinorUnits`); the settlement/adjustment forms and the allowance form already exist and are reused verbatim; all authz is API-enforced and mirrored in the UI (§8 matrix). Master-plan §11: Sonnet implement, Sonnet review. The review must check that the route relocation didn't leave a dead `ledger.tsx`/dangling tab, that the non-head guard is present, that the loans section is **read-only** (no approve/reject/close here — that's WP-3.2's tab), that `useLoans` is filtered to the member, and the money/`Alert.alert`/hooks-above-early-returns/sheet-prefill gotchas are respected (§11).

**Depends on (all merged to master before implement starts):**
- **WP-1.0** (design-system kit) — `theme.ts` tokens + `Button`, `Card`, `ListRow`, `AmountText`, `StatusBadge`, `Sheet`, `TextField`, `MonthHeader`, `Toast`, `EmptyState`/`ErrorMessage`/`LoadingSpinner`; `confirmAsync`; `Sheet`+`Toast`+`confirmAsync` are the **only** confirm/feedback path (`Alert.alert` is banned — no-op on `react-native-web`).
- **WP-1.1** (money in minor units) — API money is integer minor units; `AmountText` takes minor units; `parseMoneyToMinorUnits`/`formatMinorUnits`/`formatMinor` live in `app/src/money.ts`.
- **WP-1.2** (ledger v2) — `POST /ledger` types (`chore`/`settlement`/`adjustment`), the settlement/adjustment authz + **direction** semantics this WP relies on (§5), and `GET /ledger?user_id=` member filtering.
- **WP-1.3** (FE unified ledger) — `ledger.tsx` (the head per-member view this WP relocates), `LedgerList`/`LedgerRow`/`ledger-format.ts`, `AddEntrySheet` (chore/settlement/adjustment, `fixedUserId` head mode), `MemberCard`, `useLedger`/`useBalance`/`useApproveLedger`/`useRejectLedger`, `useCreateLedgerEntry`.
- **WP-2.2** (FE allowances) — `AllowanceSummary` + `AllowanceSheet` + `useAllowances` + `allowance-format.ts`, currently mounted **on `ledger.tsx`**. WP-2.2 §0 explicitly deferred their final home to this WP: *"WP-3.3 handoff: when `members/[userId].tsx` is built, move the allowance section + `AllowanceSheet` mount there (it may absorb/delete `ledger.tsx`)."* This WP executes that move (§1).
- **WP-3.2** (FE loans tab) — **must be merged first** (the spec is written assuming it lands; per the task note the implementer starts only after it merges). Provides `loansApi` + `export type Loan`, `useLoans`/`useRequestLoan`/`useApproveLoan`/`useRejectLoan`/`useCloseLoan` (`app/src/hooks/useLoans.ts`), the `loans.tsx` tab, `LoanApproveSheet`, and the `Loans` tab registered in `_layout.tsx`. **Note the drift from `docs/specs/WP-3.2.md`:** the *as-merged* WP-3.2 renders loan cards **inline in `loans.tsx`** and did **not** create a reusable `LoanCard` component, a `loan-format.ts`, or a `LoanRequestSheet` component — this spec is written against the **as-merged** shape (verified on `origin/wp-3.2-loans-ui`), not the WP-3.2 spec's proposed file set. WP-3.3 therefore builds its own compact read-only loan presentation and **does not** import a `LoanCard` (there is none) and **does not** modify `loans.tsx`.

**Acceptance (master-plan §9 WP-3.3 row):** "`members/[userId]` for head per §7.1 incl. add settlement/adjustment via Sheet. — *head can run a whole month (approve chores, view balance, settle) from this page.*"

---

## 0. Goal & guardrails

Give the head **one screen per member** — reached by tapping that member's card on the group Overview — from which they can run the member's whole month: **see the balance** (incl. negative "owes you"), **approve/reject pending entries inline**, **set/change/pause the pocket money**, **see the member's active loans**, and **record a settlement (payout) or adjustment** via a Sheet. Master-plan §7.1 names this file `groups/[id]/members/[userId].tsx`; today its function lives in the interim `groups/[id]/ledger.tsx` (WP-1.3). This WP **promotes** that screen to its named home and adds the loans surface.

### The core realization (scope-shaping)

**`ledger.tsx` already implements almost all of the WP-3.3 acceptance.** It renders: the member's balance header (`bal < 0 → 'owes you'`), the `AllowanceSummary` + editable `AllowanceSheet` (WP-2.2), an **"Add entry"** button opening `AddEntrySheet` in `mode="head"` with `fixedUserId={userId}` — which **already** offers Chore / **Settlement** / **Adjustment** (`AddEntrySheet.tsx:25`, `162-165`) — and the full per-member `LedgerList` with inline Approve/Reject (`handleApprove`/`handleReject`). It already redirects non-heads (`if (!isHead) return <Redirect …>`). So "add settlement/adjustment via Sheet" and "approve chores / view balance / settle" are **done**; WP-3.3's genuinely-new work is:
1. **Relocate** `ledger.tsx` → `members/[userId].tsx` and repoint navigation (§1, §2).
2. **Add a read-only active-loans section** for the member (§6).
3. **Move** the allowance UI along with the screen (executing the WP-2.2 handoff) — a relocation, not a rewrite (§1).

This is deliberately a lean WP. Do not gold-plate it.

### In scope
- Create `app/app/(app)/groups/[id]/members/[userId].tsx` by evolving the current `ledger.tsx` content (§1, §4).
- **Delete** `app/app/(app)/groups/[id]/ledger.tsx` (§1, §3) — no dead duplicate screen (product principle §1.5 "no dead tabs").
- Repoint `MemberCard` navigation in `index.tsx` (head branch) to the new route; register the hidden route + drop the `ledger` tab entry in `_layout.tsx` (§2).
- Add a **read-only** "Loans" section to the member detail: the member's `active`/`requested` loans (outstanding, EMI, progress) via `useLoans(id)` filtered client-side to `userId`, with a **"Manage in Loans tab"** affordance (§6). **No** approve/reject/close here.
- Verification floor (§10). Barrel/exports only if a new component is extracted (§3 keeps it inline → none required).

### Explicitly OUT of scope (do not build)
- **No backend / `openapi.yaml` / migration / codegen changes.** WP-3.3 consumes only endpoints already in `openapi.yaml` (balance, `ledger?user_id=`, allowances, loans). `npm run codegen` is **not** run (no contract change). If you find a genuine contract gap, note it in `docs/notes/` (master-plan §10.2) — do not invent endpoints.
- **No `GET /groups/:id/members/:userId/summary` endpoint.** Master-plan §6.1 lists a `{balance, month_earned, allowance, active_loans[], recent_entries[]}` summary endpoint, **but it was never implemented** — WP-3.1 §0 explicitly deferred it and it is absent from `backend/openapi.yaml` (verified: no `…/summary` path in master or the WP-3.2 branch). **The FE composes the page client-side from the existing per-entity queries** (exactly as `ledger.tsx` already does), which is cheaper than a new endpoint and needs no backend work. **Do not** call, add, or generate a summary endpoint. (Flag for backlog: a future WP could add it as an N+1 optimization; not needed at family scale.)
- **No loan management on this page.** Approve / reject / close / request live on the **Loans tab** (WP-3.2). The member-detail loans section is **read-only** and deep-links to that tab. This avoids duplicating loan-mutation logic and keeps WP-3.3 from touching `loans.tsx` (which may still be settling from review) → no merge-conflict surface.
- **No new settlement/adjustment/allowance Sheet.** Reuse `AddEntrySheet` (settlement/adjustment) and `AllowanceSheet` (allowance) verbatim. Do **not** fork them.
- **No `LoanCard`/`loan-format.ts` extraction.** WP-3.2 as-merged didn't create them; extracting one now would touch `loans.tsx` for no acceptance benefit. Render the read-only loan rows inline in the member-detail file (§6). *(If a later cleanup WP extracts a shared `LoanCard`, both screens can adopt it then — note it, don't do it here.)*
- **No EMI `k/n` ledger-row label.** `ledger-format.ts:53-55` still renders `emi` rows as `EMI — <note>` (the WP-3.2 spec proposed `EMI k/n` but the merge didn't land it). That's a `ledger-format` concern, not this screen's — leave it; do not special-case EMI rows here.
- **No member-facing version of this page.** It is **head-only** (§8). Members see their own summary on their Overview (`index.tsx` member branch) — untouched here.
- **No legacy-screen sweep, no dashboard changes** — WP-4.1/4.2.

---

## 1. The relocation decision (ledger.tsx → members/[userId].tsx)

**Decision: rename-in-place.** Create `app/app/(app)/groups/[id]/members/[userId].tsx` containing the evolved `ledger.tsx` logic, and **delete `ledger.tsx`**. The allowance section + `AllowanceSheet` mount **move with it** (they currently live in `ledger.tsx`). This is the move WP-2.2 §0 pre-authorized.

**Why relocate rather than add a parallel page:**
- Master-plan §7.1's nav map names the file `members/[userId].tsx` explicitly; `ledger.tsx` was always the WP-1.3 *interim* landing spot (WP-1.3 §5.3 anticipated it being absorbed).
- Keeping both would be a **dead duplicate surface** — two head-only per-member screens doing the same thing — violating product principle §1.5 ("no dead tabs, no silent failures").
- The only navigation into `ledger.tsx` today is `MemberCard.onPress` in `index.tsx` (`index.tsx:139-145`); repointing that single call site + the `_layout.tsx` registration is the entire routing change (§2).

**userId source changes from param to path.** `ledger.tsx` reads `userId` from `useLocalSearchParams` as a **query param** (passed by `MemberCard.onPress`). In `members/[userId].tsx`, `userId` is a **path segment**, so `useLocalSearchParams<{ id: string; userId: string; name?: string }>()` yields it from the path. `name` continues to travel as a query param (for the header title) — expo-router merges path + query params in `useLocalSearchParams`. Keep the existing `name`-based `Stack.Screen` title.

**Allowance answer (the WP-2.2 open question — "does allowance editing move to member detail or stay linked?"): it MOVES.** The head's allowance set/change/pause UI (`AllowanceSummary` with `onEdit` + the `AllowanceSheet` mount) relocates from `ledger.tsx` into `members/[userId].tsx` unchanged — same props, same `useAllowances`/`useSetAllowance`, same current/upcoming derivation. The member's **read-only** `AllowanceSummary` on their own Overview (`index.tsx` member branch, `index.tsx:195-200`) is **untouched**. No allowance code is rewritten — it is cut-and-pasted with the screen.

---

## 2. Route + navigation entry points

### 2.1 Navigation into the page (`index.tsx`, head branch — changed)
The head reaches member detail by **tapping a member card on the group Overview**. Repoint the existing `MemberCard.onPress` (`app/app/(app)/groups/[id]/index.tsx:139-145`) from the `ledger` route to the new nested route:

```tsx
// BEFORE (index.tsx:139-145)
onPress={() =>
  router.push({
    pathname: `/(app)/groups/${id}/ledger`,
    params: { userId: member.user_id, name: member.name },
  })
}
// AFTER
onPress={() =>
  router.push({
    pathname: `/(app)/groups/${id}/members/${member.user_id}`,
    params: { name: member.name },
  })
}
```
`userId` now rides the **path** (`/members/${member.user_id}`); only `name` stays a query param. (If the typed-route form complains, the object form `{ pathname: '/(app)/groups/[id]/members/[userId]', params: { id, userId: member.user_id, name: member.name } }` is equivalent — pick whichever `tsc` accepts and state it.) This is the **only** navigation change; nothing else routes into `ledger`.

### 2.2 Route registration (`_layout.tsx` — changed)
`ledger.tsx` is registered as a hidden tab: `<Tabs.Screen name="ledger" options={{ href: null }} />` (`_layout.tsx`, the block after the `loans` tab). Replace it with a hidden registration for the new nested route and **remove** the `ledger` entry (its file is deleted):

```tsx
// remove the ledger Tabs.Screen block; add:
<Tabs.Screen
  name="members/[userId]"
  options={{ href: null }}   // reached via router.push from a member card, not a tab
/>
```
The visible tabs stay **Overview | Chores | Loans** (post-WP-3.2). `members/[userId]` is a detail screen, not a tab — `href: null` keeps it out of the tab bar exactly as `ledger` was. **Verify** in the web export / interactive run that (a) no phantom "members" tab appears and (b) tapping a member card lands on the detail screen with the correct header title. Expo-router registers every route file under the layout as a screen; the `name` must match the route path relative to the layout **including the nested segment** (`members/[userId]`). If the tab bar still shows a stray entry, that's the registration `name` not matching — fix the `name` string, do **not** work around it by keeping `ledger.tsx`.

### 2.3 Non-head guard (authz mirror — keep)
`ledger.tsx` already guards `if (!isHead) return <Redirect href={\`/(app)/groups/${id}/\`} />` (`ledger.tsx:89-91`). **Carry it over verbatim** so a member who somehow deep-links to `/groups/:id/members/:someUserId` is bounced to their own Overview. This mirrors the API (a member `GET /ledger?user_id=<other>` / `GET /balance` is server-scoped to self — WP-1.2 §3) — the redirect is UI-only defense; the API is the real gate (master-plan §10.5). Keep the guard **after** all hooks are called (§9).

---

## 3. File plan

**Created**
- `app/app/(app)/groups/[id]/members/[userId].tsx` — the head member-detail page (evolved from `ledger.tsx` + loans section).

**Changed**
- `app/app/(app)/groups/[id]/index.tsx` — repoint `MemberCard.onPress` (head branch) to the new route (§2.1). **Member branch untouched.**
- `app/app/(app)/groups/[id]/_layout.tsx` — swap the hidden `ledger` `Tabs.Screen` for a hidden `members/[userId]` one (§2.2).

**Deleted**
- `app/app/(app)/groups/[id]/ledger.tsx` — absorbed into `members/[userId].tsx` (§1).

**Not touched (guard against scope creep)**
- `app/app/(app)/groups/[id]/loans.tsx`, `LoanApproveSheet.tsx`, `useLoans.ts` (WP-3.2 — consume, don't edit).
- `AddEntrySheet.tsx`, `AllowanceSheet.tsx`, `AllowanceSummary.tsx`, `LedgerList.tsx`, `LedgerRow.tsx`, `ledger-format.ts` (reuse as-is).
- `app/src/api.ts`, `api-types.gen.ts`, `query-keys.ts` (no new endpoint/key).
- `MemberCard.tsx` (its `onPress` is passed from `index.tsx`; the component is unchanged).

> **No new component, no barrel edit, no new hook.** The loans section is rendered inline in the member-detail file (§6). If the implementer chooses to extract a tiny presentational `MemberLoanRow`, that's allowed but not required — keep the barrel/export churn minimal and state the choice.

---

## 4. Data requirements per section

All data comes from **existing hooks/queries** — the page is a client-side composition (§0, no summary endpoint). Call **every** hook unconditionally before any early return (§9).

| Section | Source | Notes |
|---|---|---|
| **Header: balance** | `useBalance(id)` → find `b.user_id === userId` | `bal < 0` → `variant="debit"` + "owes you"; else `variant="credit"` + "owed". Balance can be **negative** after EMIs (WP-3.1 §8) — render it, never clamp. Same as `ledger.tsx:97-113`. |
| **Header: allowance** | `useAllowances(id)` filtered to `userId`; `currentAllowanceFor`/`upcomingAllowanceFor` (`allowance-format.ts`) | `AllowanceSummary` with `onEdit` (head can edit) → opens `AllowanceSheet`. Degrade gracefully: `!allowancesQuery.isError && <AllowanceSummary …/>` (WP-2.2 §8.8). Moved verbatim from `ledger.tsx:114-120`. |
| **Pending entries + inline approve/reject** | `useLedger(id, { user_id: userId })` → `LedgerList` with `isHead={true}`, `onApprove`/`onReject`, `processingId` | Pending rows render at the top (newest-first month grouping) with Approve/Reject buttons; rejected rows strike through; only approved count to the month total. `LedgerList` already does all of this — no separate "pending" list needed. `handleApprove`/`handleReject` (with `confirmAsync` on reject) carry over from `ledger.tsx:53-83`. |
| **Allowance edit** | `AllowanceSheet` (WP-2.2) mounted with `groupId`, `userId`, `memberName={name}`, `current={currentAllow}` | Verbatim from `ledger.tsx:161-168`. |
| **Active loans (read-only)** | `useLoans(id)` filtered client-side to `loan.user_id === userId` | §6. `useLoans(id)` returns the head-scope full group list; filter to this member. No mutation here. |
| **Recent ledger (filtered to member)** | `useLedger(id, { user_id: userId })` — the same query as pending | The `LedgerList` **is** the recent-ledger view (month-grouped, all entry types incl. `allowance`/`emi`/`settlement`/`adjustment`). No second query. |
| **Add settlement/adjustment** | `AddEntrySheet` `mode="head"` `fixedUserId={userId}` | §5. Verbatim from `ledger.tsx:152-159`. |

**Layout** (keep `ledger.tsx`'s structure): a summary `Card`/header block (balance + allowance) → **"Add entry"** primary button → the **Loans** section (§6, only when the member has loans) → the `LedgerList` (fills remaining space, owns pull-to-refresh). One primary action per screen = "Add entry" (master-plan §7.4). Wire `loansQuery.refetch()` into the existing pull-to-refresh `onRefresh` alongside `ledgerQuery`/`balanceQuery`/`allowancesQuery` (`ledger.tsx:143-147`).

---

## 5. Settlement + adjustment (reuse `AddEntrySheet`) — direction semantics

**No new sheet.** The head records a payout or a correction from `AddEntrySheet` (`mode="head"`, `fixedUserId={userId}`), which already exposes the entry-type picker Chore / Settlement / Adjustment (`AddEntrySheet.tsx:156-166`). Mount it exactly as `ledger.tsx:152-159`.

**Direction semantics (verified against WP-1.2 — state this explicitly in the PR):**

- **Settlement = the head pays the member out = a `debit`.** Balance is *what the head currently owes the member* (master-plan §1, §5.1: `balance = Σ approved credits − Σ approved debits`). Paying cash out **reduces** what's owed, so a settlement **lowers** the balance → it is a **`debit`**. Per **WP-1.2 §3 / §5.1**, `direction='debit'` for settlements is **assigned server-side** — the FE sends only `{ entry_type: 'settlement', user_id, amount, note? }` and must **not** send `direction`. `AddEntrySheet.tsx:100-107` does exactly this (no `direction` in the settlement payload). ✔ Verified.
- **Adjustment = an explicit correction in either direction.** The head chooses credit (add to balance) or debit (subtract). Per WP-1.2 §3, `direction` is **required** for adjustments and **client-supplied**; `AddEntrySheet.tsx:108-116` sends `{ entry_type: 'adjustment', user_id, amount, direction, note? }` with the picker value. ✔ Verified. Amounts are always stored **positive**; the sign is carried by `direction` (WP-1.2 §5.1).
- Both are **head-only** and post **`approved`** immediately with `decided_by`/`decided_at` = head/now (server-set — WP-1.2 §3). The FE does not set status/decider. The member-detail page is head-only (§8), so mounting the head `AddEntrySheet` is authz-consistent.
- **Money input:** `AddEntrySheet` parses the amount with `parseMoneyToMinorUnits` (`AddEntrySheet.tsx:76,101,109`) — never `parseFloat`; `null` (0/blank/negative/>2dp) is a validation error. Inherited, not re-implemented. ✔

Ledger rendering of the resulting rows (Settlement `−₹` red, Adjustment `±₹` by direction, month-grouped, `decided_by` caption) is already handled by `LedgerRow`/`ledger-format.ts` — verify on a real posted row, don't rebuild (§10 interactive).

---

## 6. Loans section (read-only) — the one genuinely-new piece

Surface the member's loans on their detail page so the head sees repayment status without leaving for the Loans tab, **but keep all loan mutations on the Loans tab** (WP-3.2). Read-only here.

### Data
- `const loansQuery = useLoans(id ?? '');` — the head-scope query returns **all** group loans (WP-3.1 §5.2, API-enforced); filter to the member:
  ```ts
  const memberLoans = (loansQuery.data ?? []).filter(l => l.user_id === userId);
  const activeLoans = memberLoans.filter(l => l.status === 'active');
  const requestedLoans = memberLoans.filter(l => l.status === 'requested');
  ```
  (Reuse the **existing** `useLoans(id)` + `qk.loans(groupId)` cache — do **not** add a filtered hook/key. Client-side filter is correct and shares the Loans-tab cache, so opening the tab and the detail page don't double-fetch.)
- Show the section only when `activeLoans.length + requestedLoans.length > 0` (no empty "Loans" block cluttering members with none). Closed/rejected loans are history — omit them here (they live on the Loans tab); optionally include a count ("2 past loans") but not required.

### Render (inline, read-only, kit tokens only)
A titled block ("Loans") above the `LedgerList`, each loan a compact `Card` row mirroring the visual language of `loans.tsx`'s card (so it reads as the same object) **without** the action buttons:
- **Active loan:** headline `AmountText minorUnits={loan.principal} variant="neutral"`; a status `StatusBadge` (`Active` → `tone="success"`, matching `loans.tsx`'s `loanStatusTone`); detail rows — **Outstanding** `AmountText minorUnits={loan.outstanding} variant="debit" size="sm"`, **EMI** `≈ ₹{formatMinor(loan.emi_amount)} / month`, **Progress** `{loan.installments_posted}/{loan.installments} paid`. `loan.note` as a muted caption if present. (All fields are **server-computed** — WP-3.1 §5.1; the FE only formats.)
- **Requested loan:** headline principal + `StatusBadge` `Pending` (`tone="warning"`); caption `₹{formatMinor(loan.principal)} over {loan.installments} months` + `≈ ₹{formatMinor(loan.emi_amount)} / month` estimate. No actions (approval is the head's job on the Loans tab).
- **Guard `start_period`**: it is `null` until a loan is `active` (WP-3.1 §5.1). If you show a start month, guard it (`loan.start_period ? humanMonth(loan.start_period) : …`) — a `requested` loan has none.
- **Deep-link affordance:** a `Button variant="ghost" size="sm" title="Manage in Loans tab"` (or the whole section header) that navigates to the Loans tab: `router.push(\`/(app)/groups/${id}/loans\`)`. This is where approve/reject/close/request live. Keep it a navigation, not a mutation.

> Keep the loan rows **presentational and self-contained in this file** — no `useApproveLoan`/`useRejectLoan`/`useCloseLoan` imports here (that would duplicate WP-3.2's mutation surface and tempt scope creep). If `loansQuery.isError`, degrade gracefully like the allowance section — render nothing for the Loans block (the ledger/balance are the primary content), don't error the whole screen.

---

## 7. Query keys + invalidation matrix

**No new query keys, no new mutations.** WP-3.3 reuses existing hooks; their invalidation is already correct. The page **reads** four queries and **triggers** three existing mutations (approve/reject ledger via WP-1.3, set allowance via WP-2.2, create entry via WP-1.3). For reference, the keys this page reads and what refreshes them:

| Query on this page | Key | Refreshed by (existing mutations) |
|---|---|---|
| `useBalance(id)` | `qk.balance(groupId)` | `useApproveLedger`/`useRejectLedger`/`useCreateLedgerEntry` (WP-1.3), `useSetAllowance` (WP-2.2), `useApproveLoan`/`useCloseLoan` (WP-3.2) all invalidate `balance`. |
| `useLedger(id, { user_id })` | `qk.ledger(groupId, { user_id })` | Same mutations invalidate the **prefix** `['ledger', groupId]` → every filter variant incl. this member's, so approve/reject/settle/adjust refresh the list in place. |
| `useAllowances(id)` | `qk.allowances(groupId)` | `useSetAllowance` invalidates `allowances` (+ ledger/balance/group). |
| `useLoans(id)` | `qk.loans(groupId)` | Loan mutations (on the Loans tab) invalidate `loans` (+ ledger/balance/group). Since this page shares the `qk.loans(groupId)` cache, a loan approved on the Loans tab shows fresh state here on next focus. |

**The only thing WP-3.3 must get right on invalidation is that it reuses the shared keys** (`useLoans(id)`, not a bespoke filtered key) so the caches stay coherent across the Loans tab and the detail page. It adds **no** `onSuccess` logic of its own. `refetchOnWindowFocus` (app-wide default) + the pull-to-refresh wiring (§4) cover cross-screen freshness.

---

## 8. Role-based rendering + authz mirror

**This is a head-only page.** Enforcement, in order:
1. **API (real gate):** a member calling `GET /balance` / `GET /ledger?user_id=<other>` is server-scoped to themselves (WP-1.2 §3); settlement/adjustment creates are head-only 403 (WP-1.2 §3); `PUT /allowances/:userId` is head-only (WP-2.1). A non-head literally cannot fetch another member's data or mutate it.
2. **UI mirror (this WP):** compute `isHead` from `group.members` (`useGroup`), and `if (!isHead) return <Redirect href={\`/(app)/groups/${id}/\`} />` — carried from `ledger.tsx:89-91`, placed **after all hooks** (§9). A non-head never sees the page chrome.
3. **Reachability:** the only entry point is `MemberCard`, rendered only in the **head branch** of `index.tsx` (`index.tsx:93` `if (isHead)`), so members have no in-app path here anyway; the redirect defends deep links.

Everything the page offers (approve/reject, allowance edit, settlement/adjustment) is a head action; there is **no** member-facing variant to render. The loans section is read-only for the head too (management is on the Loans tab). Mirror = hide what the role can't do (master-plan §10.5) — here, the whole page.

---

## 9. Kit / convention reuse (carried from WP-1.0/1.3/2.2/3.2 — read before coding)

1. **`Alert.alert` is banned** for confirms/feedback (no-op on `react-native-web`). Reject confirm → `confirmAsync`; all feedback → `useToast().show(...)`. Inherited from `ledger.tsx` (already compliant); any new toast/confirm on the loans deep-link or errors must follow suit. Grep-gated (§10).
2. **Money in, money out.** Display via `AmountText`/`formatMinorUnits`/`formatMinor`; input via `parseMoneyToMinorUnits`. **Never `parseFloat`.** All money on this page is either display (loans/balance) or flows through the reused `AddEntrySheet`/`AllowanceSheet` (already compliant). The loans section is **display-only** — format `principal`/`outstanding`/`emi_amount` with `formatMinor`/`AmountText`, compute nothing. Grep-gated (§10).
3. **Hooks above early returns.** `members/[userId].tsx` must call **every** hook — `useAuth`, `useGroup`, `useBalance`, `useLedger`, `useChores`, `useAllowances`, `useLoans`, `useApproveLedger`, `useRejectLedger`, all `useState` — **before** `if (isLoading) return …`, the `!isHead` redirect, or the `error` return. `ledger.tsx` already orders it this way (`ledger.tsx:33-51` hooks, then `85-95` returns); preserve it when you add `useLoans`. Miss it → "rendered fewer hooks than expected" when the role/loading branch flips. Reviewer checks this (§11).
4. **Sheet prefill staleness (`009d78c`).** The reused `AllowanceSheet` already re-prefills on the closed→open edge via the `prevVisible` ref (`AllowanceSheet.tsx`, commit `009d78c`); `AddEntrySheet` resets on close. WP-3.3 mounts them **unchanged** — do not regress the prefill by wrapping them or resetting `current`/`fixedUserId` on every render. Pass `current={currentAllow}` (derived, stable) and `fixedUserId={userId}` (stable) exactly as `ledger.tsx` does.
5. **Token-driven styling only** — `theme.color.*`/`theme.spacing.*`/`theme.fontSize.*`; no inline hex / raw spacing in the new loans section (match `loans.tsx`'s style object idiom).
6. **Every list/section gets empty + loading + error states.** Ledger: `LedgerList` handles empty; page handles `isLoading` (`LoadingSpinner`) and `error` (`ErrorMessage`) as `ledger.tsx` does. Loans section: hidden when none; hidden on `loansQuery.isError` (graceful degrade, §6). Allowance: hidden on `allowancesQuery.isError` (WP-2.2 §8.8).
7. **AuthZ mirrored in UI, enforced by API** (§8). The redirect is UI-only; the API is the gate.

---

## 10. Verification floor

Run from `app/`. State in the PR **exactly** what ran. FE floor per master-plan §10.3. **No `npm run codegen` / `codegen:check`** — WP-3.3 changes no contract (§0).

```bash
cd app

# 1. Types — primary gate.
npx tsc --noEmit

# 2. Lint — no new errors.
npm run lint

# 3. Web export — MANDATORY (§10.3 FE floor; tsc/expo-doctor miss bundle-level breakage,
#    and this WP touches routing — a bad Tabs.Screen name only shows at bundle/runtime).
npx expo export --platform web     # must exit 0, emit dist/

# 4. Guard: no new native deps.
git diff --stat package.json       # expect: no change

# 5. Grep gates — search BOTH app/app AND app/src:
#  (a) ledger.tsx is gone and nothing still routes to it:
test ! -e "app/(app)/groups/[id]/ledger.tsx" && echo "ledger.tsx deleted OK"
grep -rn "groups/\${id}/ledger\|name=\"ledger\"\|/ledger'" app/app app/src   # expect: none (all repointed)
#  (b) the new route + member-card repoint exist:
grep -rn "members/\[userId\]\|members/\${" app/app                            # expect: _layout registration + index.tsx onPress
#  (c) no parseFloat anywhere new/changed:
grep -rn "parseFloat" app/app/\(app\)/groups                                   # expect: none
#  (d) no Alert.alert for confirm/feedback outside confirm.ts:
grep -rn "Alert.alert" app/app app/src | grep -v "src/confirm.ts"             # expect: none
#  (e) loans section is read-only (no loan mutations imported into member detail):
grep -n "useApproveLoan\|useRejectLoan\|useCloseLoan\|useRequestLoan" "app/app/(app)/groups/[id]/members/[userId].tsx"
#      expect: none (read-only; only useLoans)
```

**Interactive (if a browser/dev server is available — `npm run web`):** the acceptance is *"head can run a whole month (approve chores, view balance, settle) from this page"* on web + Android.
- **Head:** open a group → Overview → tap a member card → lands on **member detail** (header title `"<name>'s ledger"` or equivalent; **no** stray "members" tab in the bar). See the balance (green `owed` / red `owes you` if negative), the **Pocket money** section with **Edit/Set**, and the member's ledger.
- **Approve a chore:** a pending chore row shows Approve/Reject → tap **Approve** → toast, row flips to approved, balance updates. Tap **Reject** on another → `confirmAsync` (web `window.confirm`) → strikethrough, excluded from total.
- **Set allowance:** **Set** → `AllowanceSheet` (a `Sheet`, opens on web) → ₹500 This month → save → "Pocket money — <month>" credit row appears, balance updates; set ₹1000 Next month → "Changing to … from <NextMonth>". (WP-2.2 behavior, verified still works post-move.)
- **Settle:** **Add entry** → Settlement → ₹300 → Save → a red `−₹300.00` **Settlement** row posts and the balance **drops** by ₹300 (debit). **Adjustment** → pick credit/debit → posts with the chosen sign. (Verifies §5 direction semantics end-to-end.)
- **Loans section:** for a member with an active loan, the **Loans** block shows outstanding / EMI / `n/m paid`; **Manage in Loans tab** navigates to the Loans tab. No approve/close buttons on the detail page.
- **Non-head guard:** as a member, deep-link `/(app)/groups/<id>/members/<otherUserId>` → redirected to own Overview (no data leak).
- **Both platforms** (web + Android): sheets, `confirmAsync`, toasts appear — **no `Alert.alert` no-op**.

State in the PR what ran locally (tsc/lint/expo export web + greps) vs. what needs a device/browser (the interactive matrix).

---

## 11. Gotchas / likely slips (reviewer checklist)

1. **Dead `ledger.tsx` / dangling tab.** The single most likely slip: creating `members/[userId].tsx` but forgetting to (a) delete `ledger.tsx`, (b) remove its `Tabs.Screen name="ledger"`, or (c) repoint `MemberCard.onPress`. Any one left behind = a dead screen, a phantom tab, or a broken tap. All three must change together (§1, §2). Grep-gated (§10a).
2. **Expo-router nested-route registration.** `members/[userId]` is a **nested** route under the Tabs `_layout.tsx`; the `Tabs.Screen name` must be the full relative path `"members/[userId]"`, not `"members"` or `"[userId]"`. Wrong name → a stray tab or a 404 on navigation. Verify in the web export/interactive run that no "members" tab appears and the card tap resolves (§2.2, §10). Don't paper over it by keeping `ledger.tsx`.
3. **`userId` is now a path param, not a query param.** In `ledger.tsx` it came from `router.push` params; in `members/[userId].tsx` it comes from the path segment via `useLocalSearchParams`. If you copy `ledger.tsx` verbatim, `userId` still resolves (params merge), but the **navigation call** in `index.tsx` must put `userId` in the path (`/members/${member.user_id}`), else the path param is empty and every query keys on `''` (§2.1).
4. **Hooks above early returns — regression risk when adding `useLoans`.** Insert `useLoans(id)` **with the other hooks at the top**, not after the `!isHead` redirect / `isLoading` return. Adding it below a return re-introduces the conditional-hook bug (§9.3).
5. **Loans section is READ-ONLY.** Do not import or wire `useApproveLoan`/`useRejectLoan`/`useCloseLoan`/`useRequestLoan` here — management is the Loans tab's job (WP-3.2). Grep-gated (§10e). Rendering approve/close buttons here would duplicate logic and touch scope the acceptance doesn't ask for.
6. **Don't modify `loans.tsx` / `AddEntrySheet` / `AllowanceSheet`.** WP-3.3 reuses them. Editing `loans.tsx` risks conflicting with a still-settling WP-3.2; forking the sheets duplicates code. Consume, don't edit (§0, §3).
7. **No summary endpoint.** Don't reach for `GET /members/:userId/summary` — it doesn't exist (§0). Compose from `useBalance`/`useLedger`/`useAllowances`/`useLoans`. Inventing it (or a hook for it) is out of scope and would fail at runtime (404).
8. **`start_period`/`outstanding` guards on loans.** `start_period` is `null` for `requested`/`rejected` loans; `outstanding` equals `principal` before any EMI. Guard `start_period` before formatting a month; don't assume a repayment has started (§6).
9. **Negative balance is expected.** After EMIs the member's balance can be `< 0` (WP-3.1 §8) — the header already renders `−₹…` "owes you". Do not clamp or hide it (carried from `ledger.tsx:97,108,111`).
10. **Settlement direction is server-set — don't send it.** The reused `AddEntrySheet` correctly omits `direction` for settlements (server assigns `debit`) and includes it only for adjustments (§5). If anyone "fixes" the settlement payload to send `direction`, that's a regression — the API owns it (WP-1.2 §3).
11. **Allowance section moved, not duplicated.** Ensure the allowance UI exists on the new page and is **removed** from `ledger.tsx` (which is deleted anyway) — and is **not** accidentally also added to the head branch of `index.tsx` (it belongs on member detail for the head; the member's own read-only copy on `index.tsx` member branch stays). §1, §4.
12. **Graceful degrade on partial errors.** `loansQuery.isError` / `allowancesQuery.isError` must **not** blank the page — hide those sections; balance + ledger remain the primary, always-rendered content (§6, §9.6).

---

## 12. Definition of Done (checklist)

- [ ] `app/app/(app)/groups/[id]/members/[userId].tsx` created from the evolved `ledger.tsx`: balance header (incl. negative "owes you"), `AllowanceSummary`+`AllowanceSheet` (editable), **"Add entry"** → `AddEntrySheet` (`mode="head"`, `fixedUserId`, offers settlement/adjustment), `LedgerList` (member-filtered, inline approve/reject), non-head `<Redirect>` — all **hooks above early returns** (§1, §4, §9).
- [ ] Read-only **Loans section**: member's `active`/`requested` loans via `useLoans(id)` filtered to `userId` (outstanding/EMI/`n/m paid`), "Manage in Loans tab" deep-link, **no loan mutations**, hidden when none/on error (§6).
- [ ] `ledger.tsx` **deleted**; `_layout.tsx` swaps the hidden `ledger` `Tabs.Screen` for a hidden `members/[userId]` one; `index.tsx` `MemberCard.onPress` repointed to `/members/${member.user_id}` (§1, §2). Visible tabs stay Overview | Chores | Loans; no phantom tab.
- [ ] Non-head guard carried over (redirect to own Overview); authz mirrors the API (§8).
- [ ] Settlement (`debit`, server-assigned direction) and adjustment (client `direction`) semantics verified against WP-1.2 §3/§5.1 and stated in the PR; money via `parseMoneyToMinorUnits`, no `parseFloat`; no `Alert.alert` (§5, §9).
- [ ] Allowance UI **moved** to member detail (WP-2.2 handoff executed); member's own read-only allowance on `index.tsx` untouched (§1).
- [ ] No new endpoint/codegen/query-key/hook; no summary endpoint; `loans.tsx`/`AddEntrySheet`/`AllowanceSheet` unedited (§0, §3, §7).
- [ ] `tsc --noEmit` + `lint` + `expo export --platform web` green; no new dep; grep gates clean (ledger.tsx gone + nothing routes to it, member route wired, no parseFloat, no Alert.alert, loans read-only) (§10).
- [ ] Interactive matrix stated in PR (approve chore, set allowance, settle, adjustment, loans read-only, non-head redirect) on web + (where possible) Android (§10).

Commit/PR title: **`WP-3.3 spec: FE member detail page`** (this spec) — implementation PR later titled **`WP-3.3: FE member detail page`**.
