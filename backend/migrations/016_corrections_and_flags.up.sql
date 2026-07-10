-- V3-3.2: (1) entry_audit — invisible prior-value log for manual-entry
-- edits/deletes (D3, §3.5). (2) groups.member_chore_submission_enabled — the D2
-- gate for the member chore-submission workflow, default OFF.
-- NOTE: chores.description is NOT added here — it already exists (migration 003).

-- (1) entry_audit. old_row is the JSON snapshot of the ledger_entries row taken
-- BEFORE the edit/delete. entry_id uses ON DELETE SET NULL so a hard delete of the
-- parent entry succeeds while the audit row (and its old_row snapshot, which holds
-- the original id) survives. No read API in MVP.
CREATE TABLE IF NOT EXISTS entry_audit (
    id       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id uuid        NULL REFERENCES ledger_entries(id) ON DELETE SET NULL,
    old_row  jsonb       NOT NULL,
    action   text        NOT NULL CHECK (action IN ('edit', 'delete')),
    actor    uuid        NOT NULL REFERENCES users(id),
    at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_entry_audit_entry ON entry_audit (entry_id);

-- (2) D2 flag. Default OFF so existing groups keep today's admin-only ledger
-- behavior; a member can submit chores only when an admin turns this on.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS member_chore_submission_enabled boolean NOT NULL DEFAULT false;
