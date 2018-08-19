ALTER TABLE repo_analyzes
  ADD COLUMN linters_version VARCHAR(64) NOT NULL DEFAULT 'v1.10';

DROP INDEX repo_analyzes_uniq_status_id_and_commit_sha;

CREATE UNIQUE INDEX repo_analyzes_uniq_status_id_and_commit_sha_and_linters_version
  ON repo_analyzes(repo_analysis_status_id, commit_sha, linters_version);

CREATE INDEX repo_analyzes_linters_version_idx
  ON repo_analyzes(linters_version);

CREATE INDEX repo_analysis_statuses_has_pending_changes_idx
  ON repo_analysis_statuses(has_pending_changes);