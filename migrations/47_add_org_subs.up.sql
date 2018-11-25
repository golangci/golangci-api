CREATE TABLE org_subs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    payment_gateway_card_token VARCHAR(64) NOT NULL,
    payment_gateway_customer_id VARCHAR(64),
    payment_gateway_subscription_id VARCHAR(64),

    billing_user_id INTEGER REFERENCES users(id) NOT NULL,
    org_id INTEGER REFERENCES orgs(id) NOT NULL,
    seats_count INTEGER NOT NULL DEFAULT 1,
    commit_state VARCHAR(32) NOT NULL
);
