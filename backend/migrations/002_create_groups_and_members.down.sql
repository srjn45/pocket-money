-- Drop indexes
DROP INDEX IF EXISTS idx_group_members_user_id;
DROP INDEX IF EXISTS idx_groups_head_user_id;

-- Drop tables
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;

-- Drop enum type
DROP TYPE IF EXISTS member_role;
