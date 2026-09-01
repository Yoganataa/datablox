CREATE TABLE IF NOT EXISTS reaction_roles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  emoji TEXT NOT NULL,
  role_id TEXT NOT NULL,
  mode TEXT NOT NULL DEFAULT 'normal',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(guild_id, message_id, emoji, role_id)
);

CREATE INDEX IF NOT EXISTS idx_reaction_roles_guild ON reaction_roles(guild_id);
CREATE INDEX IF NOT EXISTS idx_reaction_roles_message ON reaction_roles(message_id);
CREATE INDEX IF NOT EXISTS idx_reaction_roles_guild_message ON reaction_roles(guild_id, message_id);
