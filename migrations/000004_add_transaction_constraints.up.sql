-- Ensure provider_tx_id is not null
ALTER TABLE transactions ALTER COLUMN provider_tx_id SET NOT NULL;

-- Add check constraint for amount validation (was missing from original table creation)
ALTER TABLE transactions ADD CONSTRAINT chk_amount_positive 
CHECK (amount > 0); 