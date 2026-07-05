# WP-4.5 Spec: Branding Assets

**Work package:** WP-4.5 (Phase 4 — UX polish & launch readiness). Runs ∥ with 4.1/4.2/4.3/4.6/4.7.
**Type:** **Frontend assets + app-config only.** Replace the four 1×1 placeholder PNGs from WP-0.1
with the real icon / adaptive icon / splash / favicon, and align `app.json` identity with the
WP-1.0 design kit. **No backend, no API, no migration, no codegen** — this WP does **not** touch
`backend/openapi.yaml`, `app/src/api.ts`, or `app/src/api-types.gen.ts` (stated explicitly in §3).
**Depends on:** WP-0.1 (Expo SDK 54 alignment; the placeholder PNGs and the `app.json` this WP
overwrites) and WP-1.0 (`app/src/theme.ts` — the source of the brand color `primary #4F46E5`). Both
landed.
**Acceptance (master-plan §9, Phase 4 row 4.5):** "Replace the 1×1 placeholder PNGs from WP-0.1 with
real app icon, splash, adaptive icon, favicon; app display name — assets pass `expo-doctor` + render
correctly on all 3 platforms."

> Roadmap refs: master-plan line 52 (WP-0.1 note: "`app/assets/` icons are **1×1 placeholders**;
> real branding is WP-4.5" and "corrupt placeholder PNGs … asset generation crashed"), §7.4 / WP-1.0
> `theme.ts` (brand `primary #4F46E5`, `surface #FFFFFF`), §10 rule 3 (**FE verification floor**: if
> you cannot boot a browser you MUST run `npx expo export --platform web` and have it pass — it
> catches asset/bundle breakage that `expo-doctor` + `tsc` miss), §11 (low-risk FE WP: Sonnet/Haiku
> implement, Sonnet review).

---

## 0. Scope, guardrails & the decisions this WP makes

### 0.1 In scope (exactly these)

1. Replace the four placeholder PNGs in `app/assets/` (`icon.png`, `adaptive-icon.png`,
   `splash-icon.png`, `favicon.png`) with real, correctly-sized branded assets (§2).
2. One `app.json` edit: set `android.adaptiveIcon.backgroundColor` to the brand indigo so the
   adaptive foreground reads intentionally (§1). Everything else in `app.json` is verified and left
   unchanged.
3. Commit an SVG master (`app/assets/brand/pocketmoney-mark.svg`) as the canonical, editable source
   of the mark, plus a deterministic regeneration script (§2.3) so the raster assets are reproducible
   and never drift back to corrupt/placeholder state.

### 0.2 Out of scope / MUST NOT touch (guardrails)

- **Do NOT change `scheme`** (currently `"pocketmoney"`). WP-4.3 (merged) wires invite deep links as
  `pocketmoney://invite?token=xxx` (`docs/specs/WP-4.3.md` §5, F16). Changing the scheme silently
  breaks every invite deep link. **Hard guardrail — flag loudly if you reach for `scheme`.** (§5, D1)
- **Do NOT change** `slug` (`"pocket-money"` — the EAS project link), `ios.bundleIdentifier` /
  `android.package` (`"com.pocketmoney.app"` — app-store identity), or `version` (`"1.0.0"`). None
  are branding; changing them is an unrelated migration this WP does not own. (§5, D2)
- **Do NOT change `name`** — it is already `"Pocket Money"` (see §5, D3: the "app display name" line
  in the acceptance criteria is already satisfied; the remaining WP-4.5 work is the artwork + the one
  adaptive-bg color).
- **No new npm dependency.** Nothing is added to `app/package.json`. The regeneration script (§2.3)
  runs on an already-present toolchain or an ephemeral `npx`, never a `package.json` entry. (§5, D4)
- **No backend / API / codegen.** See §3.

### 0.3 The brand mark (fully specified so it is not an open question)

Wordless monogram: the **rupee glyph ₹** in **white** on the **brand indigo `#4F46E5`** (the exact
`theme.color.primary` from `app/src/theme.ts`). Rationale in §5, D5 — ₹ is already the money glyph
the app renders via `AmountText`, indigo is already the primary brand color, so the icon reads as
"this family's money" with zero new design vocabulary. The canonical vector is the committed SVG
master (§2.2); the four PNGs are rasterizations of it at the required sizes.

Exact brand values (do not hand-type — copy from `theme.ts`):

| Token | Value | Used for |
|---|---|---|
| `color.primary` | `#4F46E5` | icon tile fill, adaptive-icon background, favicon tile |
| `color.primaryText` | `#FFFFFF` | the ₹ glyph, on every surface |
| `color.surface` / white | `#FFFFFF` | splash background (unchanged, see §1) |

---

## 1. File-by-file change list (current → target)

### `app/app.json` — one field changes; everything else is verify-only

| Key | Current value | Target value | Action |
|---|---|---|---|
| `expo.name` | `"Pocket Money"` | `"Pocket Money"` | **verify only** — already correct (D3). Drives iOS/Android home-screen label + web `<title>`. |
| `expo.slug` | `"pocket-money"` | `"pocket-money"` | **verify only — do NOT change** (EAS link). |
| `expo.scheme` | `"pocketmoney"` | `"pocketmoney"` | **verify only — do NOT change** (WP-4.3 deep links, D1). |
| `expo.version` | `"1.0.0"` | `"1.0.0"` | **verify only — do NOT change.** |
| `expo.icon` | `"./assets/icon.png"` | `"./assets/icon.png"` | unchanged path; the file it points at is replaced (§2). |
| `expo.splash.image` | `"./assets/splash-icon.png"` | same | unchanged path; file replaced (§2). |
| `expo.splash.resizeMode` | `"contain"` | `"contain"` | unchanged — the mark is centered, not full-bleed. |
| `expo.splash.backgroundColor` | `"#ffffff"` | `"#ffffff"` | unchanged (D6 — white splash, the mark carries its own indigo tile). |
| `expo.android.adaptiveIcon.foregroundImage` | `"./assets/adaptive-icon.png"` | same | unchanged path; file replaced (§2). |
| `expo.android.adaptiveIcon.backgroundColor` | `"#ffffff"` | **`"#4F46E5"`** | **CHANGE THIS.** The adaptive foreground (§2) is a white ₹ on transparent; the brand-indigo backing makes it a coherent branded tile instead of a white glyph vanishing on a white plate. |
| `expo.ios.bundleIdentifier` | `"com.pocketmoney.app"` | same | **verify only — do NOT change.** |
| `expo.android.package` | `"com.pocketmoney.app"` | same | **verify only — do NOT change.** |
| `expo.web.favicon` | `"./assets/favicon.png"` | same | unchanged path; file replaced (§2). |
| `expo.web.output` | `"single"` | `"single"` | unchanged. |

**Net `app.json` diff = one line** (`adaptiveIcon.backgroundColor`: `#ffffff` → `#4F46E5`). Do not
reformat, reorder, or otherwise churn the file — a one-line diff keeps this out of every other lane's
way (§4).

### `app/assets/` — four files replaced, one added

| Path | Current | Target |
|---|---|---|
| `app/assets/icon.png` | 1×1 RGBA placeholder | 1024×1024 opaque PNG (§2.1) |
| `app/assets/adaptive-icon.png` | 1×1 RGBA placeholder | 1024×1024 transparent PNG (§2.1) |
| `app/assets/splash-icon.png` | 1×1 RGBA placeholder | 1024×1024 transparent PNG (§2.1) |
| `app/assets/favicon.png` | 1×1 RGBA placeholder | 48×48 opaque PNG (§2.1) |
| `app/assets/brand/pocketmoney-mark.svg` | — (new) | canonical vector master (§2.2) |
| `app/scripts/gen-brand-assets.mjs` **or** `.py` | — (new) | deterministic regenerator (§2.3) |

`app/assets/.gitkeep` stays. The PNGs are already git-tracked, so replacement is `git add`-then-commit
of the changed bytes.

---

## 2. Asset inventory & generation

### 2.0 Current state (verified against reality, `file` + PIL on each)

All four assets are **`PNG image data, 1 x 1, 8-bit/color RGBA`** — valid 1×1 PNGs (the WP-0.1 review
already fixed the *corrupt* versions that crashed asset generation; these are the good-but-tiny
replacements). They export cleanly today but are visually empty. This WP swaps them for real art.

### 2.1 Required sizes (Expo SDK 54)

| Asset | Dimensions | Alpha | Content rules |
|---|---|---|---|
| `icon.png` | **1024×1024** | **opaque (no alpha)** | iOS flattens alpha to black and applies its own corner mask. Full-bleed brand-indigo square, **no pre-rounded corners**, white ₹ centered at ~55% of the canvas. |
| `adaptive-icon.png` | **1024×1024** | transparent | Android foreground layer. Content MUST sit inside the **centered safe zone** — a circle of diameter ≈66% (≈676px) — because launchers mask to circle/squircle/rounded-square and crop the outer ~33%. White ₹ centered at ~45% of the canvas so it never clips. Background comes from `adaptiveIcon.backgroundColor` (`#4F46E5`), not this file. |
| `splash-icon.png` | **1024×1024** | transparent | Shown centered on the white splash (`resizeMode: contain`). The full mark: rounded-corner indigo tile (~70% of canvas, corner radius ≈18% of tile) with the white ₹, floating on white. |
| `favicon.png` | **48×48** | opaque | Expo's documented favicon size; emitted as-is into the web `<link rel="shortcut icon">`. Indigo tile + white ₹, legible at 16px. (32×32 or up to 196×196 also acceptable; 48×48 is the Expo default.) |

All PNGs 8-bit, non-interlaced, sRGB. `icon.png` and `favicon.png` must have **no alpha channel**
(iOS/Safari); the two overlay layers (`adaptive-icon`, `splash-icon`) **must** have alpha.

### 2.2 Canonical vector master — `app/assets/brand/pocketmoney-mark.svg`

Commit this exact SVG as the single editable source of truth. It is the full-mark variant (indigo
tile + white ₹); the icon/adaptive/favicon variants are trivial crops/recolors of it.

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
  <rect x="96" y="96" width="832" height="832" rx="184" fill="#4F46E5"/>
  <text x="512" y="512" fill="#FFFFFF" font-family="'DejaVu Sans','Noto Sans',sans-serif"
        font-weight="700" font-size="620" text-anchor="middle" dominant-baseline="central">&#8377;</text>
</svg>
```

`&#8377;` is U+20B9 (₹). **Determinism caveat:** `<text>` rasterization depends on an installed font
that contains U+20B9. For a font-independent, byte-reproducible master, convert the glyph to a
`<path>` in your editor before committing (Inkscape → *Path ▸ Object to Path*), or accept the
font-dependency and rely on §2.3's fallback. Either way the **acceptance gate is dimensions + a
passing web export (§3)**, not pixel-identity — so a human exporting from this SVG in any tool
(Figma/Inkscape/Illustrator/Canva/an online Expo icon generator) is a fully valid path.

### 2.3 Generation — pick ONE path; both are no-new-dependency

Item-5 decision (D5): the artwork is **fully specified** above, so this is *not* a "designer must
invent a logo" blocker. Produce the four PNGs by either:

**Path A — human/tool export (preferred for release quality).** Open `pocketmoney-mark.svg`, export
the four PNGs at the §2.1 sizes with the stated alpha/crop rules. Any tool. Drop them into
`app/assets/`. This is the release-quality path and needs no scripting.

**Path B — deterministic script (for a headless agent / CI-unblock).** Commit a small self-contained
generator that adds **nothing** to `package.json`:

- *Node variant* `app/scripts/gen-brand-assets.mjs`: rasterize the SVG with `sharp`, invoked
  ephemerally — e.g. `npx --yes -p sharp@0.34.2 node app/scripts/gen-brand-assets.mjs`. `npx --yes`
  fetches `sharp` transiently; it is **not** written to `app/package.json` (D4).
- *Python variant* `app/scripts/gen_brand_assets.py`: draw the mark with **Pillow** (present in this
  repo's tooling env; U+20B9 lives in DejaVu Sans ≥2.34, the Linux default). Draws: `icon` = full
  `#4F46E5` square + centered white ₹ (flatten to RGB, no alpha); `adaptive-icon` = transparent +
  white ₹ scaled into the 66% safe zone; `splash-icon` = transparent + rounded indigo tile (radius
  ≈18%) + white ₹; `favicon` = downscale of the icon to 48×48 (RGB).

Either script is **idempotent** — re-running reproduces identical assets, so the repo can never drift
back to the corrupt/placeholder state that bit WP-0.1. The script is a convenience/guard, not a build
step: it is **not** wired into `npm`/Metro and does **not** run on `expo export`.

> If neither path is available in the implementer's environment (no image tool at all), **stop and
> escalate** — do not commit 1×1 or hand-fabricated PNGs. A green export over empty icons is a false
> pass; §3's manual QA (§6) exists to catch exactly that.

---

## 3. Acceptance criteria & verification gates

**Gates (all must pass):**

1. **`npx expo export --platform web`** succeeds from `app/` (master-plan §10 rule 3, the FE
   verification floor). This is the load-bearing automated gate — it is what caught the corrupt-PNG
   breakage in WP-0.1 that `expo-doctor` + `tsc` both missed. `node_modules` is not installed in the
   docs worktree; run this in an environment with `npm ci` done.
2. **`npx tsc --noEmit`** clean from `app/` (no code changed, so this is a regression guard on the
   `app.json`/asset edit, not a real risk).
3. **`npx expo-doctor`** reports no asset/config errors (explicit in the acceptance line).
4. **Dimensions check** — each of the four PNGs matches §2.1 exactly and is **not 1×1**. One-liner:
   `for f in icon adaptive-icon splash-icon favicon; do file app/assets/$f.png; done` — expect
   1024×1024 for the first three and 48×48 for favicon, and RGBA on the two overlays / RGB on
   icon+favicon.
5. **`app.json` diff is exactly one line** (`adaptiveIcon.backgroundColor`) — `git diff app/app.json`
   shows only that. `scheme`, `slug`, `bundleIdentifier`, `package`, `name`, `version` unchanged
   (grep-assert them).

**Explicitly untouched (state in the PR):**

- **NO** `backend/openapi.yaml` change → **NO** `npm run codegen` → `app/src/api-types.gen.ts` and
  `app/src/api.ts` are **byte-for-byte unchanged**. This WP is pure assets + one config line; the
  contract-first pipeline (§10.1) does not apply because there is no contract change. `git diff
  --stat` must show only `app/app.json`, files under `app/assets/`, and the new `app/scripts/`
  generator — nothing else.

**Render correctness (all 3 platforms — from the acceptance line):** covered by the manual QA in §6.
The automated gates prove the assets *load*; §6 proves they *look right*.

---

## 4. Concurrency lane table

WP-4.5 owns a set of files **no other in-flight WP touches**. Confirmed by grepping every spec in
`docs/specs/` for `app.json` / `app/assets` / `favicon`: only WP-0.1 (setup, merged) and WP-4.3
(scheme **verify-only**, merged) reference them.

| File / path | WP-4.5 | WP-4.6 (spec merged, impl later) | WP-4.7 (in implementation) | Overlap? |
|---|---|---|---|---|
| `app/app.json` | **owns** (1-line adaptive-bg edit + verify) | does not touch | does not touch | **none** |
| `app/assets/*.png` (4) | **owns** (replace) | does not touch | does not touch | **none** |
| `app/assets/brand/*.svg`, `app/scripts/gen-brand-assets.*` | **owns** (new) | n/a | n/a | **none** |
| `backend/openapi.yaml`, `app/src/api.ts`, `app/src/api-types.gen.ts` | untouched | WP-4.6 edits (register→JWT) | WP-4.7 edits (2 endpoints) | none *for 4.5* |
| register/auth/empty-state screens | untouched | WP-4.6 owns | — | none |
| auth/groups handlers, profile/group/member screens | untouched | — | WP-4.7 owns | none |

**Result: zero overlap.** WP-4.5 can branch off `master` and land at any time relative to 4.6/4.7 —
no ordering dependency in either direction. It carries none of the WP-4.1 sequencing constraint that
4.6/4.7 have, because it never touches a screen.

**Rebase rule (only realistic collision):** `app/app.json`. WP-4.6 and WP-4.7 both declare app.json
out of scope, so a conflict there means the *other* side drifted out of lane — but to be safe, keep
WP-4.5's app.json edit to the **single `adaptiveIcon.backgroundColor` line** so any 3-way merge is
trivial. If WP-4.5 lands after another WP unexpectedly edited app.json, re-apply just that one line
onto their version; never overwrite their block. (The scheme guardrail in §0.2 also protects against a
careless rebase reintroducing a scheme change.)

---

## 5. Decisions on what the master plan under-specifies

The master-plan row for WP-4.5 is a single table line; every ambiguity is resolved here so the
implementer has no open questions.

- **D1 — `scheme` stays `"pocketmoney"`.** The master plan says "app display name" but says nothing
  about identifiers; WP-4.3 (merged) hard-depends on the scheme for deep links. Branding must not
  ride along an identifier change. *Rationale:* a scheme change is a silent invite-link breakage with
  no compile error to catch it.
- **D2 — `slug` / `bundleIdentifier` / `android.package` / `version` all stay.** They are store/EAS
  identity, not branding, and changing any is a separate, riskier migration outside this WP's scope.
- **D3 — `name` is already `"Pocket Money"`; no change.** The acceptance line lists "app display
  name" as if it were pending, but reality (verified in `app.json`) shows it is already set correctly.
  The remaining real work is therefore the four assets + the one adaptive-bg color. This is called out
  so a reviewer does not expect a `name` diff.
- **D4 — no new dependency.** The master plan is silent on *how* to generate assets. Decision: the
  committed SVG is the source of truth and the raster step uses either a human tool or an ephemeral
  `npx`/already-present Pillow — nothing lands in `package.json`. Keeps the app's dependency surface
  and the WP-0.1 "no bundle breakage" property intact.
- **D5 — the mark is a white ₹ on indigo `#4F46E5`.** The plan says "real app icon" without art
  direction. Decision: reuse the app's existing vocabulary (₹ = money, indigo = brand `primary`)
  rather than invent a logo, so this WP needs no design round-trip and stays low-risk per §11.
- **D6 — splash background stays white (`#ffffff`); adaptive background becomes indigo
  (`#4F46E5`).** *Rationale:* the splash mark carries its own indigo tile and reads best floating on
  white (matches `theme.color.surface`); the adaptive foreground is a bare white glyph and needs the
  indigo backing to be visible. Two different layers, two correct choices.

---

## 6. Manual QA script (render correctness — the "all 3 platforms" clause)

Run after the gates in §3 pass. The automated export only proves the assets *load*; this proves they
*look* right and that nothing regressed in deep-linking.

**Web (`npm run web`, or serve the `dist/` from `expo export --platform web`):**
1. Browser tab **title** reads **"Pocket Money"** (derives from `expo.name`).
2. Browser tab **favicon** shows the indigo-₹ mark, not the default Expo/Metro glyph and not a blank
   square. Legible at tab size.
3. Sanity: the app still boots and an **invite link `pocketmoney://invite?token=…` / `/invite?token=…`
   still resolves** — confirms the scheme guardrail (D1) held. (Open an invite URL; it should route to
   the invite screen, not error.)

**Android (dev build / emulator):**
4. **Home-screen icon**: white ₹ on an indigo tile, correctly masked (circle/squircle) with the glyph
   inside the safe zone — no clipping of the ₹, no white halo, indigo fills the whole plate.
5. **Home-screen label** under the icon reads **"Pocket Money"**.
6. **Splash**: launching shows the centered ₹ mark on a white background, not stretched/full-bleed and
   not an empty white screen.

**iOS (dev build / simulator):**
7. **Home-screen icon**: indigo tile + white ₹, OS-rounded corners, **no black corners** (proves
   `icon.png` is opaque with no alpha).
8. **Home-screen label** reads **"Pocket Money"**.
9. **Splash**: centered ₹ mark on white.

**Fail signal:** any icon showing as a plain white/black square, a blank favicon, a stretched splash,
or black iOS corners means an asset is wrong size / wrong alpha — regenerate per §2 before merging.

---

## 7. Definition of Done

- [ ] Four PNGs replaced at exact §2.1 sizes/alpha; `file` confirms none is 1×1.
- [ ] `app.json` diff is exactly the one `adaptiveIcon.backgroundColor` line (`#ffffff` → `#4F46E5`);
      `scheme`/`slug`/`bundleIdentifier`/`package`/`name`/`version` unchanged.
- [ ] `app/assets/brand/pocketmoney-mark.svg` + `app/scripts/gen-brand-assets.*` committed (source +
      idempotent regenerator).
- [ ] `npx expo export --platform web` passes; `npx tsc --noEmit` clean; `npx expo-doctor` clean.
- [ ] `git diff --stat` shows only `app/app.json`, `app/assets/**`, `app/scripts/**` — **no**
      `openapi.yaml`, `api.ts`, or `api-types.gen.ts` change (§3).
- [ ] Manual QA §6 done on web + at least one native platform; results noted in the PR.
- [ ] PR titled `WP-4.5: branding assets` (§10.9).
