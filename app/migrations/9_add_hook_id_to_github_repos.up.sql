ALTER TABLE github_repos
  ADD COLUMN hook_id varchar(32) NOT NULL UNIQUE;
