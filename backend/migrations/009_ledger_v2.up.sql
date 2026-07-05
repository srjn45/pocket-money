-- 1. New enum types.
CREATE TYPE ledger_entry_type AS ENUM ('chore', 'allowance', 'emi', 'settlement', 'adjustment');
CREATE TYPE ledger_direction  AS ENUM ('credit', 'debit');

-- 2. New columns (all nullable so the ALTER succeeds on existing rows).
ALTER TABLE ledger_entries
    ADD COLUMN entry_type  ledger_entry_type,
    ADD COLUMN direction   ledger_direction,
    ADD COLUMN loan_id      UUID,                 -- FK added in migration 011 (loans)
    ADD COLUMN period       CHAR(7),              -- 'YYYY-MM', set on allowance/emi only
    ADD COLUMN note         TEXT,
    ADD COLUMN decided_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN decided_at   TIMESTAMPTZ;

-- 3. chore_id becomes nullable (null for allowance/emi/adjustment, and for
--    the settlement rows we normalise in step 5).
ALTER TABLE ledger_entries ALTER COLUMN chore_id DROP NOT NULL;

-- 4. Backfill entry_type + direction from the existing chore linkage.
--    Existing rows are either regular-chore credits or system-"Settlement"-chore debits.
UPDATE ledger_entries le
SET entry_type = CASE WHEN c.is_system THEN 'settlement' ELSE 'chore' END::ledger_entry_type,
    direction  = CASE WHEN c.is_system THEN 'debit'      ELSE 'credit' END::ledger_direction
FROM chores c
WHERE le.chore_id = c.id;

-- 5. Normalise backfilled settlement rows to the v2 invariant (settlement entries
--    carry no chore_id — they are typed, not chore-linked). Safe because balance
--    now keys off `direction`, not the chore join.
UPDATE ledger_entries SET chore_id = NULL WHERE entry_type = 'settlement';

-- 6. Consolidate the approve/reject audit into decided_by/decided_at.
--    We have the "who" (approved_by/rejected_by) but never recorded the "when",
--    so decided_at stays NULL for historic rows (§6.2: "backfill nulls").
UPDATE ledger_entries
SET decided_by = COALESCE(approved_by_user_id, rejected_by_user_id);
ALTER TABLE ledger_entries
    DROP COLUMN approved_by_user_id,
    DROP COLUMN rejected_by_user_id;

-- 7. Enforce NOT NULL on the always-present typed columns now that they are backfilled.
ALTER TABLE ledger_entries
    ALTER COLUMN entry_type SET NOT NULL,
    ALTER COLUMN direction  SET NOT NULL;

-- 8. Unique partial index for idempotent machine posting (§5.1/§5.4).
--    Postgres 15 (pinned in docker-compose + CI): NULLS NOT DISTINCT makes the
--    NULL loan_id of allowance rows collide correctly on (group,user,type,period),
--    while emi rows (loan_id set) stay distinct per loan.
CREATE UNIQUE INDEX idx_ledger_entries_posting_unique
    ON ledger_entries (group_id, user_id, entry_type, period, loan_id)
    NULLS NOT DISTINCT
    WHERE period IS NOT NULL;

-- Helpful lookup index for the new type filter (§4 GET ?type=).
CREATE INDEX idx_ledger_entries_group_type ON ledger_entries (group_id, entry_type);
