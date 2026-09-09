package auth

// Target policy: whether executor can act on target (separate from capability).
// Hard lock: BotAdmin cannot target GuildOwner, GuildOwner cannot target Bot*, etc.

func canTarget(executor Principal, target Target, action Action) bool {
	// Only user-targeted actions require target checks.
	// purge has no target-user hierarchy, so it never reaches here.
	switch action {
	case ModerationWarn, ModerationMute, ModerationKick, ModerationBan,
		BotAdminsManage, BotOwnerManage:
		// fall through to target checks
	default:
		return true
	}

	// Missing target is always DENY (covers empty UserID, unknown user).
	if target.UserID == "" {
		return false
	}

	// BotOwner can target everyone
	if executor.BotRole == BotOwner {
		return true
	}

	// BotAdmin targets
	if executor.BotRole == BotAdmin {
		if target.BotRole == BotOwner || target.BotRole == BotAdmin {
			return false
		}
		if target.GuildRole == GuildOwner {
			return false
		}
		if target.GuildRole == GuildAdmin || target.GuildRole == GuildMember {
			return true
		}
		return false
	}

	// GuildOwner targets
	if executor.GuildRole == GuildOwner {
		if target.BotRole != BotNone {
			return false
		}
		if target.GuildRole == GuildOwner {
			return false
		}
		if target.GuildRole == GuildAdmin || target.GuildRole == GuildMember {
			return true
		}
		return false
	}

	// GuildAdmin targets
	if executor.GuildRole == GuildAdmin {
		if target.BotRole != BotNone {
			return false
		}
		if target.GuildRole == GuildMember {
			return true
		}
		return false
	}

	return false
}
