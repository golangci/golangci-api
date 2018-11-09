ALTER TABLE auths RENAME provider_user_id TO github_user_id;
ALTER TABLE auths DROP COLUMN provider;