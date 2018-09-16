ALTER TABLE repos
    ADD COLUMN provider VARCHAR(64) NOT NULL DEFAULT 'github.com';

CREATE INDEX repos_provider_idx ON repos(provider);