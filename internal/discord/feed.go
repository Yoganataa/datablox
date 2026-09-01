package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/model"
)

const votePageSize = 25
const topCount = 10

// EnsureFeed creates or refreshes the 2 pinned messages in the feed channel for a guild.
// Call after /config channel is set.
func (b *Bot) ensureFeed(ctx context.Context, guildID string) error {
	cfg, err := b.svc.Store.GetGuildConfig(ctx, guildID)
	if err != nil {
		return err
	}
	if cfg.ChannelID == "" {
		return fmt.Errorf("feed channel not set (/config)")
	}

	// Vote message (page 0)
	if err := b.syncVotePage(ctx, guildID, cfg.ChannelID, 0); err != nil {
		b.log.Warn("sync vote failed", "err", err)
	}
	// Top message
	if err := b.syncTopMessage(ctx, guildID, cfg.ChannelID); err != nil {
		b.log.Warn("sync top failed", "err", err)
	}
	return nil
}

func (b *Bot) syncVotePage(ctx context.Context, guildID, channelID string, page int) error {
	cfg, err := b.svc.Store.GetGuildConfig(ctx, guildID)
	if err != nil {
		return err
	}
	total, err := b.svc.Store.Count(ctx)
	if err != nil {
		total = 0
	}
	totalPages := (total + votePageSize - 1) / votePageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}
	exps, _, err := b.svc.Store.ListByGenre(ctx, "", votePageSize, page*votePageSize)
	if err != nil {
		return err
	}

	embed, components := b.buildVoteMessage(ctx, exps, page, total, totalPages, cfg.VoteMessageID)
	if cfg.VoteMessageID != "" {
		_, err = b.sess.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel:    channelID,
			ID:         cfg.VoteMessageID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
		})
		if err == nil {
			return nil
		}
		// message deleted → fall through to create new
		b.log.Warn("vote message missing, creating new", "err", err)
	}
	msg, err := b.sess.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
	if err != nil {
		return err
	}
	cfg.VoteMessageID = msg.ID
	return b.svc.Store.SetGuildConfig(ctx, cfg)
}

func (b *Bot) syncTopMessage(ctx context.Context, guildID, channelID string) error {
	cfg, err := b.svc.Store.GetGuildConfig(ctx, guildID)
	if err != nil {
		return err
	}
	// Top 10 from votes if any votes, fallback to trending by playing
	var top []model.Experience
	var counts map[int64]int
	if cfg.VoteMessageID != "" {
		if c, err := b.svc.Store.GetVoteCounts(ctx, cfg.VoteMessageID); err == nil && len(c) > 0 {
			counts = c
			if t, err := b.svc.Store.GetTopVoted(ctx, cfg.VoteMessageID, topCount); err == nil && len(t) > 0 {
				top = t
			}
		}
	}
	if len(top) == 0 {
		top, err = b.svc.Store.Trending(ctx, topCount)
		if err != nil {
			return err
		}
		// counts tetap kosong jika fallback
	}
	embed := b.buildTopEmbed(ctx, top, counts, cfg.VoteMessageID)
	var msgID = cfg.TopMessageID
	if msgID != "" {
		_, err = b.sess.ChannelMessageEditComplex(&discordgo.MessageEdit{
			Channel: channelID,
			ID:      msgID,
			Embeds:  &[]*discordgo.MessageEmbed{embed},
		})
		if err == nil {
			return nil
		}
		b.log.Warn("top message missing, creating new", "err", err)
	}
	msg, err := b.sess.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}})
	if err != nil {
		return err
	}
	cfg.TopMessageID = msg.ID
	return b.svc.Store.SetGuildConfig(ctx, cfg)
}

func (b *Bot) buildVoteMessage(ctx context.Context, exps []model.Experience, page, total, totalPages int, voteMessageID string) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	// Build vote counts map for this page (1 query)
	counts, _ := b.svc.Store.GetVoteCounts(ctx, voteMessageID)
	totalVotes := 0
	for _, c := range counts {
		totalVotes += c
	}
	lines := make([]string, 0, len(exps))
	options := make([]discordgo.SelectMenuOption, 0, len(exps))
	for i, e := range exps {
		idx := page*votePageSize + i + 1
		votes := counts[e.UniverseID]
		// Useful and not confusing: only name + votes + % (playing as small secondary in dropdown desc)
		voteStr := ""
		if votes > 0 && totalVotes > 0 {
			pct := votes * 100 / totalVotes
			if pct == 0 && votes > 0 {
				pct = 1
			}
			voteStr = fmt.Sprintf(" — %d vote (%d%%)", votes, pct)
		} else if votes > 0 {
			voteStr = fmt.Sprintf(" — %d vote", votes)
		} else {
			voteStr = " — 0 vote"
		}
		lines = append(lines, fmt.Sprintf("**%d.** %s%s", idx, truncate(e.Name, 40), voteStr))
		label := truncate(e.Name, 45)
		desc := e.Genre
		if votes > 0 {
			desc = fmt.Sprintf("%d vote • %s", votes, e.Genre)
		}
		if len(desc) > 50 {
			desc = desc[:50]
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", e.UniverseID),
			Description: desc,
			Emoji:       &discordgo.ComponentEmoji{Name: "🎮"},
		})
	}
	desc := strings.Join(lines, "\n")
	if desc == "" {
		desc = "_No experiences yet. Add via `/add`._"
	}
	footer := fmt.Sprintf("Datablox • %d total • %d votes total • Choose in dropdown (click again to unvote)", total, totalVotes)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🗳️ Vote — Page %d/%d", page+1, totalPages),
		Description: desc,
		Color:       0x00A2FF,
		Footer:      &discordgo.MessageEmbedFooter{Text: footer},
	}
	var components []discordgo.MessageComponent
	if len(options) > 0 {
		components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    fmt.Sprintf("vote:select:%d", page),
						Placeholder: "Choose an experience to vote…",
						Options:     options,
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{CustomID: fmt.Sprintf("vote:prev:%d", page), Label: "◀ Prev", Style: discordgo.SecondaryButton, Disabled: page == 0},
					discordgo.Button{CustomID: fmt.Sprintf("vote:next:%d", page), Label: "Next ▶", Style: discordgo.SecondaryButton, Disabled: page >= totalPages-1},
					discordgo.Button{CustomID: fmt.Sprintf("vote:search:%d", page), Label: "🔍 Search", Style: discordgo.PrimaryButton},
				},
			},
		}
	}
	return embed, components
}

func (b *Bot) buildTopEmbed(ctx context.Context, top []model.Experience, counts map[int64]int, voteMessageID string) *discordgo.MessageEmbed {
	if counts == nil && voteMessageID != "" {
		counts, _ = b.svc.Store.GetVoteCounts(ctx, voteMessageID)
	}
	totalVotes := 0
	for _, c := range counts {
		totalVotes += c
	}
	lines := make([]string, 0, len(top))
	for i, e := range top {
		votes := counts[e.UniverseID]
		if votes > 0 && totalVotes > 0 {
			pct := votes * 100 / totalVotes
			if pct == 0 && votes > 0 {
				pct = 1
			}
			lines = append(lines, fmt.Sprintf("**%d.** %s — **%d vote (%d%%)** • 👥 %s", i+1, truncate(e.Name, 35), votes, pct, formatCount(e.Playing)))
		} else if votes > 0 {
			lines = append(lines, fmt.Sprintf("**%d.** %s — **%d vote** • 👥 %s", i+1, truncate(e.Name, 35), votes, formatCount(e.Playing)))
		} else {
			// Fallback before any vote
			lines = append(lines, fmt.Sprintf("**%d.** %s — `%s` • 👥 %s", i+1, truncate(e.Name, 35), e.Genre, formatCount(e.Playing)))
		}
	}
	desc := strings.Join(lines, "\n")
	if desc == "" {
		desc = "_No votes yet. Vote in the message above!_"
	}
	footer := "Datablox • Top from votes"
	if totalVotes > 0 {
		footer = fmt.Sprintf("Datablox • %d votes total • Auto-updated", totalVotes)
	} else {
		footer = fmt.Sprintf("Datablox • No votes yet • Top by playing • %s", time.Now().Format("2006-01-02"))
	}
	return &discordgo.MessageEmbed{
		Title:       "🏆 Top 10 Voted",
		Description: desc,
		Color:       0xFF6B00,
		Footer:      &discordgo.MessageEmbedFooter{Text: footer},
	}
}

// keep old buildTopEmbed for fallback without ctx (not used)
func buildTopEmbed(top []model.Experience) *discordgo.MessageEmbed {
	lines := make([]string, 0, len(top))
	for i, e := range top {
		visits := "—"
		if e.Visits > 0 {
			visits = formatCount(int(e.Visits))
		}
		lines = append(lines, fmt.Sprintf("**%d.** %s — `%s` • 👥 %s • 👁️ %s", i+1, truncate(e.Name, 40), e.Genre, formatCount(e.Playing), visits))
	}
	desc := strings.Join(lines, "\n")
	if desc == "" {
		desc = "_No data._"
	}
	return &discordgo.MessageEmbed{
		Title:       "🔥 Top 10 Most Played",
		Description: desc,
		Color:       0xFF6B00,
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Datablox • Auto-updated on /add • %s", time.Now().Format("2006-01-02 15:04"))},
	}
}

func likePercent(e model.Experience) int {
	total := e.UpVotes + e.DownVotes
	if total == 0 {
		return 0
	}
	return int(float64(e.UpVotes)/float64(total)*100 + 0.5)
}

// Component handlers for vote paginated 25/page — neat for 1000.

func (b *Bot) handleVoteSelect(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	if len(data.Values) == 0 {
		respondEphemeral(s, i, "Please choose an experience first.")
		return
	}
	universeID, _ := strconv.ParseInt(data.Values[0], 10, 64)
	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}
	messageID := i.Message.ID
	ctx := context.Background()

	has, _ := b.svc.Store.HasVote(ctx, messageID, userID, universeID)
	exp, _ := b.svc.Store.GetByUniverseID(ctx, universeID)
	name := exp.Name
	if name == "" {
		name = data.Values[0]
	}
	if has {
		_ = b.svc.Store.RemoveVote(ctx, messageID, userID, universeID)
		count, _ := b.svc.Store.CountVotesByUniverse(ctx, messageID, universeID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("↩️ Removed vote for **%s** — now %d vote(s).", truncate(name, 40), count), Flags: 1 << 6},
		})
	} else {
		_ = b.svc.Store.AddVote(ctx, model.Vote{MessageID: messageID, UserID: userID, UniverseID: universeID, Emoji: "🎮"})
		count, _ := b.svc.Store.CountVotesByUniverse(ctx, messageID, universeID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Voted for **%s** — total %d vote(s) (click again to remove).", truncate(name, 40), count), Flags: 1 << 6},
		})
	}
	// Refresh vote + top async to show updated counts
	go func() {
		configs, _ := b.svc.Store.ListGuildConfigs(context.Background())
		for _, cfg := range configs {
			if cfg.VoteMessageID == messageID {
				_ = b.syncVotePage(context.Background(), cfg.GuildID, cfg.ChannelID, 0)
				_ = b.syncTopMessage(context.Background(), cfg.GuildID, cfg.ChannelID)
				break
			}
		}
		// Also refresh all guilds' top if vote affects ranking globally
		if has == false { // only on add
			configs, _ := b.svc.Store.ListGuildConfigs(context.Background())
			for _, cfg := range configs {
				if cfg.VoteMessageID != messageID {
					_ = b.syncTopMessage(context.Background(), cfg.GuildID, cfg.ChannelID)
				}
			}
		}
	}()
}

func (b *Bot) handleVoteNav(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	page := 0
	if strings.HasPrefix(data.CustomID, "vote:prev:") {
		fmt.Sscanf(data.CustomID, "vote:prev:%d", &page)
		page--
	} else if strings.HasPrefix(data.CustomID, "vote:next:") {
		fmt.Sscanf(data.CustomID, "vote:next:%d", &page)
		page++
	}
	guildID := i.GuildID
	cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), guildID)
	if cfg.ChannelID == "" {
		respondEphemeral(s, i, "Feed channel not set. Use /config first.")
		return
	}
	// Defer update to avoid 3s timeout, then edit
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
	_ = b.syncVotePage(context.Background(), guildID, cfg.ChannelID, page)
}

func (b *Bot) handleVoteSearchButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: fmt.Sprintf("vote:search:modal:%s", i.ID),
			Title:    "Search Experience",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{CustomID: "query", Label: "Keyword", Style: discordgo.TextInputShort, Required: true, MaxLength: 100, Placeholder: "fishing, horror..."},
				}},
			},
		},
	})
}

func (b *Bot) handleVoteSearchModal(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ModalSubmitInteractionData) {
	query := ""
	for _, row := range data.Components {
		if ar, ok := row.(*discordgo.ActionsRow); ok {
			for _, c := range ar.Components {
				if ti, ok := c.(*discordgo.TextInput); ok && ti.CustomID == "query" {
					query = ti.Value
				}
			}
		}
		if ac, ok := row.(discordgo.ActionsRow); ok {
			for _, c := range ac.Components {
				if ti, ok := c.(discordgo.TextInput); ok && ti.CustomID == "query" {
					query = ti.Value
				}
			}
		}
	}
	if query == "" {
		for _, comp := range data.Components {
			if row, ok := comp.(*discordgo.ActionsRow); ok {
				for _, c := range row.Components {
					if ti, ok := c.(*discordgo.TextInput); ok {
						query = ti.Value
					}
				}
			}
		}
	}
	if strings.TrimSpace(query) == "" {
		respondEphemeral(s, i, "Query is empty.")
		return
	}
	results, _ := b.svc.Search(context.Background(), query, "", 8)
	if len(results) == 0 {
		respondEphemeral(s, i, fmt.Sprintf("No results for `%s`.", shorten(query, 40)))
		return
	}
	embed := listEmbed(results, fmt.Sprintf("Results for **%s**", shorten(query, 40)), "")
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: 1 << 6},
	})
}

// postFeedAnnouncement posts a new experience to the feed channel (append, not flood 1000 at once).
func (b *Bot) postFeedAnnouncement(ctx context.Context, exp model.Experience) {
	configs, err := b.svc.Store.ListGuildConfigs(ctx)
	if err != nil || len(configs) == 0 {
		return
	}
	embed, btn := experienceEmbed(exp)
	embed.Footer = &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Datablox • Newly added • %s", time.Now().Format("2006-01-02 15:04"))}
	for _, cfg := range configs {
		if cfg.ChannelID == "" {
			continue
		}
		_, _ = b.sess.ChannelMessageSendComplex(cfg.ChannelID, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}, Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{btn}},
		}})
		// Also sync vote/top async to keep neat for 1000
		go func(c model.GuildConfig) {
			_ = b.syncVotePage(context.Background(), c.GuildID, c.ChannelID, 0)
			_ = b.syncTopMessage(context.Background(), c.GuildID, c.ChannelID)
		}(cfg)
	}
}
