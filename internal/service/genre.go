package service

import (
	"regexp"
	"strings"
)

var genreKeywords = map[string][]string{
	"fishing":   {"fishing", "mancing", "fish", "ikan"},
	"horror":    {"horror", "scary", "scream", "teror", "fnaf"},
	"simulation": {"simulator", "simulasion", "life"},
	"adventure":  {"adventure", "petualangan", "explore", "exploration"},
	"shooting":   {"shooting", "gun", "fps", "tembak", "zombie", "apocalypse"},
	"battle":     {"battle", "combat", "anime", "fighting", "pvp"},
	"tycoon":     {"tycoon", "bisnis"},
	"obby":       {"obby", "jump", "parkour", "impressive"},
	"racing":     {"racing", "balap", "driving", "kendaraan"},
	"rpg":        {"rpg", "roleplay", "high school", "brookhaven"},
	"survival":   {"survival", "bertahan", "endless"},
	"mining":     {"mining", "tambang", "mine"},
	"pet":        {"pet", "adopt"},
	"strategy":   {"strategy", "tower defense", "tds"},
}

// deterministic genre ordering for stable tie-breaks (longer/more specific first)
var genreOrder = []string{
	"fishing", "horror", "shooting", "strategy", "racing", "simulation",
	"adventure", "battle", "rpg", "survival", "mining", "tycoon", "obby", "pet",
}

var keywordRegexes = func() map[string][]*regexp.Regexp {
	m := make(map[string][]*regexp.Regexp)
	for genre, kws := range genreKeywords {
		for _, kw := range kws {
			m[genre] = append(m[genre], regexp.MustCompile(`\b`+regexp.QuoteMeta(kw)))
		}
	}
	return m
}()

// NormalizeGenre lowercases and trims; returns "" if unknown.
func NormalizeGenre(g string) string {
	return strings.ToLower(strings.TrimSpace(g))
}

// InferGenre guesses a genre from name+description using keyword scoring.
// Uses word-boundary matching so "scary" does not trigger racing's "car".
func InferGenre(name, description string) string {
	text := strings.ToLower(name + " " + description)
	best, bestScore := "", 0
	for _, genre := range genreOrder {
		score := 0
		for _, re := range keywordRegexes[genre] {
			if re.MatchString(text) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = genre, score
		}
	}
	if bestScore == 0 {
		return "other"
	}
	return best
}

// IsKnownGenre reports whether g is one of the supported genres.
func IsKnownGenre(g string) bool {
	g = NormalizeGenre(g)
	if g == "" {
		return true
	}
	if _, ok := genreKeywords[g]; ok {
		return true
	}
	return g == "other"
}

// ValidGenres returns the supported genre strings in stable order.
func ValidGenres() []string {
	out := make([]string, 0, len(genreOrder)+1)
	out = append(out, genreOrder...)
	out = append(out, "other")
	return out
}