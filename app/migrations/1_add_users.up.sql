CREATE TABLE users (
   id SERIAL PRIMARY KEY,
   created_at TIMESTAMP NOT NULL,
   updated_at TIMESTAMP NOT NULL,
   deleted_at TIMESTAMP,

   email VARCHAR(128) NOT NULL,
   nickname VARCHAR(128) NOT NULL,
   name VARCHAR(128)
);
