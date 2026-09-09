package auth

import "testing"

func testPrincipal(botRole BotRole, guildRole GuildRole, guildID string) Principal {
	return Principal{
		UserID:    "executor",
		GuildID:   guildID,
		BotRole:   botRole,
		GuildRole: guildRole,
	}
}

func testTarget(botRole BotRole, guildRole GuildRole, userID string) Target {
	if userID == "" {
		userID = "target"
	}
	return Target{UserID: userID, BotRole: botRole, GuildRole: guildRole}
}

func TestCanModerationBanCapability(t *testing.T) {
	g := "g1"
	cases := []struct {
		name string
		p    Principal
		want bool
	}{
		{"BotOwner", testPrincipal(BotOwner, GuildMember, g), true},
		{"BotAdmin", testPrincipal(BotAdmin, GuildMember, g), true},
		{"GuildOwner", testPrincipal(BotNone, GuildOwner, g), true},
		{"GuildAdmin", testPrincipal(BotNone, GuildAdmin, g), true},
		{"Member", testPrincipal(BotNone, GuildMember, g), false},
	}
	for _, tc := range cases {
		if got := Can(tc.p, ModerationBan, Resource{GuildID: g}); got != tc.want {
			t.Errorf("%s: Can(ban) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCanTargetBanMatrix(t *testing.T) {
	g := "g1"
	res := Resource{GuildID: g}
	cases := []struct {
		name string
		exec Principal
		tgt  Target
		want bool
	}{
		{"GuildAdmin->Member ALLOW", testPrincipal(BotNone, GuildAdmin, g), testTarget(BotNone, GuildMember, ""), true},
		{"GuildAdmin->GuildAdmin DENY", testPrincipal(BotNone, GuildAdmin, g), testTarget(BotNone, GuildAdmin, ""), false},
		{"GuildOwner->Member ALLOW", testPrincipal(BotNone, GuildOwner, g), testTarget(BotNone, GuildMember, ""), true},
		{"GuildOwner->GuildAdmin ALLOW", testPrincipal(BotNone, GuildOwner, g), testTarget(BotNone, GuildAdmin, ""), true},
		{"GuildOwner->GuildOwner DENY", testPrincipal(BotNone, GuildOwner, g), testTarget(BotNone, GuildOwner, ""), false},
		{"BotAdmin->Member ALLOW", testPrincipal(BotAdmin, GuildMember, g), testTarget(BotNone, GuildMember, ""), true},
		{"BotAdmin->GuildAdmin ALLOW", testPrincipal(BotAdmin, GuildMember, g), testTarget(BotNone, GuildAdmin, ""), true},
		{"BotAdmin->GuildOwner DENY", testPrincipal(BotAdmin, GuildMember, g), testTarget(BotNone, GuildOwner, ""), false},
		{"BotOwner->GuildOwner ALLOW", testPrincipal(BotOwner, GuildMember, g), testTarget(BotNone, GuildOwner, ""), true},
		{"BotOwner->BotOwner ALLOW", testPrincipal(BotOwner, GuildMember, g), testTarget(BotOwner, GuildMember, ""), true},
		{"target BotOwner DENY except BotOwner", testPrincipal(BotAdmin, GuildMember, g), testTarget(BotOwner, GuildMember, ""), false},
		{"target BotAdmin DENY except BotOwner", testPrincipal(BotNone, GuildOwner, g), testTarget(BotAdmin, GuildMember, ""), false},
		{"target missing DENY", testPrincipal(BotOwner, GuildMember, g), Target{}, false},
	}
	for _, tc := range cases {
		if got := CanTarget(tc.exec, ModerationBan, res, tc.tgt); got != tc.want {
			t.Errorf("%s: CanTarget(ban) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCanCrossGuildDeny(t *testing.T) {
	p := testPrincipal(BotNone, GuildAdmin, "guild-a")
	if Can(p, ModerationBan, Resource{GuildID: "guild-b"}) {
		t.Errorf("cross-guild Can should DENY")
	}
	if Can(p, ModerationBan, Resource{GuildID: ""}) {
		t.Errorf("empty GuildID should DENY")
	}
	var unknown Action = "unknown.action"
	if Can(p, unknown, Resource{GuildID: "guild-a"}) {
		t.Errorf("unknown action should DENY")
	}
}
