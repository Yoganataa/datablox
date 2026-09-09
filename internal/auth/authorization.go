package auth

// Can checks capability and resource scope validation.
// Default-deny: unknown action, unknown role, invalid resource, or GuildID mismatch → DENY.
// For guild-scoped actions, resource.GuildID must equal principal.GuildID to prevent cross-guild.
func Can(p Principal, action Action, resource Resource) bool {
	if !action.IsKnown() {
		return false
	}
	if action.Scope() == ScopeGlobal {
		return canCapability(p, action)
	}
	// ScopeGuild: GuildID must be present and match principal's GuildID
	if resource.GuildID == "" || p.GuildID == "" {
		return false
	}
	if resource.GuildID != p.GuildID {
		return false
	}
	return canCapability(p, action)
}

// CanTarget checks capability + target policy.
func CanTarget(p Principal, action Action, resource Resource, target Target) bool {
	if !Can(p, action, resource) {
		return false
	}
	return canTarget(p, target, action)
}

// DisplayRole is presentation-only metadata for UI. Never use for authorization.
// Control state must always come from Can()/CanTarget(), not from this value.
// Note: named ResolveDisplayRole (not DisplayRole) because Go forbids
// a type and func sharing the same name in one package.
type DisplayRole struct {
	Label string
	Kind  string
	Badge string
}

// ResolveDisplayRole maps Principal to display metadata.
// Priority: BotOwner > BotAdmin > GuildOwner > GuildAdmin > Member.
// Pure function: no Can()/policy lookup, no I/O.
func ResolveDisplayRole(p Principal) DisplayRole {
	switch {
	case p.BotRole == BotOwner:
		return DisplayRole{Label: "Owner Bot", Kind: "bot-owner", Badge: "primary"}
	case p.BotRole == BotAdmin:
		return DisplayRole{Label: "Admin Bot", Kind: "bot-admin", Badge: "secondary"}
	case p.GuildRole == GuildOwner:
		return DisplayRole{Label: "Owner Server", Kind: "guild-owner", Badge: "secondary"}
	case p.GuildRole == GuildAdmin:
		return DisplayRole{Label: "Admin Server", Kind: "guild-admin", Badge: "secondary"}
	default:
		return DisplayRole{Label: "Member", Kind: "member", Badge: "outline"}
	}
}
