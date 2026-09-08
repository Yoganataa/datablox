package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/model"
)

func reactionRoleCommands() *discordgo.ApplicationCommand {
	manageGuild := int64(discordgo.PermissionManageGuild)
	return &discordgo.ApplicationCommand{
		Name: "reactionrole", Description: "Manage reaction roles (parity with web panel)",
		DefaultMemberPermissions: &manageGuild,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name: "add", Description: "Add a reaction role to a message", Type: discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{Name: "channel", Description: "Channel of the message", Type: discordgo.ApplicationCommandOptionChannel, Required: true, ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText}},
					{Name: "message_id", Description: "Message ID", Type: discordgo.ApplicationCommandOptionString, Required: true},
					{Name: "emoji", Description: "Emoji (unicode or custom <:name:id>)", Type: discordgo.ApplicationCommandOptionString, Required: true},
					{Name: "role", Description: "Role to grant", Type: discordgo.ApplicationCommandOptionRole, Required: true},
					{Name: "mode", Description: "Mode", Type: discordgo.ApplicationCommandOptionString, Required: false,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "normal", Value: "normal"}, {Name: "unique (pick-one)", Value: "unique"},
							{Name: "verify (one-way)", Value: "verify"}, {Name: "reverse (toggle)", Value: "reverse"},
						}},
				},
			},
			{Name: "list", Description: "List reaction roles for this server", Type: discordgo.ApplicationCommandOptionSubCommand},
			{
				Name: "remove", Description: "Remove a reaction role", Type: discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{Name: "id", Description: "Reaction role ID (see /reactionrole list)", Type: discordgo.ApplicationCommandOptionInteger, Required: true},
				},
			},
		},
	}
}

func (b *Bot) handleReactionRole(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if !hasManageGuild(i) {
		respondEphemeral(b.sess, i, "Need Manage Server")
		return
	}
	if len(data.Options) == 0 {
		respondEphemeral(b.sess, i, "Usage: /reactionrole add|list|remove")
		return
	}
	sub := data.Options[0]
	switch sub.Name {
	case "add":
		b.handleReactionRoleAdd(ctx, i, sub)
	case "list":
		b.handleReactionRoleList(ctx, i)
	case "remove":
		b.handleReactionRoleRemove(ctx, i, sub)
	default:
		respondEphemeral(b.sess, i, "Unknown subcommand")
	}
}

func (b *Bot) handleReactionRoleAdd(ctx context.Context, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	var channelID, messageID, emoji, mode, roleID string
	for _, o := range sub.Options {
		switch o.Name {
		case "channel":
			if v, ok := o.Value.(string); ok {
				channelID = v
			}
		case "message_id":
			if v, ok := o.Value.(string); ok {
				messageID = v
			}
		case "emoji":
			if v, ok := o.Value.(string); ok {
				emoji = NormalizeEmoji(v)
			}
		case "role":
			if v, ok := o.Value.(string); ok {
				roleID = v
			}
		case "mode":
			if v, ok := o.Value.(string); ok && v != "" {
				mode = v
			}
		}
	}
	if channelID == "" || messageID == "" || emoji == "" || roleID == "" {
		respondEphemeral(b.sess, i, "Missing channel/message/emoji/role")
		return
	}
	if mode == "" {
		mode = "normal"
	}
	if _, err := b.sess.ChannelMessage(channelID, messageID); err != nil {
		respondEphemeral(b.sess, i, "Message not found in that channel. Use right-click Copy Message Link -> ID.")
		return
	}
	rr := model.ReactionRole{GuildID: i.GuildID, ChannelID: channelID, MessageID: messageID, Emoji: emoji, RoleID: roleID, Mode: mode}
	id, err := b.svc.Store.CreateReactionRole(ctx, rr)
	if err != nil {
		respondEphemeral(b.sess, i, "Failed: "+err.Error())
		return
	}
	_ = b.sess.MessageReactionAdd(channelID, messageID, emoji)
	respondEphemeral(b.sess, i, fmt.Sprintf("Reaction role #%d: %s -> <@&%s> mode `%s` on <#%s>", id, emoji, roleID, mode, channelID))
}

func (b *Bot) handleReactionRoleList(ctx context.Context, i *discordgo.InteractionCreate) {
	roles, err := b.svc.Store.ListReactionRoles(ctx, i.GuildID)
	if err != nil || len(roles) == 0 {
		respondEphemeral(b.sess, i, "No reaction roles. Add via `/reactionrole add` or web panel https://datablox.devest.live/guild/"+i.GuildID)
		return
	}
	lines := ""
	for _, r := range roles {
		lines += fmt.Sprintf("`#%d` %s -> <@&%s> `%s` msg `%s` chan <#%s>\n", r.ID, r.Emoji, r.RoleID, r.Mode, r.MessageID, r.ChannelID)
		if len(lines) > 1800 {
			lines = lines[:1800] + "... (truncated)"
			break
		}
	}
	embed := &discordgo.MessageEmbed{Title: "Reaction Roles", Description: lines, Color: 0x5865F2}
	respondEphemeralEmbed(b.sess, i, embed)
}

func (b *Bot) handleReactionRoleRemove(ctx context.Context, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	var id int64
	for _, o := range sub.Options {
		if o.Name == "id" {
			if v, ok := o.Value.(float64); ok {
				id = int64(v)
			}
		}
	}
	if id == 0 {
		respondEphemeral(b.sess, i, "Provide ID from `/reactionrole list`")
		return
	}
	if err := b.svc.Store.DeleteReactionRole(ctx, id, i.GuildID); err != nil {
		respondEphemeral(b.sess, i, "Delete failed: "+err.Error())
		return
	}
	respondEphemeral(b.sess, i, fmt.Sprintf("Removed reaction role #%d", id))
}

func respondEphemeralEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: 1 << 6},
	})
}
