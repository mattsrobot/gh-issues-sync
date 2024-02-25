ALTER TABLE issues ADD COLUMN github_id BIGINT NOT NULL;

CREATE UNIQUE INDEX issues_github_id_idx ON issues (github_id);
