# Datablox — Roblox Discord Bot + Web Panel (Go + SQLite + templ)

Single-binary Discord bot + web dashboard untuk komunitas Roblox. Katalog Roblox, voting, verifikasi Roblox OAuth, dan 8 modul server yang equal (enable what you need) — `templ` + `Tailwind` + `HTMX`, `modernc.org/sqlite` WAL, tanpa Docker wajib.

## Stack

Go 1.27 · discordgo v0.29 · modernc.org/sqlite v1.57 (pure Go, WAL) · a-h/templ v0.3.1020 · tailwindcss v4.1 + templui v1.13 · go-htmx v1.13 · FTS5 · cron/v3 · fogleman/gg + imaging (welcome banner 1000×400 JPEG) · cloudflared tunnel `datablox.devest.live → localhost:8080`

## Quick Setup

```bash
cp .env.example .env   # isi DISCORD_TOKEN + OWNER_IDS/ADMIN_IDS + OAuth
go run ./cmd/bot       # bot saja (tanpa web HMR)
go run ./cmd/web       # web HMR saja (templ --proxy :8080)

# dev all-in (HMR web + bot stabil + cloudflared) — Taskfile
task dev

# prod
templ generate && ./tailwindcss.exe -i assets/css/input.css -o assets/css/output.css --minify
go build -o bin/bot.exe ./cmd/bot && go build -o bin/web.exe ./cmd/web
```

Invite: `bot` + `applications.commands`, permissions `268435456` (least-privilege, `Send Messages` + `Embed Links` minimal untuk feed). `GUILD_ID` kosong = global (1 jam propagate), isi = guild sync instan.

## Env `internal/config/config.go:1`

```
DISCORD_TOKEN=...                 # wajib
GUILD_ID=                         # kosong = global
DATABASE_PATH=./data/bot.db       # :memory: untuk test
ROBLOX_TIMEOUT=10s
REFRESH_INTERVAL=30m
LOG_LEVEL=info
WEB_PORT=8080
WEB_URL=http://localhost:8080     # / https://datablox.devest.live (prod via cloudflared)
OWNER_ID= / OWNER_IDS=123,456     # super admin (bypass all, BotOwner)
ADMIN_ID= / ADMIN_IDS=123,456     # bot admin (global, BotAdmin, ADMIN_DISCORD_IDS legacy tetap support)
ADMIN_DISCORD_IDS=                # legacy, merge ke ADMIN_IDS
DISCORD_CLIENT_ID= / DISCORD_CLIENT_SECRET=  # Discord OAuth (hidden /auth/discord/login)
ROBLOX_CLIENT_ID= / ROBLOX_CLIENT_SECRET=    # Roblox OAuth https://create.roblox.com/dashboard/credentials
```

`OWNER_IDS > ADMIN_IDS` jika ada di dua-duanya. `isGuildAdmin` = `isOwnerBot || isAdminBot || GuildOwner || MANAGE_GUILD (0x20) || ADMINISTRATOR (0x8)`.

## Commands (12 + autocomplete)

| Command | Params | Catatan |
|---|---|---|
| `/datablox` | — | Dashboard central (ephemeral, 4 buttons + genre select) |
| `/add url genre` | `url` wajib, `genre` choice | Dedup `universe_id`, batch `GetGameDetail/GetVotes`, thumbnail `512` |
| `/search query genre` | `query` **autocomplete** | FTS5 `bm25` + fallback LIKE |
| `/config` | `MANAGE_GUILD` | Panel SelectMenu feed/verify channel + genre + regen |
| `/warn user reason` | `user`, `reason` | Infraction `warn` |
| `/mute user minutes reason` | `user`, `minutes` default 10 | Timeout `GuildMemberTimeout` + infraction `mute` |
| `/ban user reason` | `user` | `GuildBanCreate` + infraction `ban` |
| `/kick user reason` | `user` | `GuildMemberDelete` |
| `/purge amount` | `1-100` | Bulk delete `<14 hari` |
| `/level user` | `user` optional | XP `100*2^level` |
| `/leaderboard` | — | Top 10 `xp DESC` |
| `/reactionrole add/list/remove` | `add: channel message_id emoji role mode(normal/unique/verify/reverse)`, `list`, `remove id` | Parity web `POST /api/reaction-roles`, cap 20/msg, `NormalizeEmoji` specter-style, `reverse` toggle |

Semua `experience` field autocomplete via `SearchNames` FTS limit 10.

## 8 Modules — Equal (enable what you need)

Landing `Everything you need` + `guild_detail` + `dashboard` + `guilds` semua grid `lg:grid-cols-4` order alphabet `AutoMod, Bindings, Feed, Leveling, Moderation, Reaction Roles, Verify, Welcome` — sorting `enabled-first A-Z` via `internal/core/modules.go:17` `SortedModules(guildModules)` (enabled di atas A-Z, disabled di bawah A-Z). `GET /api/modules` 8 rows, `GET/POST /api/guild-modules` toggle per guild (`isGuildAdmin`).

| Modul | Deskripsi | Store |
|---|---|---|
| **Moderation** | warn/mute/ban/kick/purge + `infractions` | `infractions` |
| **AutoMod** | `anti_spam/anti_invite/mass_mention/ghost_ping` + `exempt` (next: per-rule) | `automod_config` |
| **Leveling** | XP `15-25/60s` `100*2^level`, leaderboard 10, role rewards (next) | `levels` |
| **Welcome** | channel/message/auto_role + banner `gg` JPEG | `welcome_config` |
| **Reaction Roles** | `normal/unique/verify/reverse` (drop compat), 20/msg | `reaction_roles` |
| **Bindings** | `group_id + rankMin/Max / role_id → discord_role_id` + nickname template | `guild_bindings` |
| **Verify** | Roblox PKCE `openid profile` → `verified_users` 1 Discord:1 Roblox, `GuildMemberRoleAdd` per bindings | `verified_users`, `verify_config` |
| **Feed** | 1 channel `Vote 25/page (SelectMenu + Prev/Next/Search modal) + Top 10 + Feed` kronologis | `guild_config` + `poll_votes` + `feed_config` |

Guild card `My Servers` sekarang `0/8 modules` + chips 3 teratas `+n` (bukan `No feed/No verify`), `guild_detail` toggle `ON/OFF` `fetch POST /api/guild-modules credentials:same-origin` → reload.

## Web Panel `internal/web/server.go:57`

```
GET  /, /dashboard → 302 /guilds (auth), /guilds, /guild/<id>, /verify?guild_id, /guide, /status, /privacy, /terms
GET  /api/health → {"status":"ok"}
GET  /api/modules, GET/POST /api/guild-modules (toggle), /api/bindings, /api/reaction-roles (reverse), /api/automod, /api/welcome, /api/levels?guild_id, /api/guilds (gated)
GET  /auth/discord/login|callback (state 5m + cookie), /auth/roblox/login|callback (PKCE verifier 5m), /logout
/assets/ (output.css), templui scripts
```

`layouts/base.templ:9` `Nav{IsAdmin,DiscordName,DiscordAvatar}` avatar `cdn.discordapp.com/avatars/{id}/{hash}.png`, `layouts` `Guide Status` + `Dashboard` dropdown click `details` (bukan hover).

## Detail Experience

Embed `internal/discord/embed.go:1`: `Genre | Playing | Visits` + `Rating 👍 % | Max Players | Creator` + thumbnail `512x512` + footer `Dibuat Update Refresh`.

## Struktur

```
cmd/bot, cmd/web (HMR --proxy)
internal/config (OWNER_IDS/ADMIN_IDS merge), model (Module/GuildModule/FeedConfig/VerifyConfig), store (interface), store/sqlite (FTS5, WAL, migrations 0001-0009 module_registry)
internal/roblox (ExtractPlaceID, ResolveUniverseID, GetGameDetail, GetVotes batch, GetThumbnailURL)
internal/service (genre infer, Add/RefreshOne/RefreshAll)
internal/discord (bot, commands, moderation, leveling, welcome/banner, reaction_roles reverse, panel)
internal/core (router scaffold, modules.go SortedModules)
internal/web (server, templates/pages landing/dashboard/guilds/guild_detail/verify/guide/status/privacy/terms, layouts/base)
assets/css (input.css → output.css via tailwindcss.exe)
Taskfile.yml (templ internal, tailwind internal, cloudflared internal, web, bot, dev all-in)
```

Roblox API: `apis.roblox.com/universes/v1/places/{id}/universe`, `games.roblox.com/v1/games?universeIds=...`, `thumbnails.roblox.com/v1/games/icons?size=512x512`.

## Test & Vet

```bash
templ generate
go vet ./...
go test ./internal/store/sqlite -count 1
go build -o bin/web.exe ./cmd/web && go build -o bin/bot.exe ./cmd/bot
```

Health: `GET /api/health → 200`, `GET /api/modules → 200 8 rows alphabet`.

## Migrasi `0009_module_registry.sql:1`

`modules` seed 8 + `guild_modules (guild_id, slug, enabled, config_json)` + `feed_config` + `verify_config` copy dari `guild_config` (backward compat, `guild_config` kolom lama tetap untuk dual-read).
