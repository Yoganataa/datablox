package discord

import (
	"github.com/bwmarrin/discordgo"

	"datablox/internal/auth"
	"datablox/internal/auth/adapters"
)

// targetForUser builds Target for moderation target.
// Resolves BotRole and GuildRole for target user in same guild.
func (b *Bot) targetForUser(guildID, userID string) auth.Target {
	botResolver := adapters.NewBotRoleResolverFromBoolMap(b.cfg.OwnerDiscordIDs, b.cfg.AdminDiscordIDs)
	botRole := botResolver.ResolveBotRole(userID)
	guildRole := auth.GuildMember
	if guildID != "" {
		var perms auth.Permissions
		if m, err := b.sess.GuildMember(guildID, userID); err == nil && m != nil {
			perms = ExtractPermissions(int64(m.Permissions))
			guildOwnerID := ""
			if g, err := b.sess.Guild(guildID); err == nil && g != nil {
				guildOwnerID = g.OwnerID
			} else if g, err := b.sess.State.Guild(guildID); err == nil && g != nil {
				guildOwnerID = g.OwnerID
			}
			guildResolver := adapters.NewGuildResolver()
			guildRole = guildResolver.ResolveGuildRole(guildOwnerID, userID, perms)
		} else {
			// Not in guild (e.g., ban by ID) — treat as Member for target policy
			guildRole = auth.GuildMember
		}
		_ = perms
	}
	return auth.Target{UserID: userID, GuildRole: guildRole, BotRole: botRole}
}

// principalForInteraction builds Principal for Discord interaction.
// Uses BotRole from config and GuildRole+Permissions from Discord member.
// Pure adapter composition — no Can() logic here.
func (b *Bot) principalForInteraction(i *discordgo.InteractionCreate) auth.Principal {
	userID := getInvokerID(i)
	guildID := i.GuildID

	botResolver := adapters.NewBotRoleResolverFromBoolMap(b.cfg.OwnerDiscordIDs, b.cfg.AdminDiscordIDs)
	botRole := botResolver.ResolveBotRole(userID)

	var guildRole auth.GuildRole = auth.GuildMember
	var perms auth.Permissions

	if guildID != "" && i.Member != nil {
		// GuildRole + Permissions from Discord member
		bits := int64(i.Member.Permissions)
		perms = ExtractPermissions(bits)
		// GuildOwner check via guild.OwnerID
		guildOwnerID := ""
		if g, err := b.sess.Guild(guildID); err == nil && g != nil {
			guildOwnerID = g.OwnerID
		} else if g, err := b.sess.State.Guild(guildID); err == nil && g != nil {
			guildOwnerID = g.OwnerID
		}
		guildResolver := adapters.NewGuildResolver()
		guildRole = guildResolver.ResolveGuildRole(guildOwnerID, userID, perms)
	}

	return adapters.ResolvePrincipal(userID, guildID, botRole, guildRole, perms)
}
