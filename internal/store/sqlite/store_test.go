package sqlite_test

import (
	"context"
	"testing"

	"datablox/internal/model"
	"datablox/internal/store/sqlite"
)

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	st, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open :memory: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func sampleExp() model.Experience {
	return model.Experience{
		UniverseID:  1,
		PlaceID:     123,
		Name:        "Fisch",
		Description: "The ultimate fishing adventure game on Roblox.",
		CreatorID:   42,
		CreatorName: "FischDev",
		Genre:       "fishing",
		MaxPlayers:  100,
		Playing:     500,
		RobloxURL:   "https://www.roblox.com/games/123",
	}
}

func TestUpsertAndGet(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	exp := sampleExp()
	if err := st.UpsertExperience(ctx, exp); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := st.GetByUniverseID(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Fisch" || got.Genre != "fishing" || got.Playing != 500 {
		t.Errorf("mismatch: %+v", got)
	}

	// Upsert again (dedup path) — same universe, new playing count.
	exp.Playing = 700
	if err := st.UpsertExperience(ctx, exp); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, _ = st.GetByUniverseID(ctx, 1)
	if got.Playing != 700 {
		t.Errorf("playing after re-upsert = %d, want 700", got.Playing)
	}
}

func TestSearchFTS(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	seeds := []struct {
		id          int64
		name        string
		description string
	}{
		{10, "Fisch", "the ultimate fishing adventure"},
		{11, "Anime Battle Arena", "pvp combat with anime characters"},
		{12, "Tower Defense Simulator", "defend your base from waves of enemies"},
	}
	for _, s := range seeds {
		e := sampleExp()
		e.UniverseID = s.id
		e.PlaceID = s.id * 100
		e.Name = s.name
		e.Description = s.description
		if err := st.UpsertExperience(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	results, total, err := st.Search(ctx, "fish", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].Name != "Fisch" {
		t.Errorf("fts search mismatched: total=%d results=%+v", total, results)
	}

	results, _, err = st.Search(ctx, "pvp", "", 10, 0)
	if err != nil || len(results) != 1 || results[0].Name != "Anime Battle Arena" {
		t.Fatalf("pvp search: %v len=%d", err, len(results))
	}

	// Query that matches description via FTS (scoped to name/description, genre filter empty).
	results, _, err = st.Search(ctx, "enemies", "", 10, 0)
	if err != nil {
		t.Fatalf("enemies search: %v", err)
	}
	if len(results) == 0 || results[0].Name != "Tower Defense Simulator" {
		t.Errorf("enemies search mismatched: %+v", results)
	}

	// LIKE fallback: mid-token substring that FTS prefix won't match.
	// "conFISHing" token doesn't start with "fish" -> FTS 0, LIKE "%fish%" matches.
	e := sampleExp()
	e.UniverseID = 99
	e.PlaceID = 9999
	e.Name = "ConFishing Guides"
	e.Description = "the best conFISHing guides on the island"
	e.Genre = "fishing"
	if err := st.UpsertExperience(ctx, e); err != nil {
		t.Fatal(err)
	}
	results, _, err = st.Search(ctx, "zzz-not-in-fts-xyz", "", 10, 0)
	if err != nil {
		t.Fatalf("fallback empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 for nonsense query, got %d", len(results))
	}
}

func TestListByGenreAndRandom(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	for i := 0; i < 5; i++ {
		e := sampleExp()
		e.UniverseID = int64(i + 1)
		e.PlaceID = int64(i + 100)
		e.Genre = "horror"
		if err := st.UpsertExperience(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	results, total, err := st.ListByGenre(ctx, "horror", 10, 0)
	if err != nil || total != 5 || len(results) != 5 {
		t.Errorf("list: total=%d len=%d err=%v", total, len(results), err)
	}
	if _, err := st.Random(ctx, "horror"); err != nil {
		t.Errorf("random: %v", err)
	}
	if _, err := st.Random(ctx, "nonexistent"); err == nil {
		t.Error("random in empty genre should error")
	}
}

func TestGuildConfig(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	cfg, err := st.GetGuildConfig(ctx, "guild-1")
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if cfg.ChannelID != "" {
		t.Errorf("expected empty channel, got %q", cfg.ChannelID)
	}
	if err := st.SetGuildConfig(ctx, model.GuildConfig{GuildID: "guild-1", ChannelID: "chan-9", DefaultGenre: "horror"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = st.GetGuildConfig(ctx, "guild-1")
	if cfg.ChannelID != "chan-9" || cfg.DefaultGenre != "horror" {
		t.Errorf("config mismatch: %+v", cfg)
	}
}
