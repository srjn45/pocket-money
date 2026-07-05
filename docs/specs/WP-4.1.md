# WP-4.1 Spec: Design-System Sweep of Legacy Screens

**Work package:** WP-4.1 (Phase 4 — UX polish & launch readiness). Opens Phase 4.
**Type:** Frontend-only. **No** backend, `openapi.yaml`, codegen, migration, navigation, or
data-layer changes. Visual/token sweep + one lint fix only.
**Depends on:** WP-1.0 (design-system core: `theme.ts`, kit, `confirm.ts`, `Toast`) — landed.
Every replacement below consumes components/tokens that already exist.
**Acceptance (master plan §9, Phase 4 row 4.1):** "no inline hex colors in screens; visual
consistency checklist."

> Roadmap refs: master-plan §7.4 (component kit + tokens), §1 (product principles / launch
> bar — "Nothing looks broken", kid-legible UI), §10 (Definition of Done — rule 3 verification
> floor, rule 8 reuse §7.4 components). The kit was built **once** in WP-1.0 and every Phase 1–3
> screen was born on it; this WP retrofits the **legacy** screens (login, register, dashboard,
> profile, invite, groups tab) that predate the kit so the whole app reads as one system.

---

## 0. Scope & guardrails

### In scope — the six legacy screens + two chrome files

| Path | Role | Sweep depth |
|---|---|---|
| `app/app/(auth)/login.tsx` | Login | Full re-skin **+ lint fix** (§4) |
| `app/app/(auth)/register.tsx` | Register | Full re-skin |
| `app/app/(app)/index.tsx` | Dashboard (sectioned groups) | **MINIMAL re-skin** — tokens + kit components only, **no layout redesign** (§2.3) |
| `app/app/(app)/groups/index.tsx` | "Groups" tab (legacy duplicate list) | **MINIMAL re-skin** (§2.4) — see gotcha G7 |
| `app/app/(app)/profile.tsx` | Profile | **Re-skin only, zero functional change** (§2.5) |
| `app/app/invite.tsx` | Invite landing | Full re-skin (§2.6) |
| `app/app/index.tsx` | Root redirect splash | Chrome re-skin (§2.7) |
| `app/app/(app)/_layout.tsx` | App tabs loading gate | Chrome re-skin (§2.7) |

The last two are not "screens" but carry the same `#007AFF` / `#f5f5f5` inline hexes and must be
swept so the grep gate (§6) over `app/app/` comes back clean.

### Non-goals (do NOT do here)

- **No behavior changes.** No new validation, no changed navigation, no auto-login, no new
  copy semantics. The only behavioral delta permitted is swapping `Alert.alert` feedback for
  `Toast`/`confirmAsync` (§3) — explicitly in scope because it is the kit's feedback pattern.
- **No layout redesign of the dashboard.** WP-4.2 (Dashboard v2) rebuilds the dashboard sections
  against a new `GET /groups` summary shape. Re-skinning its layout now is throwaway work — do a
  **token/component swap only** (§2.3).
- **No data-layer changes.** Do not migrate `groups/index.tsx` off its legacy `groupsApi` +
  `useState` onto TanStack Query. That is out of scope; only its styling changes.
- **No profile functionality changes.** `docs/fe-flow.md` §"Profile Screen": *"current screen is
  good enough with user details & logout button I do not want to make changes in that for now."*
  Keep the name/email display + logout. Re-skin the chrome, nothing else.
- No new dependencies. No edits to `theme.ts` or any kit component (they are frozen from WP-1.0);
  if a token is genuinely missing, note it in `docs/notes/` rather than editing the kit here.
- Do not touch already-on-kit screens (chores, loans, ledger, member detail, group overview,
  create). They were born on the kit; grep them only to confirm they stay clean.

---

## 1. Canonical token cheat-sheet (the reusable key)

Every legacy hex in these files maps to exactly one token. Import `theme` from
`../../src/theme` (adjust depth per file) and `moneyTextStyle` where amounts render. This table
is the whole job — apply it mechanically, then confirm against the per-screen inventory in §2.

| Legacy hex (all forms) | Meaning in legacy code | Replacement |
|---|---|---|
| `#007AFF` | brand blue: titles, links, primary btn bg, action btn, spinners, icons | `theme.color.primary` |
| `#fff` / `#ffffff` as **surface** bg | card/input/modal/screen surface | `theme.color.surface` |
| `#fff` as **text/icon on primary** | button label, spinner inside primary button | `theme.color.primaryText` |
| `#f5f5f5` (screen bg) | screen background | `theme.color.background` |
| `#f5f5f5` (cancel-button bg) | subtle neutral fill | `theme.color.surfaceMuted` |
| `#eee` | header bottom hairline | `theme.color.border` |
| `#ddd` | text-input border | `theme.color.border` |
| `#333` | headings / group names | `theme.color.text` |
| `#666` | subtitles, secondary labels | `theme.color.textSecondary` |
| `#999` | muted subtext | `theme.color.textMuted` |
| `#ccc` | chevron + empty-state glyph | `theme.color.textMuted` |
| `#ff3b30` | error text | `theme.color.danger` |
| `#34c759` | "earned" / credit amount | `theme.color.success` |
| `#ff9500` | "owed" amount (head total, member negative) | `theme.color.warning` |
| `#000` (shadowColor) | card drop shadow | **dropped** — encapsulated by `<Card>` |
| `rgba(0,0,0,0.5)` | modal scrim | **dropped** — encapsulated by `<Sheet>` (= `theme.color.overlay`) |

> Intentional token shifts to expect (do not "correct" them): brand goes indigo `#4F46E5`, not
> iOS blue; body text darkens to `#111827` (from `#333`); earned-green becomes `#16A34A` (from
> `#34c759`). That recoloring **is** the deliverable.

**Preserve existing color semantics** on the dashboard: the head's total is shown "owed" in
warning/orange and a member's negative balance in the same — keep `owed → warning`,
`earned → success`. Do **not** re-map negative balances to `danger` here; the danger/"owes" hint
treatment is WP-4.2/§5.3 territory, out of scope for a pure re-skin.

---

## 2. Per-screen inventory → remediation

Line numbers are from the current tree (2026-07-05). Treat each row as a checklist item.

### 2.1 `app/app/(auth)/login.tsx`  — full re-skin + lint fix

Inline hex (StyleSheet + inline props): L77 `ActivityIndicator color="#fff"`, L99 `#fff`,
L111 `#007AFF`, L115 `#666`, L120 `#ff3b30`, L126 `#ddd`, L133 `#007AFF`, L143 `#fff`,
L153 `#666`, L156 `#007AFF`. Plus the lint error at L84 (§4).

| Legacy element | Replace with |
|---|---|
| `<TextInput style={styles.input}>` ×2 (email, password) | `<TextField>` (labeled input; inherits token border + `placeholderTextColor`). Keep `autoCapitalize`/`keyboardType`/`autoComplete`/`secureTextEntry` props. |
| `{error ? <Text style={styles.error}>…}` inline error banner | Keep the inline banner but style via token (`theme.color.danger`), **or** surface via `toast.show({ tone:'danger', message })`. Prefer keeping the inline banner (form validation belongs next to the form) — just token-color it. |
| `<TouchableOpacity style={styles.button}>` + `ActivityIndicator` + disabled opacity | `<Button title="Login" onPress={handleLogin} loading={isLoading} fullWidth />` (Button renders its own spinner + disabled state; delete `buttonDisabled`, `#fff` spinner). |
| `styles.title` `#007AFF` (32px bold) | `theme.color.primary`, `theme.fontSize.xl` (28) or keep 32 as a one-off screen title — token the **color**, size may stay. |
| `styles.subtitle` `#666` | `theme.color.textSecondary`, `theme.fontSize.md`. |
| footer `styles.link` `#007AFF`, `styles.footerText` `#666` | `theme.color.primary` / `theme.color.textSecondary`. |
| `container` bg `#fff` | `theme.color.surface`. |

Keep `KeyboardAvoidingView` and the `Link`/`router` structure unchanged.

### 2.2 `app/app/(auth)/register.tsx`  — full re-skin

Identical style block to login (L128 `#fff`, L141 `#007AFF`, L145 `#666`, L150 `#ff3b30`,
L156 `#ddd`, L163 `#007AFF`, L173 `#fff`, L183 `#666`, L186 `#007AFF`; L106 spinner `#fff`).

| Legacy element | Replace with |
|---|---|
| 4× `<TextInput style={styles.input}>` (name, email, password, confirm) | 4× `<TextField>` with the same props. |
| register `<TouchableOpacity>` + spinner | `<Button title="Register" loading={isLoading} fullWidth />`. |
| title/subtitle/footer/error/link | Same token map as §2.1. |

No lint error here (footer reads "Already have an account?" — no apostrophe). Keep `ScrollView`.

### 2.3 `app/app/(app)/index.tsx`  (Dashboard) — MINIMAL re-skin, no layout redesign

WP-4.2 rebuilds this screen's sections. **Do not restructure the `SectionList` or the head/member
split.** Swap hexes for tokens and primitives for kit components in place, nothing more.

Inline hex: L123/L142 chevron `#ccc`, L150 `#007AFF`, L168/L175 action-icon `#007AFF`, L182
empty glyph `#ccc`, L237 spinner `#fff`, and the StyleSheet block L253–L421 (`#f5f5f5`, `#fff`,
`#eee`, `#333`, `#ff3b30`, `#007AFF`, `#000` shadow, `#666`, `#34c759`, `#ff9500`, `#999`, `#ddd`,
`rgba(0,0,0,0.5)`).

| Legacy element | Replace with |
|---|---|
| `if (isLoading) … <ActivityIndicator "#007AFF">` | `<LoadingSpinner />`. |
| top-level `{error ? <Text style={styles.error}>}` | token-color via `theme.color.danger` (keep inline; it is a page-level fetch error). Alternatively `<ErrorMessage message onRetry={onRefresh} />`. |
| two action `<TouchableOpacity>` (Create / Join Group) | `<Button variant="secondary" icon="add-circle" title="Create Group" />` and `<Button variant="secondary" icon="enter" title="Join Group" />`. Keep the two-across row. |
| empty state `<View style={styles.empty}>` block | `<EmptyState icon="people-outline" title="No groups yet" subtitle="Create a group or join one with an invite link" />`. |
| `renderHeadGroup` / `renderMemberGroup` `<TouchableOpacity style={styles.groupCard}>` (raw shadow) | Wrap in `<Card onPress={…}>` (drops `#000` shadow → token border+radius) with a `<ListRow>` inside, **or** token the existing `groupCard` styles in place. Minimal path: `<Card>` + `<ListRow title={name} subtitle={memberCount} left={<Avatar name={item.name} id={item.id} />} right={<AmountText …/> or the owed/earned block} />`. |
| balance text `owedAmount #ff9500` / `earnedAmount #34c759` | `theme.color.warning` / `theme.color.success` (preserve semantics — see §1 note). `AmountText` is available but the dashboard uses `Math.abs` + manual label; keeping the manual label with token colors is the minimal path. |
| chevron `Ionicons "#ccc"` | `theme.color.textMuted` (or drop — `ListRow` already provides affordance). |
| section title `#333` | `theme.color.text`. |
| Join **`<Modal>`** block (L204–245) + its styles | `<Sheet visible={joinModalVisible} onClose={…} title="Join Group">` containing a `<TextField>` + a `<Button title="Join" loading={joinMutation.isPending}>` and a secondary/ghost Cancel. `<Sheet>` supplies the `overlay` scrim and surface — delete `modalOverlay`, `modalContent`, `modalButtons`, `cancelButton`, `joinButton` styles. |
| Join feedback `Alert.alert` (L87/L101/L103) | Toast — see §3. |

### 2.4 `app/app/(app)/groups/index.tsx`  (legacy "Groups" tab) — MINIMAL re-skin

Predates the kit (raw `groupsApi.list()` + `useState`, no Query). It is still routed as the
"Groups" tab in `(app)/_layout.tsx`. **Re-skin only — do not migrate its data layer** (out of
scope). See gotcha **G7**: this is a near-duplicate of the dashboard that WP-4.2 may delete; keep
the sweep mechanical so it stays cheap, but it **must** be swept for the grep gate to pass.

Inline hex: L76 `#ccc`, L83 `#007AFF`, L97/L104 `#007AFF`, L111 `#ccc`, L160 spinner `#fff`, and
StyleSheet L176–L307 (same palette as §2.3). Remediation is identical in kind to §2.3:

| Legacy element | Replace with |
|---|---|
| loading spinner | `<LoadingSpinner />`. |
| error `<Text>` | token `theme.color.danger` or `<ErrorMessage>`. |
| Create/Join action `<TouchableOpacity>` ×2 | `<Button variant="secondary" icon=… />` ×2. |
| empty `<View style={styles.empty}>` | `<EmptyState icon="people-outline" …/>`. |
| `renderGroup` `<TouchableOpacity style={styles.groupCard}>` (raw shadow) | `<Card onPress>` + `<ListRow title={name} subtitle={"Created …"} right={<Ionicons chevron/>} />`. |
| chevron `#ccc` | `theme.color.textMuted`. |
| Join `<Modal>` + styles | `<Sheet>` + `<TextField>` + `<Button>` (as §2.3). |
| `Alert.alert` (L40/L57/L59) | Toast — §3. |

### 2.5 `app/app/(app)/profile.tsx`  — re-skin only, ZERO functional change

Keep the name/email display and the logout button and their behavior exactly (`fe-flow.md`
constraint). Only the styling changes.

Inline hex: L35 `#f5f5f5`, L39 `#fff`, L43 `#000` shadow, L53 avatar `#007AFF`, L61 `#fff`,
L66 `#333`, L70 `#666`, L74 logout `#ff3b30`, L81 `#fff`.

| Legacy element | Replace with |
|---|---|
| `container` bg `#f5f5f5` | `theme.color.background`. |
| `<View style={styles.card}>` (raw `#000` shadow) | `<Card>` (token border/radius; drops raw shadow). |
| hand-rolled avatar `<View style={styles.avatar}>` `#007AFF` + initial `<Text>` | `<Avatar name={user?.name ?? 'User'} id={user?.id ?? ''} size={80} />` (initials + id-hashed color from the kit). |
| `name` `#333` / `email` `#666` | `theme.color.text` / `theme.color.textSecondary`. |
| logout `<TouchableOpacity style={styles.logoutButton}>` `#ff3b30` | `<Button title="Logout" variant="danger" onPress={handleLogout} fullWidth />`. |

`handleLogout` (calls `logout()`, AuthGate redirects) is unchanged — do not add a confirm dialog
(that would be a behavior change; not requested).

### 2.6 `app/app/invite.tsx`  — full re-skin

Inline hex: L46 `#007AFF`, L68 `#fff`, L74 `#666`, L78 `#ff3b30`.

| Legacy element | Replace with |
|---|---|
| joining `<ActivityIndicator "#007AFF"> + <Text>` | `<LoadingSpinner message="Joining group…" />`. |
| error `<View>` + `<Text style={styles.error}>` | `<ErrorMessage message={error} />`. |
| `container` bg `#fff` | `theme.color.surface`. |

Leave the join/redirect logic (`useEffect`, `setPendingInviteToken`, `router.replace`) untouched.

### 2.7 Chrome files

- **`app/app/index.tsx`** (root splash): L11 spinner `#007AFF` + "Loading…" → `<LoadingSpinner />`;
  `container` bg `#fff` → `theme.color.surface`; `text` `#666` → `theme.color.textSecondary`.
  Keep the `Redirect` logic.
- **`app/app/(app)/_layout.tsx`** (tabs gate): L14 spinner `#007AFF` → `<LoadingSpinner />` (or
  token the color); `loading` bg `#f5f5f5` → `theme.color.background`. Keep the `<Tabs>` config
  and the auth render-gate untouched. (Tab-bar `Ionicons` receive `color` from the navigator —
  not a hardcoded hex — leave them.)

---

## 3. `Alert.alert` audit → `confirmAsync` + `Toast`

`Alert.alert` is a **no-op on react-native-web** (WP-1.0 §4). Every legacy `Alert.alert` in these
screens is *feedback* (not a confirm), so each becomes a `toast.show(...)`. `ToastProvider` already
wraps the whole tree in `app/app/_layout.tsx`, so `useToast()` works in every screen below.

| File:line | Current call | Kind | Replacement |
|---|---|---|---|
| `(app)/index.tsx:87` | `Alert.alert('Error', 'Please enter an invite token')` | validation | `toast.show({ tone:'danger', message:'Please enter an invite token' })` |
| `(app)/index.tsx:101` | `Alert.alert('Success', 'You have joined the group!')` | success | `toast.show({ tone:'success', message:'You have joined the group!' })` |
| `(app)/index.tsx:103` | `Alert.alert('Error', err…message)` | error | `toast.show({ tone:'danger', message: err instanceof Error ? err.message : 'Failed to join group' })` |
| `groups/index.tsx:40` | `Alert.alert('Error', 'Please enter an invite token')` | validation | `toast.show({ tone:'danger', … })` |
| `groups/index.tsx:57` | `Alert.alert('Success', 'You have joined the group!')` | success | `toast.show({ tone:'success', … })` |
| `groups/index.tsx:59` | `Alert.alert('Error', err…message)` | error | `toast.show({ tone:'danger', … })` |

Add `const toast = useToast();` at the top of each component and drop `Alert` from the
`react-native` import. **No `confirmAsync` calls are needed** — none of the six screens performs a
destructive confirm (profile logout is deliberately left as a direct action, §2.5). After this
change, `grep -rn "Alert.alert" app/app` must return **zero** hits (`confirm.ts` is the only place
`Alert` legitimately survives, and it is under `app/src`, not `app/app`).

---

## 4. The `login.tsx` lint fix (pre-existing `react/no-unescaped-entities`)

`app/app/(auth)/login.tsx:84` renders raw JSX text containing an apostrophe:

```jsx
<Text style={styles.footerText}>Don't have an account? </Text>
```

The unescaped `'` in `Don't` trips `react/no-unescaped-entities` (the repo's ESLint config, run
via `expo lint`). **Fix:** escape the apostrophe —

```jsx
<Text style={styles.footerText}>Don&apos;t have an account? </Text>
```

(`&rsquo;` is also acceptable; `&apos;` matches the terminal-plain house style.) This is the only
lint error introduced by legacy code in these files — `npm run lint` must show it **gone** and
report **no new** errors/warnings after the sweep (§6).

> Note: the dashboard's `"Groups You're In"` (`(app)/index.tsx:190`) is a **JavaScript string
> literal** inside the `sections` array, *not* JSX text, so it is **not** flagged and must **not**
> be "fixed" — changing it to `&apos;` would render a literal `&apos;` on screen. Leave it.

---

## 5. Visual-consistency checklist (acceptance artifact)

The reviewer verifies each row on **web** (primary) and, where feasible, Android. This is the
artifact the master-plan acceptance ("visual consistency checklist") refers to — reproduce it in
the PR description with checkboxes.

**Tokens & color**
- [ ] `grep -rn '#[0-9a-fA-F]\{3,8\}' app/app` returns **empty** (or only enumerated-and-justified
      hits — see §6; the intended count is **zero**).
- [ ] No screen still shows iOS blue `#007AFF`; every primary/brand element is indigo
      `theme.color.primary`.
- [ ] Screen backgrounds use `theme.color.background`; cards/inputs/sheets use
      `theme.color.surface`.
- [ ] Error text is `theme.color.danger`; "earned" is `success`; "owed" is `warning`
      (semantics preserved, §1).

**Components (reuse over re-implementation, §10 rule 8)**
- [ ] All text inputs are `<TextField>` (no raw `<TextInput>` in the six screens).
- [ ] All primary/secondary/danger actions are `<Button>` (no raw `<TouchableOpacity>` acting as a
      button; Button owns its own loading spinner + disabled state).
- [ ] Both Join-Group modals are `<Sheet>` (no raw `<Modal>` + `rgba(0,0,0,0.5)` overlay left).
- [ ] Group rows use `<Card>`/`<ListRow>`; profile card is `<Card>`; profile avatar is `<Avatar>`.
- [ ] Loading states are `<LoadingSpinner>`; empty states are `<EmptyState>`; fetch errors are
      `<ErrorMessage>` (or a token-colored inline banner for form validation).

**Feedback & lint**
- [ ] No `Alert.alert` remains under `app/app` (all → `Toast`), confirmed on web (where
      `Alert.alert` was silently dead before).
- [ ] `login.tsx` apostrophe lint error is gone; `npm run lint` reports no new issues.

**Behavior unchanged (regression guard)**
- [ ] Login / register / join validation and navigation behave exactly as before.
- [ ] Profile still shows name + email and logs out; no confirm added, no fields added.
- [ ] Invite still joins + redirects; dashboard sections still split head/member; the "Groups"
      tab still lists and joins.

---

## 6. Mandatory verification (§10 rule 3 — FE floor)

Run from `app/`. State the results in the PR.

```bash
# 1. Types — must pass with zero errors.
npx tsc --noEmit

# 2. Web export — MANDATORY (bundle-level breakage that tsc/expo-doctor miss; WP-0.1 precedent).
npx expo export --platform web        # must exit 0 and emit dist/

# 3. Lint — the login.tsx error must be GONE and NO new issues introduced.
npm run lint                          # (expo lint)

# 4. Hex gate — must print NOTHING (or only enumerated-and-justified lines).
grep -rn '#[0-9a-fA-F]\{3,8\}' app/app --include='*.tsx'

# 5. Alert.alert gate — must print NOTHING (confirm.ts lives under app/src, not app/app).
grep -rn 'Alert\.alert' app/app --include='*.tsx'

# 6. Raw-primitive gate — the six screens should not re-import styling primitives they replaced.
grep -rn "from 'react-native'" app/app/'(auth)'/login.tsx app/app/'(auth)'/register.tsx \
  app/app/'(app)'/profile.tsx app/app/invite.tsx    # expect no TextInput/Modal/ActivityIndicator
```

**If any hex must remain** (e.g. a genuinely one-off screen-title size the token scale doesn't
cover — colors have no such exception), it must be a **non-color** value or listed in the PR with a
one-line justification. The bar is **zero color hexes** in `app/app`.

Also boot the web app (`npm run web`) if the environment allows and click through
login → register → dashboard → join sheet → profile → logout to eyeball the checklist (§5).

---

## 7. Gotchas & likely-slips

- **G1 — Import depth.** `theme` and the kit sit at `app/src/…`. Depth differs per file:
  `app/(auth)/login.tsx` → `../../src/…`; `app/(app)/index.tsx` → `../../src/…`;
  `app/(app)/groups/index.tsx` → `../../../src/…`; `app/invite.tsx` → `../src/…`. The kit barrel
  is `../…/src/components`; `theme` is `../…/src/theme`; `useToast` from the components barrel.
- **G2 — `<Sheet>` replaces the whole `<Modal>` + overlay + button-row.** Don't keep the
  `modalOverlay`/`modalContent` styles "just in case" — Sheet owns the scrim (`theme.color.overlay`)
  and surface. Leftover overlay styles re-introduce `rgba(0,0,0,0.5)` and fail the hex gate.
- **G3 — Button owns its spinner & disabled state.** After swapping to `<Button loading=…>`,
  delete the now-dead `buttonDisabled` styles and the inline `<ActivityIndicator color="#fff">`;
  those `#fff` spinner colors are easy to miss and will fail the hex gate.
- **G4 — Don't over-fix the dashboard.** It's a **minimal** re-skin (WP-4.2 rebuilds it). Keep the
  `SectionList`, the head/member split, the `Math.abs`+label balance rendering, and the enrichment
  logic. Only swap tokens/components. Restructuring here is throwaway work.
- **G5 — Preserve color semantics.** Map `owed → warning`, `earned → success`. Do **not**
  "improve" negative balances to `danger` — that's a semantic/behavior change outside this WP.
- **G6 — The `"Groups You're In"` string is NOT a lint target** (§4 note). Escaping it renders a
  literal `&apos;`. Fix only the real JSX-text apostrophe in `login.tsx:84`.
- **G7 — `groups/index.tsx` is a legacy near-duplicate** of the dashboard groups list, still routed
  as the "Groups" tab, still on the pre-Query `groupsApi`. WP-4.2 may delete it. Sweep it
  mechanically (do **not** migrate its data layer — out of scope), but it **cannot** be skipped:
  its hexes and `Alert.alert`s are in `app/app` and would fail the gates. If a later decision
  removes the tab, note it in `docs/notes/` — don't remove it in this WP.
- **G8 — Profile: no behavior changes.** No logout confirm, no editable fields, no new rows. Swap
  the hand-rolled avatar for `<Avatar>` and the card/button for kit components — that's the entire
  change. `user?.id` may be undefined mid-hydration; pass `id={user?.id ?? ''}` to `<Avatar>`
  (its hash tolerates an empty string).
- **G9 — Don't edit the kit or `theme.ts`.** They're frozen from WP-1.0. If you feel a token is
  missing, you're probably about to introduce a behavior/design change that belongs in a later WP —
  stop and note it.
- **G10 — `AmountText` expects minor units.** If you choose to render dashboard balances through
  `<AmountText>` instead of the manual label, pass the raw integer minor-unit `balance` (it formats
  ₹ itself) — do not pre-format or `Math.abs` it into the component.

---

## 8. Acceptance criteria (Definition of Done)

1. All eight files (§0) contain **zero color hex literals**; every color comes from `theme`.
2. The six screens use the kit for inputs (`TextField`), actions (`Button`), overlays (`Sheet`),
   rows/cards (`Card`/`ListRow`/`Avatar`), and status (`LoadingSpinner`/`EmptyState`/`ErrorMessage`)
   — no raw `TextInput`/`Modal`/`ActivityIndicator`/button-shaped `TouchableOpacity` remain.
3. No `Alert.alert` under `app/app`; all feedback flows through `Toast` (verified on web).
4. `login.tsx` lint error is fixed; `npm run lint` shows no new issues.
5. `npx tsc --noEmit` passes; `npx expo export --platform web` exits 0.
6. **No behavior change**: validation, navigation, profile function, invite/join, dashboard
   sections all behave as before (§5 regression guard).
7. The §5 visual-consistency checklist is reproduced and ticked in the PR description.
8. Commit/PR reference the WP id (`WP-4.1: design-system sweep of legacy screens`).
