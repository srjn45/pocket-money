# WP-4.3 Spec: Invite / Share Polish

**Work package:** WP-4.3 (Phase 4 — UX polish & launch readiness). Runs ∥ with 4.1/4.2 but has a
**hard ordering dependency on WP-4.1** (see §0.1).
**Type:** Frontend-only for the deliverable. Contract-first has **nothing to change** here (the
invite endpoints already exist and their shapes are correct — §2), but this WP **surfaces one
genuine backend gap** in how `invite_url` is built and flags it loudly with a recommended ≤10-line
addendum (§3). The FE is specified to be correct *given* a correct base URL and to degrade
gracefully without it.
**Depends on:** WP-1.0 (kit — `Button`, `Sheet`, `TextField`, `LoadingSpinner`, `ErrorMessage`,
`EmptyState`, `Toast`), WP-1.3 (group Overview screen that hosts the Invite button), and WP-4.1
(legacy-screen re-skin, which touches `login.tsx` and `invite.tsx`). All landed / land before this.
**Acceptance (master plan §9, Phase 4 row 4.3):** "Web: copy + toast; mobile: native share sheet;
join lands in group. — per `docs/fe-flow.md`."

> Roadmap refs: master-plan §1 (launch bar — "a kid joins with one link tap", "Nothing looks
> broken"), §7.1 (`invite.tsx` = token via web query or deep link → join → redirect to group),
> §7.4 (kit + Toast), §10 (Definition of Done — rule 3 FE verification floor, rule 8 reuse kit),
> §11 (low-risk FE WP: Sonnet spec → Sonnet/**Haiku** implement → Sonnet review; this WP is listed
> as a Haiku-eligible mechanical FE task, so the spec is prescriptive).
>
> UX authority: `docs/fe-flow.md` → **Group Detail "for head user"** invite-link paragraph (web
> copies the link with a "link copied" toast; mobile opens the native share sheet with the link
> auto-populated) and the **Join** flow (`/invite?token=…` → `POST /groups/join` → redirect to the
> joined group).
> Product authority: `docs/prd.md` **F5** (invite generation → response includes `invite_url`),
> **F6** (join by token: web query or deep link → `POST /groups/join`), **F16** (generate +
> copy/share; invite route reads token from URL (web) or deep link (mobile); join redirects to
> group; deep link `pocketmoney://invite?token=xxx`).

---

## 0. Scope, guardrails & the decisions this WP makes

### In scope (this WP owns exactly these four things)

1. **Invite-link generation UX on group detail** — the head's "Invite" button + `handleInvite()` in
   `app/app/(app)/groups/[id]/index.tsx`: platform-correct share, robust copy on web (incl. the
   non-secure-context clipboard fallback, §11 G3), success/failure Toast. (§6.1)
2. **Platform-correct share** — `Platform.OS === 'web'` → `expo-clipboard` + success Toast; native
   (iOS/Android) → React Native's built-in `Share` API. The choice and the rejection of
   `expo-sharing` are justified in §4.1. **No new dependency** (both are already present — §4.1). (§4)
3. **The `/invite?token=…` web route + `pocketmoney://` deep-link handling** — `app/app/invite.tsx`:
   read the token from the web query string **and** from a custom-scheme deep link, auto-join when
   authenticated, defer-then-resume when not, and land **inside the joined group**. Includes wiring
   the **pending-invite-token round-trip through login**, which is currently dead code (§6.2, §6.3 —
   this is the one real functional bug this WP fixes). (§5, §6.2, §6.3)
4. **Deep-link config check** — verify `app.json` `scheme` and document what is and isn't
   configurable for LAN/HTTP v2 (custom scheme: yes; universal/App Links: deferred). (§5)

### Non-goals (explicit boundaries)

- **The dashboard Join sheet is WP-4.2's, not this WP's.** `docs/specs/WP-4.2.md` §5.5 owns the
  manual "Join Group" Sheet on the dashboard (`app/app/(app)/index.tsx`): paste a token → join →
  **stay on the dashboard**, the new group appears in the list. **WP-4.3 must not touch
  `app/app/(app)/index.tsx`.** The two join entry points differ deliberately (§0.2, D2) — do not
  "reconcile" them.
- **No backend implementation is required by the deliverable.** §3 flags a real gap and recommends a
  tiny config addendum; whether it is folded into this WP or filed as a follow-up is the owner's
  call. The FE spec here works either way.
- **No `openapi.yaml` change, no codegen, no migration.** The invite/join contracts and generated
  types are already correct (`InviteResponse { invite_url, token, expires_at }`, `JoinRequest
  { token }`, `POST /groups/join → GroupResponse`). If you find yourself editing the spec or
  regenerating types, you have drifted — stop.
- **No new dependency.** `expo-clipboard`, `expo-linking`, `expo-router`, and `react-native` (for
  `Share`) are all in `app/package.json` (§4.1). Do **not** add `expo-sharing` or any share/clipboard
  library.
- **No universal/App-Link setup** (`ios.associatedDomains`, `android.intentFilters`,
  `.well-known/*`). It needs a public HTTPS domain the v2 LAN/HTTP deployment does not have (§5.2) —
  deferred to backlog. Adding it now is throwaway config.
- **No auto-login-after-register work** (that is WP-4.6). This WP wires the pending token through the
  **login** path (the documented PRD flow). It leaves a one-line note for WP-4.6 to also consume the
  pending token on its future auto-login path (§6.3, note).
- No edits to `theme.ts` or any kit component (frozen from WP-1.0).

### 0.1 Sequencing (READ FIRST)

**The implementer runs AFTER WP-4.1 merges.** WP-4.1 (§2.1 login, §2.6 invite) re-skins both
`login.tsx` and `invite.tsx` onto the kit. This WP then makes **functional** changes to those same
two files (pending-token consumption in `login.tsx`; join-flow/error-state polish in `invite.tsx`).
Branch WP-4.3 off `master` **only after WP-4.1 is merged**, and take the WP-4.1-swept files as the
starting point (inherit their kit imports and Toast usage; add behavior on top). If WP-4.1's sweep of
`invite.tsx` did **not** land (the pre-4.1 file still shows raw `ActivityIndicator`/hex — verify on
your base), fold the trivial re-skin into this WP as well (`LoadingSpinner`/`ErrorMessage` +
`theme.color.surface`); the *substance* of this WP is join-flow correctness, not the skin.

**Parallelism with WP-4.2 — the exact file boundary.** WP-4.3 may run **fully in parallel with
WP-4.2's implementer** because their file sets are disjoint:

| File | WP-4.2 (dashboard) | WP-4.3 (invite/share) |
|---|---|---|
| `app/app/(app)/index.tsx` (dashboard + Join sheet) | **owns / rebuilds** | must not touch |
| `app/app/(app)/groups/index.tsx` (legacy tab) | **deletes** | must not touch |
| `app/app/(app)/_layout.tsx`, `groups/_layout.tsx` | **edits** (tab hide) | must not touch |
| `app/src/api.ts` `groupsApi.list` / `useGroups` | **edits** | must not touch |
| `app/app/invite.tsx` | must not touch | **owns** |
| `app/app/(app)/groups/[id]/index.tsx` (Invite button) | must not touch | **owns** |
| `app/app/(auth)/login.tsx` (pending-token consume) | must not touch | **owns** |
| `app/app.json` (verify scheme) | must not touch | **owns** (verify-only) |

No file is written by both. If a merge conflict appears, one of the two drifted out of its lane —
stop and reconcile against this table. (`app/src/storage.ts` already has the pending-token helpers
this WP consumes — see §6.3; no change to it is expected.)

### 0.2 Decision D1 — share mechanism per platform (and why not `expo-sharing`)

Web copies + toasts; native opens the OS share sheet with the link auto-populated (fe-flow). The
native API is **React Native's built-in `Share`** (`Share.share({ message, url })`), **not**
`expo-sharing`. Rationale in §4.1 — short version: `expo-sharing.shareAsync` shares a **local file
URI** (for exporting a PDF/image) and rejects a plain string/URL; RN `Share` is the correct API for
sharing **text/a URL** to messaging apps, which is exactly the invite use-case, and it ships with
`react-native` (no dependency). This matches what the current code already does (§1) — this WP
hardens it, it does not re-architect it.

### 0.3 Decision D2 — two join entry points, deliberately different landings

- **Invite link / deep link → `invite.tsx` → lands INSIDE the joined group** (`router.replace`
  to `/(app)/groups/{id}`). This is the "one link tap" onboarding path (§1 launch bar) and the
  fe-flow-specified behavior. **This WP owns it.**
- **Dashboard "Join Group" Sheet (manual token paste) → stays on the dashboard**, the new group
  appears in the list. **WP-4.2 owns it** (`docs/specs/WP-4.2.md` §5.5).

This divergence is intentional, not an inconsistency to fix: a user who *pasted a token while
already looking at their group list* is best left there to see the new group land; a user who
*tapped an invite link out of a chat* expects to arrive in the thing they were invited to. Do not
change WP-4.2's dashboard landing to match, and do not route invite.tsx through the dashboard.

---

## 1. Current state (what exists after WP-4.1, and what is broken)

Read from branch `wp-4.1-design-sweep` (the post-4.1 FE), verified 2026-07-05:

- **`app/app/(app)/groups/[id]/index.tsx`** already has a working `handleInvite()`:
  `groupsApi.createInvite(id)` → `Platform.OS === 'web' ? Clipboard.setStringAsync(invite.invite_url)
  + success Toast : Share.share({ message, url, title })`, wrapped in try/catch → danger Toast,
  `inviteLoading` state on a ghost `<Button icon="person-add" title="Invite" loading=… />`. Imports
  `* as Clipboard from 'expo-clipboard'` and `Share` from `react-native`. **This is the baseline —
  keep the shape; harden it (§6.1).**
- **`app/app/invite.tsx`** reads `token` via `useLocalSearchParams` (works for both the web query
  string and a deep link), and: no token → error; not authed → `setPendingInviteToken(token)` +
  `router.replace('/(auth)/login')`; authed → `joinMutation.mutateAsync(token)` →
  `router.replace('/(app)/groups/{group.id}')` (**lands in group ✓**); error → one generic message.
  Skin state depends on whether WP-4.1 §2.6 landed on your base (§0.1).
- **`useJoinGroup()`** (`app/src/hooks/useGroups.ts`) invalidates `qk.groups()` on success ✓.
- **`app/src/storage.ts`** exports `getPendingInviteToken` / `setPendingInviteToken` /
  `clearPendingInviteToken` (AsyncStorage key `pending_invite_token`).
- **`app/app.json`** has `"scheme": "pocketmoney"` ✓; `invite` route registered in
  `app/app/_layout.tsx` ✓.
- **`app/package.json`**: `expo-clipboard ~8.0.8`, `expo-linking ~8.0.12`, `expo-router ~6`,
  `react-native 0.81.5`. **No `expo-sharing`** — and none is needed (§4.1).

### The bug this WP fixes (dead pending-token round-trip)

`invite.tsx` **sets** `pending_invite_token` when a logged-out user opens an invite link, then
redirects to login — but **`login.tsx` never reads it**. Its success handler is just
`await login(...); router.replace('/(app)')`. `getPendingInviteToken` and `clearPendingInviteToken`
have **no callers** anywhere in `app/` (verified). **Consequence:** a logged-out invitee taps a link
→ token is saved → they log in → land on the dashboard → **they are never joined to the group.** The
"one link tap" onboarding (§1 launch bar) silently fails for exactly the most common invitee (a new
member who isn't logged in yet). §6.3 wires this up — it is the highest-value change in this WP.

---

## 2. Contract check — NOTHING to change (verify)

The invite/join surface is already correct and its generated types already exist:

- `POST /api/v1/groups/{id}/invite` → `InviteResponse { invite_url: string, token: string,
  expires_at: date-time }` (openapi ~L1579). `invite_url` is documented "Full invite URL".
- `POST /api/v1/groups/join` → body `JoinRequest { token }` → `GroupResponse` (has `id`), errors
  documented as **"Invalid token, token expired, or already a member"** at **409** (openapi ~L412) —
  this is the error contract §8 maps to user-facing copy.
- FE types (`InviteResponse`, join return `Group`) are already generated and imported in `api.ts`.

**Do not edit `openapi.yaml` or run codegen.** If the BE addendum in §3 is taken up, note that it is
a **config/host** change to how `invite_url`'s *value* is built — the *schema* (`invite_url: string`)
is unchanged, so still no codegen.

---

## 3. Backend gap — FLAG LOUDLY (recommended ≤10-line addendum; not required by the FE deliverable)

**Finding (confirmed, not speculative).** `CreateInvite` builds the link from the **API request
host** (`backend/internal/handlers/groups.go:377-383`):

```go
host := c.Request.Host                 // e.g. "192.168.1.5:8080"  ← the API server
scheme := "http"; if c.Request.TLS != nil { scheme = "https" }
inviteURL := fmt.Sprintf("%s://%s/invite?token=%s", scheme, host, invite.Token)
```

But the Go server serves **only the API** (`docker-compose.yml` exposes `8080:8080`; `main.go`
registers no static/SPA handler — there is no `/invite` route on the Go server). The Expo web app is
served from a **different origin** (the Expo web dev server / a static host, typically `:8081`).
**So the generated link — `http://192.168.1.5:8080/invite?token=…` — points at the API host, which
returns 404/JSON for `/invite`. The web invite link is broken in the standard split deployment.** It
only happens to work if the API and the web bundle are coincidentally same-origin.

**Recommended fix (small, config-driven — the right one):**

- Add a config field `AppBaseURL` from env `APP_BASE_URL` (e.g. `http://192.168.1.5:8081`), the
  **web app origin**. Build `inviteURL = fmt.Sprintf("%s/invite?token=%s", strings.TrimRight(
  cfg.AppBaseURL, "/"), invite.Token)`. **Fallback:** if `APP_BASE_URL` is unset, keep today's
  request-host behavior so dev/single-origin setups are unaffected. ~1 config field + ~3 handler
  lines; `openapi.yaml` `invite_url: string` is unchanged → no codegen.
- Document `APP_BASE_URL` in the backend README / `.env.example` and in `docker-compose.yml`.

**Deep-link note for the same addendum (optional, do not over-build):** the *http web link* is the
universally-working shareable for v2 (it opens the web app in any browser and auto-joins — including
on a phone without the native app). A `pocketmoney://` scheme link only opens for a recipient who
already has the app installed, so it is a bonus, not the primary. The clean v2 story is therefore:
**BE returns the correct http web link; the mobile app also *accepts* `pocketmoney://invite?token=`
deep links** (scheme is configured — §5). Generating scheme links or universal links is deferred
(§5.2). If the owner wants the BE to *also* return a scheme link, that is an additive field — spec it
separately, don't inflate this WP.

**If the addendum is NOT taken now:** the FE in this spec is still correct — `invite.tsx` reads
`?token=` from whatever origin serves it, and copy/share operate on whatever `invite_url` the server
returns. The only user-visible failure is the wrong *host* in the shared link, which is a deployment
config issue, not an FE bug. State clearly in the PR whether the addendum was taken, and if not, that
the link host is deployment-dependent until `APP_BASE_URL` exists.

---

## 4. Platform matrix + the share-mechanism decision

### 4.1 Which APIs, and why (no new dependency)

| Concern | Decision | Justification |
|---|---|---|
| **Web copy** | `expo-clipboard` → `Clipboard.setStringAsync(invite_url)` + success Toast | fe-flow says "web copies with a 'link copied' toast". `expo-clipboard ~8.0.8` already installed. |
| **Native share** | React Native built-in `Share` → `Share.share({ message, url })` | fe-flow says "mobile opens native share with the link auto-populated". `Share` ships in `react-native` — **zero deps**. |
| **Rejected: `expo-sharing`** | **not used** | `Sharing.shareAsync(url)` shares a **local file URI** through the OS sheet (for exporting a PDF/image/CSV) and **throws on a non-file string**; it cannot share a text/URL to WhatsApp. Wrong tool for an invite link. Also not installed — using it would add a dep for worse behavior. |
| **Rejected: web `navigator.share`** | **not used on web** | fe-flow explicitly wants *copy* on web, not a share sheet; `navigator.share` is absent on most desktop browsers and inconsistent on mobile web. Keep copy+toast. |

**Verified against `app/package.json`:** `expo-clipboard` present, `react-native` present (for
`Share`), `expo-sharing` **absent and to stay absent**. Do not add anything.

### 4.2 Behavior per action × platform

| Action | Web | Android | iOS |
|---|---|---|---|
| **Head taps "Invite"** on group detail | `createInvite` → `Clipboard.setStringAsync(invite_url)` → success Toast "Invite link copied". On clipboard failure (non-secure http, §11 G3) → fallback Sheet showing the URL in a selectable read-only `TextField` + danger Toast. | `createInvite` → `Share.share({ message: 'Join my group "<name>" on Pocket Money!\n\n<invite_url>', url: invite_url, title: 'Join My Group' })` → OS share sheet, link auto-populated. User cancel = no-op (no error Toast). | Same as Android; iOS uses `message` (iOS shows `url` separately — keep both so the URL is present in the message text too). |
| **Invitee opens `http://…/invite?token=…`** (web link, any device browser) | Web route `invite.tsx` renders, reads `?token`, joins (or defers to login), lands in group. | Opens in the mobile **browser** → same web route (no native app needed). | Same as Android. |
| **Invitee opens `pocketmoney://invite?token=…`** (deep link) | n/a (custom scheme) | If app installed → app opens `invite.tsx` with the token → joins/defers → lands in group. | Same as Android. |
| **Join while authenticated** | join → `router.replace('/(app)/groups/{id}')` → **in group**. | same | same |
| **Join while logged out** | save pending token → login → (§6.3) resume → join → **in group**. | same | same |

---

## 5. Deep-link configuration check

### 5.1 Custom scheme — already configured, verify only

`app/app.json` → `"scheme": "pocketmoney"` is present. With expo-router, this maps
`pocketmoney://invite?token=XYZ` → the `app/app/invite.tsx` route with `useLocalSearchParams().token
=== 'XYZ'`. **No config change needed.** Verification (device-only, list as reviewer guidance — §9):

- Android: `npx uri-scheme open "pocketmoney://invite?token=TESTTOKEN" --android` opens the app on
  the invite screen with the token populated.
- iOS: `npx uri-scheme open "pocketmoney://invite?token=TESTTOKEN" --ios` (or paste the scheme URL in
  Notes and tap).
- Confirm `invite.tsx` receives the token identically from a deep link and from a web `?token=`
  query (same `useLocalSearchParams` path — no branching on source).

### 5.2 Universal / App Links — DEFERRED (do not configure)

Making an **https** link open the **native** app requires `ios.associatedDomains` +
`android.intentFilters` **and** a public HTTPS domain hosting `.well-known/apple-app-site-association`
and `assetlinks.json`. The v2 deployment is **LAN + HTTP with no public domain** (master-plan §1;
PRD "HTTPS not required for MVP; HTTP on LAN acceptable"). Universal links are therefore
**not configurable now** and are **deferred to backlog** (post cloud-deploy + HTTPS). PRD **F16** is
satisfied without them: **custom-scheme deep link** (app installed) **+ web invite route** (browser,
no install) cover every case. **Do not add** `associatedDomains`/`intentFilters` in this WP —
note the deferral in `docs/notes/` if you touch it.

**Consequence to document in the PR:** an http invite link tapped on a phone opens the **mobile
browser** → the web invite route → auto-join in the browser. That is correct, working v2 behavior.
The `pocketmoney://` scheme is the app-installed shortcut.

---

## 6. Frontend implementation (exact file plan)

Exactly four files (three edited, one verified). No other file changes.

### 6.1 `app/app/(app)/groups/[id]/index.tsx` — Invite button + `handleInvite` (harden)

Keep the existing structure (§1); apply these hardening changes to `handleInvite`:

- **Web copy with a non-secure-context fallback (§11 G3).** Wrap `Clipboard.setStringAsync` so a
  failure/rejection does not leave the head with nothing:
  ```ts
  if (Platform.OS === 'web') {
    try {
      await Clipboard.setStringAsync(invite.invite_url);
      showToast({ message: 'Invite link copied to clipboard', tone: 'success' });
    } catch {
      // Non-secure http origin (navigator.clipboard unavailable) or user-gesture loss.
      setFallbackUrl(invite.invite_url);   // opens a Sheet with a selectable read-only TextField
      showToast({ message: 'Copy failed — select and copy the link', tone: 'danger' });
    }
  }
  ```
  The fallback Sheet is a `<Sheet visible={!!fallbackUrl} onClose={() => setFallbackUrl(null)}
  title="Invite link">` containing a read-only, `selectTextOnFocus` `<TextField value={fallbackUrl}
  editable={false} />` (or a selectable `<Text selectable>`), so the head can manually copy on an
  http LAN origin where the async Clipboard API is blocked. Reset `fallbackUrl` on close (§11 G6).
- **Native share unchanged** — `Share.share({ message, url: invite.invite_url, title })`. Keep the
  friendly message copy that includes the group name and the URL. **Do not** surface a danger Toast
  when the user *dismisses* the share sheet (RN resolves with `action: 'dismissedAction'`, not a
  throw — only the catch path Toasts). Optional: `if (result.action === Share.sharedAction) { … }` —
  no Toast is fine; do not error on dismiss.
- **`inviteLoading`** guards double-taps (button `loading`) — keep it; ensure it always clears in
  `finally`.
- **Hooks above early returns** (§11 G6): `useState` for `inviteLoading`/`fallbackUrl`, `useToast`,
  the query hooks — all before any `if (isLoading) return …`.
- **No `Alert.alert`** — all feedback via Toast/Sheet.
- Head-only affordance: the Invite button already renders in the head view only — keep it there (do
  not expose invite generation to members; the API is head-only anyway — 403).

### 6.2 `app/app/invite.tsx` — the join route (web query + deep link → land in group)

Own the final shape. On the WP-4.1-swept base (kit components); if unswept, apply the trivial skin
(§0.1). Logic:

- Read `const { token } = useLocalSearchParams<{ token?: string }>();` — one path for both web query
  and deep link. **All hooks before any early return** (§11 G6).
- `useEffect` keyed on `[token, user, authLoading]`:
  - `if (authLoading) return;` (wait for auth hydration).
  - **No token** → `setError('invalid')` (see §8 mapping), stop loading.
  - **Not authenticated** → `await setPendingInviteToken(token); router.replace('/(auth)/login');`
    (the login screen resumes it — §6.3).
  - **Authenticated** → `try { const group = await joinMutation.mutateAsync(token);
    await clearPendingInviteToken(); router.replace('/(app)/groups/' + group.id); }` — **lands
    inside the joined group** (D2). `catch (err) { setError(mapJoinError(err)); setLoading(false); }`.
    **`clearPendingInviteToken()` here is new** — it stops a stale token re-firing on a later login
    (§6.3). Clear it on the *authenticated success* path (and the login-resume path clears it too —
    idempotent, safe to call twice).
- **States** (all via kit):
  - loading → `<LoadingSpinner message="Joining group…" />`.
  - error → `<ErrorMessage message={friendly} />` plus a `<Button title="Back to groups"
    onPress={() => router.replace('/(app)')} />` so a failed invite is recoverable (§1 principle 5),
    not a dead end.
- `useJoinGroup` already invalidates `qk.groups()` (§7) — the joined group is fresh when we land.
- Leave nothing on `Alert.alert`.

### 6.3 `app/app/(auth)/login.tsx` — consume the pending invite token (fixes the dead round-trip)

In the login success handler, **after** `await login(...)`, resume a pending invite instead of always
dumping the user on the dashboard:

```ts
await login(email, password);
const pending = await getPendingInviteToken();
if (pending) {
  await clearPendingInviteToken();
  router.replace({ pathname: '/invite', params: { token: pending } });  // re-enter authed → joins → lands in group
} else {
  router.replace('/(app)');
}
```

- Re-entering `/invite?token=…` (now authenticated) reuses §6.2's authed join path → the user lands
  **inside the group**, satisfying the fe-flow "join → redirect to joined group" for the
  logged-out-invitee case. (Alternative: call `joinMutation` directly in `login.tsx` and push to the
  group — rejected: it duplicates invite.tsx's join+error handling. Route back through `/invite` and
  keep one join implementation.)
- Clearing the token **before** the redirect prevents a loop if the join later fails (the invite
  screen shows the error + "Back to groups"; the token is not re-consumed on the next manual login).
- Import `getPendingInviteToken`, `clearPendingInviteToken` from `../../src/storage`.
- **WP-4.6 handoff (note, do not build):** if WP-4.6 adds auto-login after register, that path must
  make the same pending-token check (register currently redirects to `/(auth)/login`, so the token is
  consumed here for now). Leave a one-line `docs/notes/` breadcrumb; do not implement register
  auto-login here.

### 6.4 `app/app.json` — verify only

Confirm `"scheme": "pocketmoney"` is present (it is). **No edit.** Do not add
`associatedDomains`/`intentFilters` (§5.2).

---

## 7. Query keys + invalidation (§7.3)

- The join mutation is `useJoinGroup()` — **already** `onSuccess: invalidateQueries({ queryKey:
  qk.groups() })`. Joining therefore refreshes the dashboard groups list (WP-4.2's enriched
  `GET /groups`) so the new group appears when the user later returns to it. **No new invalidation
  code is needed in this WP** — reuse the existing hook; do not add ad-hoc `invalidateQueries`.
- Landing in the group (`/(app)/groups/{id}`) mounts that screen's own queries (`useGroup`,
  `useBalance`, …) fresh — no cross-invalidation required from invite.tsx.
- The invite **generation** call (`groupsApi.createInvite`) is a plain awaited request (not a Query
  mutation) and **must not** invalidate anything — creating an invite doesn't change any cached list.
  Keep it a bare `await` inside `handleInvite`.

---

## 8. Error states (per fe-flow "Join flow")

`POST /groups/join` returns **409** for "Invalid token, token expired, or already a member" (§2,
openapi ~L412). The backend returns `{ "error": "<message>" }`. Map to human copy in `invite.tsx`:

| Situation | Source | User-facing message (`<ErrorMessage>`) |
|---|---|---|
| No token in URL / deep link | local (missing param) | "This invite link is invalid or incomplete." |
| Invalid or expired token | 409 from `/join` | Prefer the server message if present and human (it distinguishes expired vs invalid); else "This invite link is invalid or has expired. Ask for a new one." |
| Already a member | 409 from `/join` | If the server says "already a member", treat as **success-ish**: show "You're already in this group." and still offer **Back to groups** (or, if the group id is recoverable, route to it). Do not present it as a hard failure. |
| Network / 500 | thrown | "Couldn't join right now. Check your connection and try again." + a Retry button (`router.replace` back into `/invite?token=` or re-run the effect). |

- Prefer surfacing `err.message` when it is a clean server string (the fetch client already unwraps
  `{error}` — mirror how other screens read `err instanceof Error ? err.message : …`). Fall back to
  the generic copy above when the message is empty/opaque.
- Every error path renders a **recoverable** affordance (Back / Retry) — never a blank or a spinner
  that never resolves (§1 principle 5).

---

## 9. Verification (Definition of Done, §10 rule 3)

Run from `app/`. State results in the PR.

```bash
npx tsc --noEmit                       # zero type errors
npx expo export --platform web         # MANDATORY — must exit 0 (bundle breakage tsc/expo-doctor miss; WP-0.1 precedent)
npm run lint                           # no new issues
grep -rn "Alert\.alert" app/app --include='*.tsx'   # must print NOTHING (Toast/Sheet only)
grep -rn '#[0-9a-fA-F]\{3,8\}' app/app/invite.tsx app/app/'(app)'/groups/'[id]'/index.tsx \
  app/app/'(auth)'/login.tsx --include='*.tsx'       # no inline color hex in the touched files
```

**Can only be hand-verified on device / a running browser — list as reviewer guidance (the CI
export cannot exercise these):**

1. **Web copy on a NON-secure http LAN origin** (e.g. `http://192.168.x.x:8081`, *not* localhost):
   tap Invite → confirm the link is actually on the clipboard **or** the fallback Sheet appears with
   a selectable URL. This is the §11 G3 gotcha — `navigator.clipboard` is unavailable on non-secure
   contexts; the CI export cannot prove the runtime clipboard path.
2. **Native share sheet** (Android + iOS): tap Invite → OS share sheet opens with the link
   auto-populated; share to a note/chat; cancel = no error Toast.
3. **Deep link** (`npx uri-scheme open "pocketmoney://invite?token=…" --android/--ios`): app opens
   on the invite screen, joins, lands in the group.
4. **Logged-out invitee round-trip**: log out → open `/invite?token=<valid>` → redirected to login →
   log in → **land inside the joined group** (proves §6.3). Then repeat while **already logged in** →
   joins directly.
5. **Error copy**: reuse/expire a token → the invite screen shows the mapped message + a Back/Retry
   affordance (not a dead spinner). Re-open a token you're already a member of → friendly "already in
   this group".
6. **`invite_url` host sanity** (ties to §3): inspect the copied/shared URL — if it points at the
   API host:port (`:8080`) rather than the web app origin, the §3 gap is unfixed; call it out.

If the environment can boot `npm run web`, walk items 1/4/5 there; items 2/3 are device-only.

---

## 10. Acceptance criteria (Definition of Done)

1. **Group-detail Invite** works per fe-flow: web → copy + "link copied" Toast (with a graceful
   fallback Sheet on non-secure http); native → OS share sheet, link auto-populated; head-only;
   robust `try/catch`, no `Alert.alert`. (§6.1)
2. **Share mechanism** is `expo-clipboard` (web) + RN `Share` (native); **no new dependency**;
   `expo-sharing`/`navigator.share` not used; the choice is justified in the PR against §4.1.
3. **`invite.tsx`** reads the token from both the web query string and a `pocketmoney://` deep link,
   auto-joins when authed, and **lands inside the joined group** (`router.replace('/(app)/groups/
   {id}')`); kit components; recoverable error states per §8; clears the pending token on success.
4. **Pending-token round-trip fixed**: a logged-out invitee is joined and lands in the group after
   logging in (`login.tsx` consumes + clears `pending_invite_token` and re-enters `/invite`). The
   previously dead `getPendingInviteToken`/`clearPendingInviteToken` now have callers.
5. **Deep-link config verified**: `app.json` `scheme: "pocketmoney"` confirmed; custom-scheme deep
   link resolves to `invite.tsx`; universal links deliberately **not** added (deferred, §5.2, noted).
6. **Boundary respected**: no change to `app/app/(app)/index.tsx` (WP-4.2's dashboard/Join sheet),
   `groups/index.tsx`, the `_layout.tsx` files, or `api.ts`/`useGroups`. Only the four files in §6.
7. **BE gap flagged** in the PR (§3): `invite_url` is built from the API request host, which is wrong
   for a split LAN deployment; the recommended `APP_BASE_URL` addendum is stated (and, if taken,
   implemented as a config-only change with no `openapi.yaml`/codegen impact).
8. **Verification (§9) green**: `tsc` + `expo export --platform web` exit 0; lint clean; no
   `Alert.alert` under `app/app`; no inline hex in the touched files. Device-only items listed as
   reviewer guidance in the PR.
9. Commit/PR reference the WP id (`WP-4.3: invite/share polish`).

---

## 11. Gotchas & likely-slips (quick reference)

- **G1 — Stay out of WP-4.2's dashboard.** The Join sheet on `(app)/index.tsx` is WP-4.2's; this WP
  owns `invite.tsx`, `groups/[id]/index.tsx`, and `login.tsx` only (§0.1 table). Different join
  landings are intentional (D2) — don't reconcile them.
- **G2 — `invite_url` host is a real BE gap (§3), not an FE bug.** The FE can't fix a
  server-authoritative host. Flag it; recommend `APP_BASE_URL`; make the FE degrade gracefully.
- **G3 — `navigator.clipboard` needs a secure context.** On a **non-secure http LAN origin** (the v2
  default), `navigator.clipboard` is `undefined`; `expo-clipboard` may fall back to a hidden-textarea
  `execCommand('copy')` (requires the button's user gesture) or may fail outright depending on the
  browser. **Do not assume copy succeeds** — wrap in `try/catch` and provide the fallback Sheet with a
  selectable URL (§6.1). This is the single most likely silent failure; it is device/browser-verified
  only (§9 item 1), never proven by the CI export.
- **G4 — RN `Share`, not `expo-sharing`.** `expo-sharing` shares file URIs and rejects a plain URL;
  RN `Share` shares text/URLs. Do not add `expo-sharing`. (§4.1)
- **G5 — Native share dismiss is not an error.** `Share.share` resolves with
  `action: 'dismissedAction'` on cancel — do not Toast a failure on dismiss; only the `catch` path
  errors.
- **G6 — Hooks above early returns; Sheet field reset; Toast not Alert.** All hooks
  (`useLocalSearchParams`, `useAuth`, `useState`, `useToast`, query/mutation hooks) run before any
  `if (…) return`. Reset `fallbackUrl` when the fallback Sheet closes so a reopen isn't stale.
- **G7 — Clear the pending token, exactly once effectively.** Set it only on the logged-out path;
  clear it on authed join success **and** on login-resume (idempotent). Not clearing it re-fires a
  stale join on the next unrelated login; clearing too early (before a possible failure without a
  recovery affordance) can strand the user — pair the clear with the Back/Retry error UI (§6.2/§6.3).
- **G8 — One token source.** `useLocalSearchParams().token` serves both web `?token=` and the deep
  link — do not branch on `Platform.OS` to read the token; the source is transparent to the route.
- **G9 — Don't hand-build `pocketmoney://` links in the FE from a token.** Share the server's
  `invite_url` (the http web link, universally openable). The scheme link is for *handling* inbound
  deep links, not for *generating* shareables (§3, §5.2).
- **G10 — No `openapi.yaml`/codegen/migration.** The contracts are already correct (§2). If the §3
  addendum is taken, it changes the invite_url *value* (config), not its schema — still no codegen.
