# WP-0.1 Spec: Expo SDK Dependency Alignment

## 0. Summary & goal

`app/package.json` is internally incoherent: the `expo` core package is pinned to `^54.0.33` (resolves to `54.0.35`) while **every other Expo/React-Native library is still SDK-52-era** (`expo-router ~4.0.0`, `react-native 0.76.1`, `react 18.3.1`, `react-native-web ~0.19.13`, `expo-secure-store ~14.0.0`, etc.). Expo SDK 54 expects React 19 / RN 0.81; installing it against SDK-52 libs produces peer-dependency conflicts and native-module version mismatches, so the app is not guaranteed to build.

**Goal (WP-0.1 acceptance):** align `app/` to **one** Expo SDK so `npx expo-doctor` is clean and login → dashboard works on **web + Android**.

## 1. Target SDK — decision & reasoning

**Standardize everything on Expo SDK 54** (`expo 54.0.35`, React 19.1.0, RN 0.81.5, expo-router 6).

Reasoning:
- The `expo` core is **already** at SDK 54, and `app.json` already declares `newArchEnabled: true` (New Architecture is the SDK-54 default). The coherent fix is therefore to bump the *libraries up to 54*, not to downgrade `expo` back to 52 — this keeps the project on a current, mature, fully-supported release and matches the already-declared intent.
- Newer stable SDKs exist on npm (55, 56, and latest = 57). They are deliberately **out of scope** here: this is a Phase-0 *stabilization* WP, not a leapfrog upgrade. Jumping 3 SDKs (RN 0.81 → 0.82+) multiplies breaking-change surface for no Phase-0 benefit. A move to the newest SDK, if desired, should be its own dedicated upgrade WP later.
- All target versions below are taken **authoritatively** from `expo@54.0.35`'s `bundledNativeModules.json` (the exact pins Expo itself uses to align a project), so they are guaranteed mutually compatible.

Environment check: local Node is **v22.22.1**, which satisfies SDK 54's requirement of Node ≥ 20.19.4. ✅

## 2. Exact `package.json` changes

### `dependencies` — change these (current → target)

| Package | Current | Target (SDK 54) |
|---|---|---|
| `expo` | `^54.0.33` | `~54.0.35` (normalize; already on 54) |
| `expo-router` | `~4.0.0` | `~6.0.24` |
| `react` | `18.3.1` | `19.1.0` |
| `react-dom` | `18.3.1` | `19.1.0` |
| `react-native` | `0.76.1` | `0.81.5` |
| `react-native-web` | `~0.19.13` | `~0.21.0` |
| `react-native-screens` | `~4.0.0` | `~4.16.0` |
| `react-native-safe-area-context` | `4.12.0` | `~5.6.0` |
| `expo-secure-store` | `~14.0.0` | `~15.0.8` |
| `expo-constants` | `~17.0.0` | `~18.0.13` |
| `expo-clipboard` | `~7.0.0` | `~8.0.8` |
| `expo-linking` | `~7.0.0` | `~8.0.12` |
| `expo-status-bar` | `~2.0.0` | `~3.0.9` |
| `@expo/vector-icons` | `^14.0.0` | `^15.0.3` |
| `@react-native-async-storage/async-storage` | `^1.23.1` | `2.2.0` |
| `@react-native-picker/picker` | `^2.9.0` | `2.11.1` |

`main: "expo-router/entry"` stays unchanged (still correct in v6).

### `devDependencies` — change these

| Package | Current | Target |
|---|---|---|
| `@types/react` | `~18.3.0` | `~19.1.0` (installs `19.1.17`) |
| `typescript` | `~5.6.0` | `~5.9.2` (installs `5.9.3`) |
| `@babel/core` | `^7.24.0` | keep (`^7.24.0` is compatible; optional bump `^7.25.2`) |

### Install procedure (do NOT hand-edit + `npm install` blindly)

The reliable, canonical way to apply all of the above and let Expo pick exact compatible pins:

```bash
cd app
# 1. set the SDK-54 pins (either apply the table manually, then:)
npx expo install --fix          # rewrites every expo-managed dep to the SDK-54 bundled version
# 2. TS-only devDeps aren't managed by expo install; set them explicitly:
npm install -D typescript@~5.9.2 @types/react@~19.1.0
```

`expo install --fix` reads the same `bundledNativeModules.json` used to author this table, so it is the source of truth if any pin here has drifted by implementation time. After it runs, `git diff package.json` must match the table above (allowing for patch-level bumps within the same tilde range).

Delete `node_modules` and the lockfile before a clean install if the resolver fights the React 18 → 19 jump:
```bash
rm -rf node_modules package-lock.json && npm install
```

## 3. Config file changes

| File | Change | Required? |
|---|---|---|
| `babel.config.js` | **None.** Uses `babel-preset-expo` only, which is correct for SDK 54. (The old `expo-router/babel` plugin was folded into the preset long ago — do **not** add it.) | — |
| `tsconfig.json` | **None.** `extends "expo/tsconfig.base"` + `strict` + `@/*` paths + `.expo/types` include all remain valid. Typed routes keep working via `app.json → experiments.typedRoutes`. | — |
| `metro.config.js` | **Optional cleanup:** remove the `Array.prototype.toReversed` polyfill block (lines 1–6). SDK 54 requires Node ≥ 20.19.4 where `toReversed` is native; the polyfill is dead code. Leave `getDefaultConfig(__dirname)` untouched. | Optional |
| `app.json` | Keep `newArchEnabled: true` (now the SDK-54 default — harmless to keep explicit). Plugins `["expo-router","expo-secure-store"]` stay. **Optional:** the top-level `splash` block is deprecated in SDK 54 in favor of the `expo-splash-screen` config plugin; migrating it silences an expo-doctor/warning but is not required for boot. If migrating: move `splash` into `plugins` as `["expo-splash-screen", { "image": "./assets/splash-icon.png", "resizeMode": "contain", "backgroundColor": "#ffffff" }]` and drop the top-level `splash` key. | Optional |

No new config files are needed.

## 4. Breaking API changes the implementer must handle in code

A source scan (`grep` over `app/` + `app/src/`) was run against the known SDK-52→54 / React-18→19 break points. **Result: the app source is already clean** — this migration is almost entirely dependency + config. Findings:

- **No** `useRef()` calls missing an initial argument (React 19 types now require one, e.g. `useRef<T>(null)`). ✅
- **No** `defaultProps` / `propTypes` on function components (removed in React 19). ✅
- **No** `SafeAreaView` / `SafeAreaConsumer` imports (the deprecated ones changed in safe-area-context v5). ✅ Confirm `SafeAreaProvider` / `useSafeAreaInsets` usage still compiles — it is unchanged.
- **No** `forwardRef` / string refs / direct `react-dom` `ReactDOM.render` usage. ✅
- **No** `JSX.Element` / bare `JSX` namespace annotations (React 19 moved the global `JSX` namespace under `React.JSX`; `@types/react` 19 handles this, but explicit `JSX.Element` returns would need `React.JSX.Element`). ✅
- **expo-router imports in use** are all still valid in v6: `Stack`, `Tabs`, `Redirect`, `Link`, `router`, `useRouter`, `useLocalSearchParams`. No removed/renamed router APIs are used. ✅

Things the implementer must nonetheless **watch for** during `tsc --noEmit` after the bump (React 19 + RN 0.81 tighten types even where the runtime is fine):
1. **React 19 type strictness** — stricter event handler and `children` typings may surface new TS errors in components that were loosely typed. Fix by tightening types, not by loosening `strict`.
2. **`react-native-web ~0.21`** requires React 19 (this is why RN Web had to move in lockstep) — verify no deprecated RNW props (e.g. removed `accessibilityRole` legacy aliases) are used on web-only paths.
3. **expo-router v6 typed routes** — with `experiments.typedRoutes: true`, `Link href`/`router.push` arguments are type-checked against generated route types. If any `href` was passed as a plain widened `string`, TS may now flag it; cast via the generated `Href` type or pass a literal route.
4. **New Architecture** is now actually active (it was declared but the SDK-52 RN could behave differently). Smoke-test any third-party native module — here only `@react-native-picker/picker` and `expo-secure-store` — for New-Arch runtime warnings on Android.

If `tsc` surfaces errors, fix them in code; do **not** downgrade individual packages off the SDK-54 line (that reintroduces the skew this WP exists to remove).

## 5. Verification steps & success criteria

Run in `app/`, in order. All must pass before the WP is done.

| # | Command | Success looks like |
|---|---|---|
| 1 | `rm -rf node_modules package-lock.json && npm install` | Installs with **no peer-dependency `ERESOLVE` errors**; no `--legacy-peer-deps` needed. |
| 2 | `npx expo install --check` | Reports every dependency is on the SDK-54 expected version (no "expected version … but found …" lines). |
| 3 | `npx expo-doctor` | **Clean** — "No issues detected!" (this is the WP-0.1 acceptance gate). A remaining *warning* about the deprecated top-level `splash` is acceptable only if §3's optional splash migration was skipped; there must be **no errors**. |
| 4 | `npx tsc --noEmit` | **Zero type errors.** (Fix any React-19 type fallout per §4 in code.) |
| 5 | `npm run web` → open the served URL | App boots in browser with no red-box/console-fatal errors; **login screen renders → can log in → lands on dashboard** with real data from the backend. |
| 6 | `npm run android` (device/emulator; backend reachable on LAN) | Metro bundles with no missing-native-module or New-Arch errors; app launches; **login → dashboard** flow works, same as web. |

**Definition of Done for WP-0.1:** steps 1–4 pass mechanically; steps 5–6 demonstrate the `login → dashboard` happy path on both web and Android; `git diff` touches only `app/package.json`, `app/package-lock.json`, and (optionally) `app/metro.config.js` / `app/app.json`. No changes outside `app/`. Commit referencing `WP-0.1`.

**Rollback note:** if the React 18 → 19 jump proves to cascade into unexpected runtime breakage under time pressure, the *fallback* coherent target is to instead downgrade `expo` to `~52.0.49` (matching the existing SDK-52 libs, which need no changes). This is explicitly the lesser option (older SDK, contradicts "latest stable") and should only be used if the SDK-54 path is blocked; prefer fixing forward.
