CREATE TABLE repo_analyzes (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    repo_analysis_status_id INTEGER REFERENCES repo_analysis_statuses(id) NOT NULL,
    analysis_guid VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(64) NOT NULL,

    result_json JSONB
);
