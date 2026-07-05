# WP-1.3 Spec: FE Unified Ledger

**Work package:** WP-1.3 (Phase 1 — Ledger v2), master-plan §9.
**Type:** Frontend-only. Rebuilds the group **Overview** screen (both roles) + the head's per-member ledger on the **WP-1.0 design-system kit**, consuming the **WP-1.2 ledger v2 API**. Regenerates the committed FE types from the WP-1.2 `openapi.yaml` (this WP owns the regeneration). **No backend, `openapi.yaml`, or migration changes.**
**Risk:** Low-to-medium FE (no money math of its own — all formatting goes through `AmountText`; all authz is API-enforced and mirrored in the UI per §5.5). Master-plan §11: Sonnet implement, Sonnet/Opus review; review must check that types were regenerated, empty/error states exist, and no `Alert.alert` is used for confirms/feedback.

**Depends on (must be merged before implement starts):**
- **WP-1.0** (design-system core) — `theme.ts` + the kit (`Button`, `Card`, `ListRow`, `AmountText`, `StatusBadge`, `Sheet`, `TextField`, `MonthHeader`, `Toast`, `EmptyState`/`ErrorMessage`/`LoadingSpinner`), `confirmAsync`, `formatMinorUnits`. **`Avatar` was deferred by WP-1.0** ("no consumer until WP-1.3/3.3") — **WP-1.3 creates it** (§4.1). If any kit component is missing, STOP and flag — do not re-implement it here.
- **WP-1.1** (money to minor units) — API money is integer minor units; `AmountText` takes minor units directly (no `rupeesToMinor` shim). `parseMoneyToMinorUnits` exists in `app/src/money.ts` for money input.
- **WP-1.2** (ledger v2 schema + API) — `openapi.yaml` has the v2 `LedgerResponse` (`entry_type`, `direction`, nullable `chore_id`, `loan_id`, `period`, `note`, `decided_by`, `decided_at`; **no** `approved_by_user_id`/`rejected_by_user_id`), the restructured `CreateLedgerRequest` (`entry_type` required; `settlement`/`adjustment` head-only + custom amount; `direction` for adjustment), the `?type=`/`?period=` ledger filters, and `/settlements` + `/pending` **removed**. Balance = Σ approved credits − Σ approved debits.

> **Sequencing reality (2026-07-05):** at spec-authoring time the WP-1.1/1.2 backend deltas are **not yet in `openapi.yaml`** (it still shows `type: number` money and the `settlements`/`pending` paths). WP-1.3 is written against the **landed WP-1.2 contract** documented in `docs/specs/WP-1.2.md §4`. The very first implementation step (`npm run codegen`, §6) regenerates types from whatever `openapi.yaml` is at that point — which **must** be the post-WP-1.2 spec. If `LedgerResponse` still lacks `entry_type` after codegen, WP-1.2 has not landed: STOP and flag.

**Acceptance (master-plan §9 WP-1.3 row):** "Rebuild group Overview per §7.1/§7.2 for both roles on the WP-1.0 kit; inline approve/reject; strikethrough rejected; month grouping; 'decided by' shown on decided entries; remove Pending/Settlements screens — matches `docs/fe-flow.md` + `docs/FE-bugs.md` on web + Android."

---

## 0. Goal & guardrails

Turn the group detail into the **unified ledger** experience of master-plan §7.1/§7.2, killing the dead-by-design ledger/pending/settlements surface:

- **HEAD Overview** (`index.tsx`): member cards — `Avatar` (initials) + name + balance (`AmountText`) + pending-count badge — that tap through to that member's unified ledger. A head "Add entry" action (chore for any member → approved / settlement / adjustment with custom amount + note).
- **MEMBER Overview** (`index.tsx`): own summary header (balance) + own **unified ledger** (month-grouped rows per §7.2). A member "Add entry" action (own chore entry → pending).
- **Per-member ledger** (`ledger.tsx`, head-only, reached by tapping a member card): that member's full unified ledger with **inline Approve/Reject** on pending rows + the head "Add entry" Sheet pre-scoped to the member.
- **Delete** `pending.tsx`, `settlements.tsx`, their tab/route wiring, and the dead API client functions; **regenerate** `api-types.gen.ts` from the WP-1.2 `openapi.yaml`.

### Stay-in-WP guardrails
- **No** backend / `openapi.yaml` / migration edits. WP-1.3 only *consumes* the WP-1.2 contract and regenerates the committed types.
- **No** rich **member detail page** (`members/[userId].tsx`) — that is **WP-3.3** (allowance editing, loan management, `member-summary` endpoint). WP-1.3's per-member view is the **unified ledger only** (§3.3). The interim member-card tap target is `ledger.tsx` (§3.5); WP-3.3 repoints it to `members/[userId].tsx` and may absorb/delete `ledger.tsx`.
- **No** loans tab / loan rows with real loan data (WP-3.2), **no** allowance UI (WP-2.2), **no** dashboard v2 (WP-4.2), **no** legacy-screen sweep (login/register/dashboard/profile/invite — WP-4.1).
- **No** new native dependencies (keeps `expo export --platform web` green — §6). The existing `@react-native-picker/picker` (already used by the old ledger modal) may be reused inside `AddEntrySheet`; prefer it over adding a new picker lib.
- **`allowance`/`emi` entries do not exist yet** at WP-1.3 runtime (the posting engine is WP-2.1/3.1). The row renderer must still **handle them defensively** (§2.2) so those rows render correctly the moment Phase 2/3 lands — but do not build allowance/loan *data* fetching here.

---

## 1. Screen structure (component trees)

Two role branches live in `index.tsx` (the Overview), switched on `isHead` (derived from `useGroup(id)` members + `useAuth().user.id`, exactly as the current screens do). `ledger.tsx` is the head-only per-member view.

### 1.1 HEAD Overview — `groups/[id]/index.tsx`

```
<View screen bg=background>
  <ErrorMessage/> or inline error banner        // on group/balance query error
  <View header>                                  // Card-like
     "<n> members"  +  <Button title="Invite" variant="ghost" icon="person-add">   // existing invite flow, Toast on web (§5)
  <SectionTitle> "Members" </>
  <Button title="Add entry" variant="primary" icon="add" onPress=openAddSheet />    // head add-entry (member-pick in Sheet)
  <FlatList data={members-with-balance} refreshControl>
     renderItem = <MemberCard
                     avatar={<Avatar name={m.name} id={m.user_id} />}
                     name={m.name}
                     right={<AmountText minorUnits={m.balance}
                              variant={m.balance < 0 ? 'debit' : 'credit'} />}
                     pendingCount={countPendingFor(m.user_id)}   // StatusBadge tone="warning" "N pending"
                     onPress={() => router.push(`/(app)/groups/${id}/ledger?userId=${m.user_id}&name=${m.name}`)} />
     ListEmptyComponent = <EmptyState icon="people-outline"
                              title="No members yet"
                              subtitle="Tap Invite to add your family" />
  <AddEntrySheet visible mode="head" members={nonHeadMembers} chores={chores} groupId={id} onClose/>
```

- **Data:** `useGroup(id)` (members + roles), `useBalance(id)` (per-member balances; head is excluded server-side), `useLedger(id, { status: 'pending_approval' })` (whole-group pending → derive per-member counts client-side), `useChores(id)` (for the Add-entry chore picker).
- **Negative balance (master-plan §5.3):** never clamp; `variant="debit"` renders `−₹…` in danger. A tiny "owes you" / "owed" hint may sit under the amount (optional, kid-legible §1).

### 1.2 MEMBER Overview — `groups/[id]/index.tsx`

```
<View screen bg=background>
  <ErrorMessage/> or inline error banner
  <View summaryHeader>                            // Card
     "Your balance"
     <AmountText minorUnits={myBalance} variant={myBalance<0?'debit':'credit'} size="xl" />
     <caption> owed to you / you owe  </caption>   // sign-aware hint (§5.3)
  <Button title="Log a chore" variant="primary" icon="add" onPress=openAddSheet />  // member add-entry (own, pending)
  <LedgerList entries={myEntries} chores={chores} members={members}
              isHead={false} groupId={id}
              emptyTitle="No entries yet"
              emptySubtitle="Log a chore to start earning" />
  <AddEntrySheet visible mode="member" selfUserId={user.id} chores={chores} groupId={id} onClose/>
```

- **Data:** `useGroup(id)`, `useBalance(id)` (member sees own row only — API-enforced), `useLedger(id, {})` (member sees own entries only — API-enforced; no `user_id` needed), `useChores(id)`.

### 1.3 HEAD per-member ledger — `groups/[id]/ledger.tsx` (nav target, `href: null`)

```
<Stack.Screen title="<name>'s ledger" />
<View screen>
  <View summaryHeader>  "<name>'s balance"  <AmountText minorUnits={memberBalance} variant=... size="xl" />
  <Button title="Add entry" variant="primary" icon="add" onPress=openAddSheet />
  <LedgerList entries={memberEntries} chores={chores} members={members}
              isHead={true} groupId={id}
              onApprove/onReject                        // inline actions on pending rows
              emptyTitle="No entries yet"
              emptySubtitle="Add a chore, settlement, or adjustment" />
  <AddEntrySheet visible mode="head" fixedUserId={userId} chores={chores} groupId={id} onClose/>
```

- **Params:** `userId`, `name` (from the member-card tap). **Data:** `useGroup(id)`, `useBalance(id)` → pick `userId`'s row, `useLedger(id, { user_id: userId })`, `useChores(id)`.
- **Guard:** if the current user is not head, `Redirect` to `./` (defense-in-depth; the API also enforces).

### 1.4 Shared building blocks (§4)

`LedgerList` (month grouping + `MonthHeader` + rows + empty/refresh) and `LedgerRow` (the §7.2 row) are shared by member Overview (1.2) and head per-member ledger (1.3). `AddEntrySheet` is shared by all three. This is the one place the ledger row is defined — no duplication.

---

## 2. Ledger row (master-plan §7.2) — anatomy, title mapping, decided-by

### 2.1 `LedgerRow` layout (composed from kit `ListRow`)

```
<ListRow
   left={<TypeIcon entry_type />}                    // Ionicons glyph per type (§2.3)
   title={entryTitle(entry, chores)}                 // §2.2 mapping
   subtitle={`${formatDate(entry.created_at)}${entry.note ? ' · ' + entry.note : ''}`}
   strikethrough={entry.status === 'rejected'}       // §7.2: rejected → strikethrough
   right={
     <column>
       <AmountText minorUnits={entry.amount}
                   variant={entry.direction === 'credit' ? 'credit' : 'debit'} />
       {status === 'pending_approval' && !isHead && <StatusBadge label="Pending" tone="warning" />}
       {status === 'pending_approval' && isHead &&
          <Approve/Reject buttons>                    // §2.4 inline actions
       {status === 'rejected' && <StatusBadge label="Rejected" tone="danger" />}
     </column>
   } />
{decidedByLine(entry, members)}                        // §2.5, rendered under the row when present
```

- **Amount color/sign comes from `direction`, not `entry_type`** (WP-1.2 invariant): `credit → +₹ green`, `debit → −₹ red`. `AmountText` handles sign + color + minor-units formatting.
- **Approved** entries show **no** status badge (§7.2 / master-plan §7.4: approved is the quiet default).
- **Rejected** entries: title strikethrough + `Rejected` danger badge; they render in the list but **do not** count toward any total (§2.6, FE-bugs).

### 2.2 Row-title mapping (`entryTitle(entry, chores)`)

Pure function in `app/src/ledger-format.ts`:

| `entry_type` | Title | Source |
|---|---|---|
| `chore` | the chore's name (e.g. **"Dishes"**) | look up `chores.find(c => c.id === entry.chore_id)?.name`; fallback `"Chore"` if not found |
| `allowance` | **"Pocket money — Jul 2026"** | `"Pocket money — " + humanMonth(entry.period)` (period is `YYYY-MM`; reuse `MonthHeader`'s month-name logic) |
| `emi` | **"EMI — <note>"** (interim) → **"EMI 3/12 — <note>"** in WP-3.2 | `"EMI"` + (`entry.note ? " — " + note : ""`). The `n/12` installment index needs loan data (WP-3.2) and is **out of scope**; render without it now, documented so WP-3.2 slots it in |
| `settlement` | **"Settlement"** | literal (chore_id is null) |
| `adjustment` | **"Adjustment"** | literal; the `note` (the "why") shows in the subtitle |

- **Friendly words (§1 principle 3):** "Pocket money", not "Allowance"; "Settlement" for payouts. Kid-legible.
- `humanMonth('2026-07')` → `"Jul 2026"` (parse `YYYY-MM`, index a hardcoded month array — same approach `MonthHeader` already uses; export a shared helper so both agree).

### 2.3 Type-icon mapping (`TypeIcon`)

Ionicons glyphs (already installed), colored `textSecondary` (neutral — direction drives the money color, not the icon):

| type | icon |
|---|---|
| `chore` | `checkmark-circle-outline` |
| `allowance` | `cash-outline` |
| `emi` | `card-outline` |
| `settlement` | `arrow-up-circle-outline` |
| `adjustment` | `create-outline` |

### 2.4 Inline Approve / Reject (head, pending rows only)

Right-slot buttons on `status === 'pending_approval'` when `isHead`:
- **Approve** → `useApproveLedger(groupId).mutate(entry.id)`; on success `Toast{tone:'success', "Approved"}`.
- **Reject** → `confirmAsync({ title:'Reject entry', message:'This entry won't count toward the balance.', destructive:true })` → if ok, `useRejectLedger(groupId).mutate(entry.id)`; on success `Toast{tone:'info', "Rejected"}`.
- While a row's mutation is in flight, disable both buttons and show a spinner (reuse `Button loading` or a small `ActivityIndicator`). On error → `Toast{tone:'danger', message}`.
- **`Alert.alert` is forbidden** (no-op on web, master-plan §1 principle 5 + WP-1.0 §4). Use `confirmAsync` + `Toast` only.

### 2.5 Decided-by line (master-plan §5.1: "approved by Dad, 3 Jul")

`decidedByLine(entry, members)` renders a small caption (`fontSize.xs`, `textSecondary`) **only when `entry.decided_by` is set** (i.e. the entry was head-decided or head-created-approved):
- approved → `"approved by {name}, {d Mon}"`  e.g. `"approved by Dad, 3 Jul"`.
- rejected → `"rejected by {name}, {d Mon}"`.
- `{name}` = `members.find(m => m.user_id === entry.decided_by)?.name ?? 'head'`.
- `{d Mon}` from `entry.decided_at` (nullable — historic rows have no `decided_at` per WP-1.2 §1.2 step 6; if null, omit the date and show just `"approved by {name}"`).
- Machine-posted `allowance`/`emi` and still-`pending` entries have no `decided_by` → **no line**.

### 2.6 Totals — approved-only, signed (FE-bugs)

Only **approved** entries count toward any displayed total; **pending shown but not counted**, **rejected shown strikethrough and not counted** (FE-bugs.md, master-plan §5.1). This applies to the month totals (§3.1) — balances come from `/balance`, which the backend already computes approved-only.

---

## 3. Month grouping (client-side)

### 3.1 Algorithm (`groupEntriesByMonth` in `app/src/ledger-format.ts`)

```
monthKeyOf(entry): string            // 'YYYY-MM'
   = entry.period ?? toYearMonth(entry.created_at)
```
- `period` (set on `allowance`/`emi`) wins so a machine-posted row groups under its accounting month; everything else groups by `created_at`. (Migrated settlement rows carry `created_at = payout date` per WP-1.2 §1.3, so they group under the month the payout happened — matching the old settlements-by-date UX.)
- `toYearMonth(iso)` uses the `Date`'s **local** year+month (`getFullYear`/`getMonth`) — no date lib. Caveat noted: server is the clock authority (§10.7); local-tz month bucketing is acceptable for display grouping and consistent with `MonthHeader`.

```
groupEntriesByMonth(entries):
   1. sort entries by created_at DESC (newest first)
   2. bucket into an ordered map keyed by monthKeyOf(entry)
   3. for each bucket compute monthTotal = Σ over APPROVED entries of
         (direction === 'credit' ? +amount : -amount)      // pending/rejected excluded (§2.6)
   4. return ordered [{ period, entries, monthTotal }]      // months already newest-first from the sort
```

### 3.2 Rendering in `LedgerList`

Use a `SectionList` (or a flattened `FlatList` with header items) so each month renders:
```
<MonthHeader period={section.period}
             totalMinorUnits={section.monthTotal}
             totalVariant={section.monthTotal < 0 ? 'debit' : 'credit'} />
<LedgerRow ... /> × N
```
- `MonthHeader` (WP-1.0) already formats `period → "July 2026"` and right-aligns an `AmountText` total.
- Empty list → `<EmptyState>` (props passed from the screen, §1). `RefreshControl` pull-to-refresh wired to the query's `refetch`. `refetchOnWindowFocus` is on by default from the WP-0.2 Query provider.

---

## 4. Components, hooks, query-keys, api client

### 4.1 New components (`app/src/components/`)

| File | What | Notes |
|---|---|---|
| `Avatar.tsx` | Initials avatar, bg color hashed from user id (master-plan §7.4). Props: `{ name: string; id: string; size?: number }`. | **WP-1.0 deferred this to WP-1.3** — create it here, add to the barrel. Hash id → pick from a small token-derived palette; initials = first letter of name (uppercased). |
| `LedgerRow.tsx` | The §2 row (composes `ListRow` + `AmountText` + `StatusBadge` + type icon + optional inline approve/reject + decided-by line). | Props in §4.4. |
| `LedgerList.tsx` | Month-grouped list (§3): `SectionList` of `MonthHeader` + `LedgerRow`, empty/refresh. | Props in §4.4. |
| `AddEntrySheet.tsx` | The create-entry `Sheet` (§4.5). | Uses kit `Sheet`/`TextField`/`Button` + `@react-native-picker/picker`. |
| `MemberCard.tsx` | Head Overview member card (`Card` + `Avatar` + name + `AmountText` + pending `StatusBadge`). | Optional as a component vs inline; a component keeps `index.tsx` legible. |

`app/src/components/index.ts` — barrel-export `Avatar`, `LedgerRow`, `LedgerList`, `AddEntrySheet`, `MemberCard`.

### 4.2 New hooks (`app/src/hooks/useLedger.ts`)

```ts
export function useLedger(groupId: string, filters?: LedgerFilters)   // GET /ledger?status=&user_id=&type=&period=
export function useBalance(groupId: string)                           // GET /balance
export function useCreateLedgerEntry(groupId: string)                 // POST /ledger
export function useApproveLedger(groupId: string)                     // POST /ledger/:id/approve
export function useRejectLedger(groupId: string)                      // POST /ledger/:id/reject

type LedgerFilters = { status?: string; user_id?: string; type?: string; period?: string };
```

- `useLedger` → `useQuery({ queryKey: qk.ledger(groupId, filters), queryFn: () => ledgerApi.list(groupId, filters), enabled: !!groupId })`.
- `useBalance` → `useQuery({ queryKey: qk.balance(groupId), queryFn: () => ledgerApi.getBalance(groupId), enabled: !!groupId })`.
- **All three mutations invalidate `ledger` + `balance` + `group`** (master-plan §7.3: "Mutations invalidate the affected group's keys") in `onSuccess`:
  ```ts
  onSuccess: () => {
    qc.invalidateQueries({ queryKey: qk.ledger(groupId) });   // partial key → all filter variants
    qc.invalidateQueries({ queryKey: qk.balance(groupId) });
    qc.invalidateQueries({ queryKey: qk.group(groupId) });
  }
  ```
  (Invalidating `qk.ledger(groupId)` — a **prefix** `['ledger', groupId]` — matches every `['ledger', groupId, filters]` cache entry, so the head's pending-count query and the per-member ledger both refresh after an approve/reject/create.)

Follows the exact `useChores.ts` pattern (query + mutations + `useQueryClient`).

### 4.3 Query-keys (`app/src/query-keys.ts`)

- **Keep** `ledger`, `balance`, `group`, `members`, `chores` (already present and correct: `ledger: (groupId, filters?) => ['ledger', groupId, filters ?? {}]`).
- **Remove** `pending: () => ['pending', groupId]` and `settlements: () => ['settlements', groupId]` (their endpoints are gone; pending is now `useLedger(id, { status: 'pending_approval' })`).
- Leave the reserved `loans`/`allowances`/`memberSummary` keys (WP-2.x/3.x) untouched.

### 4.4 Component prop contracts

```ts
interface LedgerRowProps {
  entry: LedgerEntry;                 // Schemas['LedgerResponse'] (v2)
  chores: Chore[];                    // for chore-name title lookup
  members: Member[];                  // for decided-by name lookup
  isHead: boolean;                    // gates inline approve/reject
  onApprove?: (id: string) => void;   // required-ish when isHead && pending
  onReject?: (id: string) => void;
  processing?: boolean;               // this row's mutation in flight → disable buttons/spinner
}

interface LedgerListProps {
  entries: LedgerEntry[];
  chores: Chore[];
  members: Member[];
  isHead: boolean;
  groupId: string;
  onApprove?: (id: string) => void;
  onReject?: (id: string) => void;
  refreshing?: boolean;
  onRefresh?: () => void;
  emptyTitle: string;
  emptySubtitle?: string;
}

interface AddEntrySheetProps {
  visible: boolean;
  onClose: () => void;
  groupId: string;
  chores: Chore[];
  mode: 'head' | 'member';
  members?: Member[];        // head: pickable non-head members (Overview entry point)
  fixedUserId?: string;      // head per-member ledger: member pre-selected, picker hidden
  selfUserId?: string;       // member: own id (target is always self)
}
```

### 4.5 `AddEntrySheet` behavior (master-plan §5.5 authz, mirrored in UI)

A kit `<Sheet title="Add entry">` with fields gated by `mode`:

**mode="member"** (own chore, pending):
- **Entry kind:** chore only (members cannot settle/adjust — §5.5).
- Chore picker over **non-system** chores (`chores.filter(c => !c.is_system)`); amount is **read-only** (chore's configured amount, shown via `AmountText`; members can't set a custom amount — WP-1.2 §3).
- Caption: "This will be sent to the head for approval."
- Submit → `useCreateLedgerEntry` with `{ entry_type: 'chore', chore_id }` (no `user_id` → server defaults to self; no `amount`). Result: `pending_approval`.

**mode="head"** — an **entry-kind selector** (segmented control / picker: *Chore · Settlement · Adjustment*):
- **Member target:** if `fixedUserId` → hidden, use it; else a member picker over `members` (non-head). Required.
- **Chore:** chore picker over non-system chores; amount read-only (chore amount). → `{ entry_type:'chore', user_id, chore_id }` → **approved**.
- **Settlement:** `TextField` amount (parse via `parseMoneyToMinorUnits`, must be > 0) + optional `note`. → `{ entry_type:'settlement', user_id, amount, note }` → approved debit.
- **Adjustment:** `TextField` amount (> 0) + **direction** toggle (credit "add to balance" / debit "subtract from balance") + `note` (encouraged — the "why", §5.1). → `{ entry_type:'adjustment', user_id, amount, direction, note }` → approved.
- **allowance/emi are never offered** (machine-posted; WP-1.2 rejects them with 400).

Common: validate before submit; Save disabled while invalid or while the mutation is pending (`Button loading`). Success → `onClose()` + `Toast{tone:'success', "Entry added"}`; error → `Toast{tone:'danger', message}`. **No `Alert.alert`.**

### 4.6 `app/src/api.ts` changes

- **`ledgerApi.list`** — extend options to `{ status?, user_id?, type?, period? }`; append `type`/`period` query params (mirrors WP-1.2 `?type=&period=`).
- **`ledgerApi.create`** — new body shape:
  ```ts
  create: (groupId: string, data: {
    entry_type: 'chore' | 'settlement' | 'adjustment';
    user_id?: string;                 // head may target a member; member omits (self)
    chore_id?: string;                // required iff entry_type === 'chore'
    amount?: number;                  // minor units; required iff settlement/adjustment
    direction?: 'credit' | 'debit';   // required iff entry_type === 'adjustment'
    note?: string;
  }) => request<LedgerEntry>(`/groups/${groupId}/ledger`, { method: 'POST', body: JSON.stringify(data) }),
  ```
- **Delete `ledgerApi.listPending`** (the `/pending` endpoint is gone).
- **Delete the entire `settlementsApi` object** and the `export type Settlement = Schemas['SettlementResponse']` line (the schema no longer exists after regen — leaving it makes `tsc` fail, which is the intended forcing function).
- `ledgerApi.approve`/`reject`/`getBalance` unchanged.

### 4.7 `app/src/api-types.gen.ts` — regenerate (this WP owns it)

Run `npm run codegen` (§6). `LedgerResponse` gains `entry_type`/`direction`/`loan_id`/`period`/`note`/`decided_by`/`decided_at` and drops `approved_by_user_id`/`rejected_by_user_id`; `SettlementResponse`/`CreateSettlementRequest` and the `/settlements`,`/pending` operations disappear. Commit the regenerated file; `npm run codegen:check` must pass (no drift).

---

## 5. Navigation & files — created / changed / deleted

### 5.1 `groups/[id]/_layout.tsx` (changed)

Both roles get the **same two tabs**: **Overview** (`index`) + **Chores** (`chores`). Simplify the current role-forked tab config:
- **Remove** the `settlements` and `pending` `<Tabs.Screen>` entries entirely (files deleted).
- **`ledger`** stays registered as `href: null` (hidden nav target reached by the head member-card tap, §3.3) for **both** roles.
- **Member:** Overview (`index`) is now the member's own-ledger home (was previously hidden with `ledger` as the primary tab — flip it). Chores stays read-only.
- **Head:** Overview (`index`) = member cards; Chores. Unchanged tab set minus the dead screens.
- Keep `Ionicons` tab icons; can drop the `groupsApi.get` `useEffect`/`useState` loading in favor of `useGroup(id)` for the title + role (optional tidy — allowed, it's this screen's own logic).

### 5.2 File ledger

**Created**
- `app/src/hooks/useLedger.ts`
- `app/src/ledger-format.ts` (`entryTitle`, `humanMonth`, `monthKeyOf`, `groupEntriesByMonth`, `decidedByLine` helpers — pure, unit-testable)
- `app/src/components/Avatar.tsx`
- `app/src/components/LedgerRow.tsx`
- `app/src/components/LedgerList.tsx`
- `app/src/components/AddEntrySheet.tsx`
- `app/src/components/MemberCard.tsx`

**Changed**
- `app/src/api-types.gen.ts` (regenerated — §4.7)
- `app/src/api.ts` (§4.6: ledger list/create, delete `listPending` + `settlementsApi` + `Settlement` type)
- `app/src/query-keys.ts` (§4.3: remove `pending`, `settlements`)
- `app/src/components/index.ts` (barrel: add the 5 new components)
- `app/app/(app)/groups/[id]/_layout.tsx` (§5.1)
- `app/app/(app)/groups/[id]/index.tsx` (rebuild Overview, both roles — §1.1/§1.2, on the kit)
- `app/app/(app)/groups/[id]/ledger.tsx` (rebuild as head-only per-member unified ledger — §1.3, on the kit)

**Deleted**
- `app/app/(app)/groups/[id]/pending.tsx`
- `app/app/(app)/groups/[id]/settlements.tsx`

> Deleting the two screen files removes their expo-router routes automatically (file-based routing); the only wiring to clean is the `<Tabs.Screen name="pending"/settlements">` entries in `_layout.tsx` (§5.1).

### 5.3 Interim member-card tap behavior (the decision)

**Decision: the head member-card tap navigates to `ledger.tsx` (the per-member unified ledger, §1.3), NOT to a member-detail page.**

Rationale: the rich member-detail page (`members/[userId].tsx` — balance + **editable allowance** + **loans** + settlement/adjustment via Sheet + `member-summary` endpoint) is **WP-3.3** and is explicitly out of scope (§0). But WP-1.3 still owes the head a way to (a) view any member's full ledger and (b) approve/reject pending entries inline — both required by FE-bugs.md. The unified-ledger screen delivers exactly that and nothing more (no allowance/loan surface). It is reached by `router.push('/(app)/groups/${id}/ledger?userId=${m.user_id}&name=${m.name}')` and is hidden from the tab bar (`href: null`). **WP-3.3 handoff:** repoint the tap to `members/[userId].tsx` and fold this ledger view into that page (it may then delete `ledger.tsx`). Recorded here; do not build it now.

---

## 6. Verification commands (master-plan §10.3, FE floor)

Run from `app/`. State exactly what was run in the PR.

```bash
cd app

# 1. Regenerate committed types from the WP-1.2 openapi — THE FIRST STEP (§4.7).
npm run codegen
npm run codegen:check          # exit 0 → committed spec ⇒ committed types, no drift

# 2. Types — primary gate. Deleting settlementsApi/Settlement + the v2 LedgerResponse
#    must leave zero type errors (broken references are the forcing function).
npx tsc --noEmit

# 3. Lint — no new errors.
npm run lint

# 4. Web export — MANDATORY (§10.3 FE floor; tsc/expo-doctor miss bundle-level breakage).
npx expo export --platform web     # must exit 0, emit dist/

# 5. Grep gates:
grep -rn "settlementsApi\|listPending\|SettlementResponse" app/src app/app   # expect: none
grep -rn "Alert.alert" app/app app/src | grep -v "src/confirm.ts"            # expect: no confirm/feedback hits
grep -rn "pending.tsx\|settlements.tsx" app/app                               # expect: files gone, no refs
grep -rn "entry_type\|direction\|decided_by" app/app/\(app\)/groups          # expect: v2 fields consumed
```

Interactive (if a browser/dev server is available — `npm run web`): walk the §7 acceptance matrix on web (and Android per §9 row) — head member cards + tap-through + inline approve/reject + Add-entry Sheet (chore/settlement/adjustment); member own summary + own ledger + log-a-chore-pending; month grouping + strikethrough rejected + decided-by line; no Pending/Settlements tabs anywhere; **all overlays/toasts appear on web** (no `Alert.alert`).

---

## 7. Acceptance criteria (mapped to `docs/fe-flow.md` + `docs/FE-bugs.md`)

| # | Criterion | Source |
|---|---|---|
| 1 | **No Pending tab, no Settlements tab** anywhere; `pending.tsx`/`settlements.tsx` deleted; `_layout` shows only Overview + Chores. | FE-bugs "no need for separate settlement/pending tab"; §7.1 |
| 2 | Head Overview = **member cards** (Avatar + name + balance via `AmountText` + pending count); head's **own** balance is not shown as a card. | fe-flow "list of group members with sum…"; FE-bugs "no need to see balance for head himself"; §7.1/§7.2 |
| 3 | Tapping a member card opens that member's **unified ledger** with inline approve/reject. | fe-flow "head should see ledgers per member"; §5.3 |
| 4 | Member Overview = **own summary header + own unified ledger**; a member sees only their own entries (API-enforced, mirrored). | fe-flow member view; §5.5 |
| 5 | **Inline Approve/Reject** on pending rows (head only); mutation invalidates ledger+balance+group; **no separate approval screen**. | FE-bugs "show pending entries in ledger with approve/reject"; §4.2 |
| 6 | **Rejected entries strikethrough** and **excluded from totals**; **only approved count**; **pending shown but not counted**. | FE-bugs; master-plan §5.1; §2.6 |
| 7 | Rows follow **§7.2 anatomy**: type icon, title per `entry_type` (§2.2), date · note, signed `AmountText` (by `direction`), pending badge / rejected strikethrough. | master-plan §7.2; §2 |
| 8 | **Month grouping** with `MonthHeader` + signed month total (approved-only). | master-plan §7.2; §3 |
| 9 | **Decided-by** line ("approved by Dad, 3 Jul") on decided entries; absent on pending/machine-posted. | master-plan §5.1; §2.5 |
| 10 | **Add entry** via `Sheet`: head → chore(any member, approved) / settlement / adjustment (custom amount + note); member → own chore (pending); allowance/emi never offered. | fe-flow "add ledger entry"; FE-bugs; master-plan §5.5; §4.5 |
| 11 | Settlement is recorded via the **entry Sheet** (settlement entry_type), not a settlements screen; head-only. | FE-bugs "settlement… via ledger, head only"; §4.5 |
| 12 | **Empty states teach** (head: "No members yet — Tap Invite…"; member: "No entries yet — Log a chore to start earning"). | master-plan §1 principle 4; §1/§3.2 |
| 13 | **No `Alert.alert`** for confirms/feedback — `confirmAsync` + `Toast` (web-safe). | master-plan §1 principle 5; WP-1.0 §4; §2.4/§4.5 |
| 14 | Built **entirely on the WP-1.0 kit** (no inline hex; `AmountText`/`ListRow`/`StatusBadge`/`Sheet`/`MonthHeader`/`Avatar`); negative balances render `−₹…` in danger, never clamped. | master-plan §7.4/§5.3; WP-1.0 |
| 15 | `api-types.gen.ts` **regenerated** from WP-1.2 openapi; `codegen:check` green; `settlementsApi`/`listPending`/`Settlement` deleted; `tsc`/`lint`/`expo export --platform web` all green. | master-plan §10.1/§10.3; §4.7/§6 |

---

## 8. Out of scope (restated)

- **Rich member-detail page** `members/[userId].tsx` (editable allowance, loans, settlement/adjustment page, `GET …/members/:id/summary`) — **WP-3.3**. WP-1.3 ships only the per-member unified ledger as the interim tap target (§5.3).
- **Loans tab / real EMI rows** (loan note + `n/12` installment index) — **WP-3.2**. EMI rows render defensively without the index (§2.2).
- **Allowance UI** (set/edit allowance) — **WP-2.2**. Allowance rows render defensively (title from `period`); no allowance rows exist at runtime until WP-2.1 posts them.
- **Dashboard v2** (sectioned groups, totals) — **WP-4.2**.
- **Legacy-screen sweep** (login/register/dashboard/profile/invite onto the kit) — **WP-4.1**. WP-1.3 touches only the group Overview + per-member ledger + the deleted screens.
- **Backend/openapi/migrations** — none; WP-1.3 consumes the WP-1.2 contract and regenerates types only.

**Definition of done:** group Overview rebuilt for both roles on the WP-1.0 kit per §7.1/§7.2; head member cards → per-member unified ledger with inline approve/reject; member own summary + own ledger; Add-entry `Sheet` (chore/settlement/adjustment per §5.5); month grouping with approved-only totals; strikethrough rejected; decided-by shown on decided entries; Pending/Settlements screens + routes + dead API client fns deleted; `api-types.gen.ts` regenerated from the WP-1.2 openapi with `codegen:check` green; no `Alert.alert`; `tsc` + `lint` + `expo export --platform web` all green. Commit/PR titled `WP-1.3: FE unified ledger` (implementation); this spec commit is `WP-1.3 spec: FE unified ledger`.
