CREATE TABLE session_messages (
  session_hetu TEXT NOT NULL REFERENCES sessions(hetu_id),
  seq          INTEGER NOT NULL,
  role         TEXT NOT NULL,
  content      TEXT NOT NULL,
  ts           INTEGER,
  UNIQUE(session_hetu, seq)
);
CREATE INDEX idx_messages_session ON session_messages(session_hetu, seq);

CREATE VIRTUAL TABLE session_messages_fts USING fts5(
  session_hetu UNINDEXED,
  role         UNINDEXED,
  content
);

ALTER TABLE sessions ADD COLUMN content_synced_at INTEGER;
