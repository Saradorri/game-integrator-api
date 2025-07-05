-- Create transactions table
CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    type VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    amount NUMERIC(20,2) NOT NULL,
    currency VARCHAR(8) NOT NULL,
    provider_tx_id VARCHAR(64) UNIQUE,
    provider_withdrawn_tx_id BIGINT,
    old_balance NUMERIC(20,2) NOT NULL DEFAULT 0,
    new_balance NUMERIC(20,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id),
    CONSTRAINT fk_withdraw_tx FOREIGN KEY(provider_withdrawn_tx_id) REFERENCES transactions(id)
);

-- Indexes for performance
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_provider_tx_id ON transactions(provider_tx_id);
CREATE INDEX idx_transactions_provider_withdrawn_tx_id ON transactions(provider_withdrawn_tx_id);
