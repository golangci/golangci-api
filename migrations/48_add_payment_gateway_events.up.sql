CREATE TABLE payment_gateway_events (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    provider VARCHAR(64) NOT NULL DEFAULT 'securionpay',
    provider_id VARCHAR(64) NOT NULL DEFAULT '',
    
    type VARCHAR(32) NOT NULL DEFAULT '',
    data JSON NOT NULL DEFAULT '{}',

    UNIQUE(provider, provider_id)
);