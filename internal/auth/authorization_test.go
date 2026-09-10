package auth

import "testing"

func testPrincipal(botRole BotRole, guildRole GuildRole, guildID string) Principal {
	return Principal{
		UserID:    "executor",
		GuildID:   guildID,
		BotRole:   botRole,
		GuildRole: guildRole,
		Permissions: Permissions{
			ManageGuild: true, ManageRoles: true, ManageMessages: true,
			ModerateMembers: true, BanMembers: true, KickMembers: true,
		},
	}
}

func testPrincipalPerms(botRole BotRole, guildRole GuildRole, guildID string, perms Permissions) Principal {
	return Principal{
		UserID:      "executor",
		GuildID:     guildID,
		BotRole:     botRole,
		GuildRole:   guildRole,
		Permissions: perms,
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
		{"BotAdmin+GuildAdmin", testPrincipal(BotAdmin, GuildAdmin, g), true},
		{"BotAdmin+GuildMember", testPrincipal(BotAdmin, GuildMember, g), false},
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
		{"BotAdmin->Member ALLOW", testPrincipal(BotAdmin, GuildAdmin, g), testTarget(BotNone, GuildMember, ""), true},
		{"BotAdmin->GuildAdmin ALLOW", testPrincipal(BotAdmin, GuildAdmin, g), testTarget(BotNone, GuildAdmin, ""), true},
		{"BotAdmin->GuildOwner DENY", testPrincipal(BotAdmin, GuildAdmin, g), testTarget(BotNone, GuildOwner, ""), false},
		{"BotOwner->GuildOwner ALLOW", testPrincipal(BotOwner, GuildMember, g), testTarget(BotNone, GuildOwner, ""), true},
		{"BotOwner->BotOwner ALLOW", testPrincipal(BotOwner, GuildMember, g), testTarget(BotOwner, GuildMember, ""), true},
		{"target BotOwner DENY except BotOwner", testPrincipal(BotAdmin, GuildAdmin, g), testTarget(BotOwner, GuildMember, ""), false},
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

func TestCanPermissionGating(t *testing.T) {
	g := "g1"
	res := Resource{GuildID: g}
	full := Permissions{
		ManageGuild: true, ManageRoles: true, ManageMessages: true,
		ModerateMembers: true, BanMembers: true, KickMembers: true,
	}
	none := Permissions{}
	cases := []struct {
		name   string
		p      Principal
		action Action
		want   bool
	}{
		{"GuildAdmin+Ban ban ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{BanMembers: true}), ModerationBan, true},
		{"GuildAdmin no perms ban DENY", testPrincipalPerms(BotNone, GuildAdmin, g, none), ModerationBan, false},
		{"GuildAdmin+Kick kick ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{KickMembers: true}), ModerationKick, true},
		{"GuildAdmin no perms kick DENY", testPrincipalPerms(BotNone, GuildAdmin, g, none), ModerationKick, false},
		{"GuildAdmin+Moderate mute ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{ModerateMembers: true}), ModerationMute, true},
		{"GuildAdmin no perms mute DENY", testPrincipalPerms(BotNone, GuildAdmin, g, none), ModerationMute, false},
		{"GuildAdmin+ManageMessages purge ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{ManageMessages: true}), ModerationPurge, true},
		{"GuildAdmin no perms purge DENY", testPrincipalPerms(BotNone, GuildAdmin, g, none), ModerationPurge, false},
		{"GuildAdmin+ManageGuild modules.manage ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{ManageGuild: true}), GuildModulesManage, true},
		{"GuildAdmin no perms modules.manage DENY", testPrincipalPerms(BotNone, GuildAdmin, g, none), GuildModulesManage, false},
		{"GuildAdmin+ManageRoles reaction.manage ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{ManageRoles: true}), GuildReactionManage, true},
		{"GuildAdmin+ManageGuild reaction.manage DENY", testPrincipalPerms(BotNone, GuildAdmin, g, Permissions{ManageGuild: true}), GuildReactionManage, false},
		{"GuildOwner no perms ban ALLOW (owner semantics)", testPrincipalPerms(BotNone, GuildOwner, g, none), ModerationBan, true},
		{"GuildOwner no perms modules.manage ALLOW", testPrincipalPerms(BotNone, GuildOwner, g, none), GuildModulesManage, true},
		{"GuildAdmin no perms panel.view ALLOW", testPrincipalPerms(BotNone, GuildAdmin, g, none), GuildPanelView, true},
		{"GuildAdmin no perms warn ALLOW (DB-only)", testPrincipalPerms(BotNone, GuildAdmin, g, none), ModerationWarn, true},
		{"BotAdmin+GuildMember no perms ban DENY (no bypass)", testPrincipalPerms(BotAdmin, GuildMember, g, none), ModerationBan, false},
		{"Member full perms ban DENY", testPrincipalPerms(BotNone, GuildMember, g, full), ModerationBan, false},
		{"BotAdmin+GuildMember no ManageGuild modules.manage DENY", testPrincipalPerms(BotAdmin, GuildMember, g, none), GuildModulesManage, false},
		{"BotAdmin+GuildMember no ManageRoles reaction.manage DENY", testPrincipalPerms(BotAdmin, GuildMember, g, none), GuildReactionManage, false},
		{"BotAdmin+GuildMember no ModerateMembers mute DENY", testPrincipalPerms(BotAdmin, GuildMember, g, none), ModerationMute, false},
		{"BotAdmin+GuildMember no KickMembers kick DENY", testPrincipalPerms(BotAdmin, GuildMember, g, none), ModerationKick, false},
		{"BotAdmin+GuildMember no ManageMessages purge DENY", testPrincipalPerms(BotAdmin, GuildMember, g, none), ModerationPurge, false},
		{"BotAdmin+GuildAdmin+BanMembers ban ALLOW (perm honored)", testPrincipalPerms(BotAdmin, GuildAdmin, g, Permissions{BanMembers: true}), ModerationBan, true},
		{"BotAdmin+GuildMember+BanMembers ban DENY (Member role, staff grants nothing)", testPrincipalPerms(BotAdmin, GuildMember, g, Permissions{BanMembers: true}), ModerationBan, false},
		{"GuildMember+BanMembers ban DENY (role gate, perm insufficient)", testPrincipalPerms(BotNone, GuildMember, g, Permissions{BanMembers: true}), ModerationBan, false},
		{"GuildMember+KickMembers kick DENY", testPrincipalPerms(BotNone, GuildMember, g, Permissions{KickMembers: true}), ModerationKick, false},
		{"GuildMember+ModerateMembers mute DENY", testPrincipalPerms(BotNone, GuildMember, g, Permissions{ModerateMembers: true}), ModerationMute, false},
		{"GuildMember+ManageMessages purge DENY", testPrincipalPerms(BotNone, GuildMember, g, Permissions{ManageMessages: true}), ModerationPurge, false},
		{"GuildMember+ManageGuild modules.manage DENY", testPrincipalPerms(BotNone, GuildMember, g, Permissions{ManageGuild: true}), GuildModulesManage, false},
		{"GuildMember+ManageRoles reaction.manage DENY", testPrincipalPerms(BotNone, GuildMember, g, Permissions{ManageRoles: true}), GuildReactionManage, false},
		{"BotAdmin+GuildAdmin no perms ban DENY (no role fallback)", testPrincipalPerms(BotAdmin, GuildAdmin, g, none), ModerationBan, false},
		{"BotOwner no perms ban ALLOW (global bypass)", testPrincipalPerms(BotOwner, GuildMember, g, none), ModerationBan, true},
	}
	for _, tc := range cases {
		if got := Can(tc.p, tc.action, res); got != tc.want {
			t.Errorf("%s: Can(%s) = %v, want %v", tc.name, tc.action, got, tc.want)
		}
	}
}
