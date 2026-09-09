package auth

// Capability policy: who can do what (without target).
// BotOwner is the only role that can manage bot admins/owner.

func canCapability(p Principal, action Action) bool {
	// BotOwner can do everything
	if p.BotRole == BotOwner {
		return true
	}

	switch action {
	case BotAdminsManage, BotOwnerManage:
		// only BotOwner, already handled
		return false

	case GuildPanelView, GuildModulesView,
		GuildAutomodView,
		GuildWelcomeView,
		GuildLevelsView,
		GuildVerifyView,
		GuildFeedView,
		GuildBindingsView,
		GuildReactionView,
		ModerationWarn:
		// Views and warn (DB-only) need no Discord permission beyond the role.
		if p.BotRole == BotAdmin {
			return true
		}
		return p.GuildRole == GuildOwner || p.GuildRole == GuildAdmin

	case GuildModulesManage,
		GuildAutomodManage,
		GuildWelcomeManage,
		GuildLevelsManage,
		GuildVerifyManage,
		GuildFeedManage,
		GuildBindingsManage:
		return guildPerm(p, func(perms Permissions) bool { return perms.ManageGuild })
	case GuildReactionManage:
		return guildPerm(p, func(perms Permissions) bool { return perms.ManageRoles })
	case ModerationMute:
		return guildPerm(p, func(perms Permissions) bool { return perms.ModerateMembers })
	case ModerationKick:
		return guildPerm(p, func(perms Permissions) bool { return perms.KickMembers })
	case ModerationBan:
		return guildPerm(p, func(perms Permissions) bool { return perms.BanMembers })
	case ModerationPurge:
		return guildPerm(p, func(perms Permissions) bool { return perms.ManageMessages })
	default:
		return false
	}
}

// guildPerm gates GuildAdmin on the required Discord permission.
// BotAdmin bypasses (global staff) and GuildOwner bypasses (Discord owner
// semantics: the owner implicitly holds every permission).
func guildPerm(p Principal, need func(Permissions) bool) bool {
	if p.BotRole == BotAdmin {
		return true
	}
	if p.GuildRole == GuildOwner {
		return true
	}
	if p.GuildRole != GuildAdmin {
		return false
	}
	return need(p.Permissions)
}
