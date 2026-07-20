# Group-navigation stale-content leak — fix summary

Branch: `fix/group-nav-tab-blur-leak` (base `master` @ `dc91302`)

## The bug (as reported, reproduced)
Web: group 1 → header back → dashboard. Group 2 → shows group 2's title but group 1's
stale content. Back once → flips to group 1 (still not the dashboard). Back again → dashboard.
Each cycle accrues another phantom layer until a hard refresh resets it.

## What I found empirically (confirms the brief)
Method: registered an isolated throwaway account + two groups via the API, served the
web export, and drove real Chromium (Playwright) while inspecting `group-overview-root`
element counts and computed `display`/visibility of ancestors at each step.

Confirmed, byte-for-byte, the brief's root cause:

1. **The outer `Tabs` navigator never hid the inactive `groups` tab on this web build.**
   Even after a *single* clean sequence — enter Alpha, press header back — the Alpha
   `group-overview-root` remained a live, VISIBLE element (`visible=1`) sitting on top of
   the dashboard. Baseline probe at the "back to dashboard" step:
   `overview-root total=1 visible=1` (should be `0`). Tab blur/focus did nothing.
2. **Only a same-Stack push toggled `display:none`.** Re-entering while the prior group's
   Stack entry was still alive accumulated layers, so which group was visually on top was
   inconsistent — exactly the reported flicker.
3. The `groups` tab's internal Stack state was never reset on leave, so every re-entry
   silently added a layer and required an extra back press to escape.

I did **not** re-try the four call-site tweaks the brief already disproved. The accumulation
lives in the Tabs-preserved sub-navigator state, which none of them touch.

## The fix
Move the `groups` route tree out from under the broken Tab-blur mechanism, while keeping it
namespaced under `(app)` (so the root `AuthGate` still guards it and every existing
`/(app)/groups/...` path and `/groups/:id` URL keeps resolving — zero path churn, zero
testID churn).

`(app)` is now a **Stack** instead of a Tabs navigator:

```
app/(app)/_layout.tsx        Stack: (tabs), groups, notifications   [was: Tabs]
app/(app)/(tabs)/_layout.tsx Tabs: index (Dashboard) + profile      [new]
app/(app)/(tabs)/index.tsx   Dashboard   [moved from (app)/index.tsx]
app/(app)/(tabs)/profile.tsx Profile     [moved from (app)/profile.tsx]
app/(app)/groups/**          unchanged in place
app/(app)/notifications.tsx  unchanged in place (now a Stack sibling, not a hidden tab)
```

Entering a group is now a genuine root-level Stack push (over the `(tabs)` screen) and
leaving is a real pop — the mechanism the brief's finding #2 proved already toggles
`display:none` correctly. The two primary destinations (Dashboard, Profile) keep their
bottom tab bar / top nav; group detail and notifications are full-screen push-in screens.

Both moved screens only needed their relative-import depth bumped (`../../src` →
`../../../src`). No route strings, testIDs, or API paths changed. `create.tsx`, the group's
own header Stack, `HeaderBackButton`, `HeaderBell → notifications`, and the invite /
notification deep-links all keep working unchanged.

### Behavior change to be aware of
Before this fix the bottom tab bar (Dashboard/Profile) was **visible while inside a group**
(a side effect of the group being a hidden tab). It is now **hidden inside a group** and on
the notifications screen — those are full-screen Stack pushes with their own header + back,
and the tab bar returns when you back out to the dashboard. This is the standard detail-push
pattern and, given the root cause, is inherent to moving groups off the Tabs. Flagging it in
case the tab-bar-inside-group look was intended; it is trivial to revisit but cannot coexist
with the fix while groups remains under the Tabs.

## Verification (empirical, real Chromium against a real backend)

`npx tsc --noEmit` — clean (exit 0).
`npx expo lint` — only the two pre-existing issues remain (error at
`groups/[id]/index.tsx:370`, warning at `src/auth-context.tsx:47`); no new issues in the
changed files.
`npx expo export --platform web` — succeeds (exit 0).

Driven click sequence (register → login → Alpha → back → Beta → back → Alpha → back) with
per-step DOM assertions — **all pass** on the fixed build:

```
  [entered Alpha (1st)] overview-root total=1 visible=1
PASS  after 1st entry (Alpha): exactly 1 overview-root in DOM
PASS  Alpha name visible (header) after entering Alpha
PASS  no visible Beta name after entering Alpha
PASS  back from Alpha -> dashboard, no visible group overview
  [back to dashboard (after Alpha)] overview-root total=0 visible=0
  [entered Beta (2nd)] overview-root total=1 visible=1
PASS  after 2nd entry (Beta): exactly 1 overview-root in DOM (bug => 2)
PASS  after 2nd entry (Beta): at most 1 VISIBLE overview-root
PASS  Beta name visible (header) after entering Beta
PASS  no visible Alpha name after entering Beta
PASS  no stale Alpha content leaking into the visible screen after entering Beta
  [after ONE back from Beta] overview-root total=0 visible=0
PASS  exactly ONE back from Beta returns to dashboard (bug => still on a group screen)
  [entered Alpha (3rd)] overview-root total=1 visible=1
PASS  after 3rd entry (Alpha): exactly 1 overview-root in DOM
PASS  no stale Beta content leaking on 3rd entry (Alpha)
PASS  ONE back from 3rd entry (Alpha) returns to dashboard
===== RESULT: ALL PASS (bug absent) =====
```

The same harness, run against the pre-fix build, FAILED at
`back from Alpha -> dashboard` with `overview-root total=1 visible=1` — i.e. it is a valid
detector, not a no-op that passes regardless.

Regression smoke (same method) — all pass: Profile tab reachable; notifications reachable
via the header bell and its back returns to the dashboard; tab bar visible on dashboard,
hidden inside a group / notifications, visible again after backing out; group header-back →
dashboard; deep-link `/groups/:id` renders the group.

## Permanent guard
Added `e2e/tests/group-nav.spec.ts`, which drives the exact reported in/out cycle across two
groups and asserts the DOM never holds more than one `group-overview-root`, that backing out
clears it to `0`, and that a single back always reaches the dashboard. Passes locally against
the exported web build + live backend. It runs in CI like the other specs (self-provisioning,
existing testIDs).

## Scope / constraints honored
Frontend-only (no backend/API/migration changes). Stable testIDs. Isolated throwaway test
account + groups only (soft-deleted on completion); the shared backend was treated as
read-mostly/additive. Dev server was on a separate port and is stopped.
