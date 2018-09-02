CREATE UNIQUE INDEX github_auths_github_user_id_idx ON github_auths(github_user_id)
    WHERE github_user_id != 0;