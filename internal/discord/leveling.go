package discord

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	"datablox/internal/welcome"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleLevel(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	targetID := getInvokerID(i)
	if u := optionUser(&data, "user"); u != nil {
		targetID = u.ID
	}
	lvl, _ := b.svc.Store.GetLevel(ctx, i.GuildID, targetID)
	xpForNext := 100 * (1 << lvl.Level) // simple 100*2^level
	respondText(b.sess, i, fmt.Sprintf("🏆 <@%s> — Level %d • %d XP • %d messages", targetID, lvl.Level, lvl.XP, lvl.Messages))
	_ = xpForNext
}

func (b *Bot) handleLeaderboard(ctx context.Context, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	levels, err := b.svc.Store.Leaderboard(ctx, i.GuildID, 10)
	if err != nil || len(levels) == 0 {
		respondText(b.sess, i, "No leaderboard data yet. Chat more!")
		return
	}
	lines := make([]string, len(levels))
	for idx, l := range levels {
		lines[idx] = fmt.Sprintf("**%d.** <@%s> — Lvl %d (%d XP)", idx+1, l.UserID, l.Level, l.XP)
	}
	embed := &discordgo.MessageEmbed{
		Title:       "🏆 Leaderboard",
		Description: strings.Join(lines, "\n"),
		Color:       0xFFD700,
	}
	respondEmbed(b.sess, i, embed)
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}
	if m.GuildID == "" {
		return
	}
	// Automod checks
	if b.checkAutomod(s, m) {
		return
	}
	// Leveling: 60s cooldown, 15-25 XP
	ctx := context.Background()
	lvl, _ := b.svc.Store.GetLevel(ctx, m.GuildID, m.Author.ID)
	now := time.Now()
	if lvl.LastXPAt != nil && now.Sub(*lvl.LastXPAt) < 60*time.Second {
		return
	}
	xpGain := 15 + rand.Intn(11)
	lvl.XP += xpGain
	lvl.Messages++
	lvl.LastXPAt = &now
	// Level formula: 100*(1.2^n) cumulative — simple threshold 100*2^level
	needed := 100 * (1 << lvl.Level)
	if lvl.XP >= needed {
		lvl.Level++
		// Announce level up
		_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎉 <@%s> reached **Level %d**!", m.Author.ID, lvl.Level))
	}
	_ = b.svc.Store.SetLevel(ctx, lvl)
}

func (b *Bot) checkAutomod(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	ctx := context.Background()
	cfg, _ := b.svc.Store.GetAutomodConfig(ctx, m.GuildID)
	if cfg.GuildID == "" {
		return false
	}
	content := strings.ToLower(m.Content)
	// Anti-invite
	if cfg.AntiInvite && strings.Contains(content, "discord.gg/") {
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		return true
	}
	// Mass mention
	if cfg.MassMention > 0 && len(m.Mentions) >= cfg.MassMention {
		_ = s.ChannelMessageDelete(m.ChannelID, m.ID)
		return true
	}
	// Ghost ping check handled via MessageDelete event, not here
	return false
}

func (b *Bot) onMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	// Ghost ping detection: if deleted message had mentions, log
}

func (b *Bot) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	ctx := context.Background()
	cfg, _ := b.svc.Store.GetWelcomeConfig(ctx, m.GuildID)
	if !cfg.Enabled || cfg.ChannelID == "" {
		return
	}
	msg := cfg.Message
	if msg == "" {
		msg = "Welcome {mention} to **{server}**!"
	}
	// Fetch guild for banner
	guild, _ := s.Guild(m.GuildID)
	serverName := m.GuildID
	memberCount := 0
	if guild != nil {
		serverName = guild.Name
		memberCount = guild.MemberCount
	}
	msg = strings.ReplaceAll(msg, "{mention}", "<@"+m.User.ID+">")
	msg = strings.ReplaceAll(msg, "{server}", serverName)
	msg = strings.ReplaceAll(msg, "{user}", m.User.Username)
	msg = strings.ReplaceAll(msg, "{count}", fmt.Sprintf("%d", memberCount))
	// Try generated banner (option 3)
	if banner, err := generateWelcomeBanner(m.User.Username, m.User.AvatarURL("1024"), serverName, memberCount); err == nil && banner != nil {
		_, _ = s.ChannelMessageSendComplex(cfg.ChannelID, &discordgo.MessageSend{
			Content: msg,
			Files: []*discordgo.File{{Name: "welcome.jpg", ContentType: "image/jpeg", Reader: bytes.NewReader(banner)}},
		})
	} else {
		_, _ = s.ChannelMessageSend(cfg.ChannelID, msg)
	}
	if cfg.AutoRoleID != "" {
		_ = s.GuildMemberRoleAdd(m.GuildID, m.User.ID, cfg.AutoRoleID)
	}
}

func generateWelcomeBanner(username, avatarURL, serverName string, memberCount int) ([]byte, error) {
	return welcome.GenerateBanner(username, avatarURL, serverName, memberCount)
}
