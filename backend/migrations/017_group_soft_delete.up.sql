-- QA batch 1, Item 1: soft-delete (archive) for groups. An admin can delete a
-- group; deletion is recoverable-in-principle at the DB level (deleted_at set to
-- now()), not a hard DELETE. There is intentionally NO restore endpoint/UI in
-- this pass — recoverability means the row and all its history survive.
--
-- Default NULL so every existing group is treated as live. All GroupRepo reads
-- filter `deleted_at IS NULL`, so a deleted group vanishes from the dashboard,
-- the group list, and direct fetch-by-id (which then 404s).
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL DEFAULT NULL;

-- Partial index over live groups keeps the common `deleted_at IS NULL` filter cheap.
CREATE INDEX IF NOT EXISTS idx_groups_live ON groups (id) WHERE deleted_at IS NULL;
