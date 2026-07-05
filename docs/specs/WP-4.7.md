# WP-4.7 Spec: Account & Membership Hygiene

**Work package:** WP-4.7 (Phase 4 — UX polish & launch readiness). Runs ∥ with 4.1/4.2/4.3/4.5/4.6
but has an **ordering dependency on WP-4.1** and a **soft rebase dependency on WP-4.2 / WP-4.3**
(see §0.1).
**Type:** **Backend + Frontend**, contract-first. Two new endpoints (change password; remove/leave
membership). **No new table, no migration** (§3 proves it — flag loudly if you reach for `ALTER
TABLE`). Contract-first order: `openapi.yaml` delta → BE handlers/repos → `npm run codegen` → FE.
**Depends on:** bcrypt already wired for register/login (`auth.go`), the ledger v2 balance math
(WP-1.2), loans (WP-3.1: `ListActiveLoans`, `LockActiveLoan`, lock-order conventions), allowances +
the lazy posting engine (WP-2.1: `PostDue`/`runPosting`), and the WP-1.0 kit (`Button`, `Sheet`,
`TextField`, `useToast`, `confirmAsync`, `LoadingSpinner`, `ErrorMessage`). All landed.
**Acceptance (master-plan §9, Phase 4 row 4.7):** "Change password (logged-in, no email infra
needed); head can remove a member (allowed only when balance is settled to 0 and no active loans —
history/ledger rows are kept); member can leave under the same conditions. **authz tests:** only head
removes others, anyone may leave self; removal blocked while balance ≠ 0 or a loan is active, with a
human-readable error."

> Roadmap refs: master-plan §5.1 (balance = Σ approved credits − Σ approved debits, minor units,
> int64, `::bigint` SUMs), §5.3 (loans — `requested | active | rejected | closed`), §5.4 (lazy
> `PostDue` — a stale un-posted allowance/EMI must be posted before a balance is trusted), §5.5
> (authz matrix — head vs member), §6.1 (API delta, error shape `{"error": "..."}` with
> **400/401/403/404/409/500** — note: the house code set does **not** include 422), §10 (Definition
> of Done — contract-first, money as minor units, authz in API mirrored in UI, migrations paired
> up/down), §11 (this WP is medium-risk: it touches auth + balance invariants + a transactional
> multi-check delete → Sonnet implement with strict Opus review; the balance/loan/`PostDue` invariants
> in §5 must be in the review prompt).

---

## 0. Scope, guardrails & the decisions this WP makes

### In scope (exactly these)

1. **Change password (logged-in)** — `PUT /api/v1/auth/password` `{current_password, new_password}`.
   Verify the current password with bcrypt (mirror login), hash + store the new one. No email, no
   reset token, no session/JWT rotation (§5, D1). (§2.1, §4.1)
2. **Remove member / leave group** — one endpoint, `DELETE /api/v1/groups/:id/members/:userId`,
   serving **both** head-removes-a-member and member-leaves-self (D2). Allowed only when the target's
   **balance is settled to 0** AND they have **no active or pending loan**; both checks + the
   membership delete run in **one transaction** after a mandatory `PostDue` (§4.2, D3–D7).
3. **FE surfaces** — change-password `Sheet` on `profile.tsx`; "Remove member" on the head's
   `members/[userId].tsx`; "Leave group" on the member view of `groups/[id]/index.tsx`. All via kit,
   `confirmAsync` for the destructive actions, `Toast` for outcomes. (§5)

### Non-goals (explicit boundaries)

- **No password reset / email verification** — that is backlog §8 item 9 (needs email infra;
  explicitly deferred and out of the launch bar for a LAN deployment). This WP is the *logged-in*
  self-service change only.
- **No headship transfer, no group deletion, no "delete account".** Transfer of headship is backlog
  §8 item 7. Consequently the head can neither leave nor be removed (D6) — a group always keeps its
  head. If a head wants to disband, that is a future group-delete WP; note it in `docs/notes/`, do not
  build it here.
- **No migration, no schema change** (§3). Every operation reads/deletes existing tables. If you find
  yourself writing `ALTER TABLE`/a new numbered migration, **STOP — you have drifted**; re-read §3.
- **No change to `PostDue`, the posting engine, or the loan lifecycle endpoints.** We *call*
  `PostDue` and *reuse* the loan lock-order convention; we do not modify them.
- **No new dependency** (BE or FE). bcrypt (`golang.org/x/crypto/bcrypt`) and the kit are present.
- No edits to `theme.ts` or any kit component (frozen from WP-1.0).

### 0.1 Sequencing & rebase note (READ FIRST — concurrent work)

**Branch WP-4.7 off `master` after WP-4.1 has merged** (WP-4.1 re-skinned `profile.tsx`,
`login.tsx`, dashboard, etc. onto the kit — this WP builds on the swept `profile.tsx`). WP-4.2 and
WP-4.3 are **in flight on files this WP also touches**; write and rebase against **their specs as the
assumed post-merge state**:

| File | Touched by | WP-4.7's edit | Collision risk |
|---|---|---|---|
| `app/app/(app)/profile.tsx` | WP-4.1 (merged) | **add** change-password Button + Sheet | none (4.1 done) |
| `app/app/(app)/groups/[id]/index.tsx` | **WP-4.3** (head Invite button + `handleInvite`) | **add** "Leave group" in the **member-role** render branch | low — different render branch (head vs member); **rebase after WP-4.3 merges**, take its version as the base |
| `app/app/(app)/index.tsx` (dashboard) | **WP-4.2** (full rebuild) | **must not touch** | n/a — WP-4.7 does not edit the dashboard; it only invalidates `qk.groups()` so 4.2's totals refresh |
| `app/app/(app)/groups/[id]/members/[userId].tsx` | neither | **add** "Remove member" | none — owned by neither 4.2 nor 4.3 |
| `app/src/api.ts`, `app/src/hooks/*`, `app/src/query-keys.ts` | WP-4.2 edits `groupsApi.list`/`useGroups` | **add** `authApi.changePassword`, `groupsApi.removeMember`, `useChangePassword`/`useRemoveMember`/`useLeaveGroup` | low — additive; do not touch `groupsApi.list`/`useGroups` |
| `backend/internal/handlers/groups.go`, `auth.go`, `main.go` | WP-4.2 edits `ListGroups`/`main.go` | **add** `ChangePassword`, `RemoveMember` handlers + routes + constructor deps | low — additive; if `NewGroupHandler`'s signature was already changed by 4.2, extend the same signature, don't fork it |

**Rebase rule:** if a merge conflict appears, one side drifted out of its lane. WP-4.7 adds new
handlers/routes/methods and new FE Buttons+Sheets; it does not rewrite the dashboard, the Invite flow,
or `groupsApi.list`. Reconcile against this table.

### 0.2 The seven design decisions (each justified in §5)

- **D1 — Change password does NOT rotate the JWT / invalidate other sessions.** There is no session
  store; tokens are stateless JWTs valid until expiry. Rotating would require a token blacklist we do
  not have. For a LAN/HTTP v2 this is acceptable and documented; note it as a limitation, don't build
  a blacklist. The caller stays logged in on their current token after a successful change.
- **D2 — One endpoint (`DELETE /groups/:id/members/:userId`) for both remove and leave**, not a
  separate `POST /groups/:id/leave`. One transaction, one set of hygiene checks, one head-invariant
  guard; authz differs only by *who the caller is relative to the target*. Justified in §5.2.
- **D3 — `PostDue` runs FIRST, before the removal transaction.** A month may have rolled over with an
  un-posted allowance credit or EMI debit; without posting, the in-tx balance reads a **stale zero**
  and a genuinely non-zero member slips out (§5.4). This is the load-bearing correctness step.
- **D4 — Balance is computed the same way `GetBalanceForGroup` computes it**: `Σ(credit) − Σ(debit)`
  over **`status='approved'`** entries, `::bigint`-cast SUMs, int64. Removal requires exactly `0`.
- **D5 — Active-loan check blocks on `requested` OR `active`** (not just active). Argued in §5.3: a
  dangling `requested` loan for a departed member is an open obligation the head could later
  "approve", posting EMIs to a ghost. `rejected`/`closed` are terminal → do not block.
- **D6 — The group head can neither leave nor be removed** → `409`. With no headship transfer, this
  guarantees the "always has a head" invariant trivially.
- **D7 — On removal: the member's `allowances` rows are DELETED; the member's `pending_approval`
  ledger entries are auto-rejected; all `approved`/`rejected` ledger history is KEPT.** Rationale in
  §5.5: allowances are *forward-looking config* (deleting prevents a "ghost allowance" silently
  reactivating on rejoin); pending entries are auto-rejected so the head can't later approve a credit
  for a non-member; the ledger audit trail is immutable history and stays.

Error-code choice (consistent with the house set 400/401/403/404/409/500 — **no 422**, which the API
never uses): **403** for authz (a non-head removing someone else), **404** for a target who is not a
member, **409** for every business-rule block (head-invariant, non-zero balance, active/pending
loan), each with a distinct human-readable `{"error": "..."}` message.

---

## 1. Data-flow overview

```
PUT /api/v1/auth/password           (caller = JWT subject; no :id — always self)
  body {current_password, new_password}
  → bcrypt.CompareHashAndPassword(stored, current)  → mismatch → 403 (NOT 401)
  → bcrypt.GenerateFromPassword(new)                → UserRepo.UpdatePassword → 204

DELETE /api/v1/groups/:id/members/:userId           (remove-other = head; leave = self)
  → GetMember(caller)                               → not member → 403
  → authz: caller==target (leave, non-head) OR caller is head (remove other) else → 403
  → runPosting(PostDue(groupID, now))               ← D3: post stale allowance/EMI FIRST
  → BEGIN tx
       lock group_members(target) FOR UPDATE        → not found → 404
       target.role == head                          → 409 (D6)
       lock loans(target) WHERE status IN (requested,active) FOR UPDATE → any → 409 (D5)
       balance(target) = Σcredit−Σdebit approved     → ≠ 0 → 409 (D4)
       auto-reject target's pending_approval entries (D7)
       DELETE target's allowances rows              (D7)
       DELETE group_members(target)                 (ledger/loans KEPT — FK is to users, D7)
     COMMIT → 204
```

---

## 2. Contract delta — `backend/openapi.yaml` (do this FIRST)

Three edits: one new path for change-password, one new path for member removal, one new request
schema. Error shape stays `#/components/schemas/ErrorResponse` (`{error}`).

### 2.1 Add `PUT /api/v1/auth/password`

Insert as a new path item immediately **after** the `/api/v1/auth/me` block (ends ~line 156, before
`/api/v1/groups` at ~line 158):

```yaml
  /api/v1/auth/password:
    put:
      tags:
        - Auth
      summary: Change password (logged-in)
      description: >
        Changes the authenticated user's password. Verifies the current password
        (bcrypt) before setting the new one. No email/reset flow. Does NOT rotate
        the JWT — existing tokens remain valid until expiry (LAN v2 limitation).
      operationId: changePassword
      security:
        - BearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ChangePasswordRequest'
      responses:
        '204':
          description: Password changed successfully (no content)
        '400':
          description: Malformed body or new password too short (min 6)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Not authenticated (missing/invalid token)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '403':
          description: Current password is incorrect
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

> **Why 403, not 401, for a wrong current password:** the FE fetch client (`app/src/api.ts:51`)
> treats **any** `401` as "session expired" — it clears the token and force-redirects to login. A
> `401` here would log the user out mid-password-change. `403` ("current password is incorrect")
> keeps the session and surfaces a normal inline error. This is a hard constraint, not a preference.

### 2.2 Add `DELETE /api/v1/groups/{id}/members/{userId}`

Insert as a new path item immediately **after** the `/api/v1/groups/{id}/members` (GET list) block
(ends ~line 334, before `/api/v1/groups/{id}/invite` at ~line 335):

```yaml
  /api/v1/groups/{id}/members/{userId}:
    delete:
      tags:
        - Groups
      summary: Remove a member (head) or leave a group (self)
      description: >
        Removes a membership. The head may remove any non-head member; any member
        may remove themselves (leave). Allowed ONLY when the target member's balance
        is settled to exactly 0 AND they have no loan in `requested` or `active`
        status. Ledger history and closed/rejected loans are KEPT (they reference the
        user, not the membership); the member's allowance config rows are deleted and
        any pending ledger entries are auto-rejected. The group head can neither leave
        nor be removed. Triggers due allowance/EMI posting before evaluating the
        balance so a stale un-posted entry cannot let a non-zero member out.
      operationId: removeMember
      security:
        - BearerAuth: []
      parameters:
        - name: id
          in: path
          required: true
          description: Group ID (UUID)
          schema:
            type: string
            format: uuid
        - name: userId
          in: path
          required: true
          description: ID of the member to remove (equal to the caller for "leave")
          schema:
            type: string
            format: uuid
      responses:
        '204':
          description: Member removed / left successfully (no content)
        '400':
          description: Invalid group ID or user ID
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Not authenticated
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '403':
          description: A non-head member may only remove themselves
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: Target user is not a member of this group
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '409':
          description: >
            Removal blocked — the target is the group head, or their balance is not
            zero, or they have an active/pending loan. The error message is
            human-readable and states which.
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

### 2.3 Add the `ChangePasswordRequest` schema

Under `components.schemas`, immediately after `RegisterRequest` (~line 1387). `new_password` mirrors
register's `minLength: 6`:

```yaml
    ChangePasswordRequest:
      type: object
      properties:
        current_password:
          type: string
          description: The user's current password, verified before the change.
        new_password:
          type: string
          minLength: 6
          description: The new password (minimum 6 characters).
      required:
        - current_password
        - new_password
```

No response schema for either endpoint (both return `204` on success; errors use `ErrorResponse`).
The `DELETE` takes no body.

---

## 3. Migration check — **NONE NEEDED** (verify, and flag loudly if wrong)

Every operation uses columns/tables that already exist:

- **Change password** → `users.password_hash` (exists, migration 001) — an `UPDATE`.
- **Balance** → `ledger_entries` (`direction`, `amount BIGINT`, `status`) — a read, identical to
  `GetBalanceForGroup` (`ledger_repo.go:259`).
- **Active/pending loan** → `loans.status` (exists, migration 011) — a read.
- **Membership delete** → `DELETE FROM group_members` (exists, migration 002).
- **Allowance cleanup** → `DELETE FROM allowances` (exists, migration 010).
- **Pending auto-reject** → `UPDATE ledger_entries SET status='rejected'` (existing column values).

**Crucially, no FK points at `group_members`.** `ledger_entries`, `loans`, and `allowances` all FK to
`users(id)` / `groups(id)` **with `ON DELETE CASCADE` on the *user/group*, not the membership**
(verified: `004_create_ledger_entries.up.sql:7-8`, `010_allowances.up.sql`, `011_loans.up.sql:6-7`).
Deleting a `group_members` row therefore removes **only** the membership — the user still exists, so
their ledger/loan/allowance rows are untouched. "History/ledger rows kept" is thus automatic; we
delete allowances explicitly (D7), not because a cascade would.

**Therefore this WP adds NO migration file.** If you reach for `ALTER TABLE` or a new numbered
migration, **STOP** — re-read this section. There is no schema to change.

---

## 4. Backend implementation

### 4.1 Change password — `AuthHandler.ChangePassword` + `UserRepo.UpdatePassword`

`AuthHandler` already holds `userRepo` and imports bcrypt — no constructor change. Add a route
`protected.PUT("/auth/password", authHandler.ChangePassword)` in `main.go` (in the protected group,
next to `GET /auth/me`).

**Repo — `backend/internal/db/user_repo.go`:**

```go
// UpdatePassword replaces a user's password hash. Returns ErrNotFound if the user
// row is gone (should not happen for an authenticated caller).
func (r *UserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
    tag, err := r.pool.Exec(ctx,
        `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash)
    if err != nil {
        return fmt.Errorf("failed to update password: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}
```

**Handler — `backend/internal/handlers/auth.go`:**

```go
// ChangePasswordRequest is the body for PUT /auth/password.
type ChangePasswordRequest struct {
    CurrentPassword string `json:"current_password" binding:"required"`
    NewPassword     string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword handles PUT /api/v1/auth/password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
    userIDStr, exists := auth.GetUserID(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
        return
    }
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
        return
    }

    var req ChangePasswordRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    user, err := h.userRepo.GetByID(c.Request.Context(), userID)
    if err != nil {
        if errors.Is(err, db.ErrNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
        return
    }

    // Verify the CURRENT password. Mismatch → 403, NOT 401 (a 401 would trip the FE
    // client's global logout interceptor and end the session mid-change).
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
        c.JSON(http.StatusForbidden, gin.H{"error": "current password is incorrect"})
        return
    }

    newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
        return
    }
    if err := h.userRepo.UpdatePassword(c.Request.Context(), userID, string(newHash)); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
        return
    }

    c.Status(http.StatusNoContent) // 204 — nothing to return; JWT unchanged (D1)
}
```

### 4.2 Remove/leave — `GroupHandler.RemoveMember` (one endpoint, transactional)

`GroupHandler` currently holds `groupRepo, inviteRepo, choreRepo, postingSvc`. The removal path also
needs the **ledger**, **loan**, and **allowance** repos and a `*pgxpool.Pool` (to `Begin` the
transaction — mirror `LoanHandler`, which holds `pool`). Extend the constructor:

```go
type GroupHandler struct {
    groupRepo     *db.GroupRepo
    inviteRepo    *db.InviteRepo
    choreRepo     *db.ChoreRepo
    ledgerRepo    *db.LedgerRepo
    loanRepo      *db.LoanRepo
    allowanceRepo *db.AllowanceRepo
    postingSvc    *posting.Service
    pool          *pgxpool.Pool
}
```

Update `NewGroupHandler(...)` and its call site in `main.go` accordingly. (If WP-4.2 already widened
this signature, add to it — do not fork.) Add the route:
`protected.DELETE("/groups/:id/members/:userId", groupHandler.RemoveMember)`.

**New repo methods (all take the tx `db.Querier` so they run under the removal transaction):**

`GroupRepo` (`group_repo.go`):
```go
// LockMembershipForUpdate reads the target's membership row FOR UPDATE, returning
// its role. ErrNotFound when the user is not a member. The row lock is the tx's
// serialization anchor for this member.
func (r *GroupRepo) LockMembershipForUpdate(ctx context.Context, q Querier, groupID, userID uuid.UUID) (models.MemberRole, error) {
    var role models.MemberRole
    err := q.QueryRow(ctx,
        `SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2 FOR UPDATE`,
        groupID, userID).Scan(&role)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return "", ErrNotFound
        }
        return "", fmt.Errorf("failed to lock membership: %w", err)
    }
    return role, nil
}

// DeleteMembership removes the group_members row. Ledger/loan/allowance rows are
// unaffected (they FK users(id), not group_members) — history is preserved.
func (r *GroupRepo) DeleteMembership(ctx context.Context, q Querier, groupID, userID uuid.UUID) error {
    _, err := q.Exec(ctx,
        `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
    if err != nil {
        return fmt.Errorf("failed to delete membership: %w", err)
    }
    return nil
}
```

`LoanRepo` (`loan_repo.go`) — note the `FOR UPDATE`, and see the lock-order note below:
```go
// LockBlockingLoans locks the member's non-terminal loans (requested|active) FOR
// UPDATE and returns how many there are. The lock serializes against a concurrent
// PostDue that would FOR UPDATE the same active loans before posting EMIs (§4.5,
// WP-3.1). Ordered by id — a single-user subset, so it cannot form a lock cycle
// with PostDue's group-wide (user_id, start_period, id) order (see note).
func (r *LoanRepo) LockBlockingLoans(ctx context.Context, q Querier, groupID, userID uuid.UUID) (int, error) {
    rows, err := q.Query(ctx,
        `SELECT id FROM loans
         WHERE group_id = $1 AND user_id = $2 AND status IN ('requested','active')
         ORDER BY id
         FOR UPDATE`, groupID, userID)
    if err != nil {
        return 0, fmt.Errorf("failed to lock blocking loans: %w", err)
    }
    defer rows.Close()
    n := 0
    for rows.Next() {
        n++
    }
    return n, rows.Err()
}
```
> `LoanRepo` methods take `q Querier` and it has no `.Query` on the `Querier` interface today —
> `Querier` currently exposes only `Exec` + `QueryRow` (`ledger_repo.go:20`). **Add `Query` to the
> `Querier` interface** (both `*pgxpool.Pool` and `pgx.Tx` already satisfy it):
> `Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)`. This is a one-line,
> backward-compatible interface widening; note it in the PR.

`LedgerRepo` (`ledger_repo.go`):
```go
// MemberBalanceTx computes one member's balance (Σ approved credits − Σ approved
// debits, ::bigint) on the tx querier — identical math to GetBalanceForGroup.
func (r *LedgerRepo) MemberBalanceTx(ctx context.Context, q Querier, groupID, userID uuid.UUID) (int64, error) {
    var balance int64
    err := q.QueryRow(ctx, `
        SELECT (COALESCE(SUM(CASE WHEN direction = 'credit' THEN amount ELSE 0 END), 0)
              - COALESCE(SUM(CASE WHEN direction = 'debit'  THEN amount ELSE 0 END), 0))::bigint
        FROM ledger_entries
        WHERE group_id = $1 AND user_id = $2 AND status = 'approved'`,
        groupID, userID).Scan(&balance)
    if err != nil {
        return 0, fmt.Errorf("failed to compute member balance: %w", err)
    }
    return balance, nil
}

// RejectPendingForMember flips all of a member's pending_approval entries to
// rejected in this group, recording the actor. Pending entries never counted toward
// balance, so this does not change any total; it prevents a post-removal "ghost
// approval" (D7). Returns rows affected (informational).
func (r *LedgerRepo) RejectPendingForMember(ctx context.Context, q Querier, groupID, userID, decidedBy uuid.UUID, decidedAt time.Time) (int64, error) {
    tag, err := q.Exec(ctx, `
        UPDATE ledger_entries
        SET status = 'rejected', decided_by = $3, decided_at = $4
        WHERE group_id = $1 AND user_id = $2 AND status = 'pending_approval'`,
        groupID, userID, decidedBy, decidedAt)
    if err != nil {
        return 0, fmt.Errorf("failed to reject pending entries: %w", err)
    }
    return tag.RowsAffected(), nil
}
```

`AllowanceRepo` (`allowance_repo.go`):
```go
// DeleteForMember removes a member's allowance config rows. Safe: nothing FKs
// allowances. Already-posted allowance ledger entries (history) are untouched (D7).
func (r *AllowanceRepo) DeleteForMember(ctx context.Context, q Querier, groupID, userID uuid.UUID) error {
    _, err := q.Exec(ctx,
        `DELETE FROM allowances WHERE group_id = $1 AND user_id = $2`, groupID, userID)
    if err != nil {
        return fmt.Errorf("failed to delete allowances for member: %w", err)
    }
    return nil
}
```

**Handler sketch — `backend/internal/handlers/groups.go`:**

```go
// RemoveMember handles DELETE /api/v1/groups/:id/members/:userId (head removes a
// member; member leaves self). See WP-4.7 §4.2.
func (h *GroupHandler) RemoveMember(c *gin.Context) {
    callerIDStr, exists := auth.GetUserID(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
        return
    }
    callerID, err := uuid.Parse(callerIDStr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
        return
    }
    groupID, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
        return
    }
    targetID, err := uuid.Parse(c.Param("userId"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
        return
    }

    // Caller must be a member; get their role for the authz decision.
    caller, err := h.groupRepo.GetMember(c.Request.Context(), groupID, callerID)
    if err != nil {
        if errors.Is(err, db.ErrNotFound) {
            c.JSON(http.StatusForbidden, gin.H{"error": "not a member of this group"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
        return
    }

    // Authz (§5.5): head may remove others; anyone may remove self; a non-head may
    // NOT remove another member.
    isSelf := targetID == callerID
    if !isSelf && caller.Role != models.RoleHead {
        c.JSON(http.StatusForbidden, gin.H{"error": "only the group head can remove other members"})
        return
    }

    // D3: post any due allowance/EMI FIRST (own tx), so the balance below is current
    // and a stale un-posted entry cannot let a non-zero member out.
    if !runPosting(c, h.postingSvc, groupID) {
        return
    }

    ctx := c.Request.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
        return
    }
    defer tx.Rollback(ctx) //nolint:errcheck

    // Lock the target's membership (anchor); confirms they are a member.
    role, err := h.groupRepo.LockMembershipForUpdate(ctx, tx, groupID, targetID)
    if err != nil {
        if errors.Is(err, db.ErrNotFound) {
            c.JSON(http.StatusNotFound, gin.H{"error": "user is not a member of this group"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lock membership"})
        return
    }
    // D6: the head can neither leave nor be removed.
    if role == models.RoleHead {
        c.JSON(http.StatusConflict, gin.H{"error": "the group head cannot leave or be removed"})
        return
    }

    // D5: block on any requested/active loan (FOR UPDATE serializes vs PostDue EMI posting).
    blocking, err := h.loanRepo.LockBlockingLoans(ctx, tx, groupID, targetID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check loans"})
        return
    }
    if blocking > 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "cannot remove a member who has an active or pending loan; close or reject it first"})
        return
    }

    // D4: balance must be exactly zero (same math as GetBalanceForGroup).
    balance, err := h.ledgerRepo.MemberBalanceTx(ctx, tx, groupID, targetID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check balance"})
        return
    }
    if balance != 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "cannot remove a member whose balance is not settled to zero; settle their balance first"})
        return
    }

    // D7: reject the member's pending entries, delete their allowance config, delete
    // the membership. Ledger history (approved/rejected) and closed/rejected loans stay.
    now := time.Now()
    if _, err := h.ledgerRepo.RejectPendingForMember(ctx, tx, groupID, targetID, callerID, now); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject pending entries"})
        return
    }
    if err := h.allowanceRepo.DeleteForMember(ctx, tx, groupID, targetID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete allowances"})
        return
    }
    if err := h.groupRepo.DeleteMembership(ctx, tx, groupID, targetID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
        return
    }

    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
        return
    }
    c.Status(http.StatusNoContent) // 204
}
```

**Lock-order note (WP-3.1 conventions).** `PostDue` locks a group's active loans `FOR UPDATE` in
`(user_id, start_period, id)` order (`loan_repo.go:168`, `engine.go`). `RemoveMember` locks only the
**target's** membership row and the **target's** loans (a single-user subset, ordered by `id`) and
then only **reads** ledger rows (no `FOR UPDATE`). Because the removal transaction:
1. touches exactly one member's rows (no cross-member ordering that could invert PostDue's), and
2. acquires all its locks before any blocking read, and
3. never waits on a lock a concurrent PostDue holds *after* PostDue would wait on ours,

there is **no lock cycle** with a concurrent PostDue. The passing path has **zero** active/pending
loans → PostDue posts no EMI for this member → the post-lock balance is stable. If a loan *were*
active, we'd 409 before deleting anyway. Deadlock-free, and the `FOR UPDATE` guarantees a concurrent
EMI post can't slip a debit in between the balance read and the delete. Keep the `ORDER BY id` and the
`FOR UPDATE` — do not drop either.

**Accepted, documented edge:** a concurrent *head* settlement/adjustment committing between the
balance read and the commit could leave a non-zero balance on a just-removed member. This requires the
head to perform two conflicting actions at once and is out of scope to lock against (it would mean
locking the whole member's ledger). Note it in the PR; do not build for it.

---

## 5. Design decisions — reasoning (the "why", expanded)

**D1 — no JWT rotation on password change.** Tokens are stateless JWTs (`auth.IssueToken`); there is
no session/revocation store. Invalidating "other sessions" would require a token blacklist or a
per-user token version column — real scope we're not taking for a LAN v2. The caller keeps their
current token (still valid); a compromised-token scenario is out of the launch threat model. Document
the limitation; the reset-password backlog item (§8.9) is where session hardening belongs.

**D2 — one `DELETE` endpoint for remove + leave.** Both operations are "delete this membership under
the same hygiene rules"; only the authz predicate differs (self vs head-over-other). A separate
`POST /groups/:id/leave` would duplicate the `PostDue` + transaction + balance/loan checks + delete,
inviting drift between two copies of the most invariant-sensitive code in this WP. REST-wise, deleting
a member sub-resource by id is the natural verb, and it's consistent with the existing
`/groups/:id/members` (GET) and `/groups/:id/allowances/:userId` (PUT) shapes. The only cost — the FE
must pass its own `user.id` to leave — is trivial (it always has it). Chosen.

**D3 — `PostDue` before the check.** This is §5.4 made concrete: the ledger is only correct *after*
lazy posting. Scenario the test in §6 pins: a member settled to ₹0 in June; July rolls over; they have
a ₹500/mo allowance that hasn't been posted because nobody opened the group in July. Without `PostDue`
the in-tx balance reads 0 → removal succeeds → the member leaves owed ₹500 that never posts. Running
`PostDue` first posts the July allowance credit → balance ₹500 → 409. Correctness hinges on this
ordering.

**D4 — balance math identical to the endpoint.** Using a *different* SUM (e.g. forgetting the
`::bigint` cast, or filtering by a chores join, or counting pending) would let the removal disagree
with what the head sees on the balance screen — a trust-breaking inconsistency in a money app. Reuse
the exact `Σcredit−Σdebit over approved, ::bigint` formula (`GetBalanceForGroup`).

**D5 — `requested` blocks too.** `active` obviously blocks (outstanding EMIs = a live debt). A
`requested` loan is a pending obligation the *head* still controls: if we let the member leave with a
`requested` loan sitting there, the head could later `POST /loans/:id/approve` it → status `active`,
`start_period` set → `PostDue` then posts EMI debits against a user who is **no longer a member** (the
posting engine's allowance join filters on membership, but EMI posting reads `loans` directly and does
not re-check membership — `loan_repo.go:168`). That's a ghost-debt bug. Requiring the head to reject
(or approve-then-close) the request first is the clean gate. `rejected`/`closed` are terminal and
carry no future posting → they do **not** block, and their ledger rows stay as history.

**D6 — head is immovable.** No headship transfer exists (§8.7), so a group has exactly one head with
no way to appoint another. Allowing the head to leave/be-removed would orphan the group (no head to
approve chores, set allowances, record settlements) — a broken state with no recovery path in v2.
`409` with "the group head cannot leave or be removed" is honest and final. (When transfer/delete
ships, this guard relaxes.)

**D7 — allowances deleted, pending auto-rejected, ledger kept.** Three different lifetimes:
- *Ledger history* (`approved`/`rejected`) is the product's core promise — an immutable audit trail.
  It stays; the FK is to `users(id)`, and the user still exists, so nothing cascades it away.
- *Allowance rows* are pure forward-looking config. If left behind, they become a latent trap: should
  the same user ever rejoin, `ListPostingInputs` would re-see the old rows and `PostDue` would
  backfill allowance from the (new) join month at the stale amount — a silent "ghost allowance". They
  are referenced by nothing, so deleting them is safe and removes the trap. (Posted allowance *ledger*
  entries are unaffected — those are history, kept.)
- *Pending entries* never counted toward balance, but `POST /ledger/:id/approve` does **not** re-check
  that the entry's user is still a member (`ledger.go:432`). A pending chore left behind could be
  approved later, crediting a non-member. Auto-rejecting them in the same tx closes that hole; reject
  is a normal, allowed decision transition (not a mutation of an immutable field) and leaves a clean
  terminal audit row.

---

## 6. Backend integration tests (`groups_integration_test.go`, `auth_integration_test.go`)

Run only in CI (Docker; §10). Follow the existing harness (`setupLoanTestEnv` pattern): after
`GroupRepo.Create`, **`AddMember(head, RoleHead)` explicitly** — `Create` does not add the head
(project memory / WP-1.2). For the stale-posting test, **backdate `group_members.joined_at`** via a
direct `UPDATE` (allowance backfill floors at join month; `AddMember` stamps `now()` — project
memory). Register the routes under test on the harness router (`PUT /auth/password`,
`DELETE /groups/:id/members/:userId`, plus `GET /groups/:id/balance` for assertions).

**Change password:**
1. **Wrong current password → 403** (assert **status 403, not 401** — the FE-logout guard), body
   `error` non-empty; the stored hash is unchanged (old password still logs in).
2. **Correct change → 204**, then `POST /auth/login` with the **new** password succeeds and with the
   **old** password returns 401.
3. **New password too short (< 6) → 400** (binding `min=6`); hash unchanged.
4. **Unauthenticated (no/invalid token) → 401.**

**Removal authz (§5.5):**
5. **Head removes a settled member → 204.** Precondition: target balance 0 (e.g. a credit fully
   offset by a settlement debit), no loans. After: `group_members` row gone; the member's **ledger
   rows still present** (assert the count is unchanged — history kept); the member's **allowance rows
   deleted**.
6. **Member leaves self (settled) → 204** (caller == target, non-head).
7. **Member A removes member B → 403** ("only the group head…").
8. **Non-member caller → 403.**
9. **Target userId not a member → 404.**
10. **Head removes self / head leaves → 409** (D6), message names the head rule; membership intact.

**Blocked removal (each 409 with the right human-readable message; membership + data intact after):**
11. **Non-zero balance** (one approved credit, no offsetting debit) → 409 balance message.
12. **Active loan** (`status='active'`) → 409 loan message.
13. **Requested loan** (`status='requested'`) → 409 loan message (D5 — requested blocks).
14. **Fresh-unposted-allowance case (D3 / §5.4) — the important one.** Member settled to 0 on
    already-posted entries; give them an in-force allowance (`amount>0`, `effective_from` a prior
    month) and **backdate `joined_at`** so a due allowance exists for a rolled-over month that has
    **not** been posted yet (no group/ledger/balance read has happened). Call `DELETE …/members/…`.
    Assert: **409** (balance became non-zero), **and** a new `allowance` ledger row now exists for the
    due period (proving the handler ran `PostDue` before checking). This is the case a naive
    implementation (check balance without posting first) gets wrong.
15. **Leave-self blocked by non-zero balance → 409** (member tries to leave while owed money).
16. **Pending auto-reject (D7).** Removed member had a `pending_approval` chore entry; after a
    successful removal the entry's status is `rejected` (not left pending), and `decided_by` = the
    actor. (Balance was 0 because pending never counted.)

Optional but recommended: a concurrency smoke test (mirror the loan suite's `sync.WaitGroup` style) —
a `RemoveMember` racing a `PostDue` on the same group does not deadlock and ends in a consistent state
(either removed-with-nothing-due, or 409). Not required for acceptance; the lock reasoning in §4.2 is
the primary guarantee.

---

## 7. Frontend plan

### 7.0 Codegen STOP-gate (BEFORE any FE code)

After the openapi delta (§2) and BE build, run in `app/`:
```bash
npm run codegen        # regenerates app/src/api-types.gen.ts from openapi.yaml
```
Verify `components['schemas']['ChangePasswordRequest']` exists (with `current_password`,
`new_password`) in `api-types.gen.ts`, and **commit the regenerated file**. Do **not** hand-write the
type. `npm run codegen:check` must be clean. (The two endpoints' 204 responses generate no response
type; that's expected — the FE calls use `request<void>`.)

### 7.1 Types & data layer (`app/src/api.ts`, additive — do not touch `groupsApi.list`)

```ts
export type ChangePasswordRequest = Schemas['ChangePasswordRequest'];

// authApi (add):
changePassword: (data: ChangePasswordRequest) =>
  request<void>('/auth/password', { method: 'PUT', body: JSON.stringify(data) }),

// groupsApi (add): serves both "head removes member" and "member leaves" (caller passes own id).
removeMember: (groupId: string, userId: string) =>
  request<void>(`/groups/${groupId}/members/${userId}`, { method: 'DELETE' }),
```
`request` already returns `undefined` for 204 and throws `new Error(error.error)` for any non-ok
(so the FE gets the server's human-readable 409/403 message as `err.message`).

### 7.2 Hooks (`app/src/hooks/`)

- `useChangePassword()` — `useMutation({ mutationFn: authApi.changePassword })`. **No invalidation**
  (nothing cached changes; JWT unchanged, D1).
- `useRemoveMember(groupId)` — `useMutation({ mutationFn: (userId) => groupsApi.removeMember(groupId, userId), onSuccess: … })`.
  Invalidate: `qk.group(groupId)` (detail incl. member list), `qk.members(groupId)`,
  `qk.balance(groupId)`, `qk.loans(groupId)`, and `qk.groups()` (dashboard head-total / member-count).
- `useLeaveGroup(groupId)` — same mutation shape (call `removeMember(groupId, ownId)`), but
  `onSuccess` invalidates `qk.groups()` and, since the caller loses access to the group, also
  `qc.removeQueries({ queryKey: qk.group(groupId) })` before navigating away (avoid a 403 refetch on a
  cached detail screen).

**Invalidation matrix:**

| Action | invalidate | navigate |
|---|---|---|
| Change password | — (none) | stay on profile; close Sheet |
| Remove member (head) | `group(id)`, `members(id)`, `balance(id)`, `loans(id)`, `groups()` | `router.back()` to group overview |
| Leave group (member) | `groups()`; `removeQueries(group(id))` | `router.replace('/(app)')` (dashboard) |

### 7.3 `profile.tsx` — change-password Sheet

`profile.tsx` is already on the kit (WP-4.1). Add a `Button title="Change password"
variant="secondary"` (above Logout) that opens a `<Sheet visible title="Change password">`
containing three `<TextField secureTextEntry>`: **Current password**, **New password**, **Confirm new
password**, and a primary `<Button title="Change password" loading={mutation.isPending}>` + a
ghost/secondary Cancel.

- **Client validation before submit:** all three non-empty; `new === confirm` (else danger Toast
  "New passwords don't match"); `new.length >= 6` (else "New password must be at least 6 characters");
  optionally `new !== current`. Disable the submit button while pending.
- **Submit:** `useChangePassword().mutateAsync({ current_password, new_password })`. Success → Toast
  `{ tone:'success', message:'Password changed' }`, close Sheet, **reset all three fields**. Error →
  Toast `{ tone:'danger', message: err instanceof Error ? err.message : 'Failed to change password' }`
  — e.g. "current password is incorrect" (403; session preserved, no logout). **No `Alert.alert`.**
- **Sheet field reset** on close (Cancel *and* success) — match the established reset pattern so a
  reopened Sheet isn't pre-filled with a password. Hooks (`useToast`, `useState`, the mutation) above
  any early return.

### 7.4 `members/[userId].tsx` — "Remove member" (head only)

This head-only detail screen already imports `confirmAsync`, `useToast`, Sheets. Add a **danger**
`<Button title="Remove member" variant="danger">` at the bottom of the head view, rendered **only**
when the viewed member is **not** the head (`member.role !== 'head'`) and the viewer is head
(`isHead`, already computed).

- **Press →** `confirmAsync({ title: 'Remove member', message: 'Remove <name> from the group? Their
  ledger history is kept. This only works if their balance is ₹0 and they have no active or pending
  loans.', confirmLabel: 'Remove', destructive: true })`. If not confirmed, no-op.
- **Confirmed →** `useRemoveMember(groupId).mutateAsync(userId)`. Success → Toast `{ tone:'success',
  message:'<name> removed' }`, then `router.back()` (to the group overview). Error → Toast
  `{ tone:'danger', message: err.message }` — the server's human-readable 409 (e.g. "cannot remove a
  member whose balance is not settled to zero…") lands verbatim. **No `Alert.alert`.**
- Do not also add a Remove control to the overview `MemberCard` — the member-detail page is the single
  natural surface (keeps the overview file, which WP-4.3 touches, out of this WP).

### 7.5 `groups/[id]/index.tsx` — "Leave group" (member view)

In the **member-role** render branch (the member's own summary view — *not* the head branch WP-4.3
hardened), add a **danger** `<Button title="Leave group" variant="danger">` (e.g. under the member's
summary/ledger header).

- **Press →** `confirmAsync({ title: 'Leave group', message: 'Leave <group name>? You can rejoin with
  a new invite. This only works if your balance is ₹0 and you have no active or pending loans.',
  confirmLabel: 'Leave', destructive: true })`.
- **Confirmed →** `useLeaveGroup(groupId).mutateAsync(user.id)`. Success → Toast `{ tone:'success',
  message:'You left the group' }`, `router.replace('/(app)')`. Error → Toast `{ tone:'danger',
  message: err.message }` (the 409 block reason). **No `Alert.alert`.**
- **Rebase:** take WP-4.3's merged `groups/[id]/index.tsx` as the base; the Invite hardening lives in
  the head branch and `handleInvite`, disjoint from this member-branch Button. Hooks above early
  returns.

---

## 8. Verification (Definition of Done, §10 rule 3)

### Backend (from repo root / `backend/`)
```bash
gofmt -l backend/internal                      # prints NOTHING
go build ./...                                 # compiles (incl. the widened Querier interface)
go vet ./...                                    # clean
go test -race ./...                             # unit tests pass
# Integration needs Postgres (Docker) → runs in CI. Locally, at minimum COMPILE them:
go test -tags=integration -run xxxNONExxx ./backend/internal/handlers/... ./backend/internal/db/...
```
The `-tags=integration` compile-check (a run matching no test) proves the integration files build
without Docker (project convention — integration executes only in CI). The §6 cases must pass in CI.

### Frontend (from `app/`)
```bash
npm run codegen:check     # generated types in sync with openapi (STOP-gate §7.0)
npx tsc --noEmit          # zero type errors
npx expo export --platform web    # MANDATORY — must exit 0 (tsc/expo-doctor miss bundle breakage; WP-0.1 precedent)
npm run lint              # no new issues
grep -rn "Alert\.alert" app/app --include='*.tsx'   # prints NOTHING (Toast/confirmAsync only)
```
State results in the PR. If the environment can boot `npm run web`: profile → Change password (wrong
current → inline error, **still logged in**; correct → success, re-login with new works); head member
detail → Remove (blocked message when balance≠0 / loan active; success when settled → returns to
overview, member gone, history intact); member view → Leave (blocked vs success → lands on dashboard,
group gone from list).

---

## 9. Acceptance criteria (Definition of Done)

1. `openapi.yaml`: `PUT /auth/password` (204; 403 for wrong current password) and
   `DELETE /groups/{id}/members/{userId}` (204; 403 authz; 404 non-member; 409 head/balance/loan)
   added; `ChangePasswordRequest` schema added. **No migration file.**
2. Change password verifies the current password with bcrypt, stores a new bcrypt hash, returns 204,
   and **does not rotate the JWT**; a wrong current password returns **403, not 401**.
3. Removal/leave is **one** transactional endpoint that: runs `PostDue` first (D3); recomputes balance
   with the `GetBalanceForGroup` math (D4); blocks on `requested|active` loans (D5); refuses the head
   (D6); deletes allowances + auto-rejects pending + deletes membership while **keeping ledger/loan
   history** (D7); all under one tx with `FOR UPDATE` locks per WP-3.1 lock order.
4. AuthZ per §5.5: only the head removes others (403 otherwise); anyone leaves self; non-member 403;
   non-member target 404. Blocks return **409** with a human-readable message.
5. `npm run codegen` regenerated + committed; `codegen:check` clean; the change-password type is
   generated, not hand-written.
6. FE: change-password Sheet on `profile.tsx`; "Remove member" on `members/[userId].tsx`; "Leave
   group" on the member view of `groups/[id]/index.tsx`; all destructive actions use `confirmAsync`;
   outcomes via `Toast`; invalidation matrix (§7.2) applied; **no `Alert.alert`**, no inline hex.
7. Verification (§8) all green: gofmt/build/vet/test -race + integration compile-check; codegen:check
   + tsc + `expo export --platform web` exit 0; no `Alert.alert` under `app/app`.
8. Integration cases §6 pass in CI (incl. the fresh-unposted-allowance D3 case, the 403-not-401
   wrong-password case, requested-loan block, head-cannot-leave, pending auto-reject).
9. Commit/PR reference the WP id (`WP-4.7: account & membership hygiene`).

---

## 10. Gotchas & likely-slips (quick reference)

- **G1 — Wrong current password must be 403, NOT 401.** A 401 trips the FE client's global
  logout/redirect (`api.ts:51`) and ends the session mid-change. (§2.1, §4.1)
- **G2 — `PostDue` before the balance check, always.** Skip it and a stale un-posted allowance/EMI
  lets a non-zero member out (§5.4 / D3). This is the single highest-value correctness step; test §6.14
  pins it.
- **G3 — No migration.** Every field is an existing column; no FK targets `group_members`, so deleting
  it keeps history automatically (§3). Reaching for `ALTER TABLE` = drift.
- **G4 — `requested` blocks removal too** (D5) — not just `active`. A dangling requested loan can be
  approved later into ghost EMIs.
- **G5 — Balance math must equal `GetBalanceForGroup`**: `Σcredit−Σdebit` over `approved`,
  `::bigint`-cast SUMs, int64. Don't drop the cast (pgx scans `numeric` otherwise — WP-1.1 gotcha).
- **G6 — Keep the `FOR UPDATE` + `ORDER BY id`** on the loan lock, and run all repo calls on the tx
  `q`, not the pool. That's what serializes against a concurrent `PostDue` EMI post (§4.2 lock note).
- **G7 — Widen the `Querier` interface** to add `Query(...) (pgx.Rows, error)` for `LockBlockingLoans`
  (it currently exposes only `Exec`/`QueryRow`). Backward-compatible; note in PR.
- **G8 — 204 endpoints return no body.** `c.Status(http.StatusNoContent)`, not `c.JSON`. FE uses
  `request<void>` (client already handles 204).
- **G9 — Extend `NewGroupHandler`, don't fork it.** It needs `ledgerRepo`, `loanRepo`, `allowanceRepo`,
  `pool` added. Reconcile with WP-4.2 if it already changed the signature; update `main.go` once.
- **G10 — Integration harness:** `AddMember(head, RoleHead)` explicitly after `Create`; backdate
  `group_members.joined_at` for the D3 allowance test; Docker/integration runs in CI only.
- **G11 — FE surfaces don't collide with 4.2/4.3:** Remove lives on `members/[userId].tsx` (nobody
  else touches it); Leave lives in the **member branch** of `groups/[id]/index.tsx` (WP-4.3 hardens the
  **head** Invite there); WP-4.7 never edits the dashboard (`app/(app)/index.tsx`) — it only
  invalidates `qk.groups()`.
- **G12 — Sheet field reset + hooks above early returns + Toast (not Alert.alert)** — the standing kit
  rules; reset the password fields on Sheet close so a reopen isn't pre-filled.
- **G13 — Head-invariant is final in v2.** No transfer/delete exists; the 409 on head removal is
  correct, not a TODO. Leave a `docs/notes/` breadcrumb pointing at backlog §8.7 (transfer) / group
  delete as the future relaxation.
```
