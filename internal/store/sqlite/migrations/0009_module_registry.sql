CREATE TABLE IF NOT EXISTS modules (
  slug TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  icon TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT ''
);

INSERT OR IGNORE INTO modules (slug, name, icon, description) VALUES
  ('automod', 'AutoMod', 'shield-alert', 'Automated moderation filters'),
  ('bindings', 'Bindings', 'link', 'Roblox group role bindings'),
  ('feed', 'Feed', 'rss', 'Roblox experience catalog feed'),
  ('leveling', 'Leveling', 'trophy', 'XP and level system'),
  ('moderation', 'Moderation', 'gavel', 'Warn, mute, ban and purge'),
  ('reaction_roles', 'Reaction Roles', 'smile-plus', 'Emoji role assignment'),
  ('verify', 'Verify', 'badge-check', 'Roblox verification'),
  ('welcome', 'Welcome', 'hand', 'Welcome messages and auto-role');

CREATE TABLE IF NOT EXISTS guild_modules (
  guild_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  config_json TEXT NOT NULL DEFAULT '{}',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (guild_id, slug),
  FOREIGN KEY (slug) REFERENCES modules(slug) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_guild_modules_guild ON guild_modules(guild_id);
CREATE INDEX IF NOT EXISTS idx_guild_modules_enabled ON guild_modules(guild_id, enabled);

CREATE TABLE IF NOT EXISTS feed_config (
  guild_id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL DEFAULT '',
  vote_message_id TEXT NOT NULL DEFAULT '',
  top_message_id TEXT NOT NULL DEFAULT '',
  default_genre TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (guild_id) REFERENCES guild_config(guild_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verify_config (
  guild_id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL DEFAULT '',
  message_id TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (guild_id) REFERENCES guild_config(guild_id) ON DELETE CASCADE
);

-- seed feed/verify from existing guild_config for backward compat (all-in safe, idempotent)
INSERT OR IGNORE INTO feed_config (guild_id, channel_id, vote_message_id, top_message_id, default_genre)
  SELECT guild_id, channel_id, vote_message_id, top_message_id, default_genre FROM guild_config;

INSERT OR IGNORE INTO verify_config (guild_id, channel_id, message_id)
  SELECT guild_id, verify_channel_id, verify_message_id FROM guild_config WHERE verify_channel_id IS NOT NULL;

-- seed guild_modules enabled based on existing config (enabled first logic)
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled)
  SELECT guild_id, 'feed', CASE WHEN channel_id != '' THEN 1 ELSE 0 END FROM guild_config;

INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled)
  SELECT guild_id, 'verify', CASE WHEN verify_channel_id != '' AND verify_channel_id != '' THEN 1 ELSE 0 END FROM guild_config;

INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'automod', 0 FROM guild_config;
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'bindings', 0 FROM guild_config;
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'leveling', 0 FROM guild_config;
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'moderation', 0 FROM guild_config;
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'reaction_roles', 0 FROM guild_config;
INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) SELECT guild_id, 'welcome', 0 FROM guild_config;
