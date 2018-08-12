CREATE TABLE repo_analysis_statuses (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    name VARCHAR(256) NOT NULL UNIQUE,
    last_analyzed_at TIMESTAMP,
    version INTEGER NOT NULL
);
