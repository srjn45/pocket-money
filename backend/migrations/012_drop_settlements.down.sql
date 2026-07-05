-- Recreate the settlements table + indexes (mirror of migration 005) so the schema
-- round-trips. The up-migration's data move is NOT reversed (see note in spec).
CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,          -- BIGINT: migration 008 already converted this column type
    date DATE NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_settlements_group_id ON settlements(group_id);
CREATE INDEX idx_settlements_user_id ON settlements(user_id);
