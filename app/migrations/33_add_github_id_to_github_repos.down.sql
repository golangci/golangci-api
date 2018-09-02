DROP INDEX github_repos_github_id_idx;

ALTER TABLE github_repos
    DROP COLUMN github_id;