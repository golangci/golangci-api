CREATE TABLE github_repos (
   id SERIAL PRIMARY KEY,
   created_at TIMESTAMP NOT NULL,
   updated_at TIMESTAMP NOT NULL,
   deleted_at TIMESTAMP,

   user_id INTEGER REFERENCES users(id) NOT NULL,
   name VARCHAR(256) NOT NULL
);

CREATE UNIQUE INDEX github_repos_uniq_name
  ON github_repos(name)
  WHERE deleted_at IS NULL;
