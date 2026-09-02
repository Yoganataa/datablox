package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/config"
	"datablox/internal/service"
)

type Bot struct {
	cfg    *config.Config
	log    *slog.Logger
	svc    *service.ExperienceService
	sess   *discordgo.Session
	cmdIDs map[string]string
}

func New(cfg *config.Config, log *slog.Logger, svc *service.ExperienceService) (*Bot, error) {
	sess, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, err
	}
	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions
	return &Bot{
		cfg:    cfg,
		log:    log,
		svc:    svc,
		sess:   sess,
		cmdIDs: map[string]string{},
	}, nil
}

func (b *Bot) Session() *discordgo.Session { return b.sess }

func (b *Bot) Run(ctx context.Context) error {
	b.sess.AddHandler(b.onInteraction)
	b.sess.AddHandler(b.onMessageCreate)
	b.sess.AddHandler(b.onMessageDelete)
	b.sess.AddHandler(b.onGuildMemberAdd)
	b.sess.AddHandler(b.onReactionAdd)
	b.sess.AddHandler(b.onReactionRemove)
	if err := b.sess.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}
	defer b.sess.Close()

	if err := b.registerCommands(); err != nil {
		return err
	}
	b.log.Info("bot ready", "user", b.sess.State.User.Username)

	<-ctx.Done()
	return nil
}

func (b *Bot) registerCommands() error {
	existing := map[string]*discordgo.ApplicationCommand{}
	cmds, err := b.sess.ApplicationCommands(b.sess.State.User.ID, b.cfg.GuildID)
	if err == nil {
		for _, c := range cmds {
			existing[c.Name] = c
		}
	}

	desired := map[string]bool{}
	for _, cmd := range Commands() {
		desired[cmd.Name] = true
		if ex, ok := existing[cmd.Name]; ok {
			edited, err := b.sess.ApplicationCommandEdit(b.sess.State.User.ID, b.cfg.GuildID, ex.ID, cmd)
			if err != nil {
				b.log.Error("failed to update command", "cmd", cmd.Name, "err", err)
				continue
			}
			b.cmdIDs[cmd.Name] = edited.ID
			b.log.Info("command updated", "cmd", cmd.Name)
			continue
		}
		created, err := b.sess.ApplicationCommandCreate(b.sess.State.User.ID, b.cfg.GuildID, cmd)
		if err != nil {
			b.log.Error("failed to register command", "cmd", cmd.Name, "err", err)
			continue
		}
		b.cmdIDs[cmd.Name] = created.ID
		b.log.Info("command registered", "cmd", cmd.Name)
	}
	// Delete obsolete commands (e.g. games, game, random, trending, refresh yang merge ke /datablox)
	for name, ex := range existing {
		if !desired[name] {
			if err := b.sess.ApplicationCommandDelete(b.sess.State.User.ID, b.cfg.GuildID, ex.ID); err != nil {
				b.log.Warn("failed to delete old command", "cmd", name, "err", err)
			} else {
				b.log.Info("old command deleted", "cmd", name)
			}
		}
	}
	return nil
}

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
		return
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		ctx, cancel := context.WithTimeout(context.Background(), 15*1e9)
		defer cancel()
		switch data.Name {
		case "datablox":
			b.handlePanel(ctx, i, data)
		case "add":
			b.handleAdd(ctx, i, data)
		case "search":
			b.handleSearch(ctx, i, data)
		case "config":
			b.handleConfig(ctx, i, data)
		case "warn":
			b.handleWarn(ctx, i, data)
		case "mute":
			b.handleMute(ctx, i, data)
		case "ban":
			b.handleBan(ctx, i, data)
		case "kick":
			b.handleKick(ctx, i, data)
		case "purge":
			b.handlePurge(ctx, i, data)
		case "level":
			b.handleLevel(ctx, i, data)
		case "leaderboard":
			b.handleLeaderboard(ctx, i, data)
		default:
			respondText(s, i, "Unknown command.")
		}
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModal(s, i)
	}
}

func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	customID := data.CustomID
	switch {
	case strings.HasPrefix(customID, "vote:select:"):
		b.handleVoteSelect(s, i, data)
	case strings.HasPrefix(customID, "vote:prev:"), strings.HasPrefix(customID, "vote:next:"):
		b.handleVoteNav(s, i, data)
	case strings.HasPrefix(customID, "vote:search:"):
		b.handleVoteSearchButton(s, i)
	case customID == "verify:start":
		b.handleVerifyStart(s, i)
	case strings.HasPrefix(customID, "panel:"):
		b.handlePanelComponent(s, i, data)
	case strings.HasPrefix(customID, "config:"):
		b.handleConfigComponent(s, i, data)
	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Unknown component.", Flags: 1 << 6},
		})
	}
}

func (b *Bot) handleModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	switch {
	case strings.HasPrefix(data.CustomID, "vote:search:modal"):
		b.handleVoteSearchModal(s, i, data)
	case strings.HasPrefix(data.CustomID, "panel:search:modal"):
		b.handlePanelModal(s, i, data)
	}
}

func option(data *discordgo.ApplicationCommandInteractionData, name string) string {
	for _, o := range data.Options {
		if o.Name == name {
			if s, ok := o.Value.(string); ok {
				return s
			}
			continue
		}
		if o.Type == discordgo.ApplicationCommandOptionSubCommand && o.Options != nil {
			for _, sub := range o.Options {
				if sub.Name == name {
					if s, ok := sub.Value.(string); ok {
						return s
					}
				}
			}
		}
	}
	return ""
}

func respondText(s *discordgo.Session, i *discordgo.InteractionCreate, text string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: text},
	})
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, text string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: text, Flags: 1 << 6},
	})
}

func respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components ...discordgo.MessageComponent) {
	data := &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}}
	if len(components) > 0 {
		data.Components = []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: components},
		}
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, err error) {
	msg := "⚠️ " + err.Error()
	if len(msg) > 1800 {
		msg = msg[:1800]
	}
	respondText(s, i, msg)
}

func shorten(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
