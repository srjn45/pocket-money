-- Revert BIGINT minor units back to DECIMAL(12,2) rupees.
ALTER TABLE settlements
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;

ALTER TABLE chores
    ALTER COLUMN amount TYPE DECIMAL(12, 2) USING amount / 100.0;
