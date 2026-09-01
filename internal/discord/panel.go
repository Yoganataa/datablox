package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/service"
)

func (b *Bot) handlePanel(ctx context.Context, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	// Build central dashboard embed
	total, _ := b.svc.Store.Count(ctx)
	genres := service.ValidGenres()
	genreOpts := make([]discordgo.SelectMenuOption, 0, len(genres))
	for _, g := range genres {
		genreOpts = append(genreOpts, discordgo.SelectMenuOption{Label: g, Value: g, Description: fmt.Sprintf("Filter %s", g), Emoji: &discordgo.ComponentEmoji{Name: "🏷️"}})
	}
	embed := &discordgo.MessageEmbed{
		Title:       "📦 Datablox — Dashboard",
		Description: fmt.Sprintf("**%d** experiences saved\nChoose an action below — browse without many slash commands.", total),
		Color:       0x5865F2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Browse", Value: "🔍 Search (modal) • 🎲 Random • 🔥 Trending • 📚 List by genre", Inline: false},
			{Name: "Feed (1 Channel)", Value: "Vote #1 (25/page) + Top 10 #2 + Feed (chronological) — set via `/config`", Inline: false},
		},
		Footer: &discordgo.MessageEmbedFooter{Text: "Datablox • Central panel — ephemeral, no channel spam"},
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "panel:search", Label: "🔍 Search", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: "panel:random", Label: "🎲 Random", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "panel:trending", Label: "🔥 Trending", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "panel:refresh", Label: "🔄 Refresh", Style: discordgo.SecondaryButton},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    "panel:genre",
				Placeholder: "Choose a genre to list…",
				Options:     genreOpts,
			},
		}},
	}
	_ = b.sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Components: components, Flags: 1 << 6},
	})
}

func (b *Bot) handlePanelComponent(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	switch {
	case data.CustomID == "panel:search":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "panel:search:modal",
				Title:    "Search Experience",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.TextInput{CustomID: "query", Label: "Keyword", Style: discordgo.TextInputShort, Required: true, MaxLength: 100, Placeholder: "fishing, horror..."},
					}},
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.TextInput{CustomID: "genre", Label: "Genre (optional)", Style: discordgo.TextInputShort, Required: false, MaxLength: 20, Placeholder: "fishing / empty = all"},
					}},
				},
			},
		})
	case data.CustomID == "panel:random":
		ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
		defer cancel()
		e, err := b.svc.Random(ctx, "")
		if err != nil {
			respondEphemeral(s, i, "No experiences yet. Use `/add` first.")
			return
		}
		embed, btn := experienceEmbed(e)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{btn}},
			}, Flags: 1 << 6},
		})
	case data.CustomID == "panel:trending":
		ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
		defer cancel()
		results, err := b.svc.Store.Trending(ctx, 8)
		if err != nil || len(results) == 0 {
			respondEphemeral(s, i, "No data yet.")
			return
		}
		embed := listEmbed(results, "🔥 Trending", "")
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: 1 << 6},
		})
	case data.CustomID == "panel:refresh":
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Flags: 1 << 6},
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*1e9)
		defer cancel()
		n, err := b.svc.RefreshAll(ctx)
		if err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: "⚠️ " + err.Error(), Flags: 1 << 6})
			return
		}
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: fmt.Sprintf("✅ Refresh complete: %d updated.", n), Flags: 1 << 6})
		go func() {
			configs, _ := b.svc.Store.ListGuildConfigs(context.Background())
			for _, cfg := range configs {
				_ = b.syncVotePage(context.Background(), cfg.GuildID, cfg.ChannelID, 0)
				_ = b.syncTopMessage(context.Background(), cfg.GuildID, cfg.ChannelID)
			}
		}()
	case data.CustomID == "panel:genre":
		if len(data.Values) == 0 {
			respondEphemeral(s, i, "Choose a genre first.")
			return
		}
		genre := data.Values[0]
		ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
		defer cancel()
		results, err := b.svc.ListByGenre(ctx, genre, 8)
		if err != nil || len(results) == 0 {
			respondEphemeral(s, i, fmt.Sprintf("No experiences for genre `%s`.", genre))
			return
		}
		embed := listEmbed(results, fmt.Sprintf("Genre `%s`", genre), genre)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: 1 << 6},
		})
	case strings.HasPrefix(data.CustomID, "panel:"):
		respondEphemeral(s, i, "Unknown panel action.")
	}
}

func (b *Bot) handlePanelModal(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ModalSubmitInteractionData) {
	if data.CustomID != "panel:search:modal" {
		return
	}
	query, genre := "", ""
	for _, row := range data.Components {
		ar, ok := row.(*discordgo.ActionsRow)
		if !ok {
			if ac, ok2 := row.(discordgo.ActionsRow); ok2 {
				ar = &ac
			} else {
				continue
			}
		}
		for _, c := range ar.Components {
			if ti, ok := c.(*discordgo.TextInput); ok {
				if ti.CustomID == "query" {
					query = ti.Value
				}
				if ti.CustomID == "genre" {
					genre = ti.Value
				}
			}
			if ti, ok := c.(discordgo.TextInput); ok {
				if ti.CustomID == "query" {
					query = ti.Value
				}
				if ti.CustomID == "genre" {
					genre = ti.Value
				}
			}
		}
	}
	if strings.TrimSpace(query) == "" {
		respondEphemeral(s, i, "Query is empty.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()
	results, err := b.svc.Search(ctx, query, genre, 8)
	if err != nil || len(results) == 0 {
		respondEphemeral(s, i, fmt.Sprintf("No results for `%s`.", shorten(query, 30)))
		return
	}
	embed := listEmbed(results, fmt.Sprintf("Results for **%s**", shorten(query, 30)), genre)
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: 1 << 6},
	})
}
