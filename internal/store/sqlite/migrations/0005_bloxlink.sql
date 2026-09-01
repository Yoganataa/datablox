CREATE TABLE IF NOT EXISTS verified_users (
  discord_id TEXT PRIMARY KEY,
  roblox_id INTEGER NOT NULL UNIQUE,
  roblox_username TEXT NOT NULL,
  verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS guild_bindings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id TEXT NOT NULL,
  group_id INTEGER NOT NULL,
  roblox_role_id INTEGER,
  rank_min INTEGER,
  rank_max INTEGER,
  discord_role_id TEXT NOT NULL,
  nickname_template TEXT NOT NULL DEFAULT '{robloxName}',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(guild_id, group_id, discord_role_id)
);

CREATE INDEX IF NOT EXISTS idx_guild_bindings_guild ON guild_bindings(guild_id);
CREATE INDEX IF NOT EXISTS idx_verified_roblox ON verified_users(roblox_id);
