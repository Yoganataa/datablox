package discord

import (
	"github.com/bwmarrin/discordgo"

	"datablox/internal/auth"
)

// ExtractPermissions converts raw Discord permission bits to canonical auth.Permissions.
// Single translation point — no handler does `p & 0x..` directly.
func ExtractPermissions(bits int64) auth.Permissions {
	p := auth.Permissions{}
	if bits&discordgo.PermissionAdministrator != 0 {
		p.Administrator = true
		// Administrator implies all
		p.ManageGuild = true
		p.ManageRoles = true
		p.ManageChannels = true
		p.ManageMessages = true
		p.ModerateMembers = true
		p.BanMembers = true
		p.KickMembers = true
		return p
	}
	p.ManageGuild = bits&discordgo.PermissionManageGuild != 0
	p.ManageRoles = bits&discordgo.PermissionManageRoles != 0
	p.ManageChannels = bits&discordgo.PermissionManageChannels != 0
	p.ManageMessages = bits&discordgo.PermissionManageMessages != 0
	p.ModerateMembers = bits&discordgo.PermissionModerateMembers != 0
	p.BanMembers = bits&discordgo.PermissionBanMembers != 0
	p.KickMembers = bits&discordgo.PermissionKickMembers != 0
	return p
}

// ResolveGuildRole determines business GuildRole from Discord OwnerID and permissions.
// GuildOwner checked first, then Administrator/ManageGuild -> GuildAdmin.
func ResolveGuildRole(guildOwnerID, userID string, perms auth.Permissions) auth.GuildRole {
	if guildOwnerID != "" && guildOwnerID == userID {
		return auth.GuildOwner
	}
	if perms.Administrator || perms.ManageGuild {
		return auth.GuildAdmin
	}
	return auth.GuildMember
}
