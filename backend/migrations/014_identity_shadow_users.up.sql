-- V3-2.1: shadow users (nullable password + status + claimed_at) and the
-- notifications table (§3.1, §3.7). notifications is INSERT-ONLY in this WP;
-- the list/unread/mark-read READ API is Phase 5 (V3-5.1).

-- password_hash becomes NULLABLE. NULL ⇔ shadow user (cannot authenticate).
-- DROP NOT NULL is idempotent (no-op if already nullable).
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- status: 'shadow' | 'registered'. Existing rows backfill to 'registered' via
-- the DEFAULT (they all have a password already). Keep the DEFAULT: 'registered'
-- is the safe common case and existing INSERTs that omit status stay correct;
-- shadow creation sets status explicitly.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'registered'
        CHECK (status IN ('shadow', 'registered'));

-- when a shadow row was claimed by registration (§3.1). NULL for never-claimed.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS claimed_at timestamptz NULL;

-- In-app notifications (§3.7). INSERT-ONLY here; read API is Phase 5.
CREATE TABLE IF NOT EXISTS notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    read_at    timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Read API (Phase 5) will page a user's notifications newest-first; index now.
CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications (user_id, created_at DESC);
