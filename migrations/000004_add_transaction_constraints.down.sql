-- Drop check constraints
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_amount_positive;

-- Remove NOT NULL constraint
ALTER TABLE transactions ALTER COLUMN provider_tx_id DROP NOT NULL; 