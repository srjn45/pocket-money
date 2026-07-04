# WP-0.2 Spec: Generated API Types + TanStack Query

**Work package:** WP-0.2 (Phase 0 — Stabilize the MVP)
**Type:** Frontend-only (no backend or `openapi.yaml` changes)
**Depends on:** WP-0.1 (deps alignment) landing first is preferred but not strictly required; this WP does not itself bump the Expo SDK.
**Acceptance criteria (from master plan §9):** codegen script present in `app/package.json`; no hand-written response types remain in screens.

## 0. Scope & guardrails

In scope:
1. Add `openapi-typescript` codegen producing `app/src/api-types.gen.ts` from `backend/openapi.yaml`.
2. Add `@tanstack/react-query`, mount its provider in `app/app/_layout.tsx`.
3. Replace hand-written response interfaces in `app/src/api.ts` with aliases over the generated types.
4. Migrate every screen currently doing `useState`+`useEffect`+manual fetch to query/mutation hooks following the §7.3 key convention.

Explicitly **out of scope** (do not do these here, they belong to later WPs):
- Do **not** change `backend/openapi.yaml` (money stays `number`/`double` until WP-1.1; `/pending` and `/settlements` stay until WP-1.2). Generate from the spec **as it is today**.
- Do **not** redesign any UI, restructure tabs, or remove the Pending/Settlements screens (that is WP-1.3). Keep current screens working, only swap their data layer.
- Do **not** introduce `theme.ts` / the component kit (WP-4.1).
- Do **not** change the 401→logout behavior (only preserve it); the auth-redirect bug is WP-0.3.
- Keep `parseFloat` money handling exactly as-is for now — WP-1.1 owns the minor-units conversion. Changing it here would collide with that WP.

## 1. Codegen setup

**Package:** `openapi-typescript` (v7.x) as a `devDependency` in `app/`. It is a pure type generator: it reads the OpenAPI document and emits `.d.ts`-style TypeScript. It has no runtime footprint and pulls in no client library.

**Why it does not need the LAN backend:** `openapi-typescript` reads the **local committed file** `backend/openapi.yaml`, not a running server. The home/LAN server being off is irrelevant. The spec file is the contract (master plan §3.2, §10.1), so codegen is a deterministic, offline, source-file → source-file transform. Both `backend/openapi.yaml` and the generated `app/src/api-types.gen.ts` are **committed** to git.

**Output path:** `app/src/api-types.gen.ts` (exact path mandated by master plan §3.2 / §7.3). The file is generated, committed, and **never hand-edited** — add a leading banner comment via the tool's `--header`/`--enum` options is optional; at minimum rely on the tool's own "do not edit" behavior and note it in the file's top comment if the version supports it.

**npm scripts to add** to `app/package.json` (paths are relative to `app/`, since npm runs scripts with `cwd = app/`):

```jsonc
"scripts": {
  // ...existing...
  "codegen": "openapi-typescript ../backend/openapi.yaml -o ./src/api-types.gen.ts",
  "codegen:check": "openapi-typescript ../backend/openapi.yaml -o ./src/api-types.gen.ts && git diff --exit-code src/api-types.gen.ts"
}
```

- `codegen` — the canonical command every WP runs after touching the API (master plan §10.1: "then `npm run codegen` in `app/`").
- `codegen:check` — regenerates and fails if the committed file is stale; intended for CI so drift between `openapi.yaml` and the committed types is caught. (Wiring it into the GitHub Actions workflow is a nice-to-have; if the existing CI job is backend-only, note it in the PR rather than expanding CI scope here.)

**Invocation model — manual script, not a build/prebuild step.** Reasons:
- Codegen output is committed, so a normal `expo start` / EAS build must not depend on it.
- Coupling it to `expo start` would require `../backend/openapi.yaml` to resolve in every build context (e.g. EAS cloud builds where only `app/` is uploaded), which is fragile.
- Do **not** add it as a `prestart`/`prebuild` hook. It is run deliberately by whichever agent changes the contract, then committed. This is the discipline that keeps small-model agents from drifting (master plan §3.2).

**devDependency note:** pin a caret range, e.g. `"openapi-typescript": "^7.4.0"`. It requires Node ≥ 18 (already satisfied by the Expo toolchain). No `typescript` peer bump needed beyond the existing `~5.6.0`.

## 2. TanStack Query setup

**Package:** `@tanstack/react-query` (v5.x) as a runtime `dependency` in `app/`. v5 supports React 18 (`react@18.3.1` is installed) and React Native/Web out of the box. Do **not** add `@tanstack/react-query-persist-client` or async-storage persistence in this WP (offline sync is explicitly deferred, master plan §3). The `@tanstack/react-query-devtools` package is web/DOM-oriented; skip it to avoid RN bundling issues.

**Provider placement:** wrap the app in `QueryClientProvider` in `app/app/_layout.tsx`, **outside** `AuthProvider` so the client survives login/logout and auth hooks can also use Query later. Create the `QueryClient` once at module scope (not inside the component body) so it is stable across re-renders:

```
_layout.tsx (shape, not final code)
  const queryClient = new QueryClient({ defaultOptions: {...} })   // module scope
  <QueryClientProvider client={queryClient}>
    <AuthProvider>
      <Stack> ... </Stack>
    </AuthProvider>
  </QueryClientProvider>
```

**Default options (tuned for a sometimes-off LAN backend):**

```
defaultOptions: {
  queries: {
    retry: 1,                       // LAN server may be down; don't hammer with the default 3 retries + backoff
    staleTime: 30_000,              // 30s: avoid refetch storms when navigating between the group tabs
    refetchOnWindowFocus: true,     // master plan §7.3: refetch on focus (web tab focus / RN AppState foreground)
    refetchOnReconnect: true,
  },
  mutations: {
    retry: 0,                       // never auto-retry writes (approve/create/settle must be idempotent-by-user-intent)
  },
}
```

- `refetchOnWindowFocus` on RN: TanStack wires this to `AppState` automatically only if the focus manager is set up. Add the standard RN `AppState` → `focusManager.setFocused(...)` bridge in `_layout.tsx` (a ~6-line `useEffect` from the TanStack RN docs) so mobile gets focus-refetch too. On web it works natively.
- Keep pull-to-refresh: list screens call the query's `refetch()` from the existing `RefreshControl` (see §4).
- Global 401 handling stays in `app/src/api.ts`'s `request()` (it already calls `clearToken()` + `onUnauthorized`). Query does not replace that; failed queries will surface the thrown `Error('Unauthorized')` and the existing handler still fires. Do not duplicate 401 logic in a QueryCache `onError`.

## 3. Query-key convention (§7.3) — confirmed & refined

Adopt the master-plan keys. For WP-0.2 only the keys whose endpoints exist today are actually used; the loans/allowances/member-summary keys are reserved for later WPs but should be pre-declared in the key factory so future agents don't re-invent them.

Centralize keys in a factory module **`app/src/query-keys.ts`** so screens never hand-write array literals:

```
export const qk = {
  groups: () => ['groups'] as const,
  group: (id) => ['group', id] as const,
  members: (groupId) => ['members', groupId] as const,
  chores: (groupId) => ['chores', groupId] as const,
  ledger: (groupId, filters) => ['ledger', groupId, filters ?? {}] as const,
  balance: (groupId) => ['balance', groupId] as const,
  pending: (groupId) => ['pending', groupId] as const,          // transitional; removed in WP-1.2/1.3
  settlements: (groupId) => ['settlements', groupId] as const,  // transitional; removed in WP-1.2/1.3
  // reserved for later WPs (declare now, unused in 0.2):
  loans: (groupId) => ['loans', groupId] as const,
  allowances: (groupId) => ['allowances', groupId] as const,
  memberSummary: (groupId, userId) => ['member-summary', groupId, userId] as const,
}
```

Refinements to the raw §7.3 list:
- **`['ledger', groupId, filters]`** — `filters` is the `{ status?, user_id? }` object the current `ledgerApi.list` already accepts. Always pass a normalized object (default `{}`) so keys are stable/serializable; never pass `undefined` in the middle.
- Add **`['members', groupId]`** (the `/members` endpoint and the `.members` array inside `GroupDetail` are both used by screens to determine `isHead`). In practice most screens read members off `['group', id]` (which is `GroupDetailResponse` and already contains `members`), so a separate `members` fetch is only needed where a screen calls `groupsApi.getMembers` directly — prefer reusing `['group', id]` to avoid a redundant request.
- Add transitional **`['pending', …]`** and **`['settlements', …]`** so the still-existing Pending/Settlements screens can be migrated too rather than left as the only manual-fetch holdouts (acceptance criterion says *no* hand-written response types remain in screens). These keys and their hooks get deleted in WP-1.3.

**Mutation → invalidation matrix** (each mutation calls `queryClient.invalidateQueries` on exactly these keys):

| Mutation (current API) | Invalidates |
|---|---|
| `createGroup` | `['groups']` |
| `joinGroup` | `['groups']` |
| `createInvite` | *(none — no cached list changes)* |
| `createChore` (groupId) | `['chores', groupId]`, `['group', groupId]` (chores_count) |
| `updateChore` (chore → its groupId) | `['chores', groupId]` |
| `deleteChore` (chore → its groupId) | `['chores', groupId]`, `['group', groupId]` |
| `createLedger` (groupId) | `['ledger', groupId]` (all filter variants), `['balance', groupId]`, and `['pending', groupId]` if it lands pending |
| `approveLedger` (entry → its groupId) | `['ledger', groupId]`, `['balance', groupId]`, `['pending', groupId]` |
| `rejectLedger` (entry → its groupId) | `['ledger', groupId]`, `['balance', groupId]`, `['pending', groupId]` |
| `createSettlement` (groupId) | `['settlements', groupId]`, `['ledger', groupId]`, `['balance', groupId]` |

Notes:
- Invalidating `['ledger', groupId]` (2-element prefix) matches **every** `['ledger', groupId, filters]` variant — TanStack does prefix matching, so you don't enumerate filters. This is the intended pattern.
- Chore/ledger mutations receive an id, not a groupId, in the current API (`updateChore(id)`, `approveLedger(id)`). The hook must know the groupId to invalidate. Pass `groupId` explicitly into the mutation hook (`useApproveLedger(groupId)`), since the calling screen always has it from the route param. Do not try to derive it.

## 4. Files that change and what changes in each

Create these new modules:
- **`app/src/api-types.gen.ts`** — generated (via `npm run codegen`).
- **`app/src/query-keys.ts`** — the `qk` factory above.
- **`app/src/hooks/`** — one file per resource (`useGroups.ts`, `useGroup.ts`, `useChores.ts`, `useLedger.ts`, `useBalance.ts`, `useSettlements.ts`, `usePending.ts`) or a single `app/src/hooks.ts` — implementer's choice, but each hook wraps the existing `groupsApi`/`choresApi`/`ledgerApi`/`settlementsApi` functions from `api.ts` as the `queryFn`/`mutationFn`. **Do not rewrite the fetch client** — hooks sit on top of it.

Modify:

| File | Change |
|---|---|
| `app/package.json` | add `@tanstack/react-query` dep, `openapi-typescript` devDep, `codegen` + `codegen:check` scripts. |
| `app/app/_layout.tsx` | create module-scope `QueryClient` with the §2 defaults; wrap tree in `QueryClientProvider` outside `AuthProvider`; add the `AppState`→`focusManager` bridge. |
| `app/src/api.ts` | replace the hand-written `interface`s (§5) with type aliases over generated schemas; keep the `request()` client, the four `*Api` objects, and `setOnUnauthorized` exactly as-is. |
| `app/app/(app)/index.tsx` (Dashboard) | replace `loadGroups`/`useState`/`useEffect` with `useGroups()` + per-group `useQueries` (or a small combined hook) for detail+balance; `RefreshControl.onRefresh` → `refetch()`; join flow → `useJoinGroup()` mutation invalidating `['groups']`. Keep the enrichment logic (head vs member, totalBalance) but move it into `select`/derived state. |
| `app/app/(app)/groups/index.tsx` | replace manual groups fetch with `useGroups()`. |
| `app/app/(app)/groups/create.tsx` | replace manual create with `useCreateGroup()` mutation (invalidate `['groups']`); keep form/validation. |
| `app/app/(app)/groups/[id]/_layout.tsx` | replace the `loadGroup` effect with `useGroup(id)`; derive `isHead` from `data.members`; keep the loading spinner and the head/member tab split unchanged. |
| `app/app/(app)/groups/[id]/index.tsx` (Overview) | `useGroup(id)` + `useBalance(id)`; `onRefresh` → refetch both; invite → `useCreateInvite()` mutation; keep all rendering/styles. |
| `app/app/(app)/groups/[id]/chores.tsx` | `useChores(id)` + `useGroup(id)` (for `isHead`); create/update/delete → mutation hooks invalidating `['chores', id]` (+`['group', id]`); keep modal/form as-is. |
| `app/app/(app)/groups/[id]/ledger.tsx` | `useLedger(id, filters)` (+ `useChores`, `useBalance`, group as needed); approve/reject/create → mutation hooks with the §3 invalidations; keep current UI. |
| `app/app/(app)/groups/[id]/pending.tsx` | `usePending(id)` + approve/reject mutations. *(Screen is deleted in WP-1.3; migrate now only to satisfy "no manual fetch" — or, if the tab is already `href:null` hidden, note it as dead and leave it. Recommend: migrate minimally.)* |
| `app/app/(app)/groups/[id]/settlements.tsx` | `useSettlements(id)` + `useCreateSettlement(id)`. Same WP-1.3 caveat as pending. |
| `app/app/invite.tsx` | join via `useJoinGroup()` mutation invalidating `['groups']`. |

Do **not** touch `login.tsx` / `register.tsx` / `profile.tsx` / `auth-context.tsx` for data-layer reasons: auth flows are imperative one-shots already handled by `AuthProvider` and don't benefit from Query in this WP. Leave them. (`auth-context.tsx` keeps importing `User` from `api.ts`, which continues to work because of §5.)

**Per-file pattern for the migration** (apply uniformly): delete the `data`/`isLoading`/`refreshing`/`error` `useState`s and the `loadX`+`useEffect`; get `{ data, isLoading, error, refetch, isRefetching }` from the query hook; feed `isRefetching` to `RefreshControl.refreshing` and `refetch` to `onRefresh`; replace imperative `await xApi.mutate(); loadX()` blocks with `mutation.mutateAsync(...)` (invalidation handled inside the hook); keep every `StyleSheet`, JSX layout, `Alert`, and validation branch untouched.

## 5. Generated types replacing hand-written interfaces

The generated file exposes `paths`, `components['schemas'][...]`, and `operations`. Map the current hand-written interfaces to schema aliases. In `app/src/api.ts`, **delete** the `interface` bodies and replace with:

```
import type { components } from './api-types.gen';
type Schemas = components['schemas'];

export type User          = Schemas['UserResponse'];
export type Group         = Schemas['GroupResponse'];
export type GroupDetail   = Schemas['GroupDetailResponse'];
export type Member        = Schemas['MemberResponse'];
export type Chore         = Schemas['ChoreResponse'];
export type LedgerEntry   = Schemas['LedgerResponse'];
export type Balance       = Schemas['BalanceResponse'];
export type Settlement    = Schemas['SettlementResponse'];
export type InviteResponse = Schemas['InviteResponse'];
export type LoginResponse  = Schemas['LoginResponse'];
export type ApiError       = Schemas['ErrorResponse'];
```

Consequences / things to verify while doing this:
- **Re-export from the same module** (`api.ts`) under the **same names** so all existing `import { Group, ... } from '.../src/api'` sites (enumerated in §4) keep compiling unchanged. This is the coexistence mechanism — the hand-written client stays; only the type *source* changes.
- **Optionality differs.** Generated types make non-`required` fields `field?: T` and `nullable: true` fields `T | null`. E.g. `LedgerResponse.approved_by_user_id` becomes `string | null` (was `string | undefined`), and request bodies' optional fields may now be `?: T | null`. Fix the handful of resulting type errors at call sites (mostly in `ledger.tsx`) — this is expected churn, resolve by using generated request-body types too where convenient (`Schemas['CreateChoreRequest']`, etc.).
- **Money stays `number`.** In today's spec `amount`/`balance` are `number` (`format: double`). Screens keep `.toFixed(2)`/`parseFloat`. WP-1.1 will flip the spec to integer and regenerate; that WP owns the `AmountText` formatting migration. Do not pre-empt it.
- **Coexistence period:** none needed beyond this WP — because aliases keep the public names stable, there is no dual-type window. The hand-written `interface`s are removed in the same commit that adds the generated file. The only genuinely transitional artifacts are the `pending`/`settlements` hooks/keys, which persist until WP-1.3.
- One subtlety: the generated `LedgerResponse` still types `chore_id` as **required** (per current spec), even though WP-1.2 will make it nullable. That is correct for now — don't anticipate the schema change.

## 6. Verification steps

Run from `app/` unless noted. State in the PR exactly what was run (master plan §10.3).

1. **Install:** `npm install` (adds the two packages). Confirm `openapi-typescript` and `@tanstack/react-query` appear in `package.json` and lockfile.
2. **Codegen runs cleanly against the committed spec:**
   `npm run codegen`
   - Expect exit 0 and a written `src/api-types.gen.ts` containing `components['schemas']['GroupResponse']` etc.
   - `npm run codegen:check` → exit 0 (proves the committed file matches a fresh generation; no drift).
3. **Types compile:**
   `npx tsc --noEmit`
   - Must pass with zero errors. This is the primary gate — it proves every screen still type-checks against the generated types and the Query hooks. Resolve the optional/nullable fallout from §5 here.
4. **Lint:** `npm run lint` (`expo lint`) — no new errors.
5. **App boots on web and existing screens work** (master plan §10.3 requires web boot + exercising the changed flow). With the LAN backend reachable:
   - `npm run web`, log in → **Dashboard** loads groups (query, not manual fetch); pull-to-refresh refetches.
   - Open a group → **Overview** shows members+balances; **Chores** loads; head can create/edit/delete a chore and the list updates **without a manual reload** (proves invalidation).
   - **Ledger**: create an entry / approve / reject → balance and ledger update via invalidation.
   - Verify **focus refetch**: switch browser tabs away and back → a stale query refetches.
   - **401 path still works:** with an expired/at-cleared token, a query failure still triggers logout→login (existing `onUnauthorized` behavior preserved — do not regress it; note this is validated more thoroughly in WP-0.3).
6. **Android smoke (if WP-0.1 has aligned deps):** `npm run android`, repeat the login→dashboard→group happy path; confirm `AppState`-driven focus refetch fires on foregrounding. If deps are not yet aligned and Android won't boot, note it and defer to WP-0.1 rather than fixing deps here.
7. **Grep gate for the acceptance criterion:** confirm no screen declares its own response `interface`/`type` for API payloads and none reads JSON into an inline type — all payload types come from `api.ts` aliases (which now resolve to generated schemas). `grep -rn "interface .*Response" app/` should return nothing in screen files.

**Definition of done:** codegen + `codegen:check` green; `tsc --noEmit` and lint clean; `QueryClientProvider` mounted; all listed screens use hooks with the §3 keys/invalidations; hand-written response interfaces removed from `api.ts`; web happy-path verified with live invalidation; committed files include `api-types.gen.ts`. Commit/PR titled `WP-0.2: generated API types + TanStack Query`.
