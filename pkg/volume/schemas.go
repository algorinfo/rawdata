package volume

var dataSchemaV1 = `
CREATE TABLE IF NOT EXISTS data (
	data_id    TEXT PRIMARY KEY,
  data       BLOB NOT NULL,
	created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- CREATE INDEX  IF NOT EXISTS groupby_ix ON data(group_by);
CREATE INDEX  IF NOT EXISTS created_ix ON data(created_at);
`
