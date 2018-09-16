ALTER TABLE github_auths ADD CONSTRAINT uniq_user_id_login UNIQUE (user_id, login);
ALTER TABLE github_auths DROP CONSTRAINT uniq_user_id;
ALTER TABLE github_auths DROP CONSTRAINT uniq_login;
