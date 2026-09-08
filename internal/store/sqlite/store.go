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
	if sqlB, err := migrationsFS.ReadFile("migrations/0005_bloxlink.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column name") {
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
	if sqlB, err := migrationsFS.ReadFile("migrations/0007_reaction_roles.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	if sqlB, err := migrationsFS.ReadFile("migrations/0008_manager.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	if sqlB, err := migrationsFS.ReadFile("migrations/0009_module_registry.sql"); err == nil {
		for _, stmt := range strings.Split(string(sqlB), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate column name") && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
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

func (s *Store) CountGuilds(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM guild_config WHERE channel_id != '' OR verify_channel_id != ''`).Scan(&n)
	return n, err
}

func (s *Store) CountVotes(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM poll_votes`).Scan(&n)
	return n, err
}

func (s *Store) CountVerifiedUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM verified_users`).Scan(&n)
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

func (s *Store) ListGuildConfigsByIDs(ctx context.Context, ids []string) ([]model.GuildConfig, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT guild_id, channel_id, vote_message_id, top_message_id, verify_channel_id, verify_message_id, default_genre FROM guild_config WHERE guild_id IN (` + placeholders[0]
	for _, ph := range placeholders[1:] {
		query += "," + ph
	}
	query += ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
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

func (s *Store) ToggleVote(ctx context.Context, v model.Vote) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var one int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM poll_votes WHERE message_id = ? AND user_id = ? AND universe_id = ? LIMIT 1`, v.MessageID, v.UserID, v.UniverseID).Scan(&one)
	if err == nil {
		// exists -> remove
		if _, err := tx.ExecContext(ctx, `DELETE FROM poll_votes WHERE message_id = ? AND user_id = ? AND universe_id = ?`, v.MessageID, v.UserID, v.UniverseID); err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO poll_votes (message_id, user_id, universe_id, emoji) VALUES (?, ?, ?, ?)`, v.MessageID, v.UserID, v.UniverseID, v.Emoji); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
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

func (s *Store) CreateReactionRole(ctx context.Context, r model.ReactionRole) (int64, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO reaction_roles (guild_id, channel_id, message_id, emoji, role_id, mode) VALUES (?, ?, ?, ?, ?, ?)", r.GuildID, r.ChannelID, r.MessageID, r.Emoji, r.RoleID, r.Mode)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListReactionRoles(ctx context.Context, guildID string) ([]model.ReactionRole, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, guild_id, channel_id, message_id, emoji, role_id, mode, created_at FROM reaction_roles WHERE guild_id = ? ORDER BY created_at DESC", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ReactionRole
	for rows.Next() {
		var r model.ReactionRole
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.GuildID, &r.ChannelID, &r.MessageID, &r.Emoji, &r.RoleID, &r.Mode, &created); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(created); ok {
			r.CreatedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListReactionRolesByMessage(ctx context.Context, guildID, messageID string) ([]model.ReactionRole, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, guild_id, channel_id, message_id, emoji, role_id, mode, created_at FROM reaction_roles WHERE guild_id = ? AND message_id = ?", guildID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ReactionRole
	for rows.Next() {
		var r model.ReactionRole
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.GuildID, &r.ChannelID, &r.MessageID, &r.Emoji, &r.RoleID, &r.Mode, &created); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(created); ok {
			r.CreatedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteReactionRole(ctx context.Context, id int64, guildID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM reaction_roles WHERE id = ? AND guild_id = ?", id, guildID)
	return err
}
func (s *Store) CreateInfraction(ctx context.Context, inf model.Infraction) (int64, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO infractions (guild_id, user_id, moderator_id, type, reason, expires_at, active) VALUES (?, ?, ?, ?, ?, ?, 1)", inf.GuildID, inf.UserID, inf.ModeratorID, inf.Type, inf.Reason, inf.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListInfractions(ctx context.Context, guildID, userID string) ([]model.Infraction, error) {
	q := "SELECT id, guild_id, user_id, moderator_id, type, reason, created_at, expires_at, active FROM infractions WHERE guild_id = ?"
	args := []any{guildID}
	if userID != "" {
		q += " AND user_id = ?"
		args = append(args, userID)
	}
	q += " ORDER BY created_at DESC LIMIT 100"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Infraction
	for rows.Next() {
		var inf model.Infraction
		var created sql.NullString
		var expires sql.NullString
		var active int
		if err := rows.Scan(&inf.ID, &inf.GuildID, &inf.UserID, &inf.ModeratorID, &inf.Type, &inf.Reason, &created, &expires, &active); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(created); ok {
			inf.CreatedAt = t
		}
		if _, t, ok := parseSQLiteTime(expires); ok {
			inf.ExpiresAt = &t
		}
		inf.Active = active != 0
		out = append(out, inf)
	}
	return out, rows.Err()
}

func (s *Store) GetAutomodConfig(ctx context.Context, guildID string) (model.AutomodConfig, error) {
	var cfg model.AutomodConfig
	err := s.db.QueryRowContext(ctx, "SELECT guild_id, anti_spam, anti_invite, mass_mention, ghost_ping FROM automod_config WHERE guild_id = ?", guildID).Scan(&cfg.GuildID, &cfg.AntiSpam, &cfg.AntiInvite, &cfg.MassMention, &cfg.GhostPing)
	if err == sql.ErrNoRows {
		return model.AutomodConfig{GuildID: guildID, MassMention: 5}, nil
	}
	return cfg, err
}

func (s *Store) SetAutomodConfig(ctx context.Context, cfg model.AutomodConfig) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO automod_config (guild_id, anti_spam, anti_invite, mass_mention, ghost_ping, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(guild_id) DO UPDATE SET anti_spam=excluded.anti_spam, anti_invite=excluded.anti_invite, mass_mention=excluded.mass_mention, ghost_ping=excluded.ghost_ping, updated_at=CURRENT_TIMESTAMP", cfg.GuildID, cfg.AntiSpam, cfg.AntiInvite, cfg.MassMention, cfg.GhostPing)
	return err
}

func (s *Store) GetLevel(ctx context.Context, guildID, userID string) (model.Level, error) {
	var lvl model.Level
	var last sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT guild_id, user_id, xp, level, messages, last_xp_at FROM levels WHERE guild_id = ? AND user_id = ?", guildID, userID).Scan(&lvl.GuildID, &lvl.UserID, &lvl.XP, &lvl.Level, &lvl.Messages, &last)
	if err == sql.ErrNoRows {
		return model.Level{GuildID: guildID, UserID: userID}, nil
	}
	if err != nil {
		return lvl, err
	}
	if _, t, ok := parseSQLiteTime(last); ok {
		lvl.LastXPAt = &t
	}
	return lvl, nil
}

func (s *Store) SetLevel(ctx context.Context, lvl model.Level) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO levels (guild_id, user_id, xp, level, messages, last_xp_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(guild_id, user_id) DO UPDATE SET xp=excluded.xp, level=excluded.level, messages=excluded.messages, last_xp_at=excluded.last_xp_at", lvl.GuildID, lvl.UserID, lvl.XP, lvl.Level, lvl.Messages, lvl.LastXPAt)
	return err
}

func (s *Store) Leaderboard(ctx context.Context, guildID string, limit int) ([]model.Level, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT guild_id, user_id, xp, level, messages, last_xp_at FROM levels WHERE guild_id = ? ORDER BY xp DESC LIMIT ?", guildID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Level
	for rows.Next() {
		var lvl model.Level
		var last sql.NullString
		if err := rows.Scan(&lvl.GuildID, &lvl.UserID, &lvl.XP, &lvl.Level, &lvl.Messages, &last); err != nil {
			return nil, err
		}
		if _, t, ok := parseSQLiteTime(last); ok {
			lvl.LastXPAt = &t
		}
		out = append(out, lvl)
	}
	return out, rows.Err()
}

func (s *Store) GetWelcomeConfig(ctx context.Context, guildID string) (model.WelcomeConfig, error) {
	var cfg model.WelcomeConfig
	var enabled int
	err := s.db.QueryRowContext(ctx, "SELECT guild_id, channel_id, message, auto_role_id, enabled FROM welcome_config WHERE guild_id = ?", guildID).Scan(&cfg.GuildID, &cfg.ChannelID, &cfg.Message, &cfg.AutoRoleID, &enabled)
	if err == sql.ErrNoRows {
		return model.WelcomeConfig{GuildID: guildID}, nil
	}
	cfg.Enabled = enabled != 0
	return cfg, err
}

func (s *Store) SetWelcomeConfig(ctx context.Context, cfg model.WelcomeConfig) error {
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO welcome_config (guild_id, channel_id, message, auto_role_id, enabled, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(guild_id) DO UPDATE SET channel_id=excluded.channel_id, message=excluded.message, auto_role_id=excluded.auto_role_id, enabled=excluded.enabled, updated_at=CURRENT_TIMESTAMP", cfg.GuildID, cfg.ChannelID, cfg.Message, cfg.AutoRoleID, enabled)
	return err
}

func (s *Store) ListModules(ctx context.Context) ([]model.Module, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT slug, name, icon, description FROM modules ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Module
	for rows.Next() {
		var m model.Module
		if err := rows.Scan(&m.Slug, &m.Name, &m.Icon, &m.Description); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListGuildModules(ctx context.Context, guildID string) ([]model.GuildModule, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT guild_id, slug, enabled, config_json, updated_at FROM guild_modules WHERE guild_id = ? ORDER BY slug", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.GuildModule
	for rows.Next() {
		var gm model.GuildModule
		var enabled int
		if err := rows.Scan(&gm.GuildID, &gm.Slug, &enabled, &gm.ConfigJSON, &gm.UpdatedAt); err != nil {
			return nil, err
		}
		gm.Enabled = enabled != 0
		out = append(out, gm)
	}
	if len(out) == 0 {
		// seed defaults if missing (legacy guild)
		mods, _ := s.ListModules(ctx)
		for _, m := range mods {
			_, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO guild_modules (guild_id, slug, enabled) VALUES (?, ?, 0)", guildID, m.Slug)
		}
		return s.ListGuildModules(ctx, guildID)
	}
	return out, rows.Err()
}

func (s *Store) GetGuildModule(ctx context.Context, guildID, slug string) (model.GuildModule, error) {
	var gm model.GuildModule
	var enabled int
	err := s.db.QueryRowContext(ctx, "SELECT guild_id, slug, enabled, config_json, updated_at FROM guild_modules WHERE guild_id = ? AND slug = ?", guildID, slug).Scan(&gm.GuildID, &gm.Slug, &enabled, &gm.ConfigJSON, &gm.UpdatedAt)
	if err != nil {
		return model.GuildModule{GuildID: guildID, Slug: slug}, err
	}
	gm.Enabled = enabled != 0
	return gm, nil
}

func (s *Store) SetGuildModuleEnabled(ctx context.Context, guildID, slug string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO guild_modules (guild_id, slug, enabled, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(guild_id, slug) DO UPDATE SET enabled=excluded.enabled, updated_at=CURRENT_TIMESTAMP", guildID, slug, v)
	return err
}


