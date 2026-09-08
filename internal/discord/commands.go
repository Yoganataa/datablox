package discord

import (
	"github.com/bwmarrin/discordgo"

	"datablox/internal/service"
)

// Commands returns the slash command definitions to register.
func Commands() []*discordgo.ApplicationCommand {
	genres := service.ValidGenres()
	genreChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(genres))
	for _, g := range genres {
		genreChoices = append(genreChoices, &discordgo.ApplicationCommandOptionChoice{
			Name: g, Value: g,
		})
	}
	genreOpt := &discordgo.ApplicationCommandOption{
		Name:        "genre",
		Description: "Filter by genre",
		Type:        discordgo.ApplicationCommandOptionString,
		Required:    false,
		Choices:     genreChoices,
	}
	manageGuild := int64(discordgo.PermissionManageGuild)
	base := []*discordgo.ApplicationCommand{
		{
			Name:        "datablox",
			Description: "Open Datablox dashboard (central panel)",
		},
		{
			Name:        "add",
			Description: "Add a Roblox experience to the catalog",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "url", Description: "Roblox experience URL or ID (e.g. https://www.roblox.com/games/123/Fisch)", Type: discordgo.ApplicationCommandOptionString, Required: true},
				genreOpt,
			},
		},
		{
			Name:        "search",
			Description: "Search experiences (shortcut, also in /datablox)",
			Options: []*discordgo.ApplicationCommandOption{
				{Name: "query", Description: "Keyword (name or description)", Type: discordgo.ApplicationCommandOptionString, Required: true, Autocomplete: true},
				genreOpt,
			},
		},
		{
			Name:        "config",
			Description: "Open configuration panel (feed channel, genre)",
			DefaultMemberPermissions: &manageGuild,
		},
	}
	mods := moderationCommands()
	base = append(base, mods...)
	base = append(base, reactionRoleCommands())
	return base
}
