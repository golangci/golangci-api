ALTER TABLE auths RENAME github_user_id TO provider_user_id;
ALTER TABLE auths ADD COLUMN provider VARCHAR(64) NOT NULL DEFAULT 'github.com';