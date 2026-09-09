package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// executor checks Discord platform constraints after application authz passed.
// Not part of auth package — pure Discord execution concern.

func (b *Bot) checkSelfTarget(executorID, targetID string) error {
	if executorID == targetID {
		return fmt.Errorf("cannot target yourself")
	}
	return nil
}

func (b *Bot) checkBotPermissions(guildID string, required int64) error {
	botID := b.sess.State.User.ID
	// Prefer cached state, fall back to a single API call.
	member, err := b.sess.State.Member(guildID, botID)
	if err != nil {
		member, err = b.sess.GuildMember(guildID, botID)
		if err != nil {
			return fmt.Errorf("cannot get bot member")
		}
	}
	// Get guild for owner check and roles
	guild, err := b.sess.Guild(guildID)
	if err != nil {
		if g, err2 := b.sess.State.Guild(guildID); err2 == nil {
			guild = g
		} else {
			return fmt.Errorf("cannot get guild")
		}
	}
	// If bot is guild owner, bypass
	if guild.OwnerID == botID {
		return nil
	}
	// Check permissions via State
	perms, err := b.sess.State.UserChannelPermissions(botID, guildID)
	if err != nil {
		// Fallback: check member permissions directly
		perms = member.Permissions
	}
	if perms&discordgo.PermissionAdministrator != 0 {
		return nil
	}
	if perms&required == 0 {
		return fmt.Errorf("bot lacks required permission")
	}
	return nil
}

// checkRoleHierarchy ensures the bot outranks the target.
// allowAbsent controls ban-by-ID semantics: ban permits absent targets
// (Discord allows banning users outside the guild); mute/kick require
// the target to be present, so absent targets are denied.
func (b *Bot) checkRoleHierarchy(guildID, targetID string, allowAbsent bool) error {
	guild, err := b.sess.Guild(guildID)
	if err != nil {
		if g, err2 := b.sess.State.Guild(guildID); err2 == nil {
			guild = g
		} else {
			return fmt.Errorf("cannot get guild")
		}
	}
	botID := b.sess.State.User.ID
	if targetID == botID {
		return fmt.Errorf("cannot target bot")
	}
	// Get highest role position for bot and target
	botMember, err := b.sess.GuildMember(guildID, botID)
	if err != nil {
		return fmt.Errorf("cannot get bot member")
	}
	targetMember, err := b.sess.GuildMember(guildID, targetID)
	if err != nil {
		// Absent target: only ban-by-ID may skip the hierarchy check.
		if allowAbsent {
			return nil
		}
		return fmt.Errorf("target not in guild")
	}
	botPos := highestRolePosition(guild, botMember.Roles)
	targetPos := highestRolePosition(guild, targetMember.Roles)
	if botPos <= targetPos {
		return fmt.Errorf("target role is higher or equal to bot")
	}
	return nil
}

func highestRolePosition(guild *discordgo.Guild, roleIDs []string) int {
	maxPos := 0
	roleMap := make(map[string]*discordgo.Role, len(guild.Roles))
	for _, r := range guild.Roles {
		roleMap[r.ID] = r
	}
	for _, id := range roleIDs {
		if r, ok := roleMap[id]; ok && r.Position > maxPos {
			maxPos = r.Position
		}
	}
	return maxPos
}
