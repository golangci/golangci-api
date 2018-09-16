ALTER TABLE github_repos
    ADD COLUMN github_id INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX github_repos_github_id_idx ON github_repos(github_id)
    WHERE github_id != 0 AND deleted_at IS NULL;