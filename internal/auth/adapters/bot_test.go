package adapters

import "testing"

import "datablox/internal/auth"

func boolMap(ids ...string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestBotRoleResolverCanonical(t *testing.T) {
	cases := []struct {
		name   string
		owners map[string]bool
		admins map[string]bool
		user   string
		want   auth.BotRole
	}{
		{"owner only", boolMap("1"), boolMap("2"), "1", auth.BotOwner},
		{"admin only", boolMap("1"), boolMap("2"), "2", auth.BotAdmin},
		{"neither", boolMap("1"), boolMap("2"), "3", auth.BotNone},
		{"owner wins on intersection", boolMap("1"), boolMap("1", "2"), "1", auth.BotOwner},
		{"empty sets", boolMap(), boolMap(), "1", auth.BotNone},
	}
	for _, tc := range cases {
		r := NewBotRoleResolverFromBoolMap(tc.owners, tc.admins)
		if got := r.ResolveBotRole(tc.user); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
