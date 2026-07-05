# WP-4.7: Head-Invariant Limitation

The group head cannot leave or be removed (`DELETE /groups/:id/members/:userId` → 409).
No headship transfer endpoint exists in v2.

**Future relaxation paths:**
- **Backlog §8.7 — Headship transfer:** add `POST /groups/:id/transfer-head` and update
  the head-invariant guard in `RemoveMember` to only block when no successor is named.
- **Group delete:** add `DELETE /groups/:id` (head only, all balances settled) and relax
  the remove guard to allow the departing head.

Until one of those ships, the 409 on head removal is correct and final.
