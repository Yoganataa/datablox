CREATE TABLE IF NOT EXISTS infractions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  moderator_id TEXT NOT NULL,
  type TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME,
  active INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_infractions_guild_user ON infractions(guild_id, user_id);
CREATE INDEX IF NOT EXISTS idx_infractions_guild ON infractions(guild_id);
CREATE INDEX IF NOT EXISTS idx_infractions_type ON infractions(type);

CREATE TABLE IF NOT EXISTS automod_config (
  guild_id TEXT PRIMARY KEY,
  anti_spam INTEGER NOT NULL DEFAULT 0,
  anti_invite INTEGER NOT NULL DEFAULT 0,
  mass_mention INTEGER NOT NULL DEFAULT 5,
  ghost_ping INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS levels (
  guild_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  xp INTEGER NOT NULL DEFAULT 0,
  level INTEGER NOT NULL DEFAULT 0,
  messages INTEGER NOT NULL DEFAULT 0,
  last_xp_at DATETIME,
  PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_levels_guild_xp ON levels(guild_id, xp DESC);

CREATE TABLE IF NOT EXISTS welcome_config (
  guild_id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  auto_role_id TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
