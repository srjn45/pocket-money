# WP-2.2 Spec: FE Allowances UI

**Work package:** WP-2.2 (Phase 2 — Allowances), master-plan §9. **Second and final WP of Phase 2.**
**Type:** Frontend-only. Adds the head's **set/edit/pause allowance** UI and the member's **read-only own-allowance** view, consuming the **WP-2.1 allowance API** (`GET /groups/:id/allowances`, `PUT /groups/:id/allowances/:userId`). Regenerates the committed FE types from the post-WP-2.1 `openapi.yaml` (**this WP owns the regeneration**). **No backend, `openapi.yaml`, or migration changes.**
**Risk:** Low FE. No money math of its own (all formatting via `AmountText`/`formatMinorUnits`, all input via `parseMoneyToMinorUnits`); all authz is API-enforced and mirrored in the UI (§5.5 matrix). Master-plan §11: Sonnet implement, Sonnet review; review must check that types were regenerated, empty/paused/error states exist, no `Alert.alert`, and no client-side posting logic.

**Depends on (all merged to master before implement starts):**
- **WP-1.0** (design-system kit) — `theme.ts` tokens + `Button`, `Card`, `ListRow`, `AmountText`, `StatusBadge`, `Sheet`, `TextField`, `MonthHeader`, `Toast`, `EmptyState`/`ErrorMessage`/`LoadingSpinner`; `confirmAsync`; `Sheet`+`Toast`+`confirmAsync` are the **only** confirm/feedback path (`Alert.alert` is banned — WP-1.0 §4).
- **WP-1.1** (money in minor units) — API money is integer minor units; `AmountText` takes minor units directly; `parseMoneyToMinorUnits`/`formatMinorUnits`/`formatMinor` live in `app/src/money.ts`.
- **WP-1.3** (FE unified ledger) — the unified ledger screens (`groups/[id]/index.tsx` both roles + `groups/[id]/ledger.tsx` head per-member) are live on the kit; `LedgerRow`/`LedgerList`/`AddEntrySheet`/`MemberCard`/`Avatar` exist; `ledger-format.ts` already maps `allowance` rows to **"Pocket money — Jul 2026"** with the `cash-outline` icon and a signed credit amount; `useLedger`/`useBalance` + query-keys are in place. `api-types.gen.ts` is WP-1.3-current.
- **WP-2.1** (BE allowances + posting engine) — **in strict Opus review at spec-authoring time; will be merged to master before this WP's implementer starts.** It adds the two allowance paths + `AllowanceResponse`/`SetAllowanceRequest` schemas to `openapi.yaml`, and the lazy posting trigger on `GET /groups/:id`, `/ledger`, `/balance`. See `docs/specs/WP-2.1.md` for the exact contract.

> **Sequencing reality (2026-07-05):** at spec-authoring time the WP-2.1 backend delta is **not yet in this worktree's `openapi.yaml`** (`grep -c allowances backend/openapi.yaml` → 0). WP-2.2 is written against the **landed WP-2.1 contract** documented in `docs/specs/WP-2.1.md §4/§7`. The very first implementation step (`npm run codegen`, §1) regenerates types from whatever `openapi.yaml` is at that point — which **must** be post-WP-2.1. **STOP-gate:** if `AllowanceResponse` / `SetAllowanceRequest` are missing from `api-types.gen.ts` after codegen, WP-2.1 has not landed — **stop and flag**, do not hand-write the types.

**Acceptance (master-plan §9 WP-2.2 row):** "Head: set/edit allowance on member detail (WP-3.3 dependency-lite: can live on Overview member-card sheet until then); member: allowance visible in summary; allowance rows render in ledger. — *change allowance → next month posts new amount; history not rewritten.*"

---

## 0. Goal & guardrails

Give the head a way to **set, change, and pause** each member's monthly pocket money, and give the member a **read-only** view of their own allowance — entirely on the WP-1.0 kit, consuming the WP-2.1 API. Posting is **server-side and lazy** (§5.4); the FE never posts, never computes a due month, never touches the ledger insert path — it just **mutates the allowance config and refetches**, and the already-live ledger renders the resulting `allowance` credit rows.

### Where the UI lives (the design decision)

Master-plan §7.1 puts allowance editing on `members/[userId].tsx` (HEAD-only member-detail page) — but **that page is WP-3.3 and does not exist yet** (`app/app/(app)/groups/[id]/` has only `_layout/index/chores/ledger`). Per the WP-2.2 row's explicit "*dependency-lite: can live on Overview member-card sheet until then*", **WP-2.2 hangs the allowance UI off the existing WP-1.3 per-member ledger screen** (`groups/[id]/ledger.tsx`) — the screen the head already reaches by tapping a member card (WP-1.3 §5.3). That screen already shows the member's balance header + Add-entry + full ledger; WP-2.2 adds a **"Pocket money" allowance section** to its header with an **Edit** control that opens an `AllowanceSheet`.

- **Head** manages a member's allowance from `groups/[id]/ledger.tsx` (per-member view). **Member** sees their own allowance, read-only, in their own Overview summary header (`groups/[id]/index.tsx`, member branch).
- **WP-3.3 handoff:** when `members/[userId].tsx` is built, move the allowance section + `AllowanceSheet` mount there (it may absorb/delete `ledger.tsx` per WP-1.3 §5.3). The `AllowanceSheet`/`AllowanceSummary` components + `useAllowances` hook + `allowance-format.ts` helpers are built reusable so the move is a re-mount, not a rewrite. Record this; do not build `members/[userId].tsx` now.

### In scope
- `npm run codegen` → regenerate `api-types.gen.ts` from the post-WP-2.1 `openapi.yaml` (§1). This WP owns it.
- `app/src/api.ts`: `allowancesApi` (`list`, `set`) + `export type Allowance` (§2).
- `app/src/hooks/useAllowances.ts`: `useAllowances(groupId)` (GET) + `useSetAllowance(groupId)` (PUT) with the full invalidation set (§3).
- `app/src/allowance-format.ts`: pure helpers `currentPeriod`, `nextPeriod`, `currentAllowanceFor`, `upcomingAllowanceFor`, `describeAllowance` (§4) — unit-testable, no React.
- `app/src/components/AllowanceSummary.tsx`: read/display component (member + head header) (§5).
- `app/src/components/AllowanceSheet.tsx`: head set/change/pause form (`Sheet` + `TextField` + `confirmAsync`/`Toast`) (§6).
- Wire the summary into `groups/[id]/ledger.tsx` (head, editable) and `groups/[id]/index.tsx` (member, read-only) (§7).
- Barrel exports; verification floor (§8, §9).

### Explicitly OUT of scope (do not build)
- **No backend / `openapi.yaml` / migration changes.** WP-2.2 only *consumes* the WP-2.1 contract and regenerates types. If the contract is wrong, note it in `docs/notes/` (master-plan §10.2) — do not edit `openapi.yaml`.
- **No client-side posting logic.** Posting is lazy and server-side (WP-2.1 §5). The FE must never compute due months, insert ledger rows, or call any "post now"/preview endpoint (none exists — WP-2.1 §0). After a mutation it **refetches**; the server posts on the next `GET`.
- **No `members/[userId].tsx` member-detail page** — WP-3.3. WP-2.2 hangs allowance UI off the existing `ledger.tsx` (see above).
- **No loans / EMI UI** — Phase 3 (WP-3.2). The ledger already renders `emi` rows defensively (WP-1.3 §2.2); do not add loan data or controls.
- **No legacy-screen sweep** (login/register/dashboard/profile/invite) — WP-4.1.
- **No new native dependencies** (keeps `expo export --platform web` green — §9). Reuse `@react-native-picker/picker` (already used by `AddEntrySheet`) if a picker is needed for the effective-month control.
- **No weekly/other cadence, no proration UI.** Monthly-only; `effective_from` is `YYYY-MM` (master-plan §5.2).
- **No allowance history editing/deletion of past rows.** A change is a **new** `effective_from` row (past months immutable — WP-2.1 §5). The FE only ever `PUT`s a new/updated row.

---

## 1. Codegen — the FIRST step (this WP owns it)

Run from `app/`:
```bash
npm run codegen        # regenerate src/api-types.gen.ts from ../backend/openapi.yaml
```
This regenerates against the post-WP-2.1 `openapi.yaml`, which must now carry the two allowance paths + the `AllowanceResponse`/`SetAllowanceRequest` schemas (WP-2.1 §7). The `allowance`/`emi` `entry_type` enum values are **already** present in the committed types (WP-1.3 shipped them for defensive rendering) and in the ledger `?type=` filter — codegen leaves them unchanged.

**STOP-gate (mandatory, before writing any consumer):**
```bash
grep -n "AllowanceResponse\|SetAllowanceRequest" src/api-types.gen.ts
```
- Both present → proceed.
- Either missing → **WP-2.1 has not landed in `openapi.yaml`. STOP and flag.** Do **not** hand-write the types, do **not** patch `openapi.yaml` (that's WP-2.1's file). The forcing function is the same as WP-1.3 §4.7: committed spec ⇒ committed types.

Commit the regenerated `api-types.gen.ts`; `npm run codegen:check` must pass (no drift) at the verification floor (§9).

**Expected shape after codegen** (from WP-2.1 §7; verify, do not assume):
```ts
// components['schemas']['AllowanceResponse']
{ id: string; group_id: string; user_id: string;
  amount: number;            // minor units; 0 = paused
  effective_from: string;    // 'YYYY-MM'
  created_by: string; created_at: string; }

// components['schemas']['SetAllowanceRequest']
{ amount: number;            // required, >= 0; 0 pauses
  effective_from?: string | null; }   // optional 'YYYY-MM'; server defaults to current month
```

---

## 2. `app/src/api.ts` — `allowancesApi` + `Allowance` type

Add the type export alongside the others (line ~18-21 block):
```ts
export type Allowance = Schemas['AllowanceResponse'];
```

Add a new API object (mirror `choresApi`/`ledgerApi` style; note the `set` route is `PUT /groups/:id/allowances/:userId`):
```ts
// Allowances API (WP-2.1 contract)
export const allowancesApi = {
  // Head → all members' allowance history rows; member → own rows only (API-enforced).
  list: (groupId: string) =>
    request<Allowance[]>(`/groups/${groupId}/allowances`),

  // Head only. amount in minor units (0 = pause); effective_from optional 'YYYY-MM'
  // (server defaults to current month). A new effective_from is a new history row.
  set: (groupId: string, userId: string, data: { amount: number; effective_from?: string }) =>
    request<Allowance>(`/groups/${groupId}/allowances/${userId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
};
```
- `list` returns `Allowance[]` — **full history**, ordered by the server. The FE derives "current"/"upcoming" client-side (§4). Empty array when none (WP-2.1 §4.1).
- `set` body omits `effective_from` when the head chooses "This month" and could send it explicitly for "Next month" (§6). Either way the server validates `^\d{4}-\d{2}$` and defaults to the current month when absent (WP-2.1 §4.2).

---

## 3. `app/src/hooks/useAllowances.ts` (new)

Follow the `useChores.ts` / `useLedger.ts` pattern exactly (query + mutation + `useQueryClient`). Reuse the **already-reserved** `qk.allowances(groupId)` key (`query-keys.ts:11` — `allowances: (groupId) => ['allowances', groupId]`); **do not** add a filters arg (the endpoint takes none).

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { allowancesApi } from '../api';
import { qk } from '../query-keys';

export function useAllowances(groupId: string) {
  return useQuery({
    queryKey: qk.allowances(groupId),
    queryFn: () => allowancesApi.list(groupId),
    enabled: !!groupId,
  });
}

export function useSetAllowance(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, ...data }: { userId: string; amount: number; effective_from?: string }) =>
      allowancesApi.set(groupId, userId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.allowances(groupId) });      // the config just changed
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });          // partial key → all filter variants
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}
```

### Invalidation rationale (spell it out — reviewer checks this)
A `PUT /allowances/:userId` **does not itself post** any ledger entry (WP-2.1 §4.2 is explicit: "Do not trigger posting here — the next lazy read posts it"). But the mutation still invalidates **`ledger` + `balance` + `group`** in addition to **`allowances`**, because:
- Invalidating `ledger`/`balance`/`group` forces those queries to **refetch**, and each of those `GET`s runs the **server-side lazy posting trigger** (WP-2.1 §5). So a "set current-month allowance" immediately materializes the current-month `allowance` credit row + updated balance on the next render — **without any client-side posting**.
- `qk.ledger(groupId)` is the **prefix** `['ledger', groupId]`; invalidating it matches every `['ledger', groupId, filters]` cache entry, so the head's whole-group pending query, the per-member ledger, and the member's own ledger all refresh (same mechanism WP-1.3 §4.2 relies on).
- `qk.balance` / `qk.group` refresh the member-card balances + header balance the head is looking at.

This is the master-plan §7.3 rule ("mutations invalidate the affected group's keys") applied to allowances, and the concrete mechanism by which "change allowance → next month posts new amount" (the acceptance criterion) works with a purely lazy server engine.

---

## 4. `app/src/allowance-format.ts` (new) — pure helpers

Pure, React-free, unit-testable (mirrors `ledger-format.ts`). Keeping the "current"/"upcoming" derivation here (a) makes it testable and (b) means the display components stay dumb. Reuse `humanMonth` from `ledger-format.ts` for month labels — **do not** duplicate the month-name array.

```ts
import type { Allowance } from './api';

/** Current server-local month as 'YYYY-MM'. (Server is the clock authority — master-plan §10.7 —
 *  but the current month is a safe client-side default for the effective-from control; the server
 *  re-defaults/validates it on PUT.) */
export function currentPeriod(now: Date = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  return `${y}-${m}`;
}

/** The month after `period` ('2026-12' → '2027-01'). Integer month arithmetic (no Date rollover). */
export function nextPeriod(period: string): string {
  const [y, m] = period.split('-').map(Number);
  const total = y * 12 + (m - 1) + 1;          // next month, 0-indexed months
  const ny = Math.floor(total / 12);
  const nm = String((total % 12) + 1).padStart(2, '0');
  return `${ny}-${nm}`;
}

/** The allowance in force at `period` = the row with the greatest effective_from <= period.
 *  Returns null if none applies yet (all rows are future). Lexical compare works for zero-padded YYYY-MM. */
export function currentAllowanceFor(rows: Allowance[], period: string): Allowance | null {
  let best: Allowance | null = null;
  for (const r of rows) {
    if (r.effective_from <= period && (best === null || r.effective_from > best.effective_from)) {
      best = r;
    }
  }
  return best;
}

/** The earliest scheduled future change (smallest effective_from > period), or null. */
export function upcomingAllowanceFor(rows: Allowance[], period: string): Allowance | null {
  let best: Allowance | null = null;
  for (const r of rows) {
    if (r.effective_from > period && (best === null || r.effective_from < best.effective_from)) {
      best = r;
    }
  }
  return best;
}

/** Human label for a current allowance: null → 'Not set'; amount 0 → 'Paused'; else '₹X.XX / month'. */
export function describeAllowance(a: Allowance | null): string {
  if (a === null) return 'Not set';
  if (a.amount === 0) return 'Paused';
  return `${formatMinorUnits(a.amount)} / month`;   // import formatMinorUnits from './money'
}
```
> `describeAllowance` returns a plain label for captions; where a colored `₹` element is wanted, render the amount via `AmountText`/`formatMinorUnits` directly and reserve `describeAllowance` for the `null`/`Paused` cases (§5). Implementer: import `formatMinorUnits` from `./money`.

**Optional but encouraged:** `app/src/allowance-format.test.ts` covering `nextPeriod` (year boundary Dec→Jan), `currentAllowanceFor` (empty rows, all-future rows, amount-change selection at/after `effective_from`, amount-0 paused row returned as current), `upcomingAllowanceFor`. `money.test.ts` already establishes the jest-style test convention in this repo. (Not a hard gate — the FE floor in §9 does not run unit tests in CI — but cheap and matches the codebase.)

---

## 5. `app/src/components/AllowanceSummary.tsx` (new) — display

A small, presentational block shown in both the member Overview header (read-only) and the head per-member ledger header (with Edit). No data fetching inside — the screen passes derived values.

```ts
import type { Allowance } from '../api';

interface AllowanceSummaryProps {
  /** In-force allowance for the current month, or null if none set. */
  current: Allowance | null;
  /** Earliest scheduled future change, or null. Renders a "Changing to … from <Month>" caption. */
  upcoming?: Allowance | null;
  /** When provided, renders an Edit/Set control (head only). Omit for the member's read-only view. */
  onEdit?: () => void;
}
```

Render (built from kit tokens/components — no inline hex):
- A row: label **"Pocket money"** (`fontSize.sm`, `textSecondary`).
- Value:
  - `current === null` → muted **"Not set"** (`textMuted`).
  - `current.amount === 0` → **"Paused"** as a `StatusBadge tone="neutral"` (or muted text) — never a `₹0.00` credit.
  - else → `<AmountText minorUnits={current.amount} variant="neutral" />` followed by a `"/ month"` caption (`textSecondary`). Use `variant="neutral"` — an allowance rate is not a signed balance (same call as the chore rate in `AddEntrySheet`).
- If `upcoming` is set → a caption line (`fontSize.xs`, `textSecondary`): `"Changing to {describeAllowance(upcoming)} from {humanMonth(upcoming.effective_from)}"` (reuse `humanMonth` from `ledger-format.ts`; for a paused upcoming, `describeAllowance` yields "Paused" → "Changing to Paused from …", acceptable).
- If `onEdit` → a trailing `<Button variant="ghost" size="sm" title={current ? 'Edit' : 'Set'} icon="pencil" onPress={onEdit} />`.

Keep it a compact `View` (not a full `Card`) so it slots inside the existing summary header blocks in `ledger.tsx` / `index.tsx` without visual duplication of the surrounding card.

---

## 6. `app/src/components/AllowanceSheet.tsx` (new) — head set/change/pause

A kit `<Sheet title="Pocket money">` form; **head-only** (only mounted from `ledger.tsx`). Uses `TextField` + `confirmAsync` + `Toast`; **no `Alert.alert`**.

```ts
import type { Allowance } from '../api';

interface AllowanceSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  userId: string;                 // the member whose allowance is being set
  memberName?: string;            // for the sheet title/caption
  current: Allowance | null;      // in-force allowance (prefill the amount field)
}
```

### Fields
1. **Amount** — `<TextField label="Monthly amount (₹)" keyboardType="decimal-pad" value={amountStr} onChangeText={setAmountStr} error={amountError} placeholder="e.g. 500" />`. Prefill from `current` when set and not paused: `amountStr = current && current.amount > 0 ? formatMinor(current.amount) : ''` (`formatMinor` gives `"500.00"` without sign/symbol — WP-1.0 §3.1). Parse on submit with `parseMoneyToMinorUnits` (**never `parseFloat`** — master-plan §10.6, grep-gated §9).
2. **Effective month** — a `Picker` (reuse `@react-native-picker/picker`) with exactly two options, both derived client-side (no free-form date entry):
   - **"This month" → `currentPeriod()`** (default; send by omitting `effective_from` so the server owns the default, or send it explicitly — both are valid per WP-2.1 §4.2).
   - **"Next month (…)" → `nextPeriod(currentPeriod())`** with the month name shown, e.g. `"Next month (Aug 2026)"` via `humanMonth`.
   This directly serves the acceptance criterion "change allowance → next month posts new amount": the head picks Next month, and the change is scheduled without rewriting the current month. Keep it to these two — arbitrary back/forward-dating is out of scope (past months are immutable server-side anyway; a future-only or current/next choice is the useful, safe subset).
3. **Pause control** — a dedicated affordance (a `Button variant="danger" title="Pause allowance"` inside the sheet, or a switch). **This is required and separate from the amount field** because `parseMoneyToMinorUnits('0')` returns `null` (it enforces `> 0`, `money.ts:20`) — you **cannot** pause by typing 0. Pausing submits `{ amount: 0, effective_from }` explicitly (0 = paused, WP-2.1 §5.2). Only show "Pause" when `current && current.amount > 0` (nothing to pause otherwise). When already paused, the amount field simply lets the head set a new positive amount to **resume**.

### Submit
- **Set/resume:** parse `amountStr`; `null` → set `amountError` "Enter a valid amount (e.g. 500)." and abort (no submit). Valid → `useSetAllowance(groupId).mutate({ userId, amount, effective_from })` where `effective_from` is the picked month (omit for "This month" or pass `currentPeriod()`).
- **Pause:** `confirmAsync({ title: 'Pause pocket money', message: 'No pocket money will be posted from the selected month until you set a new amount.', confirmLabel: 'Pause', destructive: true })` → if ok, `mutate({ userId, amount: 0, effective_from })`.
- Save disabled while the mutation is pending (`Button loading={isPending}`), and while the amount is invalid for the set path.
- Success → `onClose()` + `Toast{ tone:'success', message: 'Pocket money updated' }`. Error → `Toast{ tone:'danger', message }` (the API message, e.g. "cannot set an allowance for the group head" / "target user is not a member of this group" — WP-2.1 §4.2; these shouldn't occur given the sheet is only opened for a valid non-head member, but surface them faithfully).
- Reset local state on close (mirror `AddEntrySheet`'s `resetForm`/`handleClose`).

> **Guard the head-target case at the call site, not just the API:** `ledger.tsx` only mounts `AllowanceSheet` for the member whose card was tapped (always a non-head member — the head has no member card / balance). Do not offer allowance editing for the head themselves (WP-2.1 §4.2 rejects it with 400).

---

## 7. Screen wiring

### 7.1 `app/app/(app)/groups/[id]/ledger.tsx` (head per-member — changed)

This is the head's per-member view (already shows balance header + Add-entry + `LedgerList`). Add allowance management:
- `const allowancesQuery = useAllowances(id ?? '');`
- Derive the target member's rows + current/upcoming:
  ```ts
  const period = currentPeriod();
  const memberAllowances = (allowancesQuery.data ?? []).filter(a => a.user_id === userId);
  const currentAllow = currentAllowanceFor(memberAllowances, period);
  const upcomingAllow = upcomingAllowanceFor(memberAllowances, period);
  ```
  (The head GET returns **all** members' rows; filter to `userId`. Member GET is scoped server-side but this screen is head-only — the `!isHead` redirect already guards it.)
- Render `<AllowanceSummary current={currentAllow} upcoming={upcomingAllow} onEdit={() => setAllowanceSheetVisible(true)} />` inside the summary-header region (below the balance, above/beside "Add entry").
- Mount `<AllowanceSheet visible={allowanceSheetVisible} onClose={() => setAllowanceSheetVisible(false)} groupId={id ?? ''} userId={userId ?? ''} memberName={name} current={currentAllow} />`.
- **No posting logic here.** After `useSetAllowance` succeeds its `onSuccess` invalidates ledger/balance/group/allowances; the screen's existing `useLedger`/`useBalance` refetch (triggering server-side posting) and the new `useAllowances` refetch — the new `allowance` credit row and updated balance appear on their own. Wire `allowancesQuery.refetch()` into the existing pull-to-refresh `onRefresh` alongside `ledgerQuery`/`balanceQuery`.

### 7.2 `app/app/(app)/groups/[id]/index.tsx` (member Overview — changed)

Member branch only (the head branch shows member cards, not a single allowance). In the member summary header (below "Your balance"):
- `const allowancesQuery = useAllowances(id ?? '');` (member GET returns **own rows only**, API-enforced).
- `const currentAllow = currentAllowanceFor(allowancesQuery.data ?? [], currentPeriod());`
- `const upcomingAllow = upcomingAllowanceFor(allowancesQuery.data ?? [], currentPeriod());`
- Render `<AllowanceSummary current={currentAllow} upcoming={upcomingAllow} />` — **no `onEdit`** (read-only; members cannot manage their own allowance — §5.5). Members with no allowance see "Not set"; paused members see "Paused".
- **Do not** add allowance UI to the head branch of `index.tsx` (the head's per-member allowance lives in `ledger.tsx`, §7.1). Do not touch `MemberCard` (balance + pending count only; adding allowance there is out of scope and would clutter the list).

### 7.3 Ledger row rendering — already done (verify, don't rebuild)

`allowance` ledger rows already render correctly from WP-1.3:
- `ledger-format.ts` `entryTitle` → **"Pocket money — Jul 2026"** (`humanMonth(entry.period)`; `ledger-format.ts:49-52`).
- `LedgerRow` `TYPE_ICONS.allowance` → `cash-outline` (`LedgerRow.tsx:13`).
- Amount signed by `direction` (`credit` → green `+₹`), no status badge (machine-posted → `approved`), no decided-by line (`decided_by` is NULL on machine posts — WP-1.3 §2.5), grouped under `entry.period`'s month via `monthKeyOf` (`ledger-format.ts:38-40`).

**Required polish: none beyond verification.** Confirm on a real posted allowance row (§9 interactive / after a set) that the title, icon, green credit, and month grouping render as above. If anything is off, fix it *in the shared `ledger-format.ts`/`LedgerRow`* (not per-screen) — but the WP-1.3 implementation already handles it, so expect a no-op here. **Do not** special-case allowance rows in the screens.

### 7.4 Barrel + files

`app/src/components/index.ts` — add:
```ts
export { AllowanceSummary } from './AllowanceSummary';
export { AllowanceSheet } from './AllowanceSheet';
```

**Created**
- `app/src/hooks/useAllowances.ts`
- `app/src/allowance-format.ts` (+ optional `allowance-format.test.ts`)
- `app/src/components/AllowanceSummary.tsx`
- `app/src/components/AllowanceSheet.tsx`

**Changed**
- `app/src/api-types.gen.ts` (regenerated — §1)
- `app/src/api.ts` (§2: `allowancesApi` + `Allowance` type)
- `app/src/components/index.ts` (barrel: 2 exports)
- `app/app/(app)/groups/[id]/ledger.tsx` (§7.1)
- `app/app/(app)/groups/[id]/index.tsx` (member branch — §7.2)

**Deleted** — none.

---

## 8. Kit / convention reuse (carried from WP-1.0/1.3 — read before coding)

1. **`Alert.alert` is banned** for confirms/feedback (no-op on `react-native-web`). Pause confirm → `confirmAsync`; all feedback → `useToast().show(...)`. Grep-gated (§9).
2. **Money in, money out.** Input parses via `parseMoneyToMinorUnits` (rejects `0`, `<0`, `>2dp`); display via `AmountText` / `formatMinorUnits` / `formatMinor`. **Never `parseFloat`** on money (master-plan §10.6). Grep-gated (§9).
3. **Amount 0 = paused, and `parseMoneyToMinorUnits('0')` is `null`.** Pausing must go through the dedicated Pause control sending `amount: 0` explicitly — you cannot type 0 into the amount field (§6). This is the single most likely implementation slip; the reviewer must check it.
4. **All create/edit forms use `Sheet`** (bottom sheet native / centered modal web) — never a bare `Modal` or `Alert`-driven form (WP-1.0 §3.7). `AllowanceSheet` follows `AddEntrySheet`'s structure (footer with Cancel/Save `Button`s, `resetForm` on close).
5. **Token-driven styling only** — no inline hex / raw spacing in new components (`theme.color.*`, `theme.spacing.*`, etc.). Acceptance grep in WP-1.0 §8.
6. **AuthZ mirrored in UI, enforced by API.** Member view is read-only (no `onEdit`); head editing only for non-head members. The API is the real gate (WP-2.1 §4) — the UI just hides what the role can't do (master-plan §10.5).
7. **No client-side posting / no due-month math for the ledger.** The only month arithmetic the FE does is picking the *effective_from* for the config PUT (This/Next month) and deriving *current/upcoming* for display — never inserting or computing ledger entries. Posting is the server's (WP-2.1 §3/§5).
8. **Every list/section gets empty + paused + loading + error states.** `AllowanceSummary` handles null ("Not set") and paused ("Paused"); the screens already have loading/error via `LoadingSpinner`/`ErrorMessage`. An allowances query error should not blank the whole screen — the balance/ledger are the primary content; degrade the allowance section gracefully (e.g. hide it or show "Not set") rather than erroring the screen. Prefer: if `allowancesQuery.isError`, render nothing for the allowance section (the ledger/balance still work).

---

## 9. Verification floor

Run from `app/`. State in the PR **exactly** what ran. FE floor per master-plan §10.3.

```bash
cd app

# 1. Regenerate committed types from the post-WP-2.1 openapi — THE FIRST STEP (§1).
npm run codegen
grep -n "AllowanceResponse\|SetAllowanceRequest" src/api-types.gen.ts   # STOP-gate: both MUST be present
npm run codegen:check          # exit 0 → committed spec ⇒ committed types, no drift

# 2. Types — primary gate.
npx tsc --noEmit

# 3. Lint — no new errors.
npm run lint

# 4. Web export — MANDATORY (§10.3 FE floor; tsc/expo-doctor miss bundle-level breakage).
npx expo export --platform web     # must exit 0, emit dist/

# 5. Guard: no new native deps (should print nothing new vs. master).
git diff --stat package.json

# 6. Grep gates — search BOTH app/app AND app/src (consumption lives in shared
#    components/hooks; WP-1.3 §6 missed a field because it only grepped app/app):
#  (a) allowance surface actually wired (types, api, hook, components, screens):
grep -rn "allowancesApi\|useAllowances\|useSetAllowance\|AllowanceResponse\|SetAllowanceRequest" app/src app/app
     # expect: api.ts (allowancesApi + Allowance type), useAllowances.ts (both hooks),
     #         AllowanceSheet/AllowanceSummary, ledger.tsx + index.tsx consumers
#  (b) no parseFloat on money anywhere new/changed:
grep -rn "parseFloat" app/src app/app        # expect: none in WP-2.2 files (money via parseMoneyToMinorUnits)
#  (c) no Alert.alert for confirm/feedback outside confirm.ts:
grep -rn "Alert.alert" app/app app/src | grep -v "src/confirm.ts"    # expect: none
#  (d) pause path sends amount 0 explicitly, not via the parser:
grep -rn "amount: 0" app/src/components/AllowanceSheet.tsx           # expect: the pause submit
```

**Interactive (if a browser/dev server is available — `npm run web`):**
- **Head:** open a group → tap a member card → per-member ledger. See the **Pocket money** section ("Not set" for a fresh member). Tap **Set** → `AllowanceSheet` opens (on web too — it's a `Sheet`, not `Alert`). Set ₹500, "This month" → save → toast "Pocket money updated"; the section shows "₹500.00 / month" and a **"Pocket money — <ThisMonth>"** credit row appears in the ledger with the balance updated (posting happened server-side on the invalidation-driven refetch — no client posting). Set ₹1000 effective **Next month** → section shows "Changing to ₹1000.00 / month from <NextMonth>"; the current month's posted row is **unchanged**. Tap **Pause allowance** → confirm dialog (web `window.confirm`) → paused; section shows "Paused"; no new row posts for the paused month.
- **Member:** open the group → own Overview → summary header shows the read-only **Pocket money** line ("₹500.00 / month" or "Paused"/"Not set"); **no Edit control**; the posted allowance credit rows appear in the member's own month-grouped ledger.
- **Both platforms** (web + Android per the §9 roadmap row): sheet + confirm + toast all appear (no `Alert.alert` no-op).

State in the PR what ran locally (codegen/codegen:check/tsc/lint/expo export web + greps) vs. what needs a device/browser (the interactive matrix).

---

## 10. Definition of Done (checklist)

- [ ] `npm run codegen` run; `api-types.gen.ts` regenerated & committed; **`AllowanceResponse` + `SetAllowanceRequest` present** (STOP-gate passed); `codegen:check` green (§1).
- [ ] `allowancesApi` (`list`, `set`) + `export type Allowance` in `api.ts`; `set` → `PUT /groups/:id/allowances/:userId` (§2).
- [ ] `useAllowances` (GET) + `useSetAllowance` (PUT) hooks; `useSetAllowance.onSuccess` invalidates **allowances + ledger + balance + group** (§3); reuses `qk.allowances` (no filters arg added).
- [ ] `allowance-format.ts` pure helpers (`currentPeriod`, `nextPeriod`, `currentAllowanceFor`, `upcomingAllowanceFor`, `describeAllowance`); reuse `humanMonth`/`formatMinorUnits`, no duplicated month array (§4).
- [ ] `AllowanceSummary` — "Pocket money" display with **Not set / Paused / ₹X / month** + optional "Changing to … from <Month>"; `onEdit` only when head (§5).
- [ ] `AllowanceSheet` — head set/change form on `Sheet`; amount via `parseMoneyToMinorUnits`; **This month / Next month** effective picker; **dedicated Pause control** sending `amount: 0` (not via the parser) behind a `confirmAsync`; `Toast` on success/error; no `Alert.alert` (§6).
- [ ] `ledger.tsx` (head per-member): `AllowanceSummary` + `AllowanceSheet` wired for the tapped member; `useAllowances` refetch in pull-to-refresh; **no client-side posting** (§7.1).
- [ ] `index.tsx` (member branch): read-only `AllowanceSummary` for own allowance; head branch untouched; `MemberCard` untouched (§7.2).
- [ ] Allowance ledger rows verified rendering via the existing WP-1.3 `ledger-format`/`LedgerRow` (title "Pocket money — <Mon Year>", `cash-outline`, green credit, month-grouped) — no per-screen special-casing, no rebuild (§7.3).
- [ ] Barrel exports `AllowanceSummary` + `AllowanceSheet` (§7.4).
- [ ] `tsc --noEmit` + `lint` + `expo export --platform web` all green; no new dep in `package.json`; grep gates clean incl. searching `app/src` (not just `app/app`) (§9).
- [ ] Out-of-scope respected: no backend/openapi/migration edits; no `members/[userId].tsx`; no loans/EMI UI; no legacy sweep; no weekly cadence/proration; no client posting (§0).

Commit/PR title: **`WP-2.2 spec: FE allowances UI`** (this spec) — implementation PR later titled **`WP-2.2: FE allowances UI`**.
