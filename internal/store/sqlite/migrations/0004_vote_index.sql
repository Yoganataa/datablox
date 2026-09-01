CREATE INDEX IF NOT EXISTS idx_poll_votes_message_universe ON poll_votes(message_id, universe_id);
CREATE INDEX IF NOT EXISTS idx_poll_votes_user ON poll_votes(message_id, user_id);
