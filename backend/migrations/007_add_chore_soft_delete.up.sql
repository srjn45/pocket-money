-- Add soft delete and system chore support to chores table
ALTER TABLE chores ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE chores ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT false;

-- Index for efficient filtering of non-deleted chores
CREATE INDEX idx_chores_active ON chores(group_id) WHERE deleted_at IS NULL;
