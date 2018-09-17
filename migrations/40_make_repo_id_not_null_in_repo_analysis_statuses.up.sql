ALTER TABLE repo_analysis_statuses
    ALTER COLUMN repo_id SET NOT NULL;

DROP INDEX repo_analysis_statuses_repo_id_idx;
CREATE UNIQUE INDEX repo_analysis_statuses_repo_id_idx ON repo_analysis_statuses(repo_id);

ALTER TABLE repo_analysis_statuses
    DROP COLUMN name;