# Phase 1 (currency) ships as ONE combined BE+FE PR

**Decision (2026-07-10, spec stage of autopilot run `ap-40ae610be6ef`).**
V3-1.1 (backend currency) and V3-1.2 (frontend currency) land together as a
**single PR into `autopilot/integration`**, not two.

## Why

- V3-1.1 turns every API amount field into a `Money { currency, value }` object
  (D7 / §3.8) — a **breaking runtime change on existing endpoints**. The moment
  the backend serializes `Money`, the current frontend (which reads bare numbers)
  and the e2e suite (which asserts rendered amounts) break.
- The autopilot `land` gate requires **full green CI including the e2e job**.
  A BE-only PR would leave e2e red, so it cannot land on its own; an FE-only PR
  has nothing to run against. The two halves are only green **together**.
- Therefore Phase 1 is the one place in master-plan-v3 where the usual
  "BE and FE are separate WPs/PRs" rule (§8) is deliberately overridden. Both
  specs still exist separately (`docs/specs/V3-1.1.md`, `V3-1.2.md`) and can be
  implemented by separate agents, but they **merge as one commit set / one PR**.

## Mechanics

- Both specs are committed **before** implementation (§8/§9).
- Implementation order: V3-1.1 first (openapi + BE + migration + seed), then
  V3-1.2 (`npm run codegen` reads the updated `openapi.yaml`, then FE + e2e).
  They share `backend/openapi.yaml` as the contract seam.
- Single branch, single PR. Do **not** open a BE-only PR expecting green CI.
- Review per §10: V3-1.1 is Opus-grade (API-wide breaking change); the FE half is
  reviewed in the same PR.

## Later phases revert to the norm

This combined-PR exception is **Phase-1-only**, because currency is the one
change that breaks every existing endpoint's response shape at once. Phases 2–6
keep BE and FE as separate WPs/PRs per §8 unless a future spec documents another
explicit exception.
</content>
