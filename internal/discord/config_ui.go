package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/service"
)

func (b *Bot) handleConfig(ctx context.Context, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	cfg, _ := b.svc.Store.GetGuildConfig(ctx, i.GuildID)
	feedChannel := "_(not set)_"
	if cfg.ChannelID != "" {
		feedChannel = fmt.Sprintf("<#%s>", cfg.ChannelID)
	}
	verifyChannel := "_(not set)_"
	if cfg.VerifyChannelID != "" {
		verifyChannel = fmt.Sprintf("<#%s>", cfg.VerifyChannelID)
	}
	genre := cfg.DefaultGenre
	if genre == "" {
		genre = "_(not set)_"
	}
	vote := cfg.VoteMessageID
	if vote == "" {
		vote = "_(not created)_"
	} else {
		vote = "`" + vote + "`"
	}
	top := cfg.TopMessageID
	if top == "" {
		top = "_(not created)_"
	} else {
		top = "`" + top + "`"
	}
	verifyMsg := cfg.VerifyMessageID
	if verifyMsg == "" {
		verifyMsg = "_(not created)_"
	} else {
		verifyMsg = "`" + verifyMsg + "`"
	}
	embed := &discordgo.MessageEmbed{
		Title:       "⚙️ Datablox — Configuration",
		Description: fmt.Sprintf("**Feed Channel:** %s\n**Vote msg:** %s\n**Top msg:** %s\n**Verify Channel:** %s\n**Verify msg:** %s\n**Default genre:** `%s`\n\nUse the dropdowns below to configure. Feed and Verify are **separate channels**.", feedChannel, vote, top, verifyChannel, verifyMsg, genre),
		Color:       0x5865F2,
		Footer:      &discordgo.MessageEmbedFooter{Text: "Choose a channel below — feed and verify are separate"},
	}
	genres := service.ValidGenres()
	genreOpts := make([]discordgo.SelectMenuOption, 0, len(genres))
	for _, g := range genres {
		genreOpts = append(genreOpts, discordgo.SelectMenuOption{Label: g, Value: g, Description: fmt.Sprintf("Set default to %s", g)})
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:     discordgo.ChannelSelectMenu,
				CustomID:     "config:feed_channel_select",
				Placeholder:  "Choose feed channel #... (vote #1 + top 10 #2 + feed)",
				ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews},
				MinValues:    intPtr(1),
				MaxValues:    1,
			},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				MenuType:     discordgo.ChannelSelectMenu,
				CustomID:     "config:verify_channel_select",
				Placeholder:  "Choose verify channel #... (Welcome + Verify button)",
				ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews},
				MinValues:    intPtr(1),
				MaxValues:    1,
			},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    "config:genre_select",
				Placeholder: "Choose default genre…",
				Options:     genreOpts,
			},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "config:regen", Label: "🔄 Regen Feed", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "config:regen_verify", Label: "🔄 Regen Verify", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: "config:show", Label: "👁 Show", Style: discordgo.SecondaryButton},
		}},
	}
	_ = b.sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Components: components, Flags: 1 << 6},
	})
}

func intPtr(i int) *int { return &i }

func (b *Bot) handleConfigComponent(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	switch data.CustomID {
	case "config:feed_channel_select":
		if len(data.Values) == 0 {
			respondEphemeral(s, i, "Please choose a channel first.")
			return
		}
		channelID := data.Values[0]
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		// If the channel changes, reset vote/top message IDs to avoid Unknown Message 10008 in the old channel
		if cfg.ChannelID != channelID {
			cfg.VoteMessageID = ""
			cfg.TopMessageID = ""
		}
		cfg.ChannelID = channelID
		if err := b.svc.Store.SetGuildConfig(context.Background(), cfg); err != nil {
			respondEphemeral(s, i, "Failed to save: "+err.Error())
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Feed channel saved to <#%s>. Creating vote #1 + top 10 #2...", channelID), Flags: 1 << 6},
		})
		go func() {
			_ = b.ensureFeed(context.Background(), i.GuildID)
		}()
	case "config:verify_channel_select":
		if len(data.Values) == 0 {
			respondEphemeral(s, i, "Please choose a channel first.")
			return
		}
		channelID := data.Values[0]
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		if cfg.VerifyChannelID != channelID {
			cfg.VerifyMessageID = ""
		}
		cfg.VerifyChannelID = channelID
		if err := b.svc.Store.SetGuildConfig(context.Background(), cfg); err != nil {
			respondEphemeral(s, i, "Failed to save: "+err.Error())
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Verify channel saved to <#%s>. Creating welcome verify message...", channelID), Flags: 1 << 6},
		})
		go func() {
			_ = b.ensureVerifyMessage(context.Background(), i.GuildID)
		}()
	case "config:genre_select":
		if len(data.Values) == 0 {
			respondEphemeral(s, i, "Please choose a genre first.")
			return
		}
		genre := data.Values[0]
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		cfg.DefaultGenre = genre
		_ = b.svc.Store.SetGuildConfig(context.Background(), cfg)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("✅ Default genre → `%s`", genre), Flags: 1 << 6},
		})
	case "config:regen":
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		if cfg.ChannelID == "" {
			respondEphemeral(s, i, "Feed channel not set. Choose a feed channel first.")
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("🔄 Regenerating feed in <#%s>...", cfg.ChannelID), Flags: 1 << 6},
		})
		go func() {
			_ = b.ensureFeed(context.Background(), i.GuildID)
		}()
	case "config:regen_verify":
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		if cfg.VerifyChannelID == "" {
			respondEphemeral(s, i, "Verify channel not set. Choose a verify channel first.")
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("🔄 Regenerating verify message in <#%s>...", cfg.VerifyChannelID), Flags: 1 << 6},
		})
		go func() {
			_ = b.ensureVerifyMessage(context.Background(), i.GuildID)
		}()
	case "config:show":
		cfg, _ := b.svc.Store.GetGuildConfig(context.Background(), i.GuildID)
		feedCh := "_(not set)_"
		if cfg.ChannelID != "" {
			feedCh = "<#" + cfg.ChannelID + ">"
		}
		verifyCh := "_(not set)_"
		if cfg.VerifyChannelID != "" {
			verifyCh = "<#" + cfg.VerifyChannelID + ">"
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("Feed: %s\nVote: `%s`\nTop: `%s`\nVerify: %s\nVerify msg: `%s`\nGenre: `%s`", feedCh, cfg.VoteMessageID, cfg.TopMessageID, verifyCh, cfg.VerifyMessageID, cfg.DefaultGenre), Flags: 1 << 6},
		})
	default:
		respondEphemeral(s, i, "Unknown config action.")
	}
}
