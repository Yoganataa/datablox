package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"datablox/internal/model"
	"datablox/internal/roblox"
	"datablox/internal/store"
)

type ExperienceService struct {
	Client *roblox.Client
	Store  store.Store
}

func New(client *roblox.Client, s store.Store) *ExperienceService {
	return &ExperienceService{Client: client, Store: s}
}

type AddResult struct {
	Experience model.Experience
	Created    bool
	Genre      string // final genre (may be inferred)
}

// AddExperience resolves a Roblox URL/ID, fetches metadata (including visits/votes/created/updated), infers genre when
// not provided, persists it and returns the result.
func (svc *ExperienceService) AddExperience(ctx context.Context, rawURL, genre string) (*AddResult, error) {
	placeID, err := roblox.ExtractPlaceID(rawURL)
	if err != nil {
		return nil, err
	}

	// Already known? Avoid a Roblox round-trip.
	if existing, err := svc.Store.GetByPlaceID(ctx, placeID); err == nil {
		return &AddResult{Experience: existing, Created: false, Genre: existing.Genre}, nil
	}

	universeID, err := svc.Client.ResolveUniverseID(ctx, placeID)
	if err != nil {
		return nil, err
	}

	details, err := svc.Client.GetGameDetail(ctx, universeID)
	if err != nil {
		return nil, err
	}
	if len(details) == 0 {
		return nil, fmt.Errorf("experience not found (universe %d)", universeID)
	}
	d := details[0]

	if genre == "" {
		genre = InferGenre(d.Name, d.Description)
	}
	genre = NormalizeGenre(genre)
	if genre == "" {
		genre = "other"
	}

	// Parallel fetch visits/votes + thumbnail after universeID known (details already fetched)
	var thumb string
	var votes map[int64]struct{ Up, Down int64 }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		thumb, _ = svc.Client.GetThumbnailURL(ctx, universeID)
	}()
	go func() {
		defer wg.Done()
		votes, _ = svc.Client.GetVotes(ctx, universeID)
	}()
	wg.Wait()
	v := votes[universeID]

	exp := model.Experience{
		UniverseID:      universeID,
		PlaceID:         placeID,
		Name:            d.Name,
		Description:     d.Description,
		CreatorID:       d.Creator.ID,
		CreatorName:     d.Creator.Name,
		Genre:           genre,
		MaxPlayers:      d.MaxPlayers,
		Playing:         d.Playing,
		Visits:          d.Visits,
		UpVotes:         v.Up,
		DownVotes:       v.Down,
		ThumbnailURL:    thumb,
		RobloxURL:       fmt.Sprintf("https://www.roblox.com/games/%d", placeID),
		RobloxCreated:   parseRobloxTime(d.Created),
		RobloxUpdated:   parseRobloxTime(d.Updated),
		LastRefreshedAt: time.Now().UTC(),
		CreatedAt:       time.Now().UTC(),
	}

	if err := svc.Store.UpsertExperience(ctx, exp); err != nil {
		return nil, err
	}
	return &AddResult{Experience: exp, Created: true, Genre: genre}, nil
}

// RefreshOne re-fetches a single experience by universeID and updates all enrich fields.
func (svc *ExperienceService) RefreshOne(ctx context.Context, universeID int64) (model.Experience, error) {
	details, err := svc.Client.GetGameDetail(ctx, universeID)
	if err != nil {
		return model.Experience{}, err
	}
	if len(details) == 0 {
		return model.Experience{}, fmt.Errorf("experience not found (universe %d)", universeID)
	}
	d := details[0]
	var thumb string
	var votes map[int64]struct{ Up, Down int64 }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); thumb, _ = svc.Client.GetThumbnailURL(ctx, universeID) }()
	go func() { defer wg.Done(); votes, _ = svc.Client.GetVotes(ctx, universeID) }()
	wg.Wait()
	v := votes[universeID]

	existing, err := svc.Store.GetByUniverseID(ctx, universeID)
	if err != nil {
		return model.Experience{}, err
	}

	existing.Name = d.Name
	existing.Description = d.Description
	existing.Playing = d.Playing
	existing.MaxPlayers = d.MaxPlayers
	existing.Visits = d.Visits
	existing.UpVotes = v.Up
	existing.DownVotes = v.Down
	existing.ThumbnailURL = thumb
	existing.RobloxCreated = parseRobloxTime(d.Created)
	existing.RobloxUpdated = parseRobloxTime(d.Updated)
	existing.LastRefreshedAt = time.Now().UTC()

	if err := svc.Store.UpsertExperience(ctx, existing); err != nil {
		return model.Experience{}, err
	}
	return existing, nil
}

// Search proxies to the store, normalizing an empty genre to "".
func (svc *ExperienceService) Search(ctx context.Context, query, genre string, limit int) ([]model.Experience, error) {
	genre = NormalizeGenre(genre)
	results, _, err := svc.Store.Search(ctx, query, genre, limit, 0)
	return results, err
}

// ListByGenre proxies to the store.
func (svc *ExperienceService) ListByGenre(ctx context.Context, genre string, limit int) ([]model.Experience, error) {
	genre = NormalizeGenre(genre)
	results, _, err := svc.Store.ListByGenre(ctx, genre, limit, 0)
	return results, err
}

func (svc *ExperienceService) GetByUniverseID(ctx context.Context, universeID int64) (model.Experience, error) {
	return svc.Store.GetByUniverseID(ctx, universeID)
}

func (svc *ExperienceService) Random(ctx context.Context, genre string) (model.Experience, error) {
	genre = NormalizeGenre(genre)
	return svc.Store.Random(ctx, genre)
}

// RefreshAll re-fetches playing counts, visits, votes and thumbnails in batches of 100.
func (svc *ExperienceService) RefreshAll(ctx context.Context) (int, error) {
	ids, err := svc.Store.AllUniverseIDs(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for i := 0; i < len(ids); i += 100 {
		batch := ids[i:min(i+100, len(ids))]
		details, err := svc.Client.GetGameDetail(ctx, batch...)
		if err != nil {
			continue
		}
		votes, _ := svc.Client.GetVotes(ctx, batch...)
		for _, d := range details {
			thumb, _ := svc.Client.GetThumbnailURL(ctx, d.ID)
			v := votes[d.ID]
			existing, err := svc.Store.GetByUniverseID(ctx, d.ID)
			if err != nil {
				continue
			}
			existing.Name = d.Name
			existing.Description = d.Description
			existing.Playing = d.Playing
			existing.MaxPlayers = d.MaxPlayers
			existing.Visits = d.Visits
			existing.UpVotes = v.Up
			existing.DownVotes = v.Down
			existing.ThumbnailURL = thumb
			existing.RobloxCreated = parseRobloxTime(d.Created)
			existing.RobloxUpdated = parseRobloxTime(d.Updated)
			existing.LastRefreshedAt = time.Now().UTC()
			if err := svc.Store.UpsertExperience(ctx, existing); err == nil {
				updated++
			}
		}
	}
	return updated, nil
}

// SearchNames returns up to `limit` matching names for autocomplete.
func (svc *ExperienceService) SearchNames(ctx context.Context, query string, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	results, err := svc.Search(ctx, query, "", limit)
	if err != nil || len(results) == 0 {
		return nil
	}
	out := make([]string, 0, len(results))
	for _, e := range results {
		out = append(out, e.Name)
	}
	return out
}

func parseRobloxTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatCount renders numbers with a thousands separator.
func FormatCount(n int) string {
	s := strconv.FormatInt(int64(n), 10)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

// FormatCount64 variant for int64.
func FormatCount64(n int64) string {
	return FormatCount(int(n))
}
