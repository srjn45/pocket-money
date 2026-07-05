# WP-3.2 Spec: FE Loans Tab

**Work package:** WP-3.2 (Phase 3 — Loans & EMI), master-plan §9. **Second WP of Phase 3** (backend is WP-3.1; head member-detail page is WP-3.3).
**Type:** Frontend-only. Adds the **Loans tab** to group detail: the member's **request-a-loan** flow, the head's **approve (editable terms) / reject / close** decisions, a **loan card** with repayment progress, and a **label tweak** so EMI ledger rows read `EMI k/n`. Consumes the **WP-3.1 loans API** (`GET/POST /groups/:id/loans`, `POST /loans/:id/{approve,reject,close}`). Regenerates the committed FE types from the post-WP-3.1 `openapi.yaml` (**this WP owns the regeneration**). **No backend, `openapi.yaml`, or migration changes.**
**Risk:** Low FE. No money math of its own beyond parsing one principal field (`parseMoneyToMinorUnits`) and a plain-integer installments count; **EMI is server-computed** (§5.3 of WP-3.1) — the FE never computes the schedule or posts a ledger row. All authz is API-enforced and mirrored in the UI (§8 matrix). Master-plan §11: Sonnet implement, Sonnet review; review must check that types were regenerated, empty/requested/active/closed/error states exist, no `Alert.alert`, no client-side EMI math or posting logic, and the sheet-prefill / hooks-before-early-return gotchas are handled.

**Depends on (all merged to master before implement starts):**
- **WP-1.0** (design-system kit) — `theme.ts` tokens + `Button`, `Card`, `ListRow`, `AmountText`, `StatusBadge`, `Sheet`, `TextField`, `MonthHeader`, `Toast`, `EmptyState`/`ErrorMessage`/`LoadingSpinner`; `confirmAsync`; `Sheet`+`Toast`+`confirmAsync` are the **only** confirm/feedback path (`Alert.alert` is banned — WP-1.0 §4, no-op on `react-native-web`).
- **WP-1.1** (money in minor units) — API money is integer minor units; `AmountText` takes minor units directly; `parseMoneyToMinorUnits`/`formatMinorUnits`/`formatMinor` live in `app/src/money.ts`.
- **WP-1.3** (FE unified ledger) — unified ledger screens (`groups/[id]/index.tsx` both roles + `groups/[id]/ledger.tsx` head per-member) are live on the kit; `LedgerRow`/`LedgerList`/`ledger-format.ts` exist. `ledger-format.ts` `entryTitle` already renders `emi` rows defensively as `EMI — <note>` / `EMI` (`ledger-format.ts:53-56`); `LedgerRow` `TYPE_ICONS.emi` → `card-outline` (`LedgerRow.tsx:16`), amount signed by `direction` (`debit` → red `−₹`). `qk.loans(groupId)` is **already reserved** (`query-keys.ts:10` — `loans: (groupId) => ['loans', groupId]`).
- **WP-2.2** (FE allowances UI) — the pattern this WP mirrors exactly: `hooks/useAllowances.ts` (query + mutation + invalidation), `components/AllowanceSheet.tsx` (Sheet + TextField + confirm + Toast, with the **closed→open re-prefill effect**, commit `009d78c`), `components/AllowanceSummary.tsx`, `allowance-format.ts` (pure, React-free helpers), `api.ts` `allowancesApi` shape, and the `ledger.tsx` / `index.tsx` wiring. **Read WP-2.2.md and its implementation before writing a line here** — every new file has a WP-2.2 sibling.
- **WP-3.1** (BE loans + EMI) — the **contract this WP consumes**. It adds the five loan paths + `CreateLoanRequest`/`ApproveLoanRequest`/`LoanResponse` schemas + `Loans` tag to `openapi.yaml`, migration 011, and EMI posting in the lazy engine. See `docs/specs/WP-3.1.md §5/§6/§9` for the exact contract.

> **Sequencing / drift caveat (2026-07-05):** WP-3.1's backend was merged (`521b0bb`, PR #19) but its reviewer **may still be finalizing `openapi.yaml` on its branch**. **This spec is authored against the WP-3.1 contract as documented in `docs/specs/WP-3.1.md §5/§6/§9`.** The first implementation step (`npm run codegen`, §1) regenerates types from whatever `openapi.yaml` is at that point. If the generated `LoanResponse` field set diverges from §1's "Expected shape" (e.g. a renamed field, a nullable that isn't), **the openapi drift is the source of truth for types but flag the mismatch** — reconcile the consumer code to the generated types and note the discrepancy in `docs/notes/` (master-plan §10.2). Do **not** hand-edit `openapi.yaml` (that's WP-3.1's file) and do **not** hand-write the generated types.

**Acceptance (master-plan §9 WP-3.2 row):** "Loans tab per §7.1: member request flow; head approve (editable terms)/reject/close; loan card w/ progress (paid n/m, outstanding); EMI rows in ledger. — *full request→approve→EMI-appears flow on web + Android.*"

---

## 0. Goal & guardrails

Give the **member** a way to request a zero-interest loan (enter principal + number of installments; the **server** computes the EMI), and the **head** a way to approve (optionally editing the terms), reject, or close (early payoff) loans. Render each loan as a **card** with status and repayment progress (`paid n/m`, outstanding, EMI amount). Verify (and lightly polish) that the **EMI debit rows** the WP-3.1 engine posts already show up in the existing ledger.

Posting is **server-side and lazy** (WP-3.1 §4/§5.4): the FE **never** computes an EMI schedule, a due month, or an outstanding amount for posting, and **never** inserts a ledger row. It mutates loan state (request / approve / reject / close) and **refetches**; the already-live ledger renders the resulting `emi` debit rows, and the balance query (which triggers the lazy engine on its `GET`) reflects the new debits — including going **negative** (WP-3.1 §8), already handled by the member Overview balance styling (`index.tsx:186-194` renders `debit` variant + "you owe" when `balance < 0`).

### Where the UI lives

Master-plan §7.1 puts loans on a dedicated **`loans.tsx`** tab in `groups/[id]/` (`Tabs: Overview | Chores | Loans`). That tab **does not exist yet** — `app/(app)/groups/[id]/` has only `_layout / index / chores / ledger`. This WP **creates `loans.tsx`** and registers it as the third visible tab in `_layout.tsx` (Overview | Chores | **Loans**). Both roles use the same tab; it branches on `isHead` (mirroring `index.tsx`).

### In scope
- `npm run codegen` → regenerate `api-types.gen.ts` from the post-WP-3.1 `openapi.yaml` (§1). This WP owns it.
- `app/src/api.ts`: `loansApi` (`list`, `request`, `approve`, `reject`, `close`) + `export type Loan` (§2).
- `app/src/hooks/useLoans.ts`: `useLoans(groupId)` (GET) + `useRequestLoan` / `useApproveLoan` / `useRejectLoan` / `useCloseLoan` mutations with the exact invalidation matrix (§3).
- `app/src/loan-format.ts`: pure, React-free helpers (`estimateEmi`, `loanStatusTone`, `loanStatusLabel`, `loanProgressLabel`, `outstandingLabel`, `emiInstallmentIndex`) (§4).
- `app/src/components/LoanCard.tsx`: display card (principal, EMI, `paid n/m`, outstanding, status badge, head action buttons) (§5).
- `app/src/components/LoanRequestSheet.tsx`: member request form (principal + installments → server computes EMI) (§6).
- `app/src/components/LoanApproveSheet.tsx`: head approve form with **editable** principal/installments overrides (§7).
- `app/app/(app)/groups/[id]/loans.tsx`: the tab (member + head branches) (§8.1).
- `app/app/(app)/groups/[id]/_layout.tsx`: register the Loans tab (§8.2).
- **Label tweak:** `ledger-format.ts` `entryTitle` (+ a `loan-format` helper) so `emi` rows read **`EMI k/n — <note>`** when loan context is available, else fall back to today's `EMI — <note>` (§9).
- Barrel exports; verification floor (§10, §11).

### Explicitly OUT of scope (do not build)
- **No backend / `openapi.yaml` / migration changes.** WP-3.2 only *consumes* the WP-3.1 contract and regenerates types. If the contract is wrong, note it in `docs/notes/` — do not edit `openapi.yaml`.
- **No client-side EMI math or posting.** The FE must never compute the EMI **schedule**, a **due month**, an **outstanding for posting**, insert a ledger row, or call any "post now"/preview endpoint (none exists — WP-3.1 §0). `emi_amount`, `installments_posted`, and `outstanding` all come **from the API** (`LoanResponse`). The only client-side arithmetic allowed is a **display-only** EMI estimate in the request/approve sheets (§4 `estimateEmi`), clearly labelled "≈", never sent to the server.
- **No head-initiated pre-approved loan creation UI.** The backend `POST /loans` accepts a head creating an `active` loan for a member (WP-3.1 §5.3), but the master-plan §9 WP-3.2 row scopes the head to **approve/reject/close**. Members request; the head decides. (A head "lend now" affordance can be a later polish — note it, don't build it.)
- **No `members/[userId].tsx` member-detail page** — WP-3.3. Do not build it; do not add loan management to it.
- **No interest / fees / partial member repayment / loan edit-after-decision** — none exist in the contract (WP-3.1 §0).
- **No new native dependencies** (keeps `expo export --platform web` green — §11). Reuse `@react-native-picker/picker` (already used by `AddEntrySheet`/`AllowanceSheet`) if a picker helps the installments field; a plain `TextField` is fine.
- **No legacy-screen sweep** — WP-4.1.

---

## 1. Codegen — the FIRST step (this WP owns it)

Run from `app/`:
```bash
npm run codegen        # regenerate src/api-types.gen.ts from ../backend/openapi.yaml
```
This regenerates against the post-WP-3.1 `openapi.yaml`, which must now carry the five loan paths + `CreateLoanRequest`/`ApproveLoanRequest`/`LoanResponse` schemas (WP-3.1 §9). The `emi` `entry_type` enum value + `loan_id` field are **already** in the committed types (WP-1.2/1.3 shipped them for defensive rendering) — codegen leaves them unchanged.

**STOP-gate (mandatory, before writing any consumer):**
```bash
grep -n "LoanResponse\|CreateLoanRequest\|ApproveLoanRequest" src/api-types.gen.ts
```
- All three present → proceed.
- Any missing → **WP-3.1's `openapi.yaml` delta has not landed. STOP and flag** (see the drift caveat above). Do **not** hand-write the types, do **not** patch `openapi.yaml`. Same forcing function as WP-2.2 §1 / WP-1.3 §4.7: committed spec ⇒ committed types.

Commit the regenerated `api-types.gen.ts`; `npm run codegen:check` must pass (no drift) at the verification floor (§11).

**Expected shape after codegen** (from WP-3.1 §5.1/§9; verify against the generated file, do not assume):
```ts
// components['schemas']['LoanResponse']
{ id: string; group_id: string; user_id: string;
  principal: number;            // minor units (paise), > 0
  installments: number;         // count, > 0
  emi_amount: number;           // ceil(principal/installments), minor units — SERVER-computed
  start_period?: string | null; // 'YYYY-MM'; null until active
  status: 'requested' | 'active' | 'rejected' | 'closed';
  note?: string | null;
  requested_at: string;
  decided_by?: string | null;
  decided_at?: string | null;
  installments_posted: number;  // count of posted EMI rows — SERVER-computed
  outstanding: number; }        // principal − Σ posted EMIs, minor units — SERVER-computed

// components['schemas']['CreateLoanRequest']
{ user_id?: string | null;      // borrower; ignored/self for member requests
  principal: number;            // minor units, >= 1
  installments: number;         // >= 1
  note?: string | null; }

// components['schemas']['ApproveLoanRequest']
{ principal?: number | null;    // optional override, >= 1
  installments?: number | null; }  // optional override, >= 1
```

---

## 2. `app/src/api.ts` — `loansApi` + `Loan` type

Add the type export alongside the others (the `line ~12-22` block, next to `Allowance`):
```ts
export type Loan = Schemas['LoanResponse'];
```

Add a new API object (mirror `allowancesApi`/`ledgerApi` style exactly):
```ts
// Loans API (WP-3.1 contract)
export const loansApi = {
  // Head → all loans in the group; member → own loans only (API-enforced).
  // No query params: we fetch the full list and section it client-side (§8.1),
  // matching the reserved qk.loans(groupId) key shape (no filters arg).
  list: (groupId: string) =>
    request<Loan[]>(`/groups/${groupId}/loans`),

  // Member requests a loan for themselves (server sets status='requested').
  // principal in minor units; installments a count. EMI is server-computed.
  request: (groupId: string, data: { principal: number; installments: number; note?: string }) =>
    request<Loan>(`/groups/${groupId}/loans`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // Head only. Optional principal/installments overrides; emi_amount is recomputed
  // server-side. requested → active, start_period = next calendar month.
  approve: (loanId: string, data?: { principal?: number; installments?: number }) =>
    request<Loan>(`/loans/${loanId}/approve`, {
      method: 'POST',
      body: JSON.stringify(data ?? {}),
    }),

  // Head only. requested → rejected.
  reject: (loanId: string) =>
    request<Loan>(`/loans/${loanId}/reject`, { method: 'POST' }),

  // Head only. Early payoff: posts one final EMI debit for the outstanding, active → closed.
  close: (loanId: string) =>
    request<Loan>(`/loans/${loanId}/close`, { method: 'POST' }),
};
```
- `list` returns `Loan[]` — the full set the caller is entitled to (head: all; member: own — API-enforced, WP-3.1 §5.2). Empty array when none.
- `request` omits `user_id` — the server forces self for member callers (WP-3.1 §5.3); sending someone else's id is a 400. Keep it out of the FE payload entirely.
- `approve` sends `{}` (not overrides) when the head accepts the requested terms verbatim; only includes `principal`/`installments` when the head edited them (§7).

---

## 3. `app/src/hooks/useLoans.ts` (new)

Follow `useAllowances.ts` exactly (query + mutations + `useQueryClient`). Reuse the **already-reserved** `qk.loans(groupId)` key; **do not** add a filters arg.

```ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { loansApi } from '../api';
import { qk } from '../query-keys';

export function useLoans(groupId: string) {
  return useQuery({
    queryKey: qk.loans(groupId),
    queryFn: () => loansApi.list(groupId),
    enabled: !!groupId,
  });
}

// Member: request a loan. Only the loans list changes (a `requested` loan posts
// nothing) — no ledger/balance effect until it is approved & an EMI is due.
export function useRequestLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { principal: number; installments: number; note?: string }) =>
      loansApi.request(groupId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
    },
  });
}

// Head: approve. requested → active, and the NEXT ledger/balance GET runs the lazy
// engine → the first due EMI debit materializes + balance changes. Invalidate the
// money keys too so that happens on the current screen without a manual refresh.
export function useApproveLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ loanId, ...overrides }: { loanId: string; principal?: number; installments?: number }) =>
      loansApi.approve(loanId, overrides),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });   // partial key → all filter variants
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}

// Head: reject. requested → rejected; no ledger entry, no balance effect.
export function useRejectLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => loansApi.reject(loanId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
    },
  });
}

// Head: close (early payoff). Posts a final EMI debit IMMEDIATELY + status → closed,
// so ledger + balance change now. Invalidate the money keys.
export function useCloseLoan(groupId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (loanId: string) => loansApi.close(loanId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.loans(groupId) });
      qc.invalidateQueries({ queryKey: qk.ledger(groupId) });
      qc.invalidateQueries({ queryKey: qk.balance(groupId) });
      qc.invalidateQueries({ queryKey: qk.group(groupId) });
    },
  });
}
```

### Invalidation matrix (spell it out — reviewer checks this)

| Mutation | `loans` | `ledger` | `balance` | `group` | Why |
|---|:--:|:--:|:--:|:--:|---|
| `useRequestLoan` | ✅ | — | — | — | A `requested` loan posts **no** ledger row (WP-3.1 §5.3). Only the loans list changes. |
| `useApproveLoan` | ✅ | ✅ | ✅ | ✅ | `requested`→`active`; the next `ledger`/`balance` **GET runs the lazy engine** → the first due EMI debit appears + balance updates. This is the acceptance's **approve→EMI-appears** path. |
| `useRejectLoan` | ✅ | — | — | — | `requested`→`rejected`; no money movement. |
| `useCloseLoan` | ✅ | ✅ | ✅ | ✅ | Close posts a final EMI debit **immediately** (WP-3.1 §5.6) + status change → ledger & balance change now. |

`qk.ledger(groupId)` is the **prefix** `['ledger', groupId]`; invalidating it matches every `['ledger', groupId, filters]` cache entry (the head's whole-group query, the per-member ledger, and the member's own ledger all refresh — same mechanism WP-1.3/WP-2.2 rely on). `qk.balance`/`qk.group` refresh the balance headers + member-card balances. This is the master-plan §7.3 rule ("mutations invalidate the affected group's keys") and the concrete mechanism by which "approve → EMI appears" works against a purely-lazy server engine — **no client-side posting** (§0).

---

## 4. `app/src/loan-format.ts` (new) — pure helpers

Pure, React-free, unit-testable (mirrors `allowance-format.ts` / `ledger-format.ts`). Reuse `humanMonth` from `ledger-format.ts` for month labels; reuse `formatMinorUnits`/`formatMinor` from `money.ts`. **No React, no `Alert`.**

```ts
import type { Loan, LedgerEntry } from './api';
import { formatMinorUnits } from './money';

/** Display-only EMI estimate = ceil(principal / installments) in minor units.
 *  Mirrors the server's integer-ceil (WP-3.1 §4.3) so the sheet preview matches the
 *  eventual server value, but is NEVER sent — the server computes the authoritative
 *  emi_amount. Returns null when inputs are not yet valid (so the caller shows nothing). */
export function estimateEmi(principalMinor: number | null, installments: number | null): number | null {
  if (principalMinor === null || installments === null) return null;
  if (principalMinor <= 0 || installments <= 0) return null;
  return Math.ceil(principalMinor / installments);   // integer ceil on minor units
}

/** StatusBadge tone for a loan status. */
export function loanStatusTone(status: Loan['status']): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
  switch (status) {
    case 'requested': return 'warning';   // awaiting head decision
    case 'active':    return 'info';       // repaying
    case 'closed':    return 'success';    // fully repaid / paid off
    case 'rejected':  return 'danger';
    default:          return 'neutral';
  }
}

/** Human label for the status badge. */
export function loanStatusLabel(status: Loan['status']): string {
  switch (status) {
    case 'requested': return 'Requested';
    case 'active':    return 'Active';
    case 'closed':    return 'Paid off';
    case 'rejected':  return 'Rejected';
    default:          return status;
  }
}

/** "2 / 5 paid" from the server-computed installments_posted + installments. */
export function loanProgressLabel(loan: Loan): string {
  return `${loan.installments_posted} / ${loan.installments} paid`;
}

/** "₹X outstanding" using the server-computed outstanding (minor units). */
export function outstandingLabel(loan: Loan): string {
  return `${formatMinorUnits(loan.outstanding)} outstanding`;
}

/** 1-based installment index k for an emi ledger row within its loan, and the loan's n.
 *  k = rank of this entry's period among that loan's emi periods present in `entries`
 *  (ascending). Returns null when the loan or the entry's loan_id/period is unavailable
 *  (→ caller falls back to the plain "EMI" label). See §9. */
export function emiInstallmentIndex(
  entry: LedgerEntry,
  entries: LedgerEntry[],
  loans: Loan[],
): { k: number; n: number } | null {
  if (entry.entry_type !== 'emi' || !entry.loan_id || !entry.period) return null;
  const loan = loans.find(l => l.id === entry.loan_id);
  if (!loan) return null;
  const periods = entries
    .filter(e => e.entry_type === 'emi' && e.loan_id === entry.loan_id && e.period)
    .map(e => e.period as string)
    .sort();                                  // lexical sort works for zero-padded YYYY-MM
  const k = periods.indexOf(entry.period) + 1;
  if (k <= 0) return null;
  return { k, n: loan.installments };
}
```
> `estimateEmi` is the **only** client-side loan arithmetic and is display-only (§0). Everything money-authoritative (`emi_amount`, `installments_posted`, `outstanding`) comes from `LoanResponse`.

**Optional but encouraged:** `app/src/loan-format.test.ts` covering `estimateEmi` (`1000_00/3 → 334_00`? — note: estimate is `ceil(100000/3)=33334` paise, i.e. ₹333.34; exact division `90000/3=30000`; invalid/zero inputs → null), `loanStatusTone`/`Label` for all four statuses, `loanProgressLabel`, `emiInstallmentIndex` (ordering, missing loan, non-emi entry). `money.test.ts` / `allowance-format.test.ts` establish the jest convention. Not a hard CI gate (§11) but cheap and matches the codebase.

---

## 5. `app/src/components/LoanCard.tsx` (new) — display + head actions

One loan rendered as a `Card` (kit component). Presentational + action callbacks passed in; **no data fetching inside**. Mirrors `MemberCard`'s prop-driven shape.

```ts
import type { Loan } from '../api';

interface LoanCardProps {
  loan: Loan;
  /** 'head' shows action buttons per status; 'member' is read-only. */
  role: 'head' | 'member';
  /** Head, requested loans → opens the approve sheet. */
  onApprove?: (loan: Loan) => void;
  /** Head, requested loans → reject (confirm at the call site). */
  onReject?: (loan: Loan) => void;
  /** Head, active loans → close/early-payoff (confirm at the call site). */
  onClose?: (loan: Loan) => void;
  /** Disables buttons while a mutation for this loan is pending. */
  processing?: boolean;
  /** Head view: the borrower's display name (member view omits it — it's always self). */
  memberName?: string;
}
```

Render (kit tokens/components only — no inline hex, no raw spacing):
- **Header row:** title — head view: `memberName ?? 'Member'`; member view: `formatMinorUnits(loan.principal)` as the headline (e.g. "₹1,000 loan"). Right: `<StatusBadge label={loanStatusLabel(loan.status)} tone={loanStatusTone(loan.status)} />`.
- **Terms line** (`fontSize.sm`, `textSecondary`): `{formatMinorUnits(loan.emi_amount)} × {loan.installments} mo` — the EMI amount and count. For `active`/`closed` also append the start month: `· from {humanMonth(loan.start_period)}` (guard `start_period` non-null — it is null while `requested`/`rejected`).
- **Progress** (only when `status !== 'requested' && status !== 'rejected'`): `loanProgressLabel(loan)` (e.g. "2 / 5 paid") + `outstandingLabel(loan)` (e.g. "₹600.00 outstanding"). Optionally a simple progress bar `installments_posted / installments` built from two `View`s + tokens (no new dep). For `closed`: outstanding is ₹0.00 — show "Paid off" emphasis rather than a bar.
- **Note** (if `loan.note`): a muted caption.
- **Head actions** (`role === 'head'`, gated by status):
  - `requested` → `<Button title="Approve" variant="primary" size="sm" onPress={() => onApprove?.(loan)} />` + `<Button title="Reject" variant="danger" size="sm" onPress={() => onReject?.(loan)} />`.
  - `active` → `<Button title="Close" variant="secondary" size="sm" onPress={() => onClose?.(loan)} />` (early payoff).
  - `rejected` / `closed` → no buttons (terminal).
  - All disabled while `processing`.
- **Member view:** no action buttons (read-only; §8 matrix).

Keep it a `Card`, not a bare `View`, so it reads as a distinct object in the list.

---

## 6. `app/src/components/LoanRequestSheet.tsx` (new) — member request

A kit `<Sheet title="Request a loan">` form; **member-only** (mounted from the member branch of `loans.tsx`). Uses `TextField` + `Toast`; **no `Alert.alert`**. Follows `AllowanceSheet`'s structure (footer Cancel/Save, `resetForm` on close, closed→open reset effect).

```ts
interface LoanRequestSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
}
```

### Fields
1. **Principal** — `<TextField label="Loan amount (₹)" keyboardType="decimal-pad" value={principalStr} onChangeText={...} error={principalError} placeholder="e.g. 1000" />`. Parse on submit with `parseMoneyToMinorUnits` (**never `parseFloat`** — master-plan §10.6, grep-gated §11). Note `parseMoneyToMinorUnits` returns **null** for `'0'`, empty, negative, or `>2dp` — treat null as invalid.
2. **Installments** — `<TextField label="Number of months" keyboardType="number-pad" value={installmentsStr} onChangeText={...} error={installmentsError} placeholder="e.g. 5" />`. **Installments is a plain integer count, NOT money** — parse it with `Number.parseInt` (or a tiny `parseCount` helper), validate `Number.isInteger(n) && n > 0`. **Do NOT run it through `parseMoneyToMinorUnits`** (that divides by 100 → wrong) and do NOT `parseFloat` it. A `@react-native-picker/picker` of 1–24 is an acceptable alternative to a text field (reuses an existing dep).
3. **EMI estimate (display-only):** below the fields, show `estimateEmi(parseMoneyToMinorUnits(principalStr), parseCount(installmentsStr))` as a caption like `≈ ₹200.00 / month` when both inputs parse, else nothing. **Labelled "≈"** and never sent — the server computes the authoritative `emi_amount` (§0/§4). Recompute live as the user types (no state needed — derive in render).
4. **Note (optional):** a `TextField` for a short reason ("New bicycle"), mapped to `note`.

### Submit
- Parse both fields. Principal null → `principalError` "Enter a valid amount (e.g. 1000)."; installments invalid → `installmentsError` "Enter a whole number of months." Abort on either (no submit).
- Valid → `useRequestLoan(groupId).mutateAsync({ principal, installments, note })`.
- Save disabled while pending (`Button loading`) and while either field is invalid.
- Success → `onClose()` + `Toast{ tone:'success', message: 'Loan requested' }`. Error → `Toast{ tone:'danger', message }` (surface the API message, e.g. a member naming another user — shouldn't happen since we never send `user_id`).
- Reset local state on close and re-init on the **closed→open edge** (the `prevVisible` ref pattern from `AllowanceSheet`, §Gotchas) — a request sheet has no server prefill, but keep the reset-on-open so a cancelled draft doesn't linger.

---

## 7. `app/src/components/LoanApproveSheet.tsx` (new) — head approve with editable terms

A kit `<Sheet title="Approve loan">` form; **head-only**. The distinguishing feature (master-plan: "approve (**editable terms**)") is that the head may **override** the requested `principal` and `installments` before approving. Uses `TextField` + `Toast`; **no `Alert.alert`**.

```ts
import type { Loan } from '../api';

interface LoanApproveSheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  loan: Loan | null;          // the requested loan being approved (prefills the fields)
  memberName?: string;        // for the sheet title/caption
}
```

### Fields — **prefilled from `loan`, editable**
1. **Principal** — prefilled `formatMinor(loan.principal)` (e.g. "1000.00", no sign/symbol — `money.ts:2`); parse with `parseMoneyToMinorUnits`.
2. **Installments** — prefilled `String(loan.installments)`; parse as an integer count (§6, `parseCount`), **not** money.
3. **EMI estimate (display-only):** `estimateEmi(...)` recomputed live from the (possibly edited) fields, `≈ ₹…/month`. Server recomputes authoritatively (WP-3.1 §5.4).

### Submit (approve)
- Parse both. Invalid → field error, abort.
- **Only send overrides that differ** from the requested loan: build `overrides = {}`, add `principal` if the parsed value ≠ `loan.principal`, add `installments` if ≠ `loan.installments`. If nothing changed, send `{}` (accept verbatim — the server keeps the requested terms). Either way `useApproveLoan(groupId).mutateAsync({ loanId: loan.id, ...overrides })`.
  - (Sending the unchanged values explicitly is also correct — the server recomputes `emi_amount` regardless — but diffing keeps the payload honest about intent. Either is acceptable; state which you chose.)
- Success → `onClose()` + `Toast{ tone:'success', message: 'Loan approved' }`. Error → `Toast{ tone:'danger', message }` (e.g. **409** "loan is not pending approval" if it was decided concurrently — surface it faithfully; the list will refetch and drop the stale card).
- Save disabled while pending / while a field is invalid.

### Prefill staleness (MANDATORY — commit `009d78c`)
`loan` can **change/refetch after the sheet mounts** (a background `useLoans` refetch, or the head opening the sheet for a different loan). **Re-prefill the fields on the closed→open edge**, exactly like `AllowanceSheet` (`AllowanceSheet.tsx:52-60`): keep a `prevVisible = useRef(visible)`, and in a `useEffect` reset `principalStr`/`installmentsStr` from `loan` **only when `visible && !prevVisible.current`** (the open transition). This (a) picks up a `loan` that loaded after first mount and (b) **guards against a background refetch wiping what the head is mid-typing** (do not reset on every `loan` change). Miss this and the approve sheet shows stale or empty terms — the single most likely slip here (§Gotchas).

---

## 8. Screen wiring

### 8.1 `app/app/(app)/groups/[id]/loans.tsx` (new — the tab)

Branches on `isHead` like `index.tsx`. **All hooks are called before any early return** (§Gotchas — hooks-above-early-returns).

```ts
// hooks first — unconditionally
const { id } = useLocalSearchParams<{ id: string }>();
const { user } = useAuth();
const groupQuery = useGroup(id ?? '');
const loansQuery = useLoans(id ?? '');
const requestLoan = useRequestLoan(id ?? '');
const approve...  // approve/reject/close mutations
const [requestSheetVisible, setRequestSheetVisible] = useState(false);
const [approveTarget, setApproveTarget] = useState<Loan | null>(null);
const [processingId, setProcessingId] = useState<string | null>(null);
// ...derive isHead, sections...

// THEN early returns (loading / error) — never before the hooks above
```

- `const isHead = group.members.find(m => m.user_id === user?.id)?.role === 'head';`
- `const loans = loansQuery.data ?? [];`
- **Sectioning (client-side, from the full list):** split into `requested`, `active`, and `closed`+`rejected` (terminal/history). Order sections so the head's **actionable** loans (`requested`, then `active`) are on top; history last. A `SectionList` (kit-friendly) or three labelled `FlatList`s.

**Head branch:**
- Renders every loan as `<LoanCard role="head" loan={loan} memberName={nameFor(loan.user_id)} onApprove={setApproveTarget} onReject={handleReject} onClose={handleClose} processing={processingId === loan.id} />`. `nameFor` maps `loan.user_id` → member name from `group.members`.
- **Approve** → opens `<LoanApproveSheet loan={approveTarget} .../>` (set `approveTarget`).
- **Reject** → `confirmAsync({ title: 'Reject loan', message: 'This loan request will be rejected.', confirmLabel: 'Reject', destructive: true })` → `rejectLoan.mutateAsync(loan.id)` → Toast. Set/clear `processingId`.
- **Close** → `confirmAsync({ title: 'Close loan', message: 'Posts a final EMI for the remaining balance and closes the loan.', confirmLabel: 'Close', destructive: true })` → `closeLoan.mutateAsync(loan.id)` → Toast.
- Empty state (`EmptyState icon="card-outline" title="No loans yet" subtitle="Members' loan requests will appear here"`).
- Pull-to-refresh wired to `loansQuery.refetch()` (+ `balanceQuery`/`groupQuery` if present).

**Member branch:**
- A prominent `<Button title="Request a loan" variant="primary" icon="add" onPress={() => setRequestSheetVisible(true)} fullWidth />` (one primary action per screen — master-plan §7.4).
- Renders own loans as `<LoanCard role="member" loan={loan} />` (read-only, no action buttons). Sections: active first (with progress), then requested (pending head), then history.
- `<LoanRequestSheet visible={requestSheetVisible} onClose={...} groupId={id ?? ''} />`.
- Empty state ("No loans yet — tap Request a loan to ask for one").

**Both branches:** loading → `LoadingSpinner`; `loansQuery.isError` → `ErrorMessage` (the loans tab's primary content is loans, so erroring the tab is acceptable here, unlike the allowance section which degrades gracefully within another screen).

### 8.2 `app/app/(app)/groups/[id]/_layout.tsx` (changed — register the tab)

Add a third **visible** `Tabs.Screen` for `loans`, between `chores` and the hidden `ledger` (`ledger` keeps `href: null`). Use the `card-outline` icon (consistent with the EMI ledger-row icon):
```tsx
<Tabs.Screen
  name="loans"
  options={{
    title: 'Loans',
    tabBarIcon: ({ color, size }) => (
      <Ionicons name="card-outline" size={size} color={color} />
    ),
  }}
/>
```
No role gating on the tab itself (both roles have a loans view); the screen branches internally. `_layout.tsx` already computes `isHead` — no new query needed there.

### 8.3 Barrel + files

`app/src/components/index.ts` — add:
```ts
export { LoanCard } from './LoanCard';
export { LoanRequestSheet } from './LoanRequestSheet';
export { LoanApproveSheet } from './LoanApproveSheet';
```

**Created**
- `app/src/hooks/useLoans.ts`
- `app/src/loan-format.ts` (+ optional `loan-format.test.ts`)
- `app/src/components/LoanCard.tsx`
- `app/src/components/LoanRequestSheet.tsx`
- `app/src/components/LoanApproveSheet.tsx`
- `app/app/(app)/groups/[id]/loans.tsx`

**Changed**
- `app/src/api-types.gen.ts` (regenerated — §1)
- `app/src/api.ts` (§2: `loansApi` + `Loan` type)
- `app/src/ledger-format.ts` (§9: EMI `k/n` label tweak)
- `app/src/components/index.ts` (barrel: 3 exports)
- `app/app/(app)/groups/[id]/_layout.tsx` (§8.2: register Loans tab)

**Deleted** — none.

---

## 9. EMI ledger rows — the label tweak (`ledger-format.ts`)

EMI debit rows already render from WP-1.3: `TYPE_ICONS.emi` → `card-outline`, amount signed red (`direction='debit'`), grouped by `entry.period`'s month, no status badge (machine-posted → `approved`), no decided-by line (`decided_by` NULL). **Verify all of that on a real posted EMI row** (§11 interactive). The one **specified tweak** upgrades the title from today's `EMI — <note>` to master-plan §7.2's **`EMI k/n — <note>`** (e.g. "EMI 2/5 — New bicycle").

Because `k` needs the sibling EMI rows and `n` needs the loan, thread **loans + the full entry list** into the title:
- Add an overload/param to `entryTitle` (or a small dedicated `emiRowTitle`) that accepts the current `entries` array and a `loans: Loan[]` array and uses `emiInstallmentIndex(entry, entries, loans)` (§4): when it returns `{k, n}`, render `EMI ${k}/${n}${note ? ' — ' + note : ''}`; when it returns `null` (loans not loaded, or a sibling/loan missing), **fall back to today's** `entry.note ? 'EMI — ' + note : 'EMI'`. The fallback keeps rows rendering even before loans load.
- **Threading:** `LedgerRow` currently takes `chores`/`members` — add an optional `loans?: Loan[]` prop (default `[]`) and pass the row's sibling entries it already has via `LedgerList`. `LedgerList` has the full `entries` array; pass `loans` down from the screen. The screens that render ledgers (`index.tsx` member branch, `ledger.tsx` head per-member) gain a `useLoans(id)` call and pass `loans={loansQuery.data ?? []}` to `LedgerList`. **Keep the change additive and default-safe** — every existing `LedgerRow`/`LedgerList` caller must compile with `loans` omitted (falls back to plain "EMI").

> **Scope guard:** this is a *label* enhancement, not a data-model change. If threading `loans` into `ledger.tsx`/`index.tsx` proves noisier than the acceptance warrants, the **minimum bar is: EMI rows render correctly with the WP-1.3 fallback label** (verified), and the `k/n` upgrade lands at least on the **member's own ledger** (where `useLoans` returns their loans and their own EMI siblings are all present). State in the PR exactly which surfaces got `k/n` vs. the fallback. **Do not** special-case EMI rows per-screen or duplicate `ledger-format` logic — the title stays computed in `ledger-format.ts`.

---

## 10. Kit / convention reuse (carried from WP-1.0/1.3/2.2 — read before coding)

1. **`Alert.alert` is banned** for confirms/feedback (no-op on `react-native-web`). Reject/close confirms → `confirmAsync`; all feedback → `useToast().show(...)`. Grep-gated (§11).
2. **Money in, money out.** The **principal** input parses via `parseMoneyToMinorUnits` (rejects `0`/`<0`/`>2dp` → returns **null**); display via `AmountText` / `formatMinorUnits` / `formatMinor`. **Never `parseFloat`** on money (master-plan §10.6). Grep-gated (§11).
3. **Installments is a count, not money.** Parse with `Number.parseInt` + `Number.isInteger`/`> 0`, **not** `parseMoneyToMinorUnits` (would divide by 100) and **not** `parseFloat`. This is a likely slip — the reviewer must check it (§Gotchas).
4. **EMI is server-computed.** `emi_amount`/`installments_posted`/`outstanding` come from `LoanResponse`. The FE's only loan arithmetic is the **display-only** `estimateEmi` (labelled "≈"), never sent. No due-month math, no schedule, no posting (§0).
5. **All create/edit forms use `Sheet`** (bottom-sheet native / centered modal web) — never a bare `Modal` or `Alert`-driven form (WP-1.0 §3.7). `LoanRequestSheet`/`LoanApproveSheet` follow `AllowanceSheet`/`AddEntrySheet` (footer Cancel/Save, `resetForm` on close, closed→open re-prefill effect).
6. **Token-driven styling only** — `theme.color.*`/`theme.spacing.*`; no inline hex / raw spacing in new components.
7. **AuthZ mirrored in UI, enforced by API.** Member view is read-only (no approve/reject/close). Head sees all, decides. The API is the real gate (WP-3.1 §5) — the UI just hides what the role can't do (master-plan §10.5).
8. **Every list/section gets empty + loading + error states.** The loans tab: empty ("No loans yet"), loading (`LoadingSpinner`), error (`ErrorMessage`). Loan cards cover all four statuses (requested/active/closed/rejected).
9. **Hooks above early returns.** In `loans.tsx`, call **every** hook (`useLoans`, `useGroup`, the four mutations, `useState`) before any `if (loading) return …` / role branch (§Gotchas).

---

## 11. Verification floor

Run from `app/`. State in the PR **exactly** what ran. FE floor per master-plan §10.3.

```bash
cd app

# 1. Regenerate committed types from the post-WP-3.1 openapi — THE FIRST STEP (§1).
npm run codegen
grep -n "LoanResponse\|CreateLoanRequest\|ApproveLoanRequest" src/api-types.gen.ts   # STOP-gate: all THREE present
npm run codegen:check          # exit 0 → committed spec ⇒ committed types, no drift

# 2. Types — primary gate.
npx tsc --noEmit

# 3. Lint — no new errors.
npm run lint

# 4. Web export — MANDATORY (§10.3 FE floor; tsc/expo-doctor miss bundle-level breakage).
npx expo export --platform web     # must exit 0, emit dist/

# 5. Guard: no new native deps (should print nothing new vs. master).
git diff --stat package.json

# 6. Grep gates — search BOTH app/app AND app/src (consumption lives in shared components/hooks):
#  (a) loan surface actually wired:
grep -rn "loansApi\|useLoans\|useRequestLoan\|useApproveLoan\|useRejectLoan\|useCloseLoan\|LoanResponse\|LoanCard" app/src app/app
#  (b) no parseFloat anywhere new/changed (money AND installments avoid it):
grep -rn "parseFloat" app/src app/app        # expect: none in WP-3.2 files
#  (c) no Alert.alert for confirm/feedback outside confirm.ts:
grep -rn "Alert.alert" app/app app/src | grep -v "src/confirm.ts"    # expect: none
#  (d) installments NOT parsed as money (sanity — parseMoneyToMinorUnits only on principal):
grep -rn "parseMoneyToMinorUnits" app/src/components/LoanRequestSheet.tsx app/src/components/LoanApproveSheet.tsx
#      expect: principal only; installments uses parseInt/Number
```

**Interactive (if a browser/dev server is available — `npm run web`):** the acceptance is **full request→approve→EMI-appears on web + Android**.
- **Member:** open a group → **Loans** tab → "Request a loan" → enter ₹1000, 5 months → the `≈ ₹200.00 / month` estimate shows → submit → toast "Loan requested"; a **Requested** card appears (₹1000, ₹200 × 5 mo, status badge warning). No ledger row yet, balance unchanged.
- **Head:** Loans tab → the member's **Requested** card with **Approve**/**Reject**. Tap **Approve** → sheet prefilled ₹1000 / 5 (editable) → optionally change to ₹1200 / 6 (estimate updates) → Approve → toast "Loan approved"; card flips to **Active** with `from <next month>`, progress `0 / 6 paid` (or `0 / 5`). Because approve invalidates ledger/balance, the **first EMI debit appears in the ledger** on the next render **if `start_period` is already due** (for a next-month start it posts when that month arrives / when the engine next runs) — verify an `emi` row renders as **`EMI 1/n — …`** red `−₹`, and the member's balance moves (possibly **negative** — rendered `−₹…` "you owe").
- **Head close (early payoff):** an active, partly-repaid loan → **Close** → confirm → a final `emi` debit for the exact outstanding posts, card → **Paid off** (`n/n paid`, ₹0.00 outstanding). A second close → 409 surfaced as a toast; the list refetches.
- **Member reject view:** a rejected request shows a **Rejected** (danger) card, no actions.
- **Both platforms** (web + Android): sheets, `confirmAsync` (web `window.confirm`), and toasts all appear — **no `Alert.alert` no-op**.

State in the PR what ran locally (codegen/codegen:check/tsc/lint/expo export web + greps) vs. what needs a device/browser (the interactive matrix). Note the **openapi drift caveat** (§1) if codegen produced a shape differing from §1's expected fields.

---

## 12. Gotchas / likely slips (reviewer checklist)

1. **Installments parsed as money.** The single most likely slip: running the installments field through `parseMoneyToMinorUnits` (÷100 → "5 months" becomes 0.05) or `parseFloat`. Installments is a **plain integer count** — `Number.parseInt` + `Number.isInteger(n) && n > 0`. `parseMoneyToMinorUnits` is for **principal only** (§10.3, grep-gated §11d).
2. **`parseMoneyToMinorUnits` returns `null` for `'0'`/invalid.** A loan principal of 0 is invalid anyway (loans are `> 0`), so treat `null` as a validation error and block submit — don't coerce `null` to 0.
3. **Client-side EMI/posting.** Do **not** compute the EMI schedule, a due month, or an outstanding for anything but the **display-only** `≈` estimate; do **not** insert a ledger row or call a "post now" endpoint (none exists). `emi_amount`/`installments_posted`/`outstanding` are server-authoritative (§0/§4).
4. **Sheet prefill staleness (`LoanApproveSheet`).** Re-prefill principal/installments on the **closed→open edge** via the `prevVisible` ref (commit `009d78c`, `AllowanceSheet.tsx:52-60`) — not on every `loan` change (that wipes mid-typing) and not once-on-mount (misses a `loan` that loads later). Miss it → stale/empty approve terms (§7).
5. **Hooks above early returns.** `loans.tsx` must call all hooks (`useLoans`, `useGroup`, the four mutations, `useState`) **before** any `if (loading) return` / role branch, or React throws "rendered fewer hooks than expected" when the branch flips (§8.1/§10.9).
6. **Approve invalidation is what makes "EMI appears" work.** `useApproveLoan`/`useCloseLoan` must invalidate **ledger + balance + group** (not just `loans`) so the lazy engine runs on the refetch and the EMI debit + balance surface without a manual refresh (§3). Omitting them "works" only after a pull-to-refresh — a silent acceptance miss.
7. **`start_period` is null until active.** Guard it before `humanMonth(loan.start_period)` in `LoanCard`/EMI labels — it's `null` for `requested`/`rejected` loans.
8. **Negative balance is expected, not a bug.** After EMIs a member's balance can go negative (WP-3.1 §8); the Overview already renders `−₹…` "you owe" (`index.tsx:186-194`). Do not clamp or hide it.
9. **EMI `k/n` fallback.** If loans/siblings aren't loaded, `emiInstallmentIndex` returns `null` → render the plain `EMI`/`EMI — <note>` fallback, never a broken `EMI /` or a crash (§9).
10. **openapi drift (§1).** WP-3.1's reviewer may still be finalizing `openapi.yaml`. Codegen from whatever is on master; if fields differ from §1's expected shape, reconcile the consumers to the **generated types** and flag it in `docs/notes/` — do not hand-write types or edit `openapi.yaml`.
11. **Grep both `app/src` and `app/app`.** Consumption lives in shared hooks/components; WP-1.3's review missed a field by grepping only `app/app` (§11).
12. **No head "lend now" creation.** The backend allows a head to create a pre-approved loan, but that UI is out of scope for the WP-3.2 row — don't build it (§0).

---

## 13. Definition of Done (checklist)

- [ ] `npm run codegen` run; `api-types.gen.ts` regenerated & committed; **`LoanResponse` + `CreateLoanRequest` + `ApproveLoanRequest` present** (STOP-gate); `codegen:check` green (§1).
- [ ] `loansApi` (`list`/`request`/`approve`/`reject`/`close`) + `export type Loan` in `api.ts`; routes match WP-3.1 §6 (`GET/POST /groups/:id/loans`, `POST /loans/:id/{approve,reject,close}`) (§2).
- [ ] `useLoans` (GET) + `useRequestLoan`/`useApproveLoan`/`useRejectLoan`/`useCloseLoan`; invalidation matrix per §3 (**approve & close invalidate ledger+balance+group**; request & reject → loans only); reuses `qk.loans` (no filters arg).
- [ ] `loan-format.ts` pure helpers (`estimateEmi` display-only, `loanStatusTone`/`Label`, `loanProgressLabel`, `outstandingLabel`, `emiInstallmentIndex`); reuse `humanMonth`/`formatMinorUnits`; no React (§4).
- [ ] `LoanCard` — status badge (all 4 statuses), terms (`emi × n mo`), progress (`n/m paid` + outstanding) for active/closed, head action buttons gated by status, member read-only (§5).
- [ ] `LoanRequestSheet` — principal via `parseMoneyToMinorUnits`, **installments via integer parse (not money)**, `≈` EMI estimate display-only, `Toast`, no `Alert.alert` (§6).
- [ ] `LoanApproveSheet` — **editable** principal/installments prefilled from the loan, sends only overrides, **closed→open re-prefill effect** (`009d78c`), `Toast`/409 surfaced, no `Alert.alert` (§7).
- [ ] `loans.tsx` tab — member (request + own read-only cards) / head (all cards + approve-sheet/reject-confirm/close-confirm) branches; **hooks above early returns**; empty/loading/error states; pull-to-refresh (§8.1).
- [ ] `_layout.tsx` — Loans tab registered (Overview | Chores | Loans), `card-outline` icon, `ledger` still `href:null` (§8.2).
- [ ] EMI ledger rows verified rendering (icon/red debit/month-group); title upgraded to **`EMI k/n — <note>`** with a safe fallback, computed in `ledger-format.ts` (no per-screen special-casing) (§9).
- [ ] Barrel exports `LoanCard` + `LoanRequestSheet` + `LoanApproveSheet` (§8.3).
- [ ] `tsc --noEmit` + `lint` + `expo export --platform web` green; no new dep; grep gates clean (incl. `app/src`, no `parseFloat`, no `Alert.alert`, installments not money) (§11).
- [ ] Out-of-scope respected: no backend/openapi/migration edits; no client EMI math/posting; no head "lend now" UI; no `members/[userId].tsx`; no legacy sweep (§0).

Commit/PR title: **`WP-3.2 spec: FE loans tab`** (this spec) — implementation PR later titled **`WP-3.2: FE loans tab`**.
