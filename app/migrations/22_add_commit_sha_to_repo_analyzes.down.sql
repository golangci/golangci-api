ALTER TABLE repo_analyzes
  DROP COLUMN commit_sha
  DROP INDEX repo_analyzes_uniq_status_id_and_commit_sha;

ALTER TABLE repo_analysis_statuses
  DROP COLUMN pending_commit_sha 