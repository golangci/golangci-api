CREATE TABLE orgs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    name VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,

    provider VARCHAR(64) NOT NULL DEFAULT 'github.com',
    provider_id INTEGER NOT NULL DEFAULT 0,
    provider_personal_user_id INTEGER NOT NULL DEFAULT 0,

    settings JSON NOT NULL DEFAULT '{}'
);
