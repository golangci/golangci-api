ALTER TABLE github_analyzes
  DROP COLUMN commit_sha;

ALTER TABLE github_analyzes
  DROP INDEX github_analyzes_uniq_repo_and_commit_sha;
