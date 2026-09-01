package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/model"
)

const pageSize = 8

func (b *Bot) handleAdd(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	url := option(&data, "url")
	genre := option(&data, "genre")

	// Defer within 3s to avoid "The application did not respond" — Roblox calls can take 2-8s
	_ = b.sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	res, err := b.svc.AddExperience(ctx, url, genre)
	if err != nil {
		_, _ = b.sess.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "⚠️ " + err.Error(),
			Flags:   1 << 6,
		})
		return
	}

	// Notice in input channel, detail in feed channel (1 channel feed)
	cfg, _ := b.svc.Store.GetGuildConfig(ctx, i.GuildID)
	hasFeed := cfg.ChannelID != ""
	notice := fmt.Sprintf("data **%s** added", res.Experience.Name)
	if !res.Created {
		notice = fmt.Sprintf("data **%s** already exists", res.Experience.Name)
	}

	if hasFeed {
		_, _ = b.sess.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: notice + fmt.Sprintf(" — details in <#%s>", cfg.ChannelID),
		})
		// Detail to feed channel async (no block)
		if res.Created {
			go func(exp model.Experience) {
				b.postFeedAnnouncement(context.Background(), exp)
			}(res.Experience)
		} else {
			// Already exists but still sync vote/top
			go func() {
				_ = b.syncVotePage(context.Background(), i.GuildID, cfg.ChannelID, 0)
			}()
		}
	} else {
		// Fallback: feed not set, show details in input channel
		embed, btn := experienceEmbed(res.Experience)
		tag := ""
		if !res.Created {
			tag = "Already saved"
		} else if genre == "" {
			tag = "Genre inferred: " + res.Genre
		} else {
			tag = "Added successfully"
		}
		if embed.Footer != nil && embed.Footer.Text != "" {
			embed.Footer.Text += " • " + tag
		} else {
			embed.Footer = &discordgo.MessageEmbedFooter{Text: "Datablox • " + tag}
		}
		embed.Footer.Text += " • Set feed via /config"
		_, _ = b.sess.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{btn}},
			},
		})
		if res.Created {
			go func(exp model.Experience) {
				b.postFeedAnnouncement(context.Background(), exp)
			}(res.Experience)
		}
	}
}

func (b *Bot) handleSearch(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	query := option(&data, "query")
	genre := option(&data, "genre")
	results, err := b.svc.Search(ctx, query, genre, pageSize)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	if len(results) == 0 {
		respondText(b.sess, i, "No results for `"+shorten(query, 40)+"`")
		return
	}
	embed := listEmbed(results, "Results for **"+shorten(query, 60)+"**", genre)
	respondEmbed(b.sess, i, embed)
}

func (b *Bot) handleGames(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	genre := option(&data, "genre")
	results, err := b.svc.ListByGenre(ctx, genre, pageSize)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	if len(results) == 0 {
		respondText(b.sess, i, "No experiences yet"+genreTitle(genre)+". Use `/add` first.")
		return
	}
	embed := listEmbed(results, "Experiences "+genreTitle(genre), genre)
	respondEmbed(b.sess, i, embed)
}

func (b *Bot) handleGame(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	query := option(&data, "experience")
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		if e, gerr := b.svc.GetByUniverseID(ctx, id); gerr == nil {
			embed, btn := experienceEmbed(e)
			respondEmbed(b.sess, i, embed, btn)
			return
		}
	}
	results, serr := b.svc.Search(ctx, query, "", 1)
	if serr != nil || len(results) == 0 {
		respondText(b.sess, i, "Experience not found.")
		return
	}
	embed, btn := experienceEmbed(results[0])
	respondEmbed(b.sess, i, embed, btn)
}

func (b *Bot) handleRandom(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	genre := option(&data, "genre")
	e, err := b.svc.Random(ctx, genre)
	if err != nil {
		respondText(b.sess, i, "No experiences yet"+genreTitle(genre)+". Use `/add` first.")
		return
	}
	embed, btn := experienceEmbed(e)
	respondEmbed(b.sess, i, embed, btn)
}

func (b *Bot) handleTrending(ctx context.Context, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	results, err := b.svc.Store.Trending(ctx, pageSize)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	if len(results) == 0 {
		respondText(b.sess, i, "No data yet. Use `/add` first.")
		return
	}
	respondEmbed(b.sess, i, listEmbed(results, "🔥 Trending Now", ""))
}

func (b *Bot) handleRefresh(ctx context.Context, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	query := option(&data, "experience")

	if query == "" {
		n, err := b.svc.RefreshAll(ctx)
		if err != nil {
			respondError(b.sess, i, err)
			return
		}
		respondText(b.sess, i, fmt.Sprintf("✅ Refresh complete: %d updated.", n))
		// sync feed top/vote async
		go func() {
			configs, _ := b.svc.Store.ListGuildConfigs(context.Background())
			for _, cfg := range configs {
				_ = b.syncVotePage(context.Background(), cfg.GuildID, cfg.ChannelID, 0)
				_ = b.syncTopMessage(context.Background(), cfg.GuildID, cfg.ChannelID)
			}
		}()
		return
	}

	results, err := b.svc.Search(ctx, query, "", 1)
	if err != nil || len(results) == 0 {
		respondText(b.sess, i, "Experience not found.")
		return
	}
	target := results[0]
	refreshed, err := b.svc.RefreshOne(ctx, target.UniverseID)
	if err != nil {
		respondError(b.sess, i, err)
		return
	}
	embed, btn := experienceEmbed(refreshed)
	if embed.Footer != nil && embed.Footer.Text != "" {
		embed.Footer.Text += " • Data refreshed"
	} else {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: "Datablox • Data refreshed"}
	}
	respondEmbed(b.sess, i, embed, btn)
}

func (b *Bot) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	var query string
	for _, o := range data.Options {
		if o.Type == discordgo.ApplicationCommandOptionSubCommand && len(o.Options) > 0 {
			for _, subOpt := range o.Options {
				if v, ok := subOpt.Value.(string); ok && v != "" {
					query = v
					break
				}
			}
			if query != "" {
				break
			}
		}
		if v, ok := o.Value.(string); ok && v != "" {
			query = v
			break
		}
	}
	names := b.svc.SearchNames(context.Background(), query, 10)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		if n == "" || seen[n] || len(choices) >= 10 {
			continue
		}
		seen[n] = true
		choices = append(choices, autocompleteText(n))
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func listEmbed(results []model.Experience, title, _ string) *discordgo.MessageEmbed {
	lines := make([]string, 0, len(results))
	for idx, e := range results {
		lines = append(lines, fmt.Sprintf(
			"**%d.** %s\n└ `%s` • 👥 %s • by %s",
			idx+1, truncate(e.Name, 60), e.Genre, formatCount(e.Playing), truncate(e.CreatorName, 30),
		))
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: strings.Join(lines, "\n"),
		Color:       0x00A2FF,
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("Datablox • %d results", len(results))},
	}
}

func genreTitle(genre string) string {
	if genre == "" {
		return ""
	}
	return " genre `" + genre + "`"
}

func formatCount(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(byte(c))
	}
	return b.String()
}

func extractChannelID(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "<#") && strings.HasSuffix(value, ">") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "<#"), ">")
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value
	}
	return value
}
