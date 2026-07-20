DROP INDEX IF EXISTS idx_groups_live;
ALTER TABLE groups DROP COLUMN IF EXISTS deleted_at;
