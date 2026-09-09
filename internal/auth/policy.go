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

	case GuildPanelView, GuildModulesView, GuildModulesManage,
		GuildAutomodView, GuildAutomodManage,
		GuildWelcomeView, GuildWelcomeManage,
		GuildLevelsView, GuildLevelsManage,
		GuildVerifyView, GuildVerifyManage,
		GuildFeedView, GuildFeedManage,
		GuildBindingsView, GuildBindingsManage,
		GuildReactionView, GuildReactionManage,
		ModerationWarn, ModerationMute, ModerationKick, ModerationBan, ModerationPurge:
		// BotAdmin, GuildOwner, GuildAdmin can manage/view guild resources
		if p.BotRole == BotAdmin {
			return true
		}
		if p.GuildRole == GuildOwner || p.GuildRole == GuildAdmin {
			return true
		}
		return false
	default:
		return false
	}
}
