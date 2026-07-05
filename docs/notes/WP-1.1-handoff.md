# WP-1.1 Handoff Note — Minor-Units Migration

**From:** WP-1.0 implementer  
**For:** WP-1.1 implementer  
**Date:** 2026-07-05

## What WP-1.1 must clean up after the API migration

WP-1.0 introduced `AmountText` with an **end-state API**: it takes integer minor units (paise).
The API still returns float rupees (`number`) at WP-1.0 time, so an interim shim bridges the gap.

### Step 1 — Delete the shim

In `app/src/money.ts`, delete `rupeesToMinor`:

```ts
// DELETE this entire function in WP-1.1:
export function rupeesToMinor(rupees: number): number {
  return Math.round(rupees * 100);
}
```

### Step 2 — Remove all call-site wrappers

Find every call:

```bash
grep -rn "rupeesToMinor" app/
```

Expected hits: `app/app/(app)/groups/[id]/chores.tsx` (and any other screens added after WP-1.0).

For each hit, replace `rupeesToMinor(someValue)` with `someValue` — once `Chore.amount` is an
integer (minor units) from the regenerated OpenAPI types, no conversion is needed.

### What does NOT change

- `AmountText`, `formatMinor`, and `moneyTextStyle` are already the end-state implementation.
  WP-1.1 must not modify them.
- The chore create/update payload currently sends `amount: parseFloat(...)` (float rupees).
  WP-1.1 must update this to send integer minor units (parse `"12.50"` → `1250` instead of
  `parseFloat("12.50")` → `12.5`). This is the input-parsing change described in master plan §10.6.

### Reference

See WP-1.0 spec §5 for the full rationale behind the two-phase design.
