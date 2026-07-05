-- Migrate legacy settlements into the unified ledger as approved settlement debits.
-- The settlements table has no created_by, so we attribute the payout to the group head
-- (settlements were always head-only actions). We set:
--   created_at = the payout DATE (so it groups under the month it happened, matching
--                the old settlements-by-date UI), decided_at = the original row's created_at.
INSERT INTO ledger_entries
    (id, group_id, user_id, chore_id, amount, status, entry_type, direction,
     note, created_by_user_id, decided_by, decided_at, created_at)
SELECT
    gen_random_uuid(),
    s.group_id,
    s.user_id,
    NULL,                 -- settlement entries carry no chore_id
    s.amount,             -- already BIGINT minor units (migration 008)
    'approved',
    'settlement'::ledger_entry_type,
    'debit'::ledger_direction,
    s.note,
    g.head_user_id,       -- created_by: the head (settlements were head-only)
    g.head_user_id,       -- decided_by: same
    s.created_at,         -- decided_at: when it was recorded
    s.date::timestamptz   -- created_at: the payout date (drives month grouping)
FROM settlements s
JOIN groups g ON g.id = s.group_id;

DROP INDEX IF EXISTS idx_settlements_user_id;
DROP INDEX IF EXISTS idx_settlements_group_id;
DROP TABLE IF EXISTS settlements;
