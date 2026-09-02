package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/model"
)

func moderationCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name: "warn", Description: "Warn a member",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to warn", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
			},
		},
		{
			Name: "mute", Description: "Timeout a member",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to mute", Required: true},
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "minutes", Description: "Minutes (default 10)", Required: false},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
			},
		},
		{
			Name: "ban", Description: "Ban a member",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to ban", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
			},
		},
		{
			Name: "kick", Description: "Kick a member",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to kick", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason", Required: false},
			},
		},
		{
			Name: "purge", Description: "Delete messages",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "amount", Description: "Amount 1-100", Required: true},
			},
		},
		{
			Name: "level", Description: "Check level",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User (default you)", Required: false},
			},
		},
		{
			Name: "leaderboard", Description: "Show leaderboard",
		},
	}
}

func (b *Bot) handleWarn(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageGuild(i) {
		respondEphemeral(b.sess, i, "You need Manage Server permission.")
		return
	}
	user := optionUser(&data, "user")
	reason := option(&data, "reason")
	if reason == "" {
		reason = "No reason"
	}
	targetID := ""
	if user != nil {
		targetID = user.ID
	} else {
		targetID = option(&data, "user")
	}
	_, _ = b.svc.Store.CreateInfraction(ctx, model.Infraction{GuildID: i.GuildID, UserID: targetID, ModeratorID: getInvokerID(i), Type: "warn", Reason: reason})
	respondText(b.sess, i, fmt.Sprintf("⚠️ Warned <@%s> — %s", targetID, reason))
}

func (b *Bot) handleMute(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageGuild(i) {
		respondEphemeral(b.sess, i, "You need Manage Server permission.")
		return
	}
	user := optionUser(&data, "user")
	targetID := ""
	if user != nil {
		targetID = user.ID
	}
	minutes := 10
	for _, o := range data.Options {
		if o.Name == "minutes" && o.Value != nil {
			if v, ok := o.Value.(float64); ok {
				minutes = int(v)
			}
		}
	}
	reason := option(&data, "reason")
	if reason == "" {
		reason = "No reason"
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	err := b.sess.GuildMemberTimeout(i.GuildID, targetID, &until)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	_, _ = b.svc.Store.CreateInfraction(ctx, model.Infraction{GuildID: i.GuildID, UserID: targetID, ModeratorID: getInvokerID(i), Type: "mute", Reason: reason, ExpiresAt: &until})
	respondText(b.sess, i, fmt.Sprintf("🔇 Muted <@%s> for %d min — %s", targetID, minutes, reason))
}

func (b *Bot) handleBan(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageGuild(i) {
		respondEphemeral(b.sess, i, "You need Manage Server permission.")
		return
	}
	user := optionUser(&data, "user")
	targetID := ""
	if user != nil {
		targetID = user.ID
	}
	reason := option(&data, "reason")
	if reason == "" {
		reason = "No reason"
	}
	err := b.sess.GuildBanCreateWithReason(i.GuildID, targetID, reason, 1)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	_, _ = b.svc.Store.CreateInfraction(ctx, model.Infraction{GuildID: i.GuildID, UserID: targetID, ModeratorID: getInvokerID(i), Type: "ban", Reason: reason})
	respondText(b.sess, i, fmt.Sprintf("🔨 Banned <@%s> — %s", targetID, reason))
}

func (b *Bot) handleKick(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageGuild(i) {
		respondEphemeral(b.sess, i, "You need Manage Server permission.")
		return
	}
	user := optionUser(&data, "user")
	targetID := ""
	if user != nil {
		targetID = user.ID
	}
	reason := option(&data, "reason")
	err := b.sess.GuildMemberDeleteWithReason(i.GuildID, targetID, reason)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	_, _ = b.svc.Store.CreateInfraction(ctx, model.Infraction{GuildID: i.GuildID, UserID: targetID, ModeratorID: getInvokerID(i), Type: "kick", Reason: reason})
	respondText(b.sess, i, fmt.Sprintf("👢 Kicked <@%s> — %s", targetID, reason))
}

func (b *Bot) handlePurge(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageMessages(i) {
		respondEphemeral(b.sess, i, "You need Manage Messages permission.")
		return
	}
	amount := 10
	for _, o := range data.Options {
		if o.Name == "amount" && o.Value != nil {
			if v, ok := o.Value.(float64); ok {
				amount = int(v)
			}
		}
	}
	if amount < 1 {
		amount = 1
	}
	if amount > 100 {
		amount = 100
	}
	channelID := i.ChannelID
	messages, err := b.sess.ChannelMessages(channelID, amount, "", "", "")
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	ids := make([]string, len(messages))
	for i, m := range messages {
		ids[i] = m.ID
	}
	if len(ids) == 1 {
		_ = b.sess.ChannelMessageDelete(channelID, ids[0])
	} else if len(ids) > 0 {
		_ = b.sess.ChannelMessagesBulkDelete(channelID, ids)
	}
	respondEphemeral(b.sess, i, fmt.Sprintf("🧹 Purged %d messages.", len(ids)))
}

func hasManageGuild(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}
	return i.Member.Permissions&discordgo.PermissionManageServer != 0 || i.Member.Permissions&discordgo.PermissionAdministrator != 0
}

func hasManageMessages(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}
	return i.Member.Permissions&discordgo.PermissionManageMessages != 0 || i.Member.Permissions&discordgo.PermissionAdministrator != 0
}

func getInvokerID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func optionUser(data *discordgo.ApplicationCommandInteractionData, name string) *discordgo.User {
	for _, o := range data.Options {
		if o.Name == name && o.Type == discordgo.ApplicationCommandOptionUser {
			if u, ok := o.Value.(*discordgo.User); ok {
				return u
			}
			// discordgo may put User in Resolved
			if data.Resolved != nil {
				for _, u := range data.Resolved.Users {
					return u
				}
			}
		}
	}
	return nil
}
