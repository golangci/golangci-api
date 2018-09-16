ALTER TABLE github_auths ADD COLUMN login varchar(256) NOT NULL;
ALTER TABLE github_auths ADD CONSTRAINT uniq_user_id_login UNIQUE (user_id, login);
