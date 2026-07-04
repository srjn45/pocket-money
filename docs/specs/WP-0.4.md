# WP-0.4 Spec: Chores Tab Fix

> Roadmap ref: master-plan §9 Phase 0, WP-0.4 (∥). Scope: **frontend only** — the
> settlement-as-protected-system-chore design is already fully implemented and enforced on
> the backend. Do **not** touch backend, migrations, or openapi.yaml. Money stays a decimal
> `number` (dollars/rupees as float) for this WP — the minor-units conversion is WP-1.1 and
> is explicitly out of scope here.

---

## 0. Files in scope

- `app/app/(app)/groups/[id]/chores.tsx` — the chores screen (primary).
- `app/src/api.ts` — `choresApi` + `Chore` interface (verify only; minimal/no change).
- No other files. Do not refactor the tab layout, ledger, or `settlementsApi`/`pending`
  dead surface (those belong to WP-1.2/WP-1.3).

## 1. Root cause — what is actually broken and why

I read the backend (`handlers/chores.go`, `db/chore_repo.go`, `handlers/groups.go`, routes in
`cmd/server/main.go`, `openapi.yaml`) and the frontend (`chores.tsx`, `src/api.ts`,
`auth-context.tsx`, the group `_layout.tsx`, and `auth.go` for the `/auth/me` shape). The
backend is correct and complete:

- Routes exist and match the client: `GET/POST /groups/:id/chores`, `PATCH/DELETE /chores/:id`
  (`cmd/server/main.go:81-84`).
- `is_system` is returned on every chore (`ChoreResponse`, `chores.go:51`), list is ordered
  `is_system DESC` so the system chore is pinned first (`chore_repo.go:94`).
- System chore is protected: `Update`/`Delete` return `ErrSystemChore` -> HTTP 403
  ("cannot modify/delete system chore") (`chore_repo.go:131,172`; `chores.go:228,297`).
- Every group gets a default `Settlement` system chore on creation (`groups.go:104`).
- Head-only enforcement for create/update/delete is in the API (`chores.go:143,215,291`).

**So it is NOT a wrong endpoint, NOT missing `is_system` handling, NOT a missing head-role
check, and NOT a backend gap.** `choresApi` in `api.ts:172-182` already targets the correct
paths, and the `Chore` interface (`api.ts:42-50`) already has `is_system` and it is read in
the screen (`chores.tsx:112`). The screen already renders a "System" badge, disables
edit/delete on the system chore, and shows "Variable" for its amount.

The breakage is entirely on the client, and it is a cluster of concrete defects. In order of
impact:

### 1a. PRIMARY — `Alert.alert` is a no-op on React Native Web, so on web (the verification platform) delete is impossible and every error is silently swallowed.

`chores.tsx` uses `Alert` (imported from `react-native`, line 2) for two things:
- **Delete confirmation** — `handleDelete` (line 93) does its work *inside*
  `Alert.alert(... onPress: async () => choresApi.delete ...)`. On `react-native-web`
  `Alert.alert` renders nothing and never invokes the button callbacks, so **pressing the
  trash icon on web does nothing at all** — the head literally cannot delete a chore on web.
- **Error surfacing** — save and delete failures are reported only via `Alert.alert('Error', …)`
  (lines 87, 104). On web these never appear, so a failed create/edit/delete looks like a
  dead button with no feedback. This is the "error not surfaced" symptom.

Because delete and all error feedback route through `Alert`, on web the tab *looks* broken
even though the list and the create happy-path actually work.

### 1b. Wrong currency symbol.
Line 138 renders `` `$${item.amount.toFixed(2)}` `` — hardcoded `$`. The product is INR
(`₹`, per master-plan §7.4). Cosmetic but wrong and user-visible.

### 1c. No input validation / silent failures in the form.
`handleSave` (line 66) guards with `if (!id || !choreName.trim() || !choreAmount) return;` — a
silent `return` with no message, so a user who taps Save with an empty name/amount gets
nothing. Worse, `parseFloat(choreAmount)` (lines 75, 81) yields `NaN` for non-numeric input;
the backend then 400s (`amount … required,gt=0`) and — per 1a — that error is invisible on
web. The Save button is also not disabled based on validity (only on `saving`), so nothing
signals what is wrong.

### 1d. Loading spinner can hang forever.
`loadData` (line 23) starts with `if (!id || !user?.id) return;` and `isLoading` is initialised
`true` (line 12). The early `return` skips the `finally` that sets `setIsLoading(false)`, so if
`user?.id` is ever transiently/permanently falsy the screen is stuck on the spinner with no
error. (In practice the group `_layout.tsx` gates on `user?.id` first, so this rarely triggers
today, but it is a latent "blank/stuck tab" cause and must be made defensive.)

### 1e. Minor: system chore is de-emphasised but not clearly pinned/explained.
It shows a grey "System" badge and "Variable", but there is no hint that it is the payout chore
or why it cannot be edited. Acceptable to leave, but improve per §4.

`api.ts` note: `choresApi` is correct and needs no functional change. The `Chore` interface is
correct and `is_system` is used. (The unrelated dead `settlementsApi` and
`ledgerApi.listPending` `/pending` references are out of scope for this WP.)

## 2. The fix

Behavioural target (master-plan §5.5 authz matrix, FE-bugs.md):

- **Head**: can create, edit, and soft-delete non-system chores (name, description, amount).
- **Member**: sees a **read-only** list — no Add button, no edit affordance, no delete icon.
- **System "Settlement" chore**: always visible, pinned to top (backend already orders it
  first), clearly labelled, and has **no edit or delete controls** for anyone. The backend
  will 403 any attempt regardless, but the UI must not offer the controls.

Concrete changes in `chores.tsx`:

1. **Replace `Alert` with a cross-platform pattern** (the core fix):
   - **Delete confirmation**: use a confirm that works on web + native. Simplest correct
     option: `Platform.OS === 'web' ? window.confirm(msg) : Alert.alert(...)`, or a small
     in-app confirm `Modal`. Do **not** leave delete logic solely inside `Alert.alert`'s
     button callback. On confirm -> `await choresApi.delete(chore.id)` -> `loadData()`.
   - **Errors**: surface via an on-screen mechanism that renders on web — reuse the existing
     `error` state / `styles.error` banner (or an inline error inside the modal for
     save failures), not `Alert.alert`. Every failed create/edit/delete must show a visible
     message on web.
2. **Currency**: change `` `$${item.amount.toFixed(2)}` `` to a `₹` format
   (e.g. `` `₹${item.amount.toFixed(2)}` ``). Keep `amount` as a decimal `number`
   (no minor-units conversion in this WP).
3. **Form validation with feedback**:
   - Trim + require a non-empty `name`.
   - Parse amount defensively: reject empty / `NaN` / `<= 0` with a visible inline message
     (mirror the backend rule `required,gt=0`). Only call the API when valid.
   - Disable the Save button while invalid or while `saving` (currently only `saving`).
4. **Loading**: move `setIsLoading(false)` so it always runs even on the early-return path
   (e.g. set loading false in the guard, or drop the pre-load guard and rely on the render
   guards). The tab must never hang on the spinner; if load fails, show the error banner +
   a way to retry (pull-to-refresh already exists).
5. **Role gating (keep + verify)**: `isHead` already hides the Add button (line 162) and
   gates edit/delete via `canEdit = isHead && !isSystemChore` (line 113). Keep this. Ensure a
   member sees a plain read-only list (no Add, no trash icon, rows not pressable into the
   edit modal — already handled by `disabled={!canEdit}` on the row, line 119).
6. **System chore protection in UI**: keep `canEdit` excluding `is_system`; the row must not
   open the edit modal and must not show the trash icon for the Settlement chore (already the
   case — verify it survives the refactor).

## 3. api.ts client changes

- **None required functionally.** `choresApi.list/create/update/delete` already map to the
  correct endpoints and methods; the `Chore` interface already includes `is_system: boolean`
  and it is used by the screen.
- Optional (nice-to-have, low risk): the `update`/`create` payload types in `api.ts:175-179`
  already match the backend's `CreateChoreRequest`/`UpdateChoreRequest`. Leave as-is.
- **Do not** delete or edit the unrelated `settlementsApi` / `ledgerApi.listPending`
  (`/pending`) entries here — they are removed in the ledger WPs (1.2/1.3), out of scope.

## 4. UI notes (states & affordances)

- **Empty state**: the list is effectively never empty (every group has the pinned Settlement
  chore), but keep the existing empty state (`chores.tsx:169`) for safety. For a head with only
  the system chore, the "Add Chore" button already communicates the next action; optionally
  add a one-line hint ("Add chores your family can earn money for").
- **Loading**: single spinner on first load only (existing). Pull-to-refresh already wired via
  `RefreshControl` — keep it; never show the full-screen spinner during refresh.
- **Error surfacing**: use the on-screen `error` banner for list-load failures and an inline
  message inside the modal for save failures. Must be visible on **web** (this is the whole
  point of the Alert fix). Include the server's message (`err.message`) when present.
- **Confirm-before-delete**: keep a confirmation step, but implement it so it works on web
  (`window.confirm`) and native (`Alert.alert`) — or an in-app confirm modal for both.
  Message: `Delete "<name>"? This can't be undone.` (backend soft-deletes; wording stays
  user-facing simple).
- **System chore presentation**: keep it pinned first (backend order) with the "System" badge;
  add a short caption/subtitle such as "Used to record payouts — can't be edited or deleted"
  and show its amount as "Variable" (already done). No edit/delete controls, not pressable.
- **Amount display**: `₹` prefix, two decimals, for non-system chores.

## 5. Verification steps (must be run on **web**, `npm run web`)

Preconditions: backend running, a registered head account with a created group (so the
Settlement system chore exists), and a second registered member account joined via invite.

**As head (create / edit / delete):**
1. Open the group -> Chores tab. Confirm the list loads (no infinite spinner) and shows the
   pinned **Settlement** chore first, with a "System" badge and "Variable" amount, and **no**
   edit/delete controls on it.
2. Tap **Add Chore**, submit with an empty name and/or empty amount -> confirm a **visible**
   validation message appears (not a silent no-op) and Save is disabled/blocked.
3. Add a valid chore (e.g. name "Dishes", amount "5.50") -> confirm it appears in the list as
   **₹5.50** (rupee symbol, not `$`).
4. Tap the chore to edit -> change amount to "6.00" and save -> confirm the list updates to
   **₹6.00**.
5. Tap the **trash** icon on "Dishes" -> confirm a delete confirmation appears **on web**,
   confirm it -> chore disappears from the list (soft-deleted). Reload/refresh -> still gone.
6. Attempt to trigger a server error (e.g. temporarily stop the backend, then create) ->
   confirm a **visible** error message is shown on web (not a silent failure).
7. Confirm the Settlement chore still cannot be edited or deleted (no controls; row not
   pressable into the edit modal).

**As member (read-only):**
8. Log in as the member, open the same group -> Chores tab. Confirm the list loads and shows
   all chores including "Dishes" and the pinned Settlement chore.
9. Confirm there is **no "Add Chore" button**, **no trash icons**, and tapping a chore row does
   **not** open an edit modal (read-only).

**Regression / type check:**
10. `cd app && npx tsc --noEmit` is clean.
11. (Optional sanity) On native (Expo Go / Android), delete confirmation still works via the
    native alert.

## 6. Definition of done

- Head can create, edit, and soft-delete non-system chores **on web**, with visible validation
  and error feedback.
- Member sees a read-only chores list on web.
- The Settlement system chore is pinned, labelled, and has no edit/delete controls; backend
  403 is never needed because the UI never offers the action.
- Currency renders as `₹`. No infinite spinner. `npx tsc --noEmit` clean.
- No backend/openapi/migration changes; `api.ts` functionally unchanged.
