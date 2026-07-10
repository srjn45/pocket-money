-- 013_group_currency.up.sql
-- D7: every group has an immutable currency in {EUR, USD, INR}.
-- Add with a temporary DEFAULT so existing rows backfill to INR in one shot,
-- then drop the default so future inserts MUST supply a currency explicitly.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS currency char(3) NOT NULL DEFAULT 'INR';

ALTER TABLE groups
    ADD CONSTRAINT groups_currency_check CHECK (currency IN ('EUR', 'USD', 'INR'));

ALTER TABLE groups
    ALTER COLUMN currency DROP DEFAULT;
