ALTER TABLE issues ADD COLUMN closed BOOLEAN NOT NULL;

CREATE INDEX issues_closed_idx ON issues (closed);
