-- Recurring monthly pocket money (§5.2). One row per (member, effective_from);
-- a change is a NEW row with a later effective_from (history preserved, past
-- months never rewritten). amount is minor units (paise), >= 0; 0 = paused.
CREATE TABLE allowances (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id       UUID   NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id        UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    amount         BIGINT NOT NULL CHECK (amount >= 0),   -- minor units; 0 = paused
    effective_from CHAR(7) NOT NULL,                      -- 'YYYY-MM', monthly-only
    created_by     UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id, effective_from)            -- upsert key (§5)
);

-- Posting engine reads all allowance rows for a group's members.
CREATE INDEX idx_allowances_group_user ON allowances (group_id, user_id);
