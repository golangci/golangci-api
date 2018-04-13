CREATE TABLE github_analyzes (
   id SERIAL PRIMARY KEY,
   created_at TIMESTAMP NOT NULL,
   updated_at TIMESTAMP NOT NULL,
   deleted_at TIMESTAMP,

   github_repo_id INTEGER REFERENCES github_repos(id) NOT NULL,
   github_pull_request_number INTEGER NOT NULL,
   github_delivery_guid VARCHAR(64) NOT NULL UNIQUE,

   status VARCHAR(64) NOT NULL
);
