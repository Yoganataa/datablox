PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS experiences (
  universe_id INTEGER PRIMARY KEY,
  place_id INTEGER NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT,
  creator_id INTEGER,
  creator_name TEXT,
  genre TEXT NOT NULL DEFAULT 'other',
  max_players INTEGER NOT NULL DEFAULT 0,
  playing INTEGER NOT NULL DEFAULT 0,
  thumbnail_url TEXT,
  roblox_url TEXT,
  last_refreshed_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_experiences_genre ON experiences(genre);
CREATE INDEX IF NOT EXISTS idx_experiences_playing ON experiences(playing DESC);

CREATE VIRTUAL TABLE IF NOT EXISTS experiences_fts USING fts5(
  name,
  description,
  genre,
  content='experiences',
  content_rowid='universe_id',
  tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS experiences_ai AFTER INSERT ON experiences BEGIN
  INSERT INTO experiences_fts(rowid, name, description, genre)
  VALUES (new.universe_id, new.name, new.description, new.genre);
END;

CREATE TRIGGER IF NOT EXISTS experiences_ad AFTER DELETE ON experiences BEGIN
  INSERT INTO experiences_fts(experiences_fts, rowid, name, description, genre)
  VALUES ('delete', old.universe_id, old.name, old.description, old.genre);
END;

CREATE TRIGGER IF NOT EXISTS experiences_au AFTER UPDATE ON experiences BEGIN
  INSERT INTO experiences_fts(experiences_fts, rowid, name, description, genre)
  VALUES ('delete', old.universe_id, old.name, old.description, old.genre);
  INSERT INTO experiences_fts(rowid, name, description, genre)
  VALUES (new.universe_id, new.name, new.description, new.genre);
END;

CREATE TABLE IF NOT EXISTS guild_config (
  guild_id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL DEFAULT '',
  default_genre TEXT NOT NULL DEFAULT ''
);