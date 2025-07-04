-- Create transactions table
CREATE TABLE transactions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    type VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    amount NUMERIC(20,2) NOT NULL,
    currency VARCHAR(8) NOT NULL,
    game_id VARCHAR(64),
    session_id VARCHAR(64),
    provider_tx_id VARCHAR(64) UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    deleted_at TIMESTAMP NULL,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id)
);

-- Indexes for performance
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_provider_tx_id ON transactions(provider_tx_id);
CREATE INDEX idx_transactions_deleted_at ON transactions(deleted_at); 