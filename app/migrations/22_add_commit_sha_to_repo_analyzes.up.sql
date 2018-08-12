ALTER TABLE repo_analyzes
  ADD COLUMN commit_sha VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE repo_analysis_statuses
  ADD COLUMN pending_commit_sha VARCHAR(64) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX repo_analyzes_uniq_status_id_and_commit_sha
  ON repo_analyzes(repo_analysis_status_id, commit_sha)
  WHERE commit_sha != '';