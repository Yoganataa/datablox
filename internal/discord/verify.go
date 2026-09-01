package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// ensureVerifyMessage creates or updates the persistent welcome verify message in the verify channel.
func (b *Bot) ensureVerifyMessage(ctx context.Context, guildID string) error {
	cfg, err := b.svc.Store.GetGuildConfig(ctx, guildID)
	if err != nil {
		return err
	}
	if cfg.VerifyChannelID == "" {
		return fmt.Errorf("verify channel not set (/config)")
	}
	guild, _ := b.sess.Guild(guildID)
	guildName := "this server"
	if guild != nil {
		guildName = guild.Name
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Welcome to %s!", guildName),
		Description: "Click the button below to Verify with Datablox and gain access to the rest of the server.",
		Color:       0x5865F2,
		Footer:      &discordgo.MessageEmbedFooter{Text: "Datablox • Bloxlink-like verification"},
	}
	verifyURL := strings.TrimRight(b.cfg.WebURL, "/") + "/auth/roblox/login?guild_id=" + guildID
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "Verify with Datablox", CustomID: "verify:start", Style: discordgo.PrimaryButton, Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
			discordgo.Button{Label: "Need help?", Style: discordgo.LinkButton, URL: verifyURL},
		}},
	}
	if cfg.VerifyMessageID != "" {
		_, err = b.sess.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    cfg.VerifyChannelID,
			ID:         cfg.VerifyMessageID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
		})
		if err == nil {
			return nil
		}
		b.log.Warn("verify message missing, creating new", "err", err)
	}
	msg, err := b.sess.ChannelMessageSendComplex(cfg.VerifyChannelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		return err
	}
	cfg.VerifyMessageID = msg.ID
	return b.svc.Store.SetGuildConfig(ctx, cfg)
}

// handleVerifyStart responds to the "Verify with Datablox" button with a private,
// per-user link that carries the clicker's Discord ID so the web flow can link it.
func (b *Bot) handleVerifyStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordID := i.Member.User.ID
	if discordID == "" && i.User != nil {
		discordID = i.User.ID
	}
	if discordID == "" {
		respondEphemeral(s, i, "Could not identify your Discord account. Please try again.")
		return
	}
	// If already verified globally (in another server), grant roles in this server and show ephemeral status
	if u, err := b.svc.Store.GetVerifiedUser(context.Background(), discordID); err == nil && u.RobloxID != 0 {
		go func() {
			ctx := context.Background()
			bindings, _ := b.svc.Store.ListBindings(ctx, i.GuildID)
			if len(bindings) > 0 && b.svc.Client != nil {
				groups, _ := b.svc.Client.GetUserGroups(ctx, u.RobloxID)
				groupMap := make(map[int64]struct{ Rank int })
				for _, g := range groups {
					groupMap[g.GroupID] = struct{ Rank int }{g.Rank}
				}
				for _, bnd := range bindings {
					if g, ok := groupMap[bnd.GroupID]; ok {
						matched := false
						if bnd.RobloxRoleID != nil && int64(g.Rank) == *bnd.RobloxRoleID {
							matched = true
						} else if bnd.RankMin != nil && bnd.RankMax != nil {
							if g.Rank >= *bnd.RankMin && g.Rank <= *bnd.RankMax {
								matched = true
							}
						} else if bnd.RobloxRoleID == nil && bnd.RankMin == nil {
							matched = true
						}
						if matched {
							_ = s.GuildMemberRoleAdd(i.GuildID, discordID, bnd.DiscordRoleID)
						}
					}
				}
			}
		}()
		embedVerified := &discordgo.MessageEmbed{
			Title:       "✅ Already Verified",
			Description: fmt.Sprintf("You are already verified as **%s** (`%d`). Your roles have been granted in this server.", u.RobloxUsername, u.RobloxID),
			Color:       0x00C97A,
		}
		// Still provide re-verify link if they want to change account
		verifyURL := fmt.Sprintf("%s/auth/roblox/login?discord_id=%s&guild_id=%s",
			strings.TrimRight(b.cfg.WebURL, "/"), discordID, i.GuildID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embedVerified},
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.Button{Label: "Re-verify (change account)", Style: discordgo.LinkButton, URL: verifyURL, Emoji: &discordgo.ComponentEmoji{Name: "🔄"}},
						discordgo.Button{Label: "Back to Discord", Style: discordgo.LinkButton, URL: fmt.Sprintf("https://discord.com/channels/%s", i.GuildID)},
					}},
				},
				Flags: 1 << 6,
			},
		})
		return
	}
	verifyURL := fmt.Sprintf("%s/auth/roblox/login?discord_id=%s&guild_id=%s",
		strings.TrimRight(b.cfg.WebURL, "/"), discordID, i.GuildID)
	embed := &discordgo.MessageEmbed{
		Title:       "🔐 Datablox Verification",
		Description: "Click the button below to start verifying your Roblox account. You will be asked to sign in to Roblox and grant access — then your roles will be granted automatically.",
		Color:       0x00A2FF,
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.Button{Label: "Verify with Datablox", Style: discordgo.LinkButton, URL: verifyURL, Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
				}},
			},
			Flags: 1 << 6, // ephemeral — private to the clicker
		},
	})
}
