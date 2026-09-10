package service

import (
	"testing"
	"time"

	"datablox/internal/model"
	"datablox/internal/roblox"
)

func testDetail() roblox.GameDetail {
	d := roblox.GameDetail{
		ID:          9,
		Name:        "New Name",
		Description: "New Desc",
		Playing:     111,
		Visits:      222,
		MaxPlayers:  33,
		Created:     "2024-01-02T15:04:05Z",
		Updated:     "2024-02-03T15:04:05Z",
	}
	d.Creator.ID = 7
	d.Creator.Name = "Builder"
	return d
}

func testStored() model.Experience {
	return model.Experience{
		UniverseID: 9, PlaceID: 1, Name: "Old", Description: "Old",
		CreatorID: 1, CreatorName: "OldMaker", Genre: "obby",
		Playing: 1, Visits: 100, UpVotes: 50, DownVotes: 5,
		ThumbnailURL:  "https://old.thumb/x.png",
		RobloxCreated: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		RobloxUpdated: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestMergeEnrichmentKeepsPriorOnFailure(t *testing.T) {
	d := testDetail()

	t.Run("votes failure keeps stored votes", func(t *testing.T) {
		e := testStored()
		mergeEnrichment(&e, d, 0, 0, false, "https://new.thumb/y.png", true)
		if e.UpVotes != 50 || e.DownVotes != 5 {
			t.Fatalf("votes overwritten on fetch failure: %+v", e)
		}
		if e.ThumbnailURL != "https://new.thumb/y.png" {
			t.Fatalf("successful fetch not applied: %+v", e)
		}
		if e.Name != "New Name" || e.Playing != 111 {
			t.Fatalf("detail fields not applied: %+v", e)
		}
	})

	t.Run("thumbnail failure keeps stored thumbnail", func(t *testing.T) {
		e := testStored()
		mergeEnrichment(&e, d, 9, 1, true, "", false)
		if e.ThumbnailURL != "https://old.thumb/x.png" {
			t.Fatalf("thumbnail overwritten on fetch failure: %+v", e)
		}
		if e.UpVotes != 9 || e.DownVotes != 1 {
			t.Fatalf("successful votes not applied: %+v", e)
		}
	})

	t.Run("unparseable timestamps keep stored timestamps", func(t *testing.T) {
		e := testStored()
		bad := testDetail()
		bad.Created = "not-a-date"
		bad.Updated = ""
		mergeEnrichment(&e, bad, 1, 0, true, "t", true)
		if !e.RobloxCreated.Equal(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("created overwritten by unparseable value: %+v", e)
		}
		if !e.RobloxUpdated.Equal(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("updated overwritten by empty value: %+v", e)
		}
	})

	t.Run("all success applies everything", func(t *testing.T) {
		e := testStored()
		mergeEnrichment(&e, d, 9, 1, true, "https://new.thumb/y.png", true)
		if e.UpVotes != 9 || e.DownVotes != 1 || e.ThumbnailURL != "https://new.thumb/y.png" {
			t.Fatalf("successful enrichment not applied: %+v", e)
		}
		if e.Name != "New Name" || e.CreatorName != "Builder" {
			t.Fatalf("detail fields not applied: %+v", e)
		}
		if e.LastRefreshedAt.IsZero() {
			t.Fatalf("LastRefreshedAt not stamped")
		}
	})
}
