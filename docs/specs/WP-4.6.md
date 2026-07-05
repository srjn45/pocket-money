# WP-4.6 Spec: Onboarding & First-Run

**Work package:** WP-4.6 (Phase 4 — UX polish & launch readiness). Runs ∥ with 4.1/4.2/4.3/4.5/4.7
but has a **hard ordering dependency on WP-4.1** and a **rebase dependency on WP-4.2 + WP-4.3**
(see §0.1).
**Type:** **Small backend addendum + frontend glue + copy.** One contract change (register returns a
JWT so the client is logged in immediately) + the FE auto-login wiring + one genuine empty-state gap
fix and a handful of tone polishes. **No new table, no migration** (§3). **No new dependency, no tour
/ coach-mark library** (§0.3). Contract-first order: `openapi.yaml` delta → BE handler → `npm run
codegen` → FE.
**Depends on:** register/login + JWT already wired (`auth.go`, `auth.IssueToken`), the WP-1.0 kit
(`Button`, `TextField`, `EmptyState`, `Toast`), the WP-0.3 root `AuthGate` (`app/app/_layout.tsx`),
and the pending-invite round-trip WP-4.3 wires through **login** (`app/src/storage.ts` helpers +
`login.tsx` consumption). All landed / land before this.
**Acceptance (master-plan §9, Phase 4 row 4.6):** "Auto-login after register (PRD [Proposed]);
first-run dashboard guides the head ('Create your family group') and the invited member (invite link
→ join → land in group); empty states per §1 principles across all screens — a new family completes
register→group→invite→first chore→first approval with zero guidance, <3 min (§1 bar)."

> Roadmap refs: master-plan §1 (the launch bar — "zero-training onboarding … under 3 minutes";
> principle 4 "Empty states teach"; principle 5 "Nothing looks broken"), §7.1 (nav map — `invite.tsx`
> lands in group), §7.4 (kit — `EmptyState`, `Toast`), §10 (Definition of Done — contract-first, authz
> in UI, every list gets empty/loading/error, rule 3 FE verification floor), §11 (low-risk FE WP with a
> tiny BE addendum: Sonnet spec → Sonnet/Haiku implement → Sonnet review — the contract change is one
> response body, so keep the strong-model review focused on the auth-context/AuthGate ordering, §12 G1).
>
> Product authority: `docs/prd.md` **User Flow step 1** ("User registers … → then logs in (or
> auto-login [Proposed]) → token stored → redirect to dashboard"), **F1/F2** (register/login + JWT),
> **F14** (persist JWT + redirect), **F17** (empty states). UX authority: `docs/fe-flow.md` join flow
> ("join → redirect to joined group").

---

## 0. Scope, guardrails & the decisions this WP makes

### In scope (exactly these three things)

1. **Auto-login after register** — a successful `POST /auth/register` returns a JWT + user (mirroring
   login), the FE stores it and lands the user **in the app** (not back on the login screen). Decision
   **D1** (§0.2): do this as a **backend addendum** (register issues a token), not FE call-chaining.
   (§2, §4, §5.1–§5.2)
2. **First-run guidance = smart empty states, not a wizard.** Fix the one genuinely bare empty state
   (chores, member view) and polish tone on the others so every list "teaches the next step" (§1
   principle 4). No coach-marks, no tour, no new component. (§6, decision **D3** §0.4)
3. **Invited-member first-run continuity across auto-login** — because register now lands the user
   *in the app* instead of routing through `login.tsx`, the pending-invite-token resume that WP-4.3
   wired into **login** must **also** run on the **register** path, or a logged-out invitee who taps
   "Register" (not "Login") is silently never joined. (§5.2, §7)

### Non-goals (explicit boundaries)

- **No tour / coach-mark / onboarding-wizard library, no new dependency** (BE or FE). First-run
  guidance is copy + the auto-login glue on existing kit components. (§0.3, D2)
- **No new empty-state *component* and no edit to the `EmptyState` kit component** — it is frozen from
  WP-1.0 and has **no action-button slot** (verified: `EmptyState` takes only `{icon, title,
  subtitle?}`). Actionability therefore lives in the **subtitle copy**, not in an added button (§0.4,
  D3). The primary CTA on each screen (e.g. the "Add entry"/"Log a chore" button) already sits above
  the empty state — keep it; do not move CTAs into `EmptyState`.
- **Do NOT re-spec or touch the dashboard** (`app/app/(app)/index.tsx`). Its first-run empty state
  ("No groups yet" / "Create a group or join one with an invite link") is **WP-4.2's** and already
  satisfies the head's "Create your family group" guidance and the member's "join" guidance (§6.1).
  WP-4.6 confirms it, does not edit it.
- **Do NOT re-spec the invited-member round-trip itself.** WP-4.3 owns `invite.tsx`, `login.tsx`, and
  the `storage.ts` pending-token helpers, and its fix already makes the *login* path join correctly.
  WP-4.6 adds **only** the register-path replica (§7 states exactly what is new).
- **Do NOT touch `login.tsx` or `invite.tsx`** (WP-4.3's files) — replicate the pattern into
  `register.tsx`, don't edit theirs.
- **No password/email/verification work** (backlog §8.9). No profile/members changes (WP-4.7).
- No edit to `theme.ts` or any kit component (frozen from WP-1.0). No `LoginResponse`/`UserResponse`
  schema changes beyond re-pointing register's response (§2).

### 0.1 Sequencing & rebase note (READ FIRST — concurrent work)

**Branch WP-4.6 off `master` after WP-4.1 has merged** (WP-4.1 re-skinned `register.tsx` onto the kit
— this WP builds on the swept file: it imports `Button`/`TextField` from the kit and shows errors via
`theme`-styled text today, moving toward Toast is optional, not required). **The implementer runs AFTER
WP-4.2 and WP-4.3 merge** — write and verify against their **post-merge** state:

| File | Touched by | WP-4.6's edit | Collision risk |
|---|---|---|---|
| `backend/openapi.yaml` | WP-4.2 (`GET /groups` resp), WP-4.7 (2 paths + schema) | **re-point** `POST /auth/register` 201 → `LoginResponse` | none — different path; additive/disjoint |
| `backend/internal/handlers/auth.go` | WP-4.7 (adds `ChangePassword`) | **edit** `Register` to issue a token | low — different method; if 4.7 landed, add alongside |
| `backend/internal/handlers/auth_test.go` | WP-4.7 (adds pw tests) | **update** `TestRegister_Success` assertion | low — same file, different test funcs |
| `app/src/api.ts` | WP-4.2 (`groupsApi.list`), WP-4.7 (`authApi.changePassword`) | **change** `authApi.register` return type only | low — one line, additive-adjacent |
| `app/src/auth-context.tsx` | neither | **edit** `register()` to set token+user | none |
| `app/app/(auth)/register.tsx` | WP-4.1 (re-skin, merged) | **rewrite** success handler | none (4.1 done) |
| `app/app/(app)/groups/[id]/chores.tsx` | neither | **fix** member empty-state subtitle (+ optional head polish) | none |
| `app/app/(app)/groups/[id]/loans.tsx` | neither | **optional** tone polish only | none |
| `app/app/(app)/index.tsx` (dashboard) | **WP-4.2** (rebuild) | **must not touch** | n/a — 4.6 doesn't edit it |
| `app/app/(auth)/login.tsx`, `app/app/invite.tsx` | **WP-4.3** | **must not touch** | n/a — 4.6 replicates the pattern into `register.tsx` |
| `app/src/api-types.gen.ts` | WP-4.2, WP-4.7 (regen) | **regenerate** (register response points at `LoginResponse`) | low — regen from the merged openapi; commit the diff |

**Rebase rule:** if a merge conflict appears, one side drifted out of its lane. WP-4.6 changes the
register **response** + the register **client path**; it does not rewrite the dashboard, the login/invite
flow, or `groupsApi.list`. Reconcile against this table. **Verify note for the implementer:** on your
base, confirm (a) `login.tsx` already reads `getPendingInviteToken()` after `login()` (WP-4.3 §6.3
merged) — you are replicating that block into `register.tsx`; (b) the dashboard empty state exists per
WP-4.2 §5.4; (c) `EmptyState` still has no action slot. If any is false, re-read §0.1/§6/§7 before
proceeding.

### 0.2 Decision D1 — auto-login is a BACKEND addendum (register returns token+user), not FE chaining

Register today returns `UserResponse` (201, no token — verified `auth.go:89`); login returns
`LoginResponse {token, user}` (`auth.go:133`). To land the user in the app after register we can either:

- **(chosen) BE addendum:** `Register` issues a JWT (the `auth.IssueToken` login already uses) and
  returns `LoginResponse {token, user}` at 201. The FE `register()` then mirrors `login()` exactly —
  store token, set user — in **one** round trip.
- **(rejected) FE call-chaining:** keep register→`UserResponse`, then have the FE immediately call
  `login(email, password)` with the same credentials.

**Why BE:**
1. **Atomic & mirrors login.** One request, one success/failure. The auth-context `register()` becomes
   a copy of `login()` (identical token-then-user ordering — the §12 G1 race is then handled *once*, the
   same way for both).
2. **No stranded state.** Chaining has a failure window: register succeeds (account created) but the
   follow-up login fails (transient network / rate-limit) → the user is registered yet stuck on an
   error, credentials still in flight, and a retry hits "email already exists". The BE approach cannot
   land there.
3. **Cheaper & simpler FE.** No second request, no re-sending the password, no re-deriving `LoginResponse`
   client-side.
4. **Tiny, low-risk contract change.** Only register's *response body* changes (`UserResponse` →
   `LoginResponse`, both already defined); no request change, no new schema, no migration. The master
   plan explicitly prefers this ("BE addendum … preferred, mirrors login").

Cost: one openapi edit + codegen + one integration assertion + updating one existing handler test.
Accepted.

### 0.3 Decision D2 — first-run guidance is empty states + auto-login, NOT a wizard/coach-marks

The §1 bar ("under 3 minutes, zero guidance") is met by **removing friction** (auto-login) and **making
every empty list teach the next step** (principle 4) — *not* by adding an overlay tour. A coach-mark
library is a new dependency, is fragile across web + Android + iOS, and fights the "kid-legible,
nothing looks broken" bar when it misfires. **Explicitly out of scope: any tour/coach-mark/wizard.**
The onboarding *is* the smart defaults: register → (auto-login) → dashboard empty state says "create a
group" → group Overview says "invite your family" + "add entry" → chores says "add your first chore" →
member joins via link → member ledger says "log a chore" → head approves inline. Copy carries the user;
no chrome.

### 0.4 Decision D3 — actionable empty states via SUBTITLE COPY (the kit component is frozen)

`EmptyState` (`app/src/components/EmptyState.tsx`) is `{icon, title, subtitle?}` — **no button slot**,
frozen from WP-1.0. Master plan §1 principle 4 wants each empty list to "say what to do next". We satisfy
that with an **actionable subtitle** (imperative for the actor who can act; reassuring-but-informative for
the actor who cannot), while the screen's existing primary CTA button stays where it is (above the list).
**Do not** add an action button to `EmptyState` or wrap it in a new component — that is a kit change
(future WP). If product later wants a CTA *inside* the empty state, note it in `docs/notes/`; don't build
it here.

---

## 1. Data-flow overview (what changes end to end)

**Auto-login (D1):**
```
POST /api/v1/auth/register {email, password, name, dob?, sex?}
  → create user (bcrypt hash)  → auth.IssueToken(user.id, jwtSecret)
  → 201 LoginResponse { token, user }                    ← was: 201 UserResponse (no token)
FE auth-context register(): setToken(token) → setTokenState(token) → setUser(user)   (mirror login())
FE register.tsx success: resume pending invite if any, else land in app (AuthGate takes it to /(app))
```

**Invited-member first-run, post-4.3 + post-4.6 (the continuity §7 secures):**
```
tap invite link  → invite.tsx (WP-4.3): not authed → setPendingInviteToken(t) → replace('/(auth)/login')
  ├─ user has an account → taps Login   → login.tsx (WP-4.3): consume token → replace('/invite') → join → in group
  └─ user is NEW          → taps Register → register.tsx (WP-4.6): auto-login, THEN consume token
                                              → replace('/invite') → join → lands IN the group
```
Before WP-4.6, register redirected to `/(auth)/login`, so login consumed the token. After WP-4.6,
register lands *in the app*; **WP-4.6 moves the consume onto the register path** so the new-member case
keeps working (§7).

---

## 2. Contract delta — `backend/openapi.yaml` (do this FIRST)

**One edit.** Re-point `POST /api/v1/auth/register`'s `201` response from `UserResponse` to
`LoginResponse` and update the description. `LoginResponse` already exists (§1429, `{token, user}`) — no
new schema. `RegisterRequest` is unchanged.

At `paths` → `/api/v1/auth/register` → `post` (currently ~L51–L82):

```yaml
  /api/v1/auth/register:
    post:
      tags:
        - Auth
      summary: Register a new user
      description: >
        Creates a new user account and returns a JWT plus the user, so the client is
        logged in immediately (auto-login). Same response shape as login; no separate
        login call is needed after registering.
      operationId: register
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/RegisterRequest'
      responses:
        '201':
          description: User created and logged in (returns token + user, same as login)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LoginResponse'
        '400':
          description: Bad request (validation error or email already exists)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal server error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
```

Nothing else in `openapi.yaml` changes. Do **not** touch `LoginResponse`, `UserResponse`, or
`RegisterRequest`.

---

## 3. Migration check — **NONE NEEDED** (verify, and flag loudly if wrong)

Register already creates the `users` row (migration 001); issuing a JWT is pure computation
(`auth.IssueToken`, no persistence — tokens are stateless, §WP-4.7 D1). There is **no schema, no new
column, no table.** **This WP adds NO migration file.** If you reach for `ALTER TABLE` or a new numbered
migration, **STOP** — you have drifted; re-read this section.

---

## 4. Backend implementation

### 4.1 `AuthHandler.Register` issues a token (`backend/internal/handlers/auth.go`)

`AuthHandler` already holds `jwtSecret` and imports `auth`. After a successful `userRepo.Create`, mint
a token exactly as `Login` does and return `LoginResponse` at **201** (keep the created status — it is
still a creation). Replace the final `UserResponse` return (`auth.go:88-96`):

```go
	// Issue a JWT so the client is logged in immediately (auto-login, WP-4.6 D1).
	token, err := auth.IssueToken(user.ID.String(), h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// 201 Created — same body shape as login (token + user).
	c.JSON(http.StatusCreated, LoginResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			DOB:       user.DOB,
			Sex:       user.Sex,
			CreatedAt: user.CreatedAt,
		},
	})
```

No constructor/route change (`v1.POST("/auth/register", authHandler.Register)` in `main.go:68` is
unchanged — same path/verb, same public group). Duplicate-email still returns 400 before token minting;
the token is only issued on the success path.

### 4.2 Update the existing handler test (`auth_test.go` — runs in CI)

`auth_test.go` is `//go:build integration` (Docker; CI only). `TestRegister_Success` (`auth_test.go:71`)
currently unmarshals the body into `handlers.UserResponse` and asserts `.Email/.Name/.ID/.CreatedAt`.
That will now fail (the body is `LoginResponse`). Update it:

```go
	var response handlers.LoginResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.Token)                 // NEW: register now returns a JWT
	assert.Equal(t, "test@example.com", response.User.Email)
	assert.Equal(t, "Test User", response.User.Name)
	assert.NotEmpty(t, response.User.ID)
	assert.NotZero(t, response.User.CreatedAt)
```

The `TestRegister_MissingFields` / `DuplicateEmail` / `InvalidEmail` / `ShortPassword` cases assert only
status codes and are unaffected — leave them.

### 4.3 Integration test — the returned token actually authenticates (add to `auth_test.go`)

Add one case proving the auto-login token is valid end to end (register → use token on a protected
route). Register the protected `/auth/me` route on the test router (mirror `setupAuthTestRouter`; wire
`auth.AuthMiddleware(jwtSecret)` + `authHandler.Me`), then:

```go
func TestRegister_TokenLogsIn(t *testing.T) {
	// … register (201) → read LoginResponse.Token …
	// GET /api/v1/auth/me with Authorization: Bearer <token> → 200, body.User.Email matches.
	// (Optional) POST /api/v1/auth/login with the same creds → 200 (login still works alongside).
}
```

Assertions: (1) `POST /auth/register` → 201 with non-empty `token`; (2) `GET /auth/me` with that token
→ 200 and the same email; (3) the token is a real JWT the middleware accepts (not a stub). **No group is
involved**, so the `AddMember(head, RoleHead)` harness gotcha does **not** apply here (noted only because
it is the house convention for group-touching tests). These execute in CI (Docker); locally they compile
under `-tags=integration` (§9).

---

## 5. Frontend — auto-login glue

### 5.0 Codegen STOP-gate (BEFORE any FE code)

After the openapi delta (§2) and BE build, run in `app/`:
```bash
npm run codegen        # regenerates app/src/api-types.gen.ts from openapi.yaml
```
The register **operation**'s response now references `LoginResponse` in the generated `paths`; commit
the regenerated `api-types.gen.ts`. `npm run codegen:check` must be clean. `LoginResponse` itself is
unchanged, so `Schemas['LoginResponse']` is stable — you are **not** hand-writing a new type, only
re-pointing `authApi.register`'s return (§5.1). If `codegen:check` shows an unexpectedly large diff, the
openapi edit is wrong — fix §2, don't patch the generated file.

### 5.1 `app/src/api.ts` — register returns `LoginResponse`

One line: change `authApi.register`'s return type from `User` to `LoginResponse` (the type alias
already exists at `api.ts:21`). The request payload is unchanged.

```ts
  register: (data: { email: string; password: string; name: string; dob?: string; sex?: string }) =>
    request<LoginResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
```

### 5.2 `app/src/auth-context.tsx` — `register()` mirrors `login()`

Today `register()` calls `authApi.register` and **discards** the result (`auth-context.tsx:65-67`).
Make it a copy of `login()` (`auth-context.tsx:58-63`) so the token/user ordering is **identical** — the
single most important thing for the §12 G1 race:

```ts
  const register = async (email: string, password: string, name: string) => {
    const response = await authApi.register({ email, password, name });
    await setToken(response.token);     // persist first (same as login)
    setTokenState(response.token);      // then flip in-memory token (AuthGate reacts to this)
    setUser(response.user);             // then set user
  };
```

Keep the `AuthContextValue.register` signature (`(email, password, name) => Promise<void>`) — callers
are unchanged. Do **not** add navigation here (auth-context never navigates; the screen does).

### 5.3 `app/app/(auth)/register.tsx` — success handler: auto-login + pending-invite resume

Keep all existing client-side validation (required fields, valid email, password match, min-6). Only the
**success branch** of `handleRegister` changes. Today it does `await register(...); router.replace(
'/(auth)/login')` (`register.tsx:46-47`) — bouncing the freshly-authed user back to login. Replace with
the auto-login landing, **replicating WP-4.3's login-resume block verbatim** (one join implementation,
one resume pattern):

```ts
    try {
      await register(email, password, name);
      const pending = await getPendingInviteToken();
      if (pending) {
        await clearPendingInviteToken();
        // Re-enter the invite route (now authenticated) → invite.tsx joins → lands in the group.
        router.replace({ pathname: '/invite', params: { token: pending } });
      } else {
        router.replace('/(app)');   // first-run: dashboard empty state guides "create a group"
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setIsLoading(false);
    }
```

- Import `getPendingInviteToken`, `clearPendingInviteToken` from `../../src/storage` (the helpers
  WP-4.3 already ships; `invite.tsx`/`login.tsx` are their other callers).
- This is the **exact** shape of `login.tsx`'s WP-4.3 §6.3 handler — keep them identical so a future
  change touches one pattern. Do **not** call `joinMutation` directly here; route back through `/invite`
  and reuse invite.tsx's authed join + error handling.
- The pending token is cleared **before** the redirect so a later failed join doesn't re-fire it on the
  next manual auth (mirrors WP-4.3 G7).
- See §12 G1 for the AuthGate ordering interaction — this is the load-bearing gotcha of the WP.

### 5.4 Register error copy stays inline (no scope creep)

Leave register's error surface as the existing `theme`-styled inline `<Text>` (WP-4.1's skin).
Converting to Toast is **not** in scope and not required; do not refactor the form. The only behavioral
change is the success-branch navigation above.

---

## 6. Empty-state inventory & exact copy (per §1 principle 4)

Every list screen was inventoried on the post-4.2/4.3 base (verified 2026-07-05). Most were born on the
WP-1.0 kit in Phases 1–3 with actionable copy already; **exactly one is genuinely bare** (chores, member
view). The table is the deliverable — implement the **"Spec'd copy"** column. Roles differ only where a
role can/can't act; where copy is role-independent it's one row.

| # | Screen / file | Role | Current copy (title / subtitle) | Verdict | **Spec'd copy (WP-4.6)** |
|---|---|---|---|---|---|
| E1 | Dashboard `(app)/index.tsx` | any | "No groups yet" / "Create a group or join one with an invite link" | ✅ actionable — **WP-4.2 owns** | **NO CHANGE** — do not touch (WP-4.2's file); satisfies the head "create your family group" + member "join" first-run guidance |
| E2 | Group Overview — members list `groups/[id]/index.tsx` (head branch) | head | "No members yet" / "Tap Invite to add your family" | ✅ actionable | keep as-is (fresh-group first-run; the Invite + Add-entry CTAs sit above it) |
| E3 | Group Overview — own ledger `groups/[id]/index.tsx` (member branch, via `LedgerList`) | member | "No entries yet" / "Log a chore to start earning" | ✅ actionable | keep as-is |
| E4 | **Chores `groups/[id]/chores.tsx`** | **head** | "No chores yet" / "Add chores your family can earn money for" | ⚠️ ok, sharpen to §1 example | **"No chores yet" / "Add your first chore — the amount your family earns for doing it"** |
| E5 | **Chores `groups/[id]/chores.tsx`** | **member** | "No chores yet" / **(no subtitle — BARE)** | ❌ **GAP** (principle 4/5) | **"No chores yet" / "Your family head hasn't added any chores yet — check back soon"** |
| E6 | Loans `groups/[id]/loans.tsx` | head | "No loans" / "Members can request loans from this tab" | ⚠️ minor tone | **"No loans yet" / "When a member requests a loan, it appears here for you to approve"** |
| E7 | Loans `groups/[id]/loans.tsx` | member | "No loans yet" / 'Tap "Request Loan" to borrow money repaid via pocket money' | ✅ actionable | keep (optional trim: "Tap Request Loan to borrow — repaid from your pocket money") |
| E8 | Member detail `groups/[id]/members/[userId].tsx` (via `LedgerList`) | head | "No entries yet" / "Add a chore, settlement, or adjustment" | ✅ actionable | keep as-is |

**The only functionally required change is E5** (member sees a bare "No chores yet" — principle 5
"nothing looks broken" + principle 4). E4/E6 are **low-risk tone polishes** — apply them for §1 tone
consistency; if the reviewer prefers minimal churn, E5 alone satisfies acceptance and E4/E6 can be
dropped. E7's trim is optional. **Do not** change E1/E2/E3/E8.

### 6.1 Implementation notes

- **E5 (required):** in `chores.tsx` the member subtitle is `undefined` today
  (`subtitle={isHead ? 'Add chores…' : undefined}`). Give the member branch a real string:
  ```tsx
  subtitle={isHead
    ? 'Add your first chore — the amount your family earns for doing it'
    : "Your family head hasn't added any chores yet — check back soon"}
  ```
  This is the whole E4+E5 change (one ternary). Keep the head-only "Add" button above it unchanged.
- **E6 (optional):** in `loans.tsx`, `emptyTitle`/`emptySubtitle` are computed from `isHead`
  (`loans.tsx:286-289`) — update the head strings only; leave the member branch (E7) or apply the
  optional trim.
- **Apostrophes:** these are user-facing strings inside JSX/TS string literals — use a normal `'` inside
  a double-quoted string (`"…hasn't…"`) as shown, or escape appropriately; do not introduce a lint
  error. No `&apos;` needed (it's a string, not JSX text).
- **No `EmptyState` component edits, no inline hex, no new icons** — reuse the existing `icon` props
  (`list-outline` for chores, `card-outline` for loans, `people-outline` for members).

---

## 7. Invited-member happy path — traced end to end (what 4.6 ADDS on top of 4.3)

WP-4.3 already makes the **logged-out invitee who logs in** land in the group. WP-4.6 changes register
from "bounce to login" to "auto-login", which **removes** the login screen from the *new-account*
invitee's path — so the token consumption WP-4.3 put in `login.tsx` would be skipped. WP-4.6's §5.3
replica restores continuity. Full trace (✚ = added/secured by WP-4.6; the rest is WP-4.3, unchanged):

1. Head (in the group) taps **Invite** → gets the link (WP-4.3 §6.1). Shares it.
2. New invitee opens `http://…/invite?token=XYZ` (web) or `pocketmoney://invite?token=XYZ` (deep link).
3. `invite.tsx` (WP-4.3): reads `token`, sees **not authenticated** → `setPendingInviteToken(XYZ)` →
   `router.replace('/(auth)/login')`.
4. Invitee has **no account** → taps **Register** → `register.tsx`.
5. Invitee submits the form → `register()` (auto-login, §5.2) sets token + user → **✚** the §5.3 success
   handler reads `getPendingInviteToken()` → **XYZ present** → `clearPendingInviteToken()` →
   `router.replace({ pathname: '/invite', params: { token: XYZ } })`.
6. `invite.tsx` re-runs (WP-4.3): now **authenticated** → `joinMutation.mutateAsync(XYZ)` →
   `clearPendingInviteToken()` (idempotent) → `router.replace('/(app)/groups/{group.id}')` → **lands
   inside the joined group.** `useJoinGroup` invalidated `qk.groups()` so the dashboard is fresh too.
7. Member sees the group Overview member view; the ledger empty state (E3) says "Log a chore to start
   earning" → member logs their first chore (pending) → head approves inline → **first approval done.**

**What 4.6 adds on top of 4.3, precisely:** *only* step 5's consume-and-resume on the **register** path
(the `login.tsx` equivalent is WP-4.3's and stays). Everything else in the trace is WP-4.3 behavior 4.6
relies on. The **no-invite** first-run (someone who registers fresh, not from a link) has no pending
token → step 5 falls to `router.replace('/(app)')` → dashboard E1 guides "create a group".

> **AuthGate interaction (critical — see §12 G1):** setting the token in step 5 makes the root
> `AuthGate` (`token && inAuthGroup → replace('/(app)')`) also want to navigate. `/invite` is outside
> both `(auth)` and `(app)` route groups, so once the app is *on* `/invite`, AuthGate is inert. The risk
> is the ordering of the two `replace` calls. §12 G1 is the required reading and the manual test §10-T3
> pins it.

---

## 8. Query invalidation (§7.3)

WP-4.6 adds **no** new mutations and **no** new invalidation code:

- **Auto-login** changes auth *state*, not any cached list; the app entering `/(app)` mounts the
  dashboard's `useGroups()` fresh. Nothing to invalidate.
- **The invited-member resume** reuses WP-4.3's `useJoinGroup()`, which already
  `invalidateQueries({ queryKey: qk.groups() })` on success — the dashboard reflects the new group when
  the user later returns to it. Do **not** add ad-hoc `invalidateQueries` in `register.tsx`.
- **Empty-state copy** is pure presentation — no data layer impact.

If you find yourself writing an `invalidateQueries`/`useMutation` in this WP, you've drifted — the only
mutation-shaped thing here is `register()`, and it flows through auth-context, not Query.

---

## 9. Verification (Definition of Done, §10 rule 3)

### Backend (from repo root / `backend/`)
```bash
gofmt -l backend/internal          # prints NOTHING (all files formatted)
go build ./...                     # compiles
go vet ./...                       # clean
go test -race ./...                # unit tests pass
# Integration (auth_test.go is //go:build integration) needs Postgres → runs in CI. Locally, COMPILE:
go test -tags=integration -run xxxNONExxx ./backend/internal/handlers/...
```
The `-tags=integration` no-match run proves the integration files (incl. the updated
`TestRegister_Success` and new `TestRegister_TokenLogsIn`) build without Docker (project convention —
integration executes only in CI). The §4.2/§4.3 cases must pass in CI. **AddMember(head, RoleHead) is
N/A here** (no group in these tests) — noted so the implementer doesn't add it spuriously.

### Frontend (from `app/`)
```bash
npm run codegen:check     # generated types in sync with openapi (STOP-gate §5.0)
npx tsc --noEmit          # zero type errors
npx expo export --platform web    # MANDATORY — must exit 0 (tsc/expo-doctor miss bundle breakage; WP-0.1 precedent)
npm run lint              # no new issues
grep -rn "Alert\.alert" app/app --include='*.tsx'   # unchanged — prints NOTHING (don't introduce any)
```
State all results in the PR. The auto-login + invite-resume paths are **runtime** behavior the CI export
cannot exercise — run the §10 walkthrough manually if the environment can boot `npm run web`; otherwise
list §10 as reviewer guidance in the PR.

---

## 10. The <3-minute acceptance walkthrough (numbered manual test script for the reviewer)

This is the §1 launch-bar test: **a new family completes register → group → invite → first chore →
first approval with zero guidance, in under 3 minutes.** Run it on `npm run web` (and, for T3 deep-link,
a device). Two accounts: **H** (head) and **M** (member). Start logged out.

**T1 — Auto-login (no invite).**
1. Open the app → redirected to login. Tap **Register**.
2. Register **H** (name/email/password). On submit → **land directly in the app on the Dashboard** — *not*
   back on the login screen. (Auto-login: D1/§5.2/§5.3 no-pending branch.)
3. Dashboard shows the E1 empty state "No groups yet / Create a group or join one with an invite link".

**T2 — Head first-run (create → invite → first chore).**
4. Tap **Create Group**, name it → land in the group **Overview (head)**. Members list shows E2
   "No members yet / Tap Invite to add your family".
5. Tap **Invite** → link copied (web toast) / share sheet (native). Copy the link aside for T3.
6. Go to the **Chores** tab → E4 "No chores yet / Add your first chore — the amount your family earns…".
   Add one chore (name + amount). It appears in the list.

**T3 — Invited member first-run (the continuity 4.6 secures).**
7. Log out. Open the **invite link** from step 5 while logged out → redirected to **login**.
8. Tap **Register** and register **M** (a *new* account). On submit → **land INSIDE the joined group**
   (member Overview), *not* on the dashboard and *not* stranded on login. (This is §7 step 5–6; if it
   lands on the dashboard instead, the §12 G1 AuthGate race has bitten — treat as a blocker.)
9. As **M**, the Chores tab shows E5 "No chores yet / Your family head hasn't added any chores yet…"
   only if H hadn't added one; since H did (step 6), M sees the chore list. The member ledger (E3) shows
   "No entries yet / Log a chore to start earning".

**T4 — First chore + first approval (≤2 taps each, §1 principle 2).**
10. As **M**, on the member Overview tap **Log a chore** → pick the chore → submit → a **pending** entry
    appears in M's ledger.
11. Log out M, log in as **H** (H uses the normal login path — verify login still works alongside the new
    register response). Open the group → member detail for M (or Overview) → the pending entry shows
    **Approve/Reject** inline → tap **Approve** → entry approved, M's balance updates.
12. **Stopwatch check:** steps 1–11 with no external explanation, under 3 minutes. If any step needed a
    manual/tooltip to proceed, the empty-state copy for that step failed the bar — note which.

**T5 — Edge: logged-out invitee who LOGS IN (regression guard for WP-4.3, unchanged by 4.6).**
13. With M's account existing, log out, open a fresh invite link → login screen → tap **Login** (not
    Register) → after login, land **in the group** (WP-4.3's login-resume path still works; 4.6 didn't
    break it).

**T6 — Negative: register failure surfaces.**
14. Try to register with an already-used email → inline error "email already exists"; the user is **not**
    logged in and stays on the register screen (no half-authed state — D1 rationale #2).

Report T1–T6 outcomes in the PR (or as reviewer guidance if the environment can't run them).

---

## 11. Acceptance criteria (Definition of Done)

1. `openapi.yaml`: `POST /auth/register` `201` returns `LoginResponse` (token + user); description says
   auto-login; `RegisterRequest`/`LoginResponse`/`UserResponse` schemas otherwise unchanged. **No
   migration file.**
2. `AuthHandler.Register` issues a JWT via `auth.IssueToken` and returns `LoginResponse` at 201;
   duplicate-email still 400 before token minting.
3. `TestRegister_Success` updated to assert `LoginResponse` (non-empty `token` + nested user); a new
   integration case proves the returned token authenticates `GET /auth/me`. Both pass in CI.
4. `npm run codegen` regenerated + committed; `codegen:check` clean; `authApi.register` returns
   `LoginResponse` (generated `LoginResponse` type, not hand-written).
5. `auth-context.register()` mirrors `login()` (persist token → set token → set user); `register.tsx`
   success handler auto-logs-in and **resumes a pending invite** (consume + clear + re-enter `/invite`),
   else lands on `/(app)`. No navigation added to auth-context.
6. Invited **new-account** member lands **inside the joined group** after registering from an invite link
   (§7 / §10-T3); the logged-out-invitee-who-logs-in path (WP-4.3) still works (§10-T5).
7. Empty states: the chores **member** bare state is fixed (E5) and every list "teaches the next step"
   per §6; `EmptyState` component unchanged; dashboard (E1) untouched; no inline hex, no new dep.
8. Verification (§9) green: gofmt/build/vet/test -race + integration compile-check; codegen:check + tsc +
   `expo export --platform web` exit 0; no new `Alert.alert`. The <3-min walkthrough (§10) passes or is
   listed as reviewer guidance.
9. Files stayed in lane (§0.1 table): no edits to `app/app/(app)/index.tsx` (WP-4.2), `login.tsx` /
   `invite.tsx` (WP-4.3), `theme.ts`, or any kit component.
10. Commit/PR reference the WP id (`WP-4.6: onboarding & first-run`).

---

## 12. Gotchas & likely-slips (quick reference)

- **G1 — The AuthGate ↔ auto-login navigation race (the #1 risk of this WP).** The root `AuthGate`
  (`app/app/_layout.tsx`) runs `if (token && inAuthGroup) router.replace('/(app)')`. When register sets
  the token (§5.2) while the user is still on `(auth)/register`, AuthGate *also* wants to navigate to
  `/(app)`. For the **no-invite** branch this is harmless (both target `/(app)`). For the
  **pending-invite** branch, register's `router.replace('/invite')` and AuthGate's `router.replace(
  '/(app)')` compete; whichever fires **last** wins. `/invite` is outside both route groups, so *once
  there* AuthGate is inert — the only exposure is ordering. **Required:** (a) mirror `login()` exactly in
  `register()` so token/user ordering matches the path WP-4.3 already ships (consistency = one race, one
  fix); (b) in `register.tsx`, issue the `/invite` replace as the statement immediately after
  `await register()` with nothing else awaited between the token flip and the navigation; (c) **verify
  §10-T3** — the new-account invitee must land in the **group**, not the dashboard. **If T3 lands on the
  dashboard**, the deterministic in-scope fix is to make `AuthGate` skip its `token && inAuthGroup →
  /(app)` redirect while a pending invite token exists (read `getPendingInviteToken()` in the guard and
  bail); prefer the mirror-first approach and only add the guard if the race is observed. Note that this
  same race latently applies to WP-4.3's login-resume — verify them together.
- **G2 — Register must consume the pending token now.** Pre-4.6, register bounced to login and login
  consumed it. Post-4.6, register lands in the app; if §5.3 doesn't consume+resume, the **most common
  invitee** (a brand-new member) is silently never joined — the exact "one link tap" failure WP-4.3 fixed
  for the login case. This is the whole point of §7. Don't ship auto-login without it.
- **G3 — 201, not 200.** Register still returns **201 Created** (it creates a user); only the *body*
  becomes `LoginResponse`. Don't copy Login's 200.
- **G4 — Update the existing handler test.** `TestRegister_Success` unmarshals `UserResponse` today; it
  will fail against `LoginResponse`. Fix it (§4.2) or CI breaks. It's `//go:build integration` (CI-only).
- **G5 — No migration, no schema.** Token minting is stateless computation (§3). `ALTER TABLE` = drift.
- **G6 — Don't touch the dashboard / login / invite files.** Dashboard empty state is WP-4.2's;
  login/invite + storage helpers are WP-4.3's. Replicate the resume pattern into `register.tsx`; don't
  edit theirs (§0.1 table). Different files, no merge conflict if you stay in lane.
- **G7 — `EmptyState` has no button slot.** Actionable = subtitle copy (D3). Don't add a button to the
  frozen kit component or wrap it in a new one. The screen's CTA already sits above the list.
- **G8 — E5 is the only *required* copy fix; E4/E6 are optional polish.** The member-view chores bare
  subtitle is the real gap (principle 5). Don't over-edit E1/E2/E3/E8 — they're already actionable.
- **G9 — auth-context never navigates.** Set token+user in `register()`; do the `router.replace` in the
  screen (as `login` does). Mixing navigation into the context reintroduces the G1 race in a place the
  screen can't control.
- **G10 — Codegen even though the FE type is re-pointed by hand.** The generated `paths` for the register
  operation change; run `npm run codegen`, commit `api-types.gen.ts`, keep `codegen:check` clean
  (§10 rule 1, contract-first).
