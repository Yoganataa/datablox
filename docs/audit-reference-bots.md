# Audit Referensi Bot Discord → Datablox Convergence

> Tanggal: 2026-09-04 • Stack Datablox: Go 1.27 + discordgo + SQLite (WAL) + templ+HTMX+Tailwind • Deploy: `cloudflared → :8003` `datablox.devest.live`
> Referensi utama: **specter-go (0xSalik)** — Go+d discordgo+HTMX; sekunder: **ArkenBot** (Next.js+Fastify+Redis), **Bloxlink** (Roblox verify), **Carl-bot/Dyno** (reaction-roles/automod)

## 1. Inventory Datablox Saat Ini

### Bot interface (`internal/discord/*.go`)
| File | Slash / Component | Panjang | Store |
|---|---|---|---|
| `commands.go:10` `Commands()` | `/datablox` (panel central ephemeral), `/add url+genre`, `/search query+genre (autocomplete)`, `/config (MANAGE_GUILD)` | base 4 | `experiences`, `guild_configs` |
| `moderation.go` | `/warn /mute(minutes) /ban /kick /purge(1-100) /level /leaderboard` `hasManageGuild/hasManageMessages` + `infractions` insert | 5+2 | `infractions` |
| `config_ui.go:12` `handleConfig` | embed + 3 SelectMenu (feed channel GuildText/News, verify channel, genre) + buttons `config:regen/regen_verify/show` | — | `guild_config` |
| `feed.go` | `ensureFeed/syncVotePage/syncTopMessage/buildVoteMessage` vote 25/page + Prev/Next/Search modal + top 10 + chronological feed (1 channel) | 25/page | `poll_votes`, `experiences` |
| `panel.go` | `handlePanel` ephemeral with `panel:search/random/trending/refresh` + genre select | — | `experiences` |
| `verify.go:63` | `verify:start` button → check `VerifiedUser` → ephemeral Already Verified + `GuildMemberRoleAdd` per `guild_bindings` + `Need help? → /auth/roblox/login?guild_id=` | — | `verified_users`, `guild_bindings` |
| `reaction_roles.go` | `onReactionAdd/Remove` `ListReactionRolesByMessage` modes `normal/unique/verify/drop` (drop=ignore) + `emojiMatches` | 20/msg | `reaction_roles` |
| `leveling.go` | `onMessageCreate` 60s cooldown 15-25 XP `100*2^level`, `handleLevel/Leaderboard`, `checkAutomod`, `onGuildMemberAdd` banner 1000×400 `gg+imaging` JPEG + AutoRole | embed | `levels`, `automod_config`, `welcome_config` |
| `handlers.go` | `onInteraction switch ApplicationCommand/MessageComponent/ModalSubmit` — scattered, no router | — | — |

**Global:** `Bot{svc.Store, sess}` `Run(){Open, registerCommands}` global register (prod) + `ApplicationCommandDelete` cleanup. Intents `Guilds GuildMessages GuildMessageReactions + Members`.

### Web interface (`internal/web/server.go:54 routes()`)
```
GET  /, /dashboard→302 /guilds, /guilds, /guild/:id (fetchGuildViaBot+fetchChannelViaBot), /dashboard/guilds|/guild/ alias,
     /verify?guild_id, /guide, /status, /privacy, /terms
GET  /api/health
GET  /api/guilds (admin + isGuildAdmin via fetchUserGuilds discord_token) — public stats not exposed here
GET|POST /api/bindings (isAdmin global)
GET|POST|DELETE /api/reaction-roles (isAdmin+isGuildAdmin, mode normal/verify/unique)
GET|POST /api/automod (isAdmin+isGuildAdmin) {anti_spam, anti_invite, mass_mention, ghost_ping}
GET|POST /api/welcome (same) {enabled, channel_id, message, auto_role_id}
GET  /api/levels?guild_id (public read, top 20)
GET|POST /auth/discord/login|callback (state 5m pendingOAuth+cookie, cdn avatar)
GET|POST /auth/roblox/login|callback (PKCE verifier 5m)
GET  /logout
/assets/ (Tailwind output)
```
`layouts/base.templ:15 Nav{IsAdmin,DiscordName,DiscordAvatar,Lang}` cookies `discord_id(HttpOnly) discord_name discord_avatar discord_token(HttpOnly) verify_guild_id oauth_state code_verifier` `Secure+SameSiteLax`.
`pages/guild_detail.templ` 7 cards: Header (icon+memberCount) + Channels (feed/verify names) + Messages (View in Discord links) + Reaction Roles table + Moderation + Leveling + Welcome + Bindings.

### Store (`internal/model/model.go` + `migrations/0001-0008`)
`experiences(PK universe_id, FTS5)`, `guild_config(guild_id, channel_id, vote/top/verify ids, default_genre)`, `poll_votes(PK message_id,user_id,universe_id)`, `verified_users(PK discord_id→roblox_id)`, `guild_bindings UNIQUE(guild_id,group_id,role_id, RankMin/Max, NicknameTemplate)`, `reaction_roles UNIQUE(guild_id,message_id,emoji,role_id) mode`, `infractions`, `automod_config`, `levels(PK guild+user, xp/level/messages/last_xp_at)`, `welcome_config`. Service `ExperienceService` wraps Roblox API `thumbnails/icons?size=512`.

## 2. Reference Deep Dive

### specter-go — PRIMARY (Go, HTMX, pgx)
- **Router `internal/core/router.go`**: `Command{Def, Group, RequiredPerm, Handler}` + `RegisterComponent(prefix)` + `Definitions()` bulk + `Handle()` top-level `recover()` + `handleCommand` per-handler `recover()` + `Gate.Check(i, Group, RequiredPerm)` 2-tier auth (Discord `default_member_permissions` hides, runtime gate authoritative). Datablox: scattered `switch` di `handlers.go`, no Group, no Gate, no per-handler recover.
- **Reaction roles `reactionroles/handler.go`**: `NormalizeEmoji` regex `<a?:name:id>` → `name:id`, `emojiKey(e.ID? name:id : name)`, menu Types `normal/unique/verify/reverse` with semantics: verify/reverse one-way (remove does nothing), unique removes other roles then add, normal add/remove. Lookup `GetMenuByMessage+ListEntries` timeout 5s. Datablox: `emojiMatches` ad-hoc, `drop` instead of `reverse`, verify removes reaction to keep counter 1 (specter doesn't), no `NormalizeEmoji`.
- **Access `internal/access`**: per-guild allow/deny per Group layered on Discord perms. Datablox: `isAdmin(ADMIN_DISCORD_IDS)` global + `isGuildAdmin(fetchUserGuilds MANAGE_GUILD)` only, no per-command group.
- **Dashboard**: `html/template+HTMX+Tailwind CDN` same as Datablox (no SPA) — validates keeping `templ`. Pages per subsystem (moderation/rapsheet/automod/levels/reactionroles/music) each own query. Auth: `Manage Server` for config, `DJ role` exception for music player.
- **Data**: `pgx pool + embed.FS migrations + internal/db/queries/*` hand-written SQL per domain. Datablox SQLite WAL + `modernc.org/sqlite` + `internal/store/sqlite` similar but single package, not domain-scoped.
- **Ops**: single binary + PG + Lavalink, global vs `DEV_GUILD_ID` instant register, panic recovery everywhere.

### ArkenBot / ShadowCore / botfleet — DASHBOARD REALTIME
- Split `bot/api/web` monorepo `pnpm+Turbo` Fastify `:4000` + Next.js `:3000` + Redis `ioredis+BullMQ` pub-sub WS live. Worth if need ticket/transcript/realtime queue (Datablox feed could use WS for vote live-update instead of poll). Cost: toolchain + infra. **Decision: keep monolith HTMX** (aligns with specter), defer split.

### Bloxlink — ROBLOX BINDING REFERENCE
- Flow `/setup → bind(groupId) → choose Verified role → merge/replace` (merge creates missing roles, replace deletes all). Datablox: `/api/bindings` POST manual, no setup wizard, no merge semantics. Nickname `{smart-name}/{robloxUsername}/{display}` not exposed in web detail except raw `NicknameTemplate`.
- Verification game/code vs Datablox PKCE `openid profile` (cleaner, but need Roblox `api.roblox.com/v1/userinfo` + `groups` check).

### Carl-bot — REACTION ROLE REFERENCE
- 250 pair/server, modes `unique/verify/reversed/binding/temporary` + button/dropdown dashboard. Dyno cheaper but basic. Datablox 20/msg cap (reasonable) but only 3 modes; needs `reverse` + `temporary` expiry and dashboard builder.

## 3. Gap Matrix (Bot vs Web vs Reference)

| Feature | Datablox Bot | Datablox Web | Reference (specter/Carl/Bloxlink) | Gap |
|---|---|---|---|---|
| Feed 1-channel | `/config` SelectMenu set `guild_config.channel_id` + `ensureFeed` | `guild_detail` Channels card display `feedChannelName + #id` + View link — **no POST** | specter guildsetup auto-provision log category per event | **Write only via bot** → need web POST parity |
| Verify channel | same SelectMenu + `ensureVerifyMessage` | display only | Bloxlink Verify channel + join DM | same gap |
| Reaction roles | runtime only `onReaction*` | **creation only via web** `POST /api/reaction-roles` (isGuildAdmin) | Carl dashboard builder + bot command `/reactionrole` both → same service | **No bot create/list** → admin must swap UI |
| Bindings | runtime role add on verify | `POST/DELETE /api/bindings` (isAdmin global) showed in `guild_detail` table | Bloxlink `/bind` wizard merge/replace + nickname template | web `isAdmin` too coarse (should be `isGuildAdmin`), no wizard |
| Moderation | `/warn/mute/ban/kick/purge` | `guild_detail` Moderation card read `infractions` only | specter `/ban/unban/kick/timeout/warning/rapsheet/clear/lock/massban` + hierarchy + DM appeal | missing `unban/timeout` native, `clear` alias, rapsheet search |
| Automod | `checkAutomod` anti_invite/discord.gg + mass_mention bool | `GET/POST /api/automod` bools | specter per-rule `{keyword/regex/caps/link/spam}` + `exempt roles/channels` + `actions{block/alert/timeout}` + per-rule role scoping | rule engine flat → false positives, no exempt |
| Leveling | XP 15-25/60s `100*2^level` + `/level/leaderboard` | `GET /api/levels` top20 JSON | specter `100*1.2^n` + rank card `gg` + role rewards stacking + exemptions | no role rewards, no exemptions, no decay, no leaderboard pagination UI |
| Welcome | `onGuildMemberAdd` banner JPEG + AutoRole | `GET/POST /api/welcome` | specter welcome/goodbye embed + DM + captcha | no preview, no goodbye, no captcha |
| Access | `hasManageGuild(Member.Permissions&MANAGE_GUILD)` per handler | `isAdmin+isGuildAdmin(fetchUserGuilds)` | 2-tier `RequiredPerm→DefaultMemberPermissions` + `Gate{Group}` | no Group ACL, no hierarchy check for bot vs target |
| Router | `switch i.Type / Data.Name` | — | `core.Router` prefix map + `recover()` + `Definitions()` bulk | scattered, panic can kill goroutine (specter protects) |
| Search | `/search` autocomplete + `panel:search` modal | landing trusted-by stats `CountGuilds/CountVotes` | specter `search mode=multi` FTS | Datablox FTS5 exists but not exposed via web API |
| Realtime | go `syncVotePage` on vote | HTMX templ watch `--proxy :8003` | ArkenBot WS live queue reordering | vote requires refresh; could push WS |

## 4. Convergence Design (ADR)

### ADR-1: Keep monolith Go+HTMX (specter model), not Next.js split
- **Context:** ArkenBot/ShadowCore show split adds Redis/BullMQ + build pipeline for realtime that Datablox (community bot, ~1k guilds) doesn't need.
- **Decision:** Stay single binary `cmd/bot` (+ `cmd/web` HMR shim) + `templ+HTMX+Tailwind` — same as specter. Defer `api/web` split until >2.5k guilds or tickets require WS.
- **Consequence:** Less infra, `task dev` stays `templ+tailwind+cloudflared+bot:web`.

### ADR-2: Adopt `internal/core` router (specter) for safety
- Introduce `internal/core/{router.go, context.go, deps.go}` with `Command{Def,Group,RequiredPerm}` → surface `DefaultMemberPermissions` + runtime `Gate` (lightweight map, no DB yet). `RegisterComponent(prefix)` for `vote:`, `panel:`, `config:`. Wrap every handler in `recover()`. Bulk `ApplicationCommandBulkOverwrite` instead of per-create loop.
- **Migration:** wrap existing `Commands()` + `moderationCommands()` incrementally; `handlers.go` becomes `router.Handle`.

### ADR-3: Unify reaction-roles (parity bot↔web)
- Add `reverse` mode (specter semantic: add if missing else remove on add, removal no-op) — rename `drop` → `reverse` compat alias. Add `NormalizeEmoji` regex. New slash `reactionrole` group (`create/list/remove`) gated `MANAGE_GUILD`, calls same `Store` as web `POST /api/reaction-roles`. Cap 20/msg kept, extend to 250/server like Carl later.
- **Web:** keep table, add mode badge `reverse` + create form with channel-search helper (specter `getGuildChannels` pattern).

### ADR-4: Bindings wizard (Bloxlink merge)
- Change web `POST /api/bindings` gate `isAdmin` → `isGuildAdmin` (align with reaction-roles). Add `merge` semantics: if `DiscordRoleID` missing, `GuildRoleCreate` (if bot has `ManageRoles`). `NicknameTemplate` default `{robloxUsername}`.

### ADR-5: Automod rule engine v1 (exempt-aware)
- Extend `automod_config` with JSON `rules TEXT` (or new `automod_rules` table idempotently) with `exempt_roles, exempt_channels, actions`. Keep bools as `v0` compat. Engine checks `exempt` before `anti_invite/mass_mention/spam`.

### Tradeoffs
- Postgres `pgx` pool vs SQLite WAL: stay SQLite (zero ops) until concurrency >500 rps; specter PG benefits not needed now.
- No ORM: keep hand-written SQL per domain (split `store/sqlite` into `queries/*` later).

## 5. Prioritized Backlog

1. **P0 Unblock parity** (this PR): `reverse` mode + `NormalizeEmoji` + bot `/reactionrole` create/list — **implemented below**.
2. P1 Bindings gate fix + merge on web (small, needs `GuildRoleCreate` perm check).
3. P1 Router `internal/core` scaffolding + bulk register + panic recovery (non-breaking wrapper).
4. P2 Automod exempt + regex + alert channel routing (needs `automod_rules` migration).
5. P2 Leveling role rewards + exemptions + leaderboard UI pagination.
6. P3 Welcome preview + goodbye + invite attribution logging.

## 6. Verification
- `go vet ./...`, `templ generate`, `go test ./...`, `webfetch https://datablox.devest.live` landing has `Invite bot`, guild detail shows `#roblox` not ID, reaction modes include `reverse`.
