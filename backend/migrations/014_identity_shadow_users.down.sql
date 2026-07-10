-- Reverse of 014. DROP COLUMN cascades the inline CHECK on status.
DROP INDEX IF EXISTS idx_notifications_user_created;
DROP TABLE IF EXISTS notifications;

ALTER TABLE users DROP COLUMN IF EXISTS claimed_at;
ALTER TABLE users DROP COLUMN IF EXISTS status;

-- Restore the original NOT NULL. This is a dev/test teardown path
-- (TestMigrations_UpAndDown rolls the whole chain down on an empty DB); it
-- assumes no shadow rows (password_hash NULL) remain, which holds there.
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
