-- Drop the FK first (it references loans), then the table/enum. loan_id stays a
-- bare nullable column exactly as migration 009 created it.
ALTER TABLE ledger_entries DROP CONSTRAINT IF EXISTS fk_ledger_entries_loan;

DROP INDEX IF EXISTS idx_loans_group_user;
DROP INDEX IF EXISTS idx_loans_group_status;
DROP TABLE IF EXISTS loans;
DROP TYPE IF EXISTS loan_status;
