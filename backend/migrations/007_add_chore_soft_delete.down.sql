-- Remove soft delete and system chore support from chores table
DROP INDEX IF EXISTS idx_chores_active;
ALTER TABLE chores DROP COLUMN IF EXISTS is_system;
ALTER TABLE chores DROP COLUMN IF EXISTS deleted_at;
