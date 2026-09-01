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
