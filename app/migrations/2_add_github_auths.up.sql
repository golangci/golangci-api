CREATE TABLE github_auths (
   id SERIAL PRIMARY KEY,
   created_at TIMESTAMP NOT NULL,
   updated_at TIMESTAMP NOT NULL,
   deleted_at TIMESTAMP,

   access_token VARCHAR(128) NOT NULL,
   raw_data TEXT NOT NULL,
   user_id INTEGER REFERENCES users(id) NOT NULL
);
