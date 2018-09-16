ALTER TABLE github_auths DROP COLUMN login;
ALTER TABLE github_auths DROP CONSTRAINT uniq_user_id_login;
