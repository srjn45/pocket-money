-- Reverse V3-2.2 renames; restores the exact pre-015 schema, zero data loss.
ALTER INDEX IF EXISTS idx_groups_admin_user_id RENAME TO idx_groups_head_user_id;
ALTER TABLE groups RENAME COLUMN admin_user_id TO head_user_id;
ALTER TYPE member_role RENAME VALUE 'admin' TO 'head';
