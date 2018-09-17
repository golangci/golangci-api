ALTER TABLE repo_analysis_statuses
    ADD COLUMN repo_id INTEGER REFERENCES repos(id);

CREATE INDEX repo_analysis_statuses_repo_id_idx ON repo_analysis_statuses(repo_id);

UPDATE repo_analysis_statuses SET repo_id=(SELECT id FROM repos WHERE name = repo_analysis_statuses.name LIMIT 1);