DROP INDEX IF EXISTS idx_ledger_entries_group_type;
DROP INDEX IF EXISTS idx_ledger_entries_posting_unique;

-- Restore the approve/reject columns and reconstruct them from decided_by + status.
ALTER TABLE ledger_entries
    ADD COLUMN approved_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN rejected_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
UPDATE ledger_entries SET approved_by_user_id = decided_by WHERE status = 'approved';
UPDATE ledger_entries SET rejected_by_user_id = decided_by WHERE status = 'rejected';

-- Re-point backfilled settlement rows at the group's system chore so NOT NULL can be restored.
UPDATE ledger_entries le
SET chore_id = (SELECT id FROM chores c WHERE c.group_id = le.group_id AND c.is_system = true LIMIT 1)
WHERE le.chore_id IS NULL AND le.entry_type = 'settlement';

ALTER TABLE ledger_entries
    DROP COLUMN decided_at,
    DROP COLUMN decided_by,
    DROP COLUMN note,
    DROP COLUMN period,
    DROP COLUMN loan_id,
    DROP COLUMN direction,
    DROP COLUMN entry_type;

-- Restore NOT NULL (succeeds on the round-trip test's fresh DB, and on any data
-- that predates allowance/emi/adjustment rows — see reversibility note in spec).
ALTER TABLE ledger_entries ALTER COLUMN chore_id SET NOT NULL;

DROP TYPE IF EXISTS ledger_direction;
DROP TYPE IF EXISTS ledger_entry_type;
