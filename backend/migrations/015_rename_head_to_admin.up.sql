-- V3-2.2: rename per-membership role 'head'->'admin' and group owner column
-- groups.head_user_id -> groups.admin_user_id (§3.2/§4/§6 Naming). D6 authz
-- scoping is code-only; this migration is the naming half. Reversible, no data
-- loss. golang-migrate applies 015 exactly once (version-tracked).

-- 1. Rename the enum label in place. Transactional; rewrites every existing
--    group_members.role='head' row to 'admin' atomically (no UPDATE needed).
ALTER TYPE member_role RENAME VALUE 'head' TO 'admin';

-- 2. Rename the group-owner column + its index to match the role vocabulary.
ALTER TABLE groups RENAME COLUMN head_user_id TO admin_user_id;
ALTER INDEX IF EXISTS idx_groups_head_user_id RENAME TO idx_groups_admin_user_id;
