package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"datablox/internal/model"
)

func cols() string {
	return `universe_id, place_id, name, description, creator_id, creator_name,
		genre, max_players, playing, visits, up_votes, down_votes,
		thumbnail_url, roblox_url, roblox_created, roblox_updated,
		last_refreshed_at, created_at`
}

func colsWithPrefix(p string) string {
	cols := []string{
		"universe_id", "place_id", "name", "description", "creator_id", "creator_name",
		"genre", "max_players", "playing", "visits", "up_votes", "down_votes",
		"thumbnail_url", "roblox_url", "roblox_created", "roblox_updated",
		"last_refreshed_at", "created_at",
	}
	prefixed := make([]string, len(cols))
	for i, c := range cols {
		prefixed[i] = p + c
	}
	return strings.Join(prefixed, ", ")
}

func scanExperiences(rows *sql.Rows) ([]model.Experience, error) {
	var out []model.Experience
	for rows.Next() {
		e, err := scanExperienceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanExperience(row interface{ Scan(...any) error }) (model.Experience, error) {
	return scanExperienceRow(row)
}

func scanExperienceRow(row interface{ Scan(...any) error }) (model.Experience, error) {
	var e model.Experience
	var description, creatorName, thumbnailURL, robloxURL sql.NullString
	var creatorID sql.NullInt64
	var visits, upVotes, downVotes sql.NullInt64
	var robloxCreated, robloxUpdated, lastRefreshed, createdAt sql.NullString
	err := row.Scan(
		&e.UniverseID, &e.PlaceID, &e.Name, &description, &creatorID, &creatorName,
		&e.Genre, &e.MaxPlayers, &e.Playing, &visits, &upVotes, &downVotes,
		&thumbnailURL, &robloxURL, &robloxCreated, &robloxUpdated,
		&lastRefreshed, &createdAt,
	)
	if err != nil {
		return model.Experience{}, err
	}
	e.Description = description.String
	e.CreatorID = creatorID.Int64
	e.CreatorName = creatorName.String
	e.Visits = visits.Int64
	e.UpVotes = upVotes.Int64
	e.DownVotes = downVotes.Int64
	e.ThumbnailURL = thumbnailURL.String
	e.RobloxURL = robloxURL.String
	if _, t, ok := parseSQLiteTime(robloxCreated); ok {
		e.RobloxCreated = t
	}
	if _, t, ok := parseSQLiteTime(robloxUpdated); ok {
		e.RobloxUpdated = t
	}
	if _, t, ok := parseSQLiteTime(lastRefreshed); ok {
		e.LastRefreshedAt = t
	}
	if _, t, ok := parseSQLiteTime(createdAt); ok {
		e.CreatedAt = t
	}
	return e, nil
}

var sqliteTimeLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
}

func parseSQLiteTime(v sql.NullString) (bool, time.Time, bool) {
	if !v.Valid || v.String == "" {
		return false, time.Time{}, false
	}
	for _, layout := range sqliteTimeLayouts {
		if t, err := time.Parse(layout, v.String); err == nil {
			return true, t, true
		}
	}
	return true, time.Time{}, false
}

// toFTSQuery builds a safe FTS5 MATCH expression scoped to name/description.
// Scoping avoids false positives from the genre column (e.g. query "fish"
// matching genre "fishing" on every row).
func toFTSQuery(q string) string {
	parts := strings.Fields(q)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Map(func(r rune) rune {
			switch r {
			case '"', '*', ':', '(', ')', '-', '^':
				return ' '
			}
			return r
		}, p)
		if p == "" {
			continue
		}
		tokens = append(tokens, fmt.Sprintf("%q*", p))
	}
	if len(tokens) == 0 {
		return q
	}
	joined := strings.Join(tokens, " AND ")
	return fmt.Sprintf("(name:(%s) OR description:(%s))", joined, joined)
}
