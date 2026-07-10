DROP INDEX IF EXISTS idx_entry_audit_entry;
DROP TABLE IF EXISTS entry_audit;
ALTER TABLE groups DROP COLUMN IF EXISTS member_chore_submission_enabled;
