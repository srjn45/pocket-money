-- Drop indexes
DROP INDEX IF EXISTS idx_invite_tokens_group_id;
DROP INDEX IF EXISTS idx_invite_tokens_token;

-- Drop invite_tokens table
DROP TABLE IF EXISTS invite_tokens;
