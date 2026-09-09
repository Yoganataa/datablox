package auth

import "testing"

func TestResolveDisplayRolePriority(t *testing.T) {
	cases := []struct {
		name  string
		p     Principal
		label string
		kind  string
	}{
		{"BotOwner wins over GuildOwner", Principal{BotRole: BotOwner, GuildRole: GuildOwner}, "Owner Bot", "bot-owner"},
		{"BotOwner+Member", Principal{BotRole: BotOwner, GuildRole: GuildMember}, "Owner Bot", "bot-owner"},
		{"BotAdmin wins over GuildOwner", Principal{BotRole: BotAdmin, GuildRole: GuildOwner}, "Admin Bot", "bot-admin"},
		{"BotAdmin+Member", Principal{BotRole: BotAdmin, GuildRole: GuildMember}, "Admin Bot", "bot-admin"},
		{"GuildOwner", Principal{BotRole: BotNone, GuildRole: GuildOwner}, "Owner Server", "guild-owner"},
		{"GuildAdmin", Principal{BotRole: BotNone, GuildRole: GuildAdmin}, "Admin Server", "guild-admin"},
		{"Member", Principal{BotRole: BotNone, GuildRole: GuildMember}, "Member", "member"},
	}
	for _, tc := range cases {
		got := ResolveDisplayRole(tc.p)
		if got.Label != tc.label || got.Kind != tc.kind {
			t.Errorf("%s: got %+v, want label=%q kind=%q", tc.name, got, tc.label, tc.kind)
		}
		if got.Badge == "" {
			t.Errorf("%s: Badge must not be empty", tc.name)
		}
	}
}
