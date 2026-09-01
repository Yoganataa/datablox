package service

import "testing"

func TestInferGenre(t *testing.T) {
	cases := []struct {
		name, desc, want string
	}{
		{"Fisch", "the ultimate fishing adventure", "fishing"},
		{"Manic Maning", "mancing di pulau terpencil", "fishing"},
		{"Dead Silence", "scary horror game", "horror"},
		{"Anime Battle Arena", "pvp combat with anime characters", "battle"},
		{"Tower Defense Simulator", "strategy tower defense", "strategy"},
		{"Random Obby", "jump through parkour obby", "obby"},
		{"No Keywords Here", "just a plain description", "other"},
	}
	for _, c := range cases {
		if got := InferGenre(c.name, c.desc); got != c.want {
			t.Errorf("InferGenre(%q, %q) = %q, want %q", c.name, c.desc, got, c.want)
		}
	}
}

func TestIsKnownGenre(t *testing.T) {
	if !IsKnownGenre("fishing") {
		t.Error("fishing should be known")
	}
	if IsKnownGenre("nonsense") {
		t.Error("nonsense should not be known")
	}
	if !IsKnownGenre("") {
		t.Error("empty should be allowed (auto-infer)")
	}
}

func TestFormatCount(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567"}
	for in, want := range cases {
		if got := FormatCount(in); got != want {
			t.Errorf("FormatCount(%d) = %q, want %q", in, got, want)
		}
	}
}