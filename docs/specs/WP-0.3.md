# WP-0.3 Spec: Auth Redirect Fix

## 0. Scope

Two bugs from `docs/FE-bugs.md` (roadmap WP-0.3):

- **Bug A** — "whenever user is unauthorised we should redirect to login page" (a `401` from any API call must land the user on `/(auth)/login`).
- **Bug B** — "on logout redirect to login page" (tapping **Logout** on the Profile screen must land on `/(auth)/login`).

Must work identically on **Web, Android, and iOS** under expo-router. This WP touches only the frontend (`app/`). No API, no schema changes. It is a low-risk FE bug fix, but its correctness is load-bearing for every other screen, so the acceptance matrix is mandatory.

---

## 1. Root-Cause Analysis (from the actual code)

### 1.1 How auth state and navigation are wired today

- `app/src/api.ts` — the fetch wrapper `request<T>()` intercepts `response.status === 401`: it calls `await clearToken()`, then invokes an optional module-level callback `onUnauthorized()`, then `throw new Error('Unauthorized')`. The callback is registered via `setOnUnauthorized()`.
- `app/src/auth-context.tsx` — `AuthProvider` holds `user`, `token`, `isLoading`. It wires the callback once: `useEffect(() => setOnUnauthorized(() => logout()), [])`. `logout()` does `await clearToken(); setTokenState(null); setUser(null)` — **it performs no navigation**. `loadMe()` runs on mount to hydrate `token`/`user` from storage and flips `isLoading` to `false`.
- `app/app/_layout.tsx` (root) — renders a `Stack` wrapped in `AuthProvider`. **Contains no auth logic** and no navigation reaction.
- `app/app/index.tsx` — a **one-shot** decision evaluated only while this route is mounted: `isLoading` → spinner; `token` → `<Redirect href="/(app)" />`; else `<Redirect href="/(auth)/login" />`. Being a render-time `<Redirect>`, it only fires when `index` itself is the active route (essentially cold-start), not in response to later auth-state changes.
- `app/app/(app)/_layout.tsx` — the **only reactive guard in the app**: `useEffect(() => { if (!isLoading && !token) router.replace('/(auth)/login'); }, [token, isLoading, router])`, plus a render gate that shows a spinner while `isLoading || !token || !user`.
- `app/app/(auth)/_layout.tsx` and `app/app/invite.tsx` — **no auth guard at all**.
- `app/app/(app)/profile.tsx` — `handleLogout` calls `await logout()` and then does nothing, with a comment: *"Auth guard in _layout.tsx will handle redirect."*

### 1.2 Why a 401 does not reliably redirect (Bug A)

1. **No navigation is performed by the 401 path itself.** `api.ts` clears the token and fires `onUnauthorized → logout()`, which only mutates React state. The *only* thing that turns that state change into a screen change is the `useEffect` inside `(app)/_layout.tsx`. Redirect therefore happens **only if a screen under the `(app)` group is currently mounted** so that layout re-renders and its effect re-runs.
2. **Routes outside `(app)` are unguarded.** A 401 raised from the top-level `invite.tsx` route (web `?token=` / deep-link join flow) or anywhere in the `(auth)` group clears the token but has **no** reactive guard to move the user — they are stranded on the current screen.
3. **The error is swallowed as a generic error first.** Because `request()` throws `Error('Unauthorized')`, individual screens' `try/catch` blocks set a local error message. The redirect is a side effect racing against those error states; the user can see a generic error banner instead of (or before) landing on login.
4. **Stale-closure wiring.** `setOnUnauthorized(() => logout())` captures the `logout` from the first render inside a `[]`-dep effect. It happens to be harmless today (logout only uses stable setters), but it is fragile — any future `logout` that closes over changing state would use a stale copy.

### 1.3 Why logout does not reliably redirect (Bug B)

`profile.tsx` deliberately performs **no navigation** and delegates entirely to the `(app)/_layout` effect. That effect *can* fire (Profile lives under `(app)`), but the design is fragile and non-deterministic:

- The redirect depends on a **child layout's** effect firing during the same commit in which its render gate also flips to the spinner branch (`!token`). Navigation triggered from inside the layout that is simultaneously tearing itself down is exactly the kind of ordering that behaves differently across platforms (web history router vs. native stack) and across expo-router versions.
- There is **no explicit, deterministic** `router.replace` tied to the user action. The single source of truth for "should I be on login" is scattered across `index.tsx` (one-shot) and `(app)/_layout.tsx` (reactive but scoped), with the redirect target string `'/(auth)/login'` hardcoded in two places.

**Common root cause of both bugs:** auth-driven navigation is **not centralized and not globally reactive**. It relies on a single child-layout `useEffect` scoped to `(app)`, while the actions that drop auth (`logout`, 401) only mutate context state and never navigate. The fix is to make redirect a single, root-level, reactive rule over auth state that covers the entire route tree.

---

## 2. The Fix

### 2.1 Principle

Introduce **one** auth-navigation guard at the **root**, inside the `AuthProvider` subtree, using expo-router's `useSegments()` + `useRouter()`. It reacts to every auth-state change *and* every navigation, so it correctly catches logout, 401 (from any route), and cold-start-with-expired-token — with identical behavior on web/iOS/Android. `api.ts` and `logout()` keep only their existing job (clear token + update state); they never navigate directly. State change → root guard → redirect. This is the idiomatic expo-router "protected routes" pattern and removes all duplicated/scoped guards.

### 2.2 Changes

**(a) Root guard component — `app/app/_layout.tsx`.**
Add an inner component (rendered *inside* `AuthProvider` so it can call `useAuth()`), e.g. `AuthGate`, that runs a single effect:

```ts
const segments = useSegments();
const router = useRouter();
const { token, isLoading } = useAuth();

useEffect(() => {
  if (isLoading) return;                         // don't navigate until hydration finishes
  const inAuthGroup = segments[0] === '(auth)';
  if (!token && !inAuthGroup) {
    router.replace('/(auth)/login');             // unauthenticated & not on an auth screen → login
  } else if (token && inAuthGroup) {
    router.replace('/(app)');                    // authenticated but sitting on login → app
  }
}, [token, isLoading, segments]);
```

Render the existing `Stack` unconditionally (or gate the whole tree on a single top-level spinner while `isLoading`). Because it keys off `segments`, it re-evaluates on every route change, so it also covers the unguarded `invite` and `(auth)` routes.

- **Decision:** the `invite` route should remain reachable while unauthenticated only if the join flow supports anonymous preview; per current code `invite.tsx` uses authed APIs, so treat it as protected (guard redirects to login, then back). If product wants invite links to deep-link an unauthenticated user to login and *return* to the invite after login, that is a follow-up (out of scope for WP-0.3); note it in `docs/notes/` rather than expanding scope.

**(b) Simplify `app/app/(app)/_layout.tsx`.**
Remove the now-redundant `useEffect` redirect. Keep only the **render gate** that shows a spinner while `isLoading || !token || !user` (prevents a flash of app UI before the root guard redirects). Redirect responsibility moves entirely to the root guard. This eliminates the duplicated hardcoded target and the child-layout-effect fragility.

**(c) Keep `app/app/index.tsx` as-is or thin it.**
With the root guard in place, `index.tsx`'s `<Redirect>` is still fine as the initial landing decision and does no harm. Optionally reduce it to just a spinner and let the root guard route away; either is acceptable. Do not add a second competing reactive redirect here.

**(d) `app/app/(app)/profile.tsx`.**
Keep `await logout()`; the root guard now deterministically redirects on the resulting `token=null`. No `router` call needed in the screen. Update the stale comment to reference the root guard. (Optionally add a defensive `router.replace('/(auth)/login')` right after `await logout()` — harmless and makes the logout path explicit — but it must not be the *only* mechanism.)

**(e) Fix the `onUnauthorized` wiring in `app/src/auth-context.tsx`.**
Keep `setOnUnauthorized(() => logout())`, but make the closure robust: register it in an effect that either depends on `logout` or wraps `logout` in a `useCallback`/`useRef` so the callback always calls the current `logout`. Functionally the 401 path stays: `api.ts` clears token + calls `onUnauthorized → logout()` → `token=null` → root guard redirects.

### 2.3 What is intentionally NOT changed

- `api.ts`'s 401 interception (clear token + notify + throw) stays. It must **not** import `expo-router` or navigate directly — the api layer must remain navigation-agnostic; navigation stays a UI-layer concern driven by state.
- Token storage (`SecureStore`/`AsyncStorage`) logic is untouched.
- No changes to login success navigation (`login.tsx` keeps `router.replace('/(app)')`); the root guard is compatible with it (authenticated + in `(auth)` → it would also push to `(app)`, so the explicit call and the guard agree, no loop).

---

## 3. Edge Cases

1. **401 fires mid-navigation / while a request is in flight.** The api layer clears the token and updates context state regardless of which route is active. Because the root guard keys off `segments` + `token`, it fires on the *next* committed navigation state with `token=null`, landing on login. No dependence on a specific screen being mounted. If the user was mid-transition, the guard's `router.replace` supersedes the in-flight navigation (replace, not push, so no broken back-stack).

2. **Concurrent 401s (multiple parallel requests fail).** Each calls `clearToken()` + `logout()`; these are idempotent (token already null, storage already cleared). The guard's condition `!token && !inAuthGroup` is only true until the redirect completes, after which `inAuthGroup` becomes true — so at most one effective redirect; extra invocations are no-ops.

3. **Cold start with an expired token already in storage.** `loadMe()` sets `token` from storage (briefly non-null), calls `authApi.me()`, which 401s → token cleared + `logout()`; `loadMe`'s own `catch` also nulls token/user; `finally` sets `isLoading=false`. The root guard is gated on `!isLoading`, so it waits out hydration, then sees `token=null` and redirects to login. No flash of authenticated UI because both `(app)/_layout`'s render gate and the guard's `isLoading` check hold the spinner until resolution. **Requirement:** the guard MUST early-return while `isLoading` to avoid a redirect-to-login flicker before `me()` resolves for a *valid* token.

4. **Logout while not on an `(app)` screen** (e.g. future logout entry points): guard still fires because it is global, not scoped to `(app)`.

5. **Login screen for an already-authenticated user** (e.g. deep-link to `/(auth)/login` with a valid token): guard's `token && inAuthGroup` branch redirects to `/(app)` — no manual navigation required.

6. **No redirect loop:** login success sets `token` and navigates to `(app)`; guard sees `token && !inAuthGroup` → no action. Logout sets `token=null`; guard redirects to login once; on login screen `inAuthGroup` is true so the `!token` branch is inert. The two branches are mutually exclusive by construction.

---

## 4. Verification (manual test matrix)

Run `npx tsc --noEmit` (must be clean) and boot on both platforms: `npm run web` and Android (Expo Go / dev build). No automated e2e is required for this WP; the matrix below is the acceptance gate.

Preconditions: a running backend, one registered head user, at least one group so post-login screens make API calls.

| # | Scenario | Steps | Expected result | Web | Android |
|---|---|---|---|---|---|
| 1 | **Logout** (Bug B) | Log in → open Profile tab → tap **Logout** | Immediately lands on `/(auth)/login`; back gesture/button does NOT return into `(app)`; no spinner-stuck state | ☐ | ☐ |
| 2 | **Expired/invalid token on an API call** (Bug A) | Log in; then invalidate the session server-side or hand-edit stored token to a bogus value; trigger any data fetch (pull-to-refresh dashboard/groups) | The failing 401 clears auth and redirects to `/(auth)/login`; no infinite spinner, no generic error left on screen | ☐ | ☐ |
| 3 | **Cold start, expired token in storage** | Log in, kill the app; expire/invalidate the token server-side; relaunch | App shows brief loading, then lands on login (no flash of dashboard, no stuck spinner) | ☐ | ☐ |
| 4 | **Cold start, valid token** (regression) | Log in, kill app, relaunch with valid token | Lands directly on `/(app)` dashboard; NO flash of login screen | ☐ | ☐ |
| 5 | **401 outside `(app)`** | Open `invite` route while token is invalid (web `?token=` link / deep link) | Redirects to login instead of stranding on invite | ☐ | ☐ |
| 6 | **Login success** (regression) | From login, enter valid credentials | Lands on `/(app)`; no redirect loop back to login | ☐ | ☐ |
| 7 | **Already-authed hits login route** | While logged in, navigate/deep-link to `/(auth)/login` | Redirected to `/(app)` | ☐ | ☐ |

**Definition of Done (per master-plan §9 WP-0.3 + §10):** rows 1–6 pass on both Web and Android; `npx tsc --noEmit` clean; redirect logic exists in exactly one place (root guard) with no duplicated hardcoded `'/(auth)/login'` reactive redirects left in child layouts; `api.ts` remains navigation-agnostic. State in the PR which platforms/rows were exercised. Commit references `WP-0.3`.
