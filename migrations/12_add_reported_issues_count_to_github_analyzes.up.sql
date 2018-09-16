ALTER TABLE github_analyzes
  ADD COLUMN reported_issues_count INTEGER NOT NULL DEFAULT -1;
