CREATE UNIQUE INDEX github_analyzes_uniq_repo_and_commit_sha
  ON github_analyzes(github_repo_id, commit_sha)
  WHERE commit_sha != '';
