CREATE TABLE issues
(
  id              BIGSERIAL PRIMARY KEY,
  created_at      TIMESTAMPTZ NOT NULL,
  updated_at      TIMESTAMPTZ,
  title           VARCHAR(2000) NOT NULL,
  issue_number    bigint NOT NULL,
  comments_count  bigint NOT NULL,
  repo_name       VARCHAR(255) NOT NULL,
  repo_owner      VARCHAR(255) NOT NULL,
  author          JSONB NOT NULL,
  labels          JSONB NOT NULL DEFAULT '[]',
  assignees       JSONB NOT NULL DEFAULT '[]'
);

CREATE INDEX issues_repo_name_idx ON issues (repo_name);
CREATE INDEX issues_repo_owner_idx ON issues (repo_owner);
