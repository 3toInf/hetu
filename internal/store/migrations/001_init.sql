CREATE TABLE projects (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  path       TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL
);

CREATE TABLE agents (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  enabled    INTEGER NOT NULL DEFAULT 1,
  binary     TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE sessions (
  hetu_id       TEXT PRIMARY KEY,
  agent         TEXT NOT NULL,
  external_id   TEXT NOT NULL,
  project_id    INTEGER REFERENCES projects(id),
  host          TEXT NOT NULL DEFAULT 'local',
  cwd           TEXT,
  title         TEXT,
  status        TEXT NOT NULL,
  driven        INTEGER NOT NULL DEFAULT 0,
  unread        INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  last_event_at INTEGER,
  UNIQUE(agent, external_id)
);

CREATE TABLE session_events (
  id           INTEGER PRIMARY KEY,
  session_hetu TEXT NOT NULL REFERENCES sessions(hetu_id),
  seq          INTEGER NOT NULL,
  kind         TEXT NOT NULL,
  payload      TEXT,
  ts           INTEGER NOT NULL,
  UNIQUE(session_hetu, seq)
);

CREATE INDEX idx_sessions_project ON sessions(project_id);
CREATE INDEX idx_sessions_updated ON sessions(updated_at DESC);