# QA feedback batch 1 — implementation summary

Branch: `fix/qa-feedback-batch-1` (off `master` @ d15ef2a). PR base: `master`.

Local verification (no Docker on this host → integration/e2e run only in CI):
- backend: `go build ./...`, `go vet ./...`, `go test ./...` (unit) all pass; `gofmt -l` clean.
- frontend: `npm run codegen` regenerated `src/api-types.gen.ts`; `npx tsc --noEmit` clean; `npx expo lint` introduces **no new** issues (1 pre-existing `react/no-unescaped-entities` error on the untouched member-view banner "You'll receive", + 2 pre-existing warnings in profile.tsx / auth-context.tsx).
- migration 017 reviewed by SQL inspection and picked up by `go build` (embedded via `migrations/embed.go`); **not** executed against the live local Postgres (per instructions) — CI's postgres service runs it.

---

## Item 1 — Delete group (soft delete / archive, admin-only)
- Migration `017_group_soft_delete.{up,down}.sql`: adds `groups.deleted_at TIMESTAMPTZ NULL DEFAULT NULL` + partial index `idx_groups_live` over live rows.
- `GroupRepo` audit — every read of `groups` now filters `deleted_at IS NULL`: `GetByID`, `GetCurrency`, `GetChoreSubmissionEnabled`, `ListForUserWithSummary` (the `my_groups` CTE), and the `SetChoreSubmissionEnabled` UPDATE. New `SoftDelete(id)` does `UPDATE ... SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, returning `ErrNotFound` when 0 rows (→ idempotent 404).
- New `DELETE /api/v1/groups/:id` → `GroupHandler.DeleteGroup`: admin-only (`caller.Role == RoleAdmin`, same gate as `UpdateGroup`), 204 on success, 403 non-admin, 404 when already deleted / not found. Route registered in `cmd/server/main.go`.
- openapi: new `delete` under `/api/v1/groups/{id}` (operationId `deleteGroup`).
- Frontend: `groupsApi.delete`, `useDeleteGroup`, and a destructive **Delete group** button (admin-only, in the overview member-list footer, testID `group-delete-button`) with a confirm dialog; on success navigates to the dashboard. Deleted group then disappears from all lists (dashboard/list) and direct fetch 404s.
- Out of scope (per instructions): no restore/undelete endpoint or UI.

## Item 2 — "always opens the first group I clicked" (stale group detail)
**Root-cause finding (traced from code; could not click through a live browser).** I could not reproduce a persistent-staleness defect from the *current* code, and I did not add a speculative navigator hack. Reasoning:
- Every group screen reads `id` reactively via `useLocalSearchParams` and every group-scoped query is keyed by `id` (`qk.group(id)`, `qk.statement(id,…)`, etc.), so a change of `id` — whether React Navigation **pushes a fresh `[id]` instance** or **updates params on the existing instance** — re-derives and refetches for the new group. Dashboard cards already navigate with full interpolated URLs (`router.push(\`/(app)/groups/${item.id}\`)`). This is exactly the fix documented in `docs/specs/V3-4.4.md §4.2`, and it is already in place.
- The one residual mechanism consistent with the report is a **desync between the URL and the mounted screen** caused by the broken back control (Item 3): a back press that changes route/history but fails to pop the mounted group screen leaves the previous group on screen; the next "open group B" then races that retained/desynced state. Fixing the back control to perform a real `router.back()`/replace (Item 3) removes that desync, so the back→open-other-group path yields a clean, correctly-keyed group screen.
- Net: **no separate speculative code change for Item 2**; it is addressed by the pre-existing reactive-param/keyed-query design plus the Item 3 back-navigation fix. Flagged here explicitly rather than papered over.

## Item 3 — Back button visible but inert on the group page
**Root cause.** The three group section screens (`groups/[id]/index`, `chores`, `loans/index`) are the **root** screen of their own nested Stack (`groups/[id]/_layout`). The native-stack default back chevron on a root screen calls *that* navigator's scoped `goBack()`, which has nothing to pop in its own stack — so the chevron renders (outer navigators have history) but does nothing.
- Fix: new `HeaderBackButton` component wired as `headerLeft` on those three root screens. It uses expo-router's global `router.back()` (pops real cross-navigator history) with a `router.replace('/(app)')` fallback when there is nothing to pop (deep link / web refresh). The drill-in screens (`loans/[loanId]`, `members/[userId]`) have real in-stack history, so their native back is left untouched.
- testID `header-back-button`.

## Item 4 — Optional base-pay field on the add-member form
- Backend: `AddMemberRequest.base_pay *models.Money` (optional). Validated (currency must match group via `checkMoneyCurrency`; value ≥ 0) before the tx. New `AllowanceRepo.SetAllowanceTx` runs the allowance upsert on the add-member transaction's `Querier`, so the membership insert and the base-pay seed commit **atomically** (no half-done member-without-allowance). `effective_from` = current month; `created_by` = the admin.
- openapi: `base_pay` added to `AddMemberRequest`; FE types regenerated.
- Frontend: optional numeric **Base pay** input in `AddMemberSheet` (parsed with `parseMoneyToMinorUnits`, mirroring `AddEntrySheet`); sent only when non-empty; `AddMemberSheet` now takes a `currency` prop (passed from the group overview). testID `add-member-base-pay`.

## Item 5 — Loan EMI: start this month vs next month
- Backend: optional `start_current_month *bool` on both `CreateLoanRequest` and `ApproveLoanRequest`. New helper `loanStartPeriod(now, startCurrentMonth)` → current month when true, else `loanNextPeriod(now)` (unchanged default). Wired into **both** the admin pre-approved create path (`CreateLoan`) and the approval path (`ApproveLoan`). Omitting the field preserves the existing next-month behavior, so existing callers/tests are unaffected.
- openapi: `start_current_month` added to both loan request schemas; FE types regenerated.
- Frontend: a two-option **First installment** toggle ("Next month" default / "This month") in the loan section of `AddEntrySheet` (admin pre-approved loan) and in `LoanApproveSheet` (approval). Member self-request in `loans/index` intentionally has no toggle — a request has no start period until approval. testIDs `loan-start-*-month`, `loan-approve-start-*-month`.
