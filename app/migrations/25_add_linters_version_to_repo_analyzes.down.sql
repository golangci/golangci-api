ALTER TABLE repo_analyzes
  DROP COLUMN linters_version;

CREATE UNIQUE INDEX repo_analyzes_uniq_status_id_and_commit_sha
  ON repo_analyzes(repo_analysis_status_id, commit_sha)
  WHERE commit_sha != '';

DROP INDEX repo_analyzes_uniq_status_id_and_commit_sha_and_linters_version;
DROP INDEX repo_analyzes_linters_version_idx;
DROP INDEX repo_analysis_statuses_has_pending_changes_idx;