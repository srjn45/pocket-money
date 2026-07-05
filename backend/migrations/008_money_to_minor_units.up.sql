-- Convert all money columns from DECIMAL(12,2) rupees to BIGINT minor units (paise).
-- round(amount*100) is exact for DECIMAL(12,2) input (at most 2 fractional digits).
ALTER TABLE chores
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);

ALTER TABLE ledger_entries
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);

ALTER TABLE settlements
    ALTER COLUMN amount TYPE BIGINT USING round(amount * 100);
