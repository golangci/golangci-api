ALTER TABLE github_auths DROP CONSTRAINT uniq_user_id_login;
ALTER TABLE github_auths ADD CONSTRAINT uniq_user_id UNIQUE (user_id);
ALTER TABLE github_auths ADD CONSTRAINT uniq_login UNIQUE (login);
