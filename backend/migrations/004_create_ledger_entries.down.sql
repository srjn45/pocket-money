-- Drop indexes
DROP INDEX IF EXISTS idx_ledger_entries_user_id;
DROP INDEX IF EXISTS idx_ledger_entries_group_status;
DROP INDEX IF EXISTS idx_ledger_entries_group_id;

-- Drop ledger_entries table
DROP TABLE IF EXISTS ledger_entries;

-- Drop enum type
DROP TYPE IF EXISTS ledger_status;
