# Datablox — Roblox Discord Bot (Go + SQLite)

Single-binary Discord bot untuk katalog Roblox. `go run` untuk dev, tanpa Docker, rapi sampai 1000 experience.

## Stack
Go 1.27 · discordgo v0.29 · modernc.org/sqlite (pure Go) · FTS5 · cron/v3

## Setup

```bash
cp .env.example .env   # isi DISCORD_TOKEN, GUILD_ID opsional (guild = sync instan)
go run ./cmd/bot       # dev
go build -o bin/bot ./cmd/bot && ./bin/bot  # prod
```

**Env** `internal/config/config.go:1`:
```
DISCORD_TOKEN=...
GUILD_ID=123...        # kosong = global (1 jam propagate)
DATABASE_PATH=./data/bot.db  # :memory: untuk test
ROBLOX_TIMEOUT=10s
REFRESH_INTERVAL=30m
LOG_LEVEL=info
```

Invite: `bot` + `applications.commands`, permissions `Send Messages (2048) + Embed Links (16384) = 18432`.

## Commands (8 + autocomplete max)

| Command | Params | Catatan UX |
|---|---|---|
| `/add url genre` | `url` wajib, `genre` choice (auto-infer jika kosong) | Dedup by `universe_id`, thumbnail `512→150`, `visits/votes/created/updated` langsung, trigger feed |
| `/search query genre` | `query` **autocomplete** | FTS5 `name/description` `bm25`, fallback LIKE |
| `/games genre` | `genre` choice | `playing DESC` |
| `/game experience` | **autocomplete** | Detail kaya 6 field + thumbnail |
| `/random genre` | `genre` choice | `(playing+1)*RANDOM()` weighted |
| `/trending` | — | Top `playing` |
| `/refresh experience` | **autocomplete**, kosong = semua | `RefreshOne` force fetch (bukan dedup), `RefreshAll` batch 100 |
| `/config action value` | `action: channel/genre/show/regen` | `channel` = 1 channel feed (vote #1 + top10 #2 + feed #3+), `regen` rebuild |

Semua field `experience` (`search`, `game`, `refresh`) autocomplete via `SearchNames` FTS limit 10 (<10ms).

## Feed 1 Channel — Rapi Sampai 1000

```
/config action:channel value:#datablox-feed
```
- Pesan #1 **Vote** — paginated `25/halaman` (40 halaman untuk 1000), `SelectMenu 25 opsi` + `Prev/Next/Search` buttons, multi-vote boleh
- Pesan #2 **Top 10** — `Trending`, edit on `/add`/`/refresh`
- Pesan #3+ **Feed** — tiap `/add` post 1 embed baru (kronologis), vote/top edit in-place (1 edit, bukan 1000 post burst)

Batasan Discord: `20 reactions/msg` tidak dipakai — ganti SelectMenu 25, `10 embeds/msg`, `5 msg/5s per channel`. Paginasi edit 1 message, bukan flood 1000.

**Vote flow:** pilih di dropdown → `poll_votes(message_id,user_id,universe_id)` multi-vote → ephemeral `Voted untuk ... total N vote` → `Prev/Next` → `ChannelMessageEditComplex` ganti page → `Search` → modal → ephemeral hasil.

## Detail Experience Kaya

Embed `internal/discord/embed.go:1`: `Genre | Playing | Visits (👁️)` + `Rating 👍 % (up/down) | Max Players | Creator` + thumbnail `512x512` + footer `Dibuat 2020-01-02 • Update 3d lalu • Refresh 5m lalu`.

Tambahan DB `0002_enrich`, `0003_feed`: `visits`, `up_votes`, `down_votes`, `roblox_created/updated`, `vote_message_id`, `top_message_id`, `poll_votes`, index `idx_experiences_genre_playing`.

## Struktur

```
cmd/bot/main.go
internal/config, model, store (interface portable ke Postgres), store/sqlite (+FTS5, migrations 0001-0003)
internal/roblox/client.go (ExtractPlaceID, ResolveUniverseID, GetGameDetail, GetVotes batch, GetThumbnailURL)
internal/service (genre infer word-boundary, experience Add/RefreshOne/RefreshAll)
internal/discord (bot, commands, handlers, embed, feed paginated 25)
internal/scheduler
```

## Test & Vet

```bash
go vet ./...; go test ./...; go build -o bin/bot ./cmd/bot
```

Roblox API: `apis.roblox.com/universes/v1/places/{id}/universe`, `games.roblox.com/v1/games?universeIds=...`, `.../v1/games/votes?universeIds=...`, `thumbnails.roblox.com/v1/games/icons?size=512x512`.
