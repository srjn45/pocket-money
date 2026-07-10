-- 013_group_currency.down.sql
ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_currency_check;
ALTER TABLE groups DROP COLUMN IF EXISTS currency;
