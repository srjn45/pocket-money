# WP-1.0 Spec: Design-System Core

**Work package:** WP-1.0 (Phase 1 — Ledger v2), runs ∥ with 1.1.
**Type:** Frontend-only. No backend, `openapi.yaml`, or migration changes.
**Depends on:** WP-0.2 (TanStack Query + generated types) landed — the kit is consumed by
query-backed screens. Does **not** depend on WP-1.1; in fact it lands **before** WP-1.1, which
creates the money-units sequencing wrinkle resolved in §5.
**Acceptance (master plan §9, Phase 1 row 1.0):** "components exist, token-driven, typed;
chores screen uses them; no behavior change."

> Roadmap refs: master-plan §7.4 (design system), §7.2 (ledger row anatomy), §1 (product
> principles / launch bar), §10 (Definition of Done). This WP builds the kit **once**, early,
> so every Phase 1–3 screen is born on it; the legacy-screen sweep is WP-4.1 and is **out of
> scope here** (only the chores screen is re-skinned, as proof-of-use).

---

## 0. Scope & guardrails

### Files created

| Path | What |
|---|---|
| `app/src/theme.ts` | Design tokens (colors, spacing, radius, type scale, money text style). The single source of truth for every hex/number below. |
| `app/src/money.ts` | `formatMinor()` + the **interim** `rupeesToMinor()` shim (§5). Deleted-shim marked for WP-1.1. |
| `app/src/confirm.ts` | `confirmAsync()` — the cross-platform confirm helper (web-safe; `Alert.alert` no-op fix, §4). |
| `app/src/components/Button.tsx` | `Button` (primary/secondary/danger/ghost). |
| `app/src/components/Card.tsx` | `Card`. |
| `app/src/components/ListRow.tsx` | `ListRow`. |
| `app/src/components/AmountText.tsx` | `AmountText` (signed, colored, ₹ from **minor units**). |
| `app/src/components/StatusBadge.tsx` | `StatusBadge`. |
| `app/src/components/Sheet.tsx` | `Sheet` (bottom-sheet native / centered modal web). |
| `app/src/components/TextField.tsx` | `TextField` (labeled input + inline error). |
| `app/src/components/MonthHeader.tsx` | `MonthHeader` (built for WP-1.3 ledger; no proof-of-use on chores). |
| `app/src/components/Toast.tsx` | `ToastProvider` + `useToast()` (in-app, web-safe). |

### Files modified

| Path | Change |
|---|---|
| `app/src/components/EmptyState.tsx` | Re-point hardcoded hex/sizes to `theme` tokens. Public props **unchanged**. |
| `app/src/components/ErrorMessage.tsx` | Same — tokens only, props unchanged. |
| `app/src/components/LoadingSpinner.tsx` | Same — tokens only, props unchanged. |
| `app/src/components/index.ts` | Barrel-export every new component + `ToastProvider`/`useToast`. |
| `app/app/_layout.tsx` | Mount `<ToastProvider>` inside `AuthProvider`, wrapping the `Stack` (so the toast overlay paints above every screen). No other change. |
| `app/app/(app)/groups/[id]/chores.tsx` | Re-skin on the kit (§6). **No behavior change.** |

### Out of scope (do NOT do here)

- **No** backend / `openapi.yaml` / migration edits. Money in the API is still `number`
  (float rupees) — see §5.
- **No** sweep of login/register/dashboard/profile/invite/overview/ledger screens — that is
  WP-4.1. Only `chores.tsx` is re-skinned.
- **No** dark mode. Light-mode-only, but **all** colors live in `theme.ts` so dark mode is a
  later token addition, never a screen rewrite (master plan §7.4).
- **No** new native dependencies (no `@gorhom/bottom-sheet`, no `reanimated`, no toast lib).
  The kit is built on `react-native` primitives + `@expo/vector-icons` (already installed) so
  the web export stays green. `Sheet` uses RN `Modal`; `Toast` uses an in-app overlay.
- **No** `Avatar` component. §7.4 lists it, but it has no consumer until WP-1.3/3.3; defer it
  to whichever WP first needs initials-avatars to avoid speculative surface. (Note this in the
  PR so the later WP knows to add it.)
- **No** logic/data-layer changes to chores — the query hooks, mutations, validation, and
  authz gating from WP-0.2/0.4 are preserved verbatim; only presentation swaps to the kit.

---

## 1. `app/src/theme.ts` — tokens

The tokens below are **exact**. Screens and components import from here; no inline hex or raw
spacing numbers anywhere in the kit or the re-skinned screen (that is an acceptance grep, §8).

```ts
export const theme = {
  color: {
    // brand / semantic (master plan §7.4)
    primary:      '#4F46E5', // indigo — primary actions, links, focus
    primaryText:  '#FFFFFF', // text/icon on a primary surface
    primaryMuted: '#EEF2FF', // tint bg for secondary/ghost pressed, info badges
    success:      '#16A34A', // credits, positive/confirmed
    danger:       '#DC2626', // debits, destructive, errors
    warning:      '#D97706', // pending / needs-attention

    // neutrals
    text:          '#111827', // primary text
    textSecondary: '#6B7280', // subtitles, captions
    textMuted:     '#9CA3AF', // disabled, placeholder, empty-state glyphs
    border:        '#E5E7EB', // hairlines, input borders, card outlines
    surface:       '#FFFFFF', // cards, sheets, inputs
    surfaceMuted:  '#F9FAFB', // system/disabled row bg
    background:    '#F5F5F5', // screen background (matches current screens)
    overlay:       'rgba(0,0,0,0.5)', // sheet/modal scrim

    // money direction (aliases so ledger reads intent, not raw color)
    credit: '#16A34A', // = success
    debit:  '#DC2626', // = danger
  },

  spacing: { xs: 4, sm: 8, md: 12, lg: 16, xl: 24 }, // §7.4: 4/8/12/16/24

  radius: { sm: 8, md: 12, lg: 16, pill: 999 }, // §7.4 default radius = md (12)

  fontSize: { xs: 13, sm: 15, md: 17, lg: 22, xl: 28 }, // §7.4 type scale

  fontWeight: {
    regular:  '400',
    medium:   '500',
    semibold: '600',
    bold:     '700',
  },
} as const;

// Money text style — tabular-nums + bold (§7.4: "money amounts in a tabular-nums bold style").
// Spread into a Text style; `fontVariant` aligns digit columns so amounts line up in lists.
export const moneyTextStyle = {
  fontVariant: ['tabular-nums'] as const,
  fontWeight: theme.fontWeight.bold,
} as const;

export type Theme = typeof theme;
```

**Token table (canonical):**

| Group | Token | Value | Use |
|---|---|---|---|
| color | primary | `#4F46E5` | primary buttons, links, active |
| | primaryMuted | `#EEF2FF` | ghost/secondary bg, info badge |
| | success / credit | `#16A34A` | credit amounts, success toast/badge |
| | danger / debit | `#DC2626` | debit amounts, destructive, error |
| | warning | `#D97706` | pending badge, warning toast |
| | text | `#111827` | primary text |
| | textSecondary | `#6B7280` | subtitle/caption |
| | textMuted | `#9CA3AF` | disabled / placeholder |
| | border | `#E5E7EB` | borders / hairlines |
| | surface | `#FFFFFF` | card/sheet/input |
| | surfaceMuted | `#F9FAFB` | system/disabled rows |
| | background | `#F5F5F5` | screen bg |
| | overlay | `rgba(0,0,0,0.5)` | scrim |
| spacing | xs/sm/md/lg/xl | 4/8/12/16/24 | gaps, padding, margins |
| radius | sm/md/lg/pill | 8/12/16/999 | inputs / cards / sheets / badges |
| fontSize | xs/sm/md/lg/xl | 13/15/17/22/28 | caption / body / title / heading / hero |
| fontWeight | regular…bold | 400/500/600/700 | — |
| money | moneyTextStyle | tabular-nums + 700 | all ₹ amounts |

---

## 2. Component file layout

```
app/src/
  theme.ts                 # tokens
  money.ts                 # formatMinor(), rupeesToMinor() [interim]
  confirm.ts               # confirmAsync()
  components/
    index.ts               # barrel — exports every component + ToastProvider/useToast
    Button.tsx
    Card.tsx
    ListRow.tsx
    AmountText.tsx
    StatusBadge.tsx
    Sheet.tsx
    TextField.tsx
    MonthHeader.tsx
    Toast.tsx              # ToastProvider, useToast
    EmptyState.tsx         # (exists) → tokens
    ErrorMessage.tsx       # (exists) → tokens
    LoadingSpinner.tsx     # (exists) → tokens
```

One component per file. Each component owns its `StyleSheet.create(...)` derived from `theme`
tokens (no shared "global styles" module — keep it colocated and legible). `index.ts` is the
only public import surface screens use:

```ts
export { Button } from './Button';
export { Card } from './Card';
export { ListRow } from './ListRow';
export { AmountText } from './AmountText';
export { StatusBadge } from './StatusBadge';
export { Sheet } from './Sheet';
export { TextField } from './TextField';
export { MonthHeader } from './MonthHeader';
export { ToastProvider, useToast } from './Toast';
export { EmptyState } from './EmptyState';
export { ErrorMessage } from './ErrorMessage';
export { LoadingSpinner } from './LoadingSpinner';
```

`theme`, `moneyTextStyle`, `formatMinor`, `rupeesToMinor`, `confirmAsync` are imported directly
from `../theme` / `../money` / `../confirm` (not re-exported through the components barrel).

---

## 3. Component contracts (TypeScript)

All props interfaces below are **the contract**. Implementers may add private internals but must
not change these public signatures without updating this spec. Icon props are typed
`keyof typeof Ionicons.glyphMap` (matches the existing `EmptyState` convention).

### 3.1 `money.ts`

```ts
/** Format integer minor units (paise) as a rupee string WITHOUT sign or symbol: 1250 -> "12.50". */
export function formatMinor(minorUnits: number): string {
  return (Math.abs(minorUnits) / 100).toFixed(2);
}

/**
 * INTERIM SHIM — remove in WP-1.1.
 * The API still returns money as float rupees (openapi `number`) until the minor-units
 * migration (WP-1.1 / migration 008). AmountText's contract is minor units (the end state),
 * so call sites convert with this until the API flips to integers, then delete every call.
 * Half-up rounding avoids the classic 12.345 -> 1234 float bug for the transitional period.
 */
export function rupeesToMinor(rupees: number): number {
  return Math.round(rupees * 100);
}
```

### 3.2 `AmountText`

The money element. **Its API is the end state: it takes integer minor units**, never floats.
See §5 for how call sites feed it during the WP-1.0→1.1 window.

```ts
import type { TextStyle } from 'react-native';

interface AmountTextProps {
  /** Amount in integer minor units (paise). e.g. 1250 renders "₹12.50". */
  minorUnits: number;
  /**
   * credit -> green, prefixed "+"; debit -> red, prefixed "−" (U+2212 minus).
   * neutral -> default text color, no sign (used for a chore's configured rate).
   * Default: 'neutral'.
   */
  variant?: 'credit' | 'debit' | 'neutral';
  /** Override sign display. Default: show +/− for credit/debit, nothing for neutral. */
  showSign?: boolean;
  /** Maps to theme.fontSize. sm=15, md=17 (default), lg=22, xl=28. */
  size?: 'sm' | 'md' | 'lg' | 'xl';
  style?: TextStyle;
  /** Accessibility: overrides the computed label (e.g. "credit of 12 rupees 50 paise"). */
  accessibilityLabel?: string;
}
```

Behavior:
- Renders `"{sign}₹{formatMinor(minorUnits)}"`. `sign` = `'+'` (credit) / `'−'` (debit, U+2212) /
  `''` (neutral). `formatMinor` already uses `Math.abs`, so a negative `minorUnits` still formats
  correctly; for a raw balance, the caller picks `variant` from the sign (see §5 balance note).
- Color: credit→`theme.color.credit`, debit→`theme.color.debit`, neutral→`theme.color.text`.
- Always applies `moneyTextStyle` (tabular-nums + bold). `size` sets `fontSize`. `style` merges last.
- **Negative-balance rendering (master plan §5.3):** a below-zero balance is shown as
  `−₹…` in danger via `variant="debit"` — the component never clamps or hides the sign.

### 3.3 `Button`

```ts
import type { ViewStyle } from 'react-native';
import { Ionicons } from '@expo/vector-icons';

interface ButtonProps {
  title: string;
  onPress: () => void;
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'; // default 'primary'
  size?: 'sm' | 'md';                                       // default 'md'
  disabled?: boolean;
  /** Shows a spinner in place of the title and blocks presses. */
  loading?: boolean;
  /** Leading icon. */
  icon?: keyof typeof Ionicons.glyphMap;
  /** Stretch to container width (default true for full-width form/primary buttons). */
  fullWidth?: boolean;
  style?: ViewStyle;
}
```

Variant styling (all radius `theme.radius.md`, padding from spacing; `md` = `py:16 px:16`,
`sm` = `py:8 px:12`):

| variant | bg | text/icon | border |
|---|---|---|---|
| primary | `primary` | `primaryText` | none |
| secondary | `primaryMuted` | `primary` | none |
| danger | `danger` | `#FFFFFF` | none |
| ghost | transparent | `primary` | none |

- `disabled` **or** `loading` → `opacity 0.5` and `onPress` no-ops. `loading` renders
  `<ActivityIndicator>` (color = the variant's text color) instead of the title.
- This is the single primary-action button per §7.4 ("one primary action per screen as a
  prominent button"). FAB styling is deferred; use `fullWidth` primary for now.

### 3.4 `Card`

```ts
import type { ViewStyle } from 'react-native';

interface CardProps {
  children: React.ReactNode;
  /** If set, the whole card is pressable (renders a Pressable/TouchableOpacity). */
  onPress?: () => void;
  /** Apply default inner padding (theme.spacing.lg). Default true. */
  padded?: boolean;
  disabled?: boolean;
  style?: ViewStyle;
}
```

`surface` bg, `radius.md`, hairline `border`, `marginBottom: spacing.sm`. Pressable only when
`onPress` set (then `disabled` greys/blocks it).

### 3.5 `ListRow`

The generic row primitive that the ledger row (§7.2) and chore row are composed from.

```ts
import type { ViewStyle } from 'react-native';

interface ListRowProps {
  title: string;
  subtitle?: string;
  /** Leading slot: icon, avatar, type glyph. */
  left?: React.ReactNode;
  /** Trailing slot: AmountText, StatusBadge, action buttons. */
  right?: React.ReactNode;
  onPress?: () => void;
  disabled?: boolean;
  /** Renders the title with strikethrough (rejected ledger entries, §7.2). Default false. */
  strikethrough?: boolean;
  style?: ViewStyle;
}
```

Layout: `flexDirection: row`, `left` — title/subtitle column (flex:1) — `right`, vertically
centered, `gap: spacing.md`, padding `spacing.lg`, `surface` bg. Title `fontSize.md`
`semibold`; subtitle `fontSize.xs` `textSecondary`. `strikethrough` sets
`textDecorationLine: 'line-through'` + `textMuted` on the title. Pressable only with `onPress`.

### 3.6 `StatusBadge`

```ts
interface StatusBadgeProps {
  label: string;
  /** Default 'neutral'. */
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info';
}
```

Pill (`radius.pill`), `fontSize.xs` `semibold` uppercase, tinted bg + darker fg per tone:

| tone | bg | text |
|---|---|---|
| neutral | `surfaceMuted` | `textSecondary` |
| success | `#DCFCE7` | `success` |
| warning | `#FEF3C7` | `warning` |
| danger | `#FEE2E2` | `danger` |
| info | `primaryMuted` | `primary` |

(The tint bgs are the only additional hexes; keep them as local constants in `StatusBadge.tsx`
derived from the token hues — acceptable, they're badge-specific.) The ledger WP maps entry
status→tone: `pending_approval`→`warning`, `rejected`→`danger`, `approved`→(no badge). The chore
re-skin uses `tone="neutral" label="System"`.

### 3.7 `Sheet` (cross-platform — the critical one)

```ts
interface SheetProps {
  visible: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  /** Pinned action region at the bottom (e.g. Cancel / Save buttons). */
  footer?: React.ReactNode;
}
```

Implementation (RN `Modal`, no new deps):
- `<Modal transparent visible={visible} onRequestClose={onClose} animationType={...}>`.
- **Native (`Platform.OS !== 'web'`)**: bottom-anchored sheet — scrim (`overlay`) fills screen,
  content pinned to the bottom with top corners `radius.lg`, `animationType="slide"`. Wrap the
  content in `KeyboardAvoidingView` (`behavior="padding"` iOS) so inputs aren't covered.
  Bottom padding = `spacing.xl` (a fixed safe-area-ish inset — we deliberately do **not** pull
  in `SafeAreaProvider`, which isn't mounted; a real inset is a later polish, not a blocker).
- **Web (`Platform.OS === 'web'`)**: centered modal card — scrim fills viewport (flex-center),
  content is a `surface` card, `radius.md`, `maxWidth: 480`, `width: '100%'`, `animationType="fade"`.
- Common: tapping the scrim (a full-bleed `Pressable`) calls `onClose`; tapping the content does
  not (stop propagation). Optional close "×" in the header when `title` is set. `title` renders
  `fontSize.lg` `bold`. `footer` renders below the scrollable children with a top hairline.
- **Why this is web-safe:** `react-native-web` renders `Modal` as a real DOM portal, so unlike
  `Alert.alert` it actually appears on web. This is the sanctioned replacement for ad-hoc
  `Modal` usage and for any `Alert.alert`-based form. All create/edit forms (chores here, ledger
  and loans later) use `Sheet` (master plan §7.4: "bottom-sheet overlay for all create forms").

### 3.8 `TextField`

```ts
import type { TextInputProps, ViewStyle } from 'react-native';

interface TextFieldProps extends Omit<TextInputProps, 'style'> {
  label?: string;
  /** Inline error text shown below the input; also turns the border danger-colored. */
  error?: string;
  containerStyle?: ViewStyle;
}
```

Forwards **all** `TextInput` props (`value`, `onChangeText`, `placeholder`, `keyboardType`,
`secureTextEntry`, etc.) so it's a drop-in for the raw inputs in the current forms. Renders:
optional `label` (`fontSize.sm` `medium` `textSecondary`) → bordered input (`border`,
`radius.sm`, `padding: spacing.md`, `fontSize.md`, placeholder `textMuted`) → optional `error`
(`fontSize.xs` `danger`). When `error` is set the border becomes `danger`.

### 3.9 `MonthHeader`

Built for the WP-1.3 ledger (month grouping, §7.2); **no consumer in the chores re-skin** — that
is expected, it ships unused-but-typed and is exercised by WP-1.3.

```ts
interface MonthHeaderProps {
  /** 'YYYY-MM' (server period format, master plan §10.7). */
  period: string;
  /** Optional right-aligned month total, in minor units. */
  totalMinorUnits?: number;
  /** Direction for the total's color/sign. Default 'neutral'. */
  totalVariant?: 'credit' | 'debit' | 'neutral';
}
```

Renders a sticky-style header row: left = human month (`"July 2026"`, formatted from `period`
without pulling a date lib — parse `YYYY-MM`, index a hardcoded month-name array), `fontSize.sm`
`semibold` `textSecondary`, `surfaceMuted` bg; right = `<AmountText minorUnits={totalMinorUnits}
variant={totalVariant} size="sm" />` when provided.

### 3.10 `Toast` — `ToastProvider` + `useToast` (web-safe, replaces `Alert.alert` for feedback)

```ts
type ToastTone = 'success' | 'danger' | 'info';

interface ShowToastOptions {
  message: string;
  tone?: ToastTone;   // default 'info'
  durationMs?: number; // default 3000
}

interface ToastContextValue {
  show: (opts: ShowToastOptions) => void;
}

export function ToastProvider(props: { children: React.ReactNode }): JSX.Element;
export function useToast(): ToastContextValue; // throws if used outside ToastProvider
```

- In-app overlay (an absolutely-positioned `View` at the top of the provider's tree, above the
  navigator) — **not** `Alert.alert`, which is a no-op on `react-native-web`. Renders on web and
  native identically.
- Tone → left border/icon color: `success`→success, `danger`→danger, `info`→primary.
  `surface` bg, `radius.md`, subtle shadow/elevation, auto-dismiss after `durationMs`, tap to
  dismiss early. Single-toast (replace) is fine; a queue is optional.
- Positioned near the top (native: below status bar via a fixed top offset; web: fixed top-center)
  so it never fights the bottom `Sheet`.
- Mounted once in `app/app/_layout.tsx` inside `AuthProvider`, wrapping the `Stack`:

  ```tsx
  <QueryClientProvider client={queryClient}>
    <AuthProvider>
      <ToastProvider>
        <AuthGate />
        <Stack> ... </Stack>
        <StatusBar style="auto" />
      </ToastProvider>
    </AuthProvider>
  </QueryClientProvider>
  ```

### 3.11 `confirm.ts` — `confirmAsync` (web-safe destructive confirm)

```ts
interface ConfirmOptions {
  title: string;
  message: string;
  confirmLabel?: string; // default 'OK'
  cancelLabel?: string;  // default 'Cancel'
  destructive?: boolean; // styles the native confirm button; default false
}

/** Resolves true if the user confirms. Works on web (window.confirm) and native (Alert.alert). */
export function confirmAsync(opts: ConfirmOptions): Promise<boolean>;
```

- **Web:** `Promise.resolve(window.confirm(message))` — `Alert.alert` renders nothing on
  `react-native-web`, so it must not be used for confirms (this bug was the core of WP-0.4).
- **Native:** wrap `Alert.alert(title, message, [cancel→resolve(false), confirm→resolve(true)])`
  in a `Promise`.
- Centralizes the exact pattern currently inlined in `chores.tsx:99-124`; the re-skin replaces
  that inline block with a `confirmAsync` call (§6).

### 3.12 Migrated existing components (`EmptyState`, `ErrorMessage`, `LoadingSpinner`)

**Props are unchanged** (they already have consumers). Only the `StyleSheet` values are
re-pointed to tokens so they stop drifting from the palette:

- `EmptyState`: glyph `textMuted` (was `#ccc`); title `fontSize.md`/`semibold`/`textSecondary`;
  subtitle `fontSize.sm`/`textMuted`; paddings from spacing.
- `ErrorMessage`: icon `danger` (was `#ff3b30`); button `primary` bg (was `#007AFF`);
  radius/spacing from tokens; message `fontSize.md`/`text`.
- `LoadingSpinner`: `ActivityIndicator color={theme.color.primary}` (was `#007AFF`); text
  `fontSize.md`/`textSecondary`; `surface` bg.

No behavior/layout change beyond color/size token alignment.

---

## 4. Cross-platform notes (the web-safe rules)

The recurring cross-platform trap in this codebase is **`Alert.alert` is a no-op on
`react-native-web`** (proven in WP-0.4). The kit removes every reason a screen would reach for it:

| Need | Kit primitive | Why web-safe |
|---|---|---|
| Create/edit form overlay | `Sheet` | `Modal` renders a real DOM portal on web; native gets a bottom sheet. |
| Transient feedback ("Saved", "Failed to delete") | `useToast().show(...)` | In-app overlay `View`; renders on web + native (no `Alert`). |
| Destructive confirm ("Delete X?") | `confirmAsync(...)` | `window.confirm` on web, `Alert.alert` only on native. |

Rules for this WP and every screen that consumes the kit afterward:
1. **Never** call `Alert.alert` directly for confirms or feedback — use `confirmAsync` / `Toast`.
   (`Alert` may still be imported *inside* `confirm.ts` for the native branch — that's its home.)
2. `Sheet` and `Toast` branch on `Platform.OS` internally; consumers pass the same props on all
   platforms.
3. No web-only or native-only APIs leak into component public props.
4. No new native module dependency — everything is RN core + already-installed `@expo/vector-icons`,
   so `npx expo export --platform web` (§9) stays green (this is the §10 rule-3 floor that
   `tsc`/`expo-doctor` miss).

---

## 5. AmountText minor-units sequencing (the wrinkle — read carefully)

**End state (master plan §3.1, §10.6):** money is integer minor units everywhere except UI
formatting; `AmountText` formats `₹` from minor units.

**Reality at WP-1.0 implementation time:** WP-1.1 (migration 008 + int64 + openapi `integer`)
has **not** landed yet. The API and the generated `Chore.amount` are still **float rupees**
(`number`, e.g. `5.5`). WP-1.0 lands first.

**Resolution — build the end-state API now, bridge at the call site:**

1. `AmountText.minorUnits` **is** integer minor units — the permanent contract. Do **not** give
   it a "rupees" mode; a dual-mode prop would have to be un-picked in WP-1.1 and invites drift.
2. Every **call site** in WP-1.0 that has a float-rupee amount converts with the interim shim:

   ```tsx
   import { AmountText } from '../../../../src/components';
   import { rupeesToMinor } from '../../../../src/money';

   // chore.amount is float rupees until WP-1.1 flips the API to integer minor units.
   // WP-1.1 removes rupeesToMinor(...) and passes chore.amount directly.
   <AmountText minorUnits={rupeesToMinor(chore.amount)} variant="neutral" />
   ```

3. `rupeesToMinor` lives in `app/src/money.ts` with a **`remove in WP-1.1`** banner (§3.1). Its
   only job is this transitional window. When WP-1.1 changes the openapi `amount` to `integer`
   and regenerates types, `chore.amount` becomes minor units and every `rupeesToMinor(x)` call
   collapses to `x`; WP-1.1 deletes the shim and the wrapping calls. AmountText itself never changes.
4. **WP-1.1 handoff note (put in the PR + `docs/notes/`):** "WP-1.1 must (a) delete
   `rupeesToMinor` from `app/src/money.ts`, and (b) drop the `rupeesToMinor(...)` wrapper at
   every `AmountText` call site — grep `rupeesToMinor` to find them all. `AmountText`,
   `formatMinor`, and `moneyTextStyle` are already end-state and need no change."
5. **Balance / signed values (for later WPs):** a caller with a signed balance picks
   `variant` from the sign and may pass the raw signed `minorUnits` (AmountText abs-values for the
   ₹ string but colors/prefixes by `variant`) — e.g. `variant={bal < 0 ? 'debit' : 'credit'}`.
   Not exercised in the chores re-skin; documented so WP-1.3 doesn't re-derive it.

---

## 6. Chores-screen re-skin notes (proof-of-use)

Re-skin `app/app/(app)/groups/[id]/chores.tsx` onto the kit. **This is a presentation swap with
ZERO behavior change** — the WP-0.2 query hooks, WP-0.4 validation/authz/confirm logic, and the
head/member/system-chore rules all survive exactly. The acceptance bar is "chores screen uses
[the kit]; no behavior change" (master plan §9).

Mapping (current → kit):

| Current construct (`chores.tsx`) | Replace with |
|---|---|
| Inline `#007AFF` "Add Chore" `TouchableOpacity` (`:180-185`) | `<Button title="Add Chore" icon="add" variant="primary" onPress={() => openModal()} />` |
| `styles.error` red text banner (`:178`) | keep an inline error banner **or** surface load errors via existing `ErrorMessage`; per-action failures → `useToast().show({ tone:'danger', message })` |
| Chore row `TouchableOpacity` + `choreCard` styles (`:130-165`) | `<ListRow>` — `title={item.name}`, `subtitle={item.description}`, `left={<Ionicons .../>}` (type glyph), `right={<AmountText .../>}` + trash `Button variant="ghost"` for head, `onPress={canEdit ? () => openModal(item) : undefined}` |
| System-chore "System" badge (`:139-143`) | `<StatusBadge label="System" tone="neutral" />` in the row's `left`/title area; keep the "Used to record payouts…" caption as `subtitle` |
| `₹{item.amount.toFixed(2)}` (`:156`) | `<AmountText minorUnits={rupeesToMinor(item.amount)} variant="neutral" />` (see §5) |
| System chore "Variable" text (`:154`) | keep a plain `Text` "Variable" (`textSecondary`, italic) — a rate-less system chore has no amount to format |
| `<Modal>` add/edit form (`:205-257`) | `<Sheet visible={modalVisible} onClose={() => setModalVisible(false)} title={editingChore ? 'Edit Chore' : 'Add Chore'} footer={<Cancel/Save buttons>}>` |
| Raw `<TextInput>` fields (`:214-234`) | `<TextField label="Chore name" ... />`, `<TextField label="Description (optional)" ... />`, `<TextField label="Amount (₹)" keyboardType="decimal-pad" error={formError || undefined} ... />` |
| Modal Cancel/Save `TouchableOpacity`s (`:236-254`) | `<Button variant="ghost" title="Cancel" />` + `<Button variant="primary" title="Save" loading={saving} disabled={!isFormValid()} />` in the Sheet `footer` |
| Inline `handleDelete` `Platform.OS`/`window.confirm`/`Alert` block (`:99-124`) | `const ok = await confirmAsync({ title:'Delete Chore', message:\`Delete "${chore.name}"? This can't be undone.\`, destructive:true }); if (!ok) return;` then `mutateAsync`; on error `toast.show({tone:'danger', ...})` |
| Full-screen `ActivityIndicator` (`:168-173`) | `<LoadingSpinner />` |
| `formError` inline text (`:212`) | `TextField error` on the amount field (and/or a Sheet-level error line) |
| Empty state (`:187-192`) | `<EmptyState icon="list-outline" title="No chores yet" subtitle={isHead ? 'Add chores your family can earn money for' : undefined} />` |

Preserved exactly (do not touch): `useChores`/`useGroup` queries, `isHead` derivation,
`create/update/deleteChore` mutations and their args, `isFormValid()`, the
`parseFloat`/`amount: number` payload to the API (money stays float in the request until WP-1.1),
`canEdit = isHead && !isSystemChore`, system-chore protection (no edit/delete controls; row not
pressable), `RefreshControl` pull-to-refresh, and member read-only behavior. The chore
create/update payload still sends `amount: parseFloat(...)` (a float) — **do not** convert the
*input* to minor units here; only the *display* goes through `AmountText`/`rupeesToMinor`. (Input
parsing to minor units is WP-1.1 per master plan §10.6.)

Result: same flows, same authz, same validation; every pixel now comes from tokens/kit. A member
still sees a read-only list (no Add button, no trash, rows not pressable); the Settlement system
chore is still pinned, badged, captioned, and uneditable.

---

## 7. Out of scope (restated, explicit)

- Backend / openapi / migrations — none.
- `Avatar` component — deferred (no consumer yet); note for WP-1.3/3.3.
- Legacy-screen sweep (login/register/dashboard/profile/invite/overview/ledger) — WP-4.1.
- Dark mode — later WP; only requirement now is that colors are token-sourced.
- Input-to-minor-units parsing and API money type changes — WP-1.1.
- New native deps / bottom-sheet or toast libraries — forbidden (keeps web export green).
- FAB styling, animation polish, safe-area insets in `Sheet` — later polish, not blockers.

---

## 8. Acceptance criteria

1. `app/src/theme.ts` exports the tokens in §1 with the exact values; `moneyTextStyle` present.
2. All ten kit components exist at the §2 paths with the §3 prop signatures, each token-driven
   (no inline hex / raw spacing in component `StyleSheet`s except the StatusBadge tint bgs and
   the `overlay` value, which are token-derived).
3. `EmptyState`/`ErrorMessage`/`LoadingSpinner` re-pointed to tokens; their public props unchanged;
   existing importers still compile.
4. `AmountText` takes **minor units**, formats `₹`, colors/signs by `variant`, uses tabular-nums
   bold; negative balances render `−₹…` in danger (not clamped).
5. `Sheet` renders as a bottom sheet on native and a centered modal on web (both actually appear
   on web — no `Alert.alert` for forms). `Toast` and `confirmAsync` are web-safe.
6. `ToastProvider` mounted once in `app/app/_layout.tsx`; `useToast` works from a screen.
7. Chores screen re-skinned entirely on the kit (§6) with **no behavior change**: head CRUD,
   member read-only, system-chore pinned/badged/protected, validation, confirm-before-delete,
   pull-to-refresh, and visible errors on web all still work.
8. `rupeesToMinor` shim present in `app/src/money.ts` with the "remove in WP-1.1" banner; used at
   the chore-amount call site; a WP-1.1 handoff note is written to `docs/notes/`.
9. `npx tsc --noEmit` clean; `npm run lint` no new errors; `npx expo export --platform web`
   succeeds (§9).
10. No backend/openapi/migration files changed; no new dependency added to `app/package.json`.

## 9. Verification commands

Run from `app/`. State in the PR exactly what was run (master plan §10.3). Rule-3 floor: because
this environment may not run a browser, the **web export must succeed** — it catches bundle-level
breakage that `tsc`/`expo-doctor` miss (proven in WP-0.1).

```bash
cd app

# 1. Types — the primary gate. Every component + the re-skinned screen type-check.
npx tsc --noEmit

# 2. Lint — no new errors.
npm run lint

# 3. Web export — MANDATORY (master plan §10 rule 3). Must exit 0 and emit dist/.
npx expo export --platform web

# 4. Guard: no new deps sneaked in (should print nothing new vs. main).
git diff --stat package.json

# 5. Grep gates:
#    (a) no raw Alert.alert for confirms/feedback outside confirm.ts
grep -rn "Alert.alert" app src | grep -v "src/confirm.ts"        # expect: no confirm/feedback hits
#    (b) the interim shim is used at the chore amount, and is findable for WP-1.1
grep -rn "rupeesToMinor" app src                                  # expect: money.ts def + chores.tsx call
#    (c) chores screen imports from the kit
grep -n "from '../../../../src/components'" app/app/\(app\)/groups/\[id\]/chores.tsx
```

Interactive (if a browser/dev server is available — `npm run web`, per §10.3): open a group →
Chores; as head add/edit/delete a chore (Sheet form appears on web, amount shows `₹`, delete
confirm appears, delete error shows a toast); as member confirm read-only; confirm the Settlement
chore stays pinned/uneditable. **No behavior change** vs. the pre-re-skin screen.

**Definition of done:** tokens + all ten components exist and are typed and token-driven; the
three existing components are on tokens; the chores screen is fully re-skinned with no behavior
change; `AmountText` is minor-units end-state with the documented interim call-site conversion;
`tsc`/`lint`/`expo export --platform web` all green; no backend/openapi/dependency changes.
Commit/PR titled `WP-1.0: design-system core`.
