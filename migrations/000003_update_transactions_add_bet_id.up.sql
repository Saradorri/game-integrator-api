-- Update transactions table: remove game_id and session_id, add bet_id
ALTER TABLE transactions 
    DROP COLUMN IF EXISTS game_id,
    DROP COLUMN IF EXISTS session_id,
    ADD COLUMN bet_id INTEGER NOT NULL;

-- Add index on bet_id for performance
CREATE INDEX idx_transactions_bet_id ON transactions(bet_id); 