package adapters

import "datablox/internal/auth"

// GuildResolver resolves GuildRole + Permissions from Discord data.
// It does not import discordgo; caller passes already-extracted values.
type GuildResolver struct{}

func NewGuildResolver() *GuildResolver { return &GuildResolver{} }

// ResolveGuildRole returns business GuildRole.
// guildOwnerID is guild.OwnerID, userID is actor, perms is canonical auth.Permissions.
func (r *GuildResolver) ResolveGuildRole(guildOwnerID, userID string, perms auth.Permissions) auth.GuildRole {
	if guildOwnerID != "" && guildOwnerID == userID {
		return auth.GuildOwner
	}
	if perms.Administrator || perms.ManageGuild {
		return auth.GuildAdmin
	}
	return auth.GuildMember
}

// ResolvePrincipal builds Principal from all resolved parts.
// BotRole from BotRoleResolver, GuildRole+Permissions from Discord.
func ResolvePrincipal(userID, guildID string, botRole auth.BotRole, guildRole auth.GuildRole, perms auth.Permissions) auth.Principal {
	return auth.Principal{
		UserID:      userID,
		GuildID:     guildID,
		BotRole:     botRole,
		GuildRole:   guildRole,
		Permissions: perms,
	}
}
