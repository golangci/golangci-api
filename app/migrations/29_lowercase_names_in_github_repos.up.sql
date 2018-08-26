UPDATE github_repos
  SET display_name = name, name = lower(name) WHERE display_name = '';

