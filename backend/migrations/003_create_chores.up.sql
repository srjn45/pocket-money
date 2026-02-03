-- Create chores table
CREATE TABLE chores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    amount DECIMAL(12, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index on group_id for faster lookups
CREATE INDEX idx_chores_group_id ON chores(group_id);
