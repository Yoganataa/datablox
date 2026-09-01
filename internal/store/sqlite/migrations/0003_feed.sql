ALTER TABLE guild_config ADD COLUMN vote_message_id TEXT NOT NULL DEFAULT '';
ALTER TABLE guild_config ADD COLUMN top_message_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS poll_votes (
  message_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  universe_id INTEGER NOT NULL,
  emoji TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (message_id, user_id, universe_id)
);

CREATE INDEX IF NOT EXISTS idx_poll_votes_message ON poll_votes(message_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_universe ON poll_votes(universe_id);

-- Composite index for ListByGenre + Trending at 1000 scale
CREATE INDEX IF NOT EXISTS idx_experiences_genre_playing ON experiences(genre, playing DESC, universe_id);
