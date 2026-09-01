package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"datablox/internal/model"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite WAL: single writer
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	// 0001 — idempotent CREATE TABLE IF NOT EXISTS
	if sqlB, err := migrationsFS.ReadFile("migrations/0001_init.sql"); err == nil {
		if _, err := s.db.Exec(string(sqlB)); err != nil {
			return err
		}
	}
	// 0002 — enrich columns, ignore duplicate-column errors for existing DBs
	if sqlB, err := migrationsFS.ReadFile("migrations/0002_enrich.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	if sqlB, err := migrationsFS.ReadFile("migrations/0003_feed.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	if sqlB, err := migrationsFS.ReadFile("migrations/0004_vote_index.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	if sqlB, err := migrationsFS.ReadFile("migrations/0006_verify_channel.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) UpsertExperience(ctx context.Context, exp model.Experience) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO experiences (
			universe_id, place_id, name, description, creator_id, creator_name,
			genre, max_players, playing, visits, up_votes, down_votes,
			thumbnail_url, roblox_url, roblox_created, roblox_updated,
			last_refreshed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(universe_id) DO UPDATE SET
			place_id = excluded.place_id,
			name = excluded.name,
			description = excluded.description,
			creator_id = excluded.creator_id,
			creator_name = excluded.creator_name,
			genre = excluded.genre,
			max_players = excluded.max_players,
			playing = excluded.playing,
			visits = excluded.visits,
			up_votes = excluded.up_votes,
			down_votes = excluded.down_votes,
			thumbnail_url = excluded.thumbnail_url,
			roblox_url = excluded.roblox_url,
			roblox_created = excluded.roblox_created,
			roblox_updated = excluded.roblox_updated,
			last_refreshed_at = excluded.last_refreshed_at,
			updated_at = CURRENT_TIMESTAMP`,
		exp.UniverseID, exp.PlaceID, exp.Name, exp.Description, exp.CreatorID, exp.CreatorName,
		exp.Genre, exp.MaxPlayers, exp.Playing, exp.Visits, exp.UpVotes, exp.DownVotes,
		exp.ThumbnailURL, exp.RobloxURL, exp.RobloxCreated, exp.RobloxUpdated,
		exp.LastRefreshedAt,
	)
	return err
}

func (s *Store) GetByUniverseID(ctx context.Context, universeID int64) (model.Experience, error) {
	return s.getOne(ctx, `WHERE universe_id = ?`, universeID)
}

func (s *Store) GetByPlaceID(ctx context.Context, placeID int64) (model.Experience, error) {
	return s.getOne(ctx, `WHERE place_id = ?`, placeID)
}

func (s *Store) getOne(ctx context.Context, where string, arg any) (model.Experience, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+cols()+` FROM experiences `+where, arg)
	return scanExperience(row)
}

func (s *Store) Search(ctx context.Context, query, genre string, limit, offset int) ([]model.Experience, int, error) {
	if query == "" {
		return s.ListByGenre(ctx, genre, limit, offset)
	}
	ftsQuery := toFTSQuery(query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+colsWithPrefix("e.")+`
		FROM experiences_fts f
		JOIN experiences e ON e.universe_id = f.rowid
		WHERE experiences_fts MATCH ? AND (? = '' OR e.genre = ?)
		ORDER BY bm25(experiences_fts)
		LIMIT ? OFFSET ?`,
		ftsQuery, genre, genre, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	results, err := scanExperiences(rows)
	rows.Close()
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return s.searchLike(ctx, query, genre, limit, offset)
	}
	total, err := s.searchTotal(ctx, query, genre)
	if err != nil {
		total = len(results)
	}
	return results, total, nil
}

func (s *Store) searchLike(ctx context.Context, query, genre string, limit, offset int) ([]model.Experience, int, error) {
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+cols()+`
		FROM experiences
		WHERE (name LIKE ? OR description LIKE ?) AND (? = '' OR genre = ?)
		ORDER BY playing DESC
		LIMIT ? OFFSET ?`,
		like, like, genre, genre, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	results, err := scanExperiences(rows)
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	return results, len(results), nil
}

func (s *Store) searchTotal(ctx context.Context, query, genre string) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM experiences_fts f
		JOIN experiences e ON e.universe_id = f.rowid
		WHERE experiences_fts MATCH ? AND (? = '' OR e.genre = ?)`,
		toFTSQuery(query), genre, genre).Scan(&total)
	return total, err
}

func (s *Store) ListByGenre(ctx context.Context, genre string, limit, offset int) ([]model.Experience, int, error) {
	q := `
		SELECT ` + cols() + ` FROM experiences
		WHERE (? = '' OR genre = ?)
		ORDER BY playing DESC
		LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, genre, genre, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	results, err := scanExperiences(rows)
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experiences WHERE (? = '' OR genre = ?)`, genre, genre).Scan(&total); err != nil {
		total = len(results)
	}
	return results, total, nil
}

func (s *Store) Random(ctx context.Context, genre string) (model.Experience, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+cols()+` FROM experiences
		WHERE (? = '' OR genre = ?)
		ORDER BY (playing + 1) * ABS(RANDOM()) DESC
		LIMIT 1`, genre, genre)
	return scanExperience(row)
}

func (s *Store) Trending(ctx context.Context, limit int) ([]model.Experience, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+cols()+` FROM experiences
		ORDER BY playing DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	results, err := scanExperiences(rows)
	rows.Close()
	return results, err
}

func (s *Store) AllUniverseIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT universe_id FROM experiences`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM experiences`).Scan(&n)
	return n, err
}

func (s *Store) UpdatePlayingAndThumbnail(ctx context.Context, universeID int64, playing, maxPlayers int, thumbnailURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE experiences
		SET playing = ?, max_players = ?, thumbnail_url = ?,
		    last_refreshed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE universe_id = ?`,
		playing, maxPlayers, thumbnailURL, universeID)
	return err
}

func (s *Store) GetGuildConfig(ctx context.Context, guildID string) (model.GuildConfig, error) {
	var cfg model.GuildConfig
	err := s.db.QueryRowContext(ctx, `
		SELECT guild_id, channel_id, vote_message_id, top_message_id, verify_channel_id, verify_message_id, default_genre FROM guild_config WHERE guild_id = ?`,
		guildID).Scan(&cfg.GuildID, &cfg.ChannelID, &cfg.VoteMessageID, &cfg.TopMessageID, &cfg.VerifyChannelID, &cfg.VerifyMessageID, &cfg.DefaultGenre)
	if err == sql.ErrNoRows {
		return model.GuildConfig{GuildID: guildID}, nil
	}
	return cfg, err
}

func (s *Store) SetGuildConfig(ctx context.Context, cfg model.GuildConfig) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO guild_config (guild_id, channel_id, vote_message_id, top_message_id, verify_channel_id, verify_message_id, default_genre)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(guild_id) DO UPDATE SET
			channel_id = excluded.channel_id,
			vote_message_id = excluded.vote_message_id,
			top_message_id = excluded.top_message_id,
			verify_channel_id = excluded.verify_channel_id,
			verify_message_id = excluded.verify_message_id,
			default_genre = excluded.default_genre`,
		cfg.GuildID, cfg.ChannelID, cfg.VoteMessageID, cfg.TopMessageID, cfg.VerifyChannelID, cfg.VerifyMessageID, cfg.DefaultGenre)
	return err
}

func (s *Store) ListGuildConfigs(ctx context.Context) ([]model.GuildConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT guild_id, channel_id, vote_message_id, top_message_id, verify_channel_id, verify_message_id, default_genre FROM guild_config WHERE channel_id != '' OR verify_channel_id != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.GuildConfig
	for rows.Next() {
		var cfg model.GuildConfig
		if err := rows.Scan(&cfg.GuildID, &cfg.ChannelID, &cfg.VoteMessageID, &cfg.TopMessageID, &cfg.VerifyChannelID, &cfg.VerifyMessageID, &cfg.DefaultGenre); err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}

func (s *Store) AddVote(ctx context.Context, v model.Vote) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO poll_votes (message_id, user_id, universe_id, emoji)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(message_id, user_id, universe_id) DO UPDATE SET emoji = excluded.emoji`,
		v.MessageID, v.UserID, v.UniverseID, v.Emoji)
	return err
}

func (s *Store) RemoveVote(ctx context.Context, messageID, userID string, universeID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM poll_votes WHERE message_id = ? AND user_id = ? AND universe_id = ?`, messageID, userID, universeID)
	return err
}

func (s *Store) HasVote(ctx context.Context, messageID, userID string, universeID int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM poll_votes WHERE message_id = ? AND user_id = ? AND universe_id = ? LIMIT 1`, messageID, userID, universeID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetUserVotes(ctx context.Context, messageID, userID string) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT universe_id FROM poll_votes WHERE message_id = ? AND user_id = ?`, messageID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) GetVotesByMessage(ctx context.Context, messageID string) ([]model.Vote, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT message_id, user_id, universe_id, emoji, created_at FROM poll_votes WHERE message_id = ?`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Vote
	for rows.Next() {
		var v model.Vote
		var created sql.NullString
		if err := rows.Scan(&v.MessageID, &v.UserID, &v.UniverseID, &v.Emoji, &created); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(created); ok {
			v.CreatedAt = t
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) CountVotesByUniverse(ctx context.Context, messageID string, universeID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM poll_votes WHERE message_id = ? AND universe_id = ?`, messageID, universeID).Scan(&n)
	return n, err
}

func (s *Store) GetVoteCounts(ctx context.Context, messageID string) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT universe_id, COUNT(*) FROM poll_votes WHERE message_id = ? GROUP BY universe_id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]int)
	for rows.Next() {
		var id int64
		var c int
		if err := rows.Scan(&id, &c); err != nil {
			return nil, err
		}
		m[id] = c
	}
	return m, rows.Err()
}

func (s *Store) GetTopVoted(ctx context.Context, messageID string, limit int) ([]model.Experience, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+colsWithPrefix("e.")+`
		FROM poll_votes v
		JOIN experiences e ON e.universe_id = v.universe_id
		WHERE v.message_id = ?
		GROUP BY v.universe_id
		ORDER BY COUNT(*) DESC, MAX(v.created_at) DESC
		LIMIT ?`, messageID, limit)
	if err != nil {
		return nil, err
	}
	exps, err := scanExperiences(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	// If no votes yet, fallback to trending by playing
	if len(exps) == 0 {
		return s.Trending(ctx, limit)
	}
	return exps, nil
}

func (s *Store) UpsertVerifiedUser(ctx context.Context, u model.VerifiedUser) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO verified_users (discord_id, roblox_id, roblox_username, verified_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT(discord_id) DO UPDATE SET roblox_id=excluded.roblox_id, roblox_username=excluded.roblox_username, updated_at=CURRENT_TIMESTAMP", u.DiscordID, u.RobloxID, u.RobloxUsername)
	return err
}

func (s *Store) GetVerifiedUser(ctx context.Context, discordID string) (model.VerifiedUser, error) {
	var u model.VerifiedUser
	var verifiedAt, updatedAt sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT discord_id, roblox_id, roblox_username, verified_at, updated_at FROM verified_users WHERE discord_id = ?", discordID).Scan(&u.DiscordID, &u.RobloxID, &u.RobloxUsername, &verifiedAt, &updatedAt)
	if err != nil {
		return model.VerifiedUser{}, err
	}
	if _, t, ok := parseSQLiteTime(verifiedAt); ok {
		u.VerifiedAt = t
	}
	if _, t, ok := parseSQLiteTime(updatedAt); ok {
		u.UpdatedAt = t
	}
	return u, nil
}

func (s *Store) ListVerifiedUsers(ctx context.Context, limit int) ([]model.VerifiedUser, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT discord_id, roblox_id, roblox_username, verified_at, updated_at FROM verified_users ORDER BY verified_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.VerifiedUser
	for rows.Next() {
		var u model.VerifiedUser
		var verifiedAt, updatedAt sql.NullString
		if err := rows.Scan(&u.DiscordID, &u.RobloxID, &u.RobloxUsername, &verifiedAt, &updatedAt); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(verifiedAt); ok {
			u.VerifiedAt = t
		}
		if _, t, ok := parseSQLiteTime(updatedAt); ok {
			u.UpdatedAt = t
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreateBinding(ctx context.Context, b model.GuildBinding) (int64, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO guild_bindings (guild_id, group_id, roblox_role_id, rank_min, rank_max, discord_role_id, nickname_template) VALUES (?, ?, ?, ?, ?, ?, ?)", b.GuildID, b.GroupID, b.RobloxRoleID, b.RankMin, b.RankMax, b.DiscordRoleID, b.NicknameTemplate)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListBindings(ctx context.Context, guildID string) ([]model.GuildBinding, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, guild_id, group_id, roblox_role_id, rank_min, rank_max, discord_role_id, nickname_template, created_at FROM guild_bindings WHERE guild_id = ? ORDER BY created_at DESC", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.GuildBinding
	for rows.Next() {
		var b model.GuildBinding
		var created sql.NullString
		if err := rows.Scan(&b.ID, &b.GuildID, &b.GroupID, &b.RobloxRoleID, &b.RankMin, &b.RankMax, &b.DiscordRoleID, &b.NicknameTemplate, &created); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(created); ok {
			b.CreatedAt = t
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBinding(ctx context.Context, id int64, guildID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM guild_bindings WHERE id = ? AND guild_id = ?", id, guildID)
	return err
}

