-- Revert transactions table changes: remove bet_id, add back game_id and session_id
ALTER TABLE transactions 
    DROP COLUMN IF EXISTS bet_id,
    ADD COLUMN game_id VARCHAR(64),
    ADD COLUMN session_id VARCHAR(64);

-- Drop the bet_id index
DROP INDEX IF EXISTS idx_transactions_bet_id; 