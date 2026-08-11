-- Core schema. TEXT ids/timestamps (RFC3339) throughout for easy future
-- portability to Postgres (swap TEXT PK -> UUID, TEXT timestamp -> TIMESTAMPTZ).

CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  email         TEXT UNIQUE,
  password_hash TEXT,
  display_name  TEXT NOT NULL,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

CREATE TABLE rooms (
  id                 TEXT PRIMARY KEY,
  name               TEXT NOT NULL,
  mode               TEXT NOT NULL CHECK (mode IN ('online','local')),
  owner_id           TEXT REFERENCES users(id) ON DELETE SET NULL,
  status             TEXT NOT NULL CHECK (status IN ('WAITING','PRESENTING','PAUSED','ENDED')) DEFAULT 'WAITING',
  join_code          TEXT NOT NULL UNIQUE,
  access_protected   INTEGER NOT NULL DEFAULT 0,
  pin_hash           TEXT,
  host_lan_addr      TEXT,
  expires_at         TEXT NOT NULL,
  created_at         TEXT NOT NULL,
  updated_at         TEXT NOT NULL
);
CREATE INDEX idx_rooms_join_code ON rooms(join_code);

CREATE TABLE room_participants (
  id           TEXT PRIMARY KEY,
  room_id      TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
  display_name TEXT NOT NULL,
  role         TEXT NOT NULL CHECK (role IN ('OWNER','HOST','PRESENTER','PARTICIPANT','VIEWER')),
  status       TEXT NOT NULL CHECK (status IN ('CONNECTED','DISCONNECTED','KICKED')) DEFAULT 'CONNECTED',
  is_primary   INTEGER NOT NULL DEFAULT 0,
  joined_at    TEXT NOT NULL,
  last_seen_at TEXT NOT NULL
);
CREATE INDEX idx_participants_room ON room_participants(room_id);

CREATE TABLE room_invites (
  id         TEXT PRIMARY KEY,
  room_id    TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
  join_code  TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked    INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_invites_room ON room_invites(room_id);

CREATE TABLE room_sessions (
  id             TEXT PRIMARY KEY,
  room_id        TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  participant_id TEXT NOT NULL REFERENCES room_participants(id) ON DELETE CASCADE,
  token_jti      TEXT NOT NULL UNIQUE,
  issued_at      TEXT NOT NULL,
  expires_at     TEXT NOT NULL,
  revoked        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_sessions_jti ON room_sessions(token_jti);

CREATE TABLE room_permissions (
  room_id TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  role    TEXT NOT NULL,
  action  TEXT NOT NULL,
  allowed INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (room_id, role, action)
);

CREATE TABLE room_actions (
  id             TEXT PRIMARY KEY,
  room_id        TEXT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  participant_id TEXT NOT NULL REFERENCES room_participants(id) ON DELETE CASCADE,
  action_type    TEXT NOT NULL,
  status         TEXT NOT NULL CHECK (status IN ('PENDING','ACCEPTED','DECLINED','CANCELLED')) DEFAULT 'PENDING',
  created_at     TEXT NOT NULL,
  resolved_at    TEXT
);
CREATE INDEX idx_actions_room_status ON room_actions(room_id, status);
