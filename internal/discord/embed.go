package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"datablox/internal/model"
	"datablox/internal/service"
)

// experienceEmbed renders a Discord embed + Play button for an experience.
func experienceEmbed(e model.Experience) (*discordgo.MessageEmbed, discordgo.MessageComponent) {
	desc := e.Description
	if len([]rune(desc)) > 180 {
		desc = string([]rune(desc)[:177]) + "..."
	}
	if desc != "" {
		desc = "```" + desc + "\n```"
	} else {
		desc = "_No description._"
	}

	// Roblox global rating (not community vote)
	likeValue := "—"
	if e.UpVotes+e.DownVotes > 0 {
		total := float64(e.UpVotes + e.DownVotes)
		pct := int(float64(e.UpVotes)/total*100 + 0.5)
		likeValue = fmt.Sprintf("👍 %d%% (%s / %s)", pct, service.FormatCount(int(e.UpVotes)), service.FormatCount(int(e.DownVotes)))
	}

	visitsValue := "—"
	if e.Visits > 0 {
		visitsValue = fmt.Sprintf("👁️ %s", service.FormatCount64(e.Visits))
	}

	embed := &discordgo.MessageEmbed{
		Title:       e.Name,
		Description: desc,
		Color:       0x00A2FF, // Roblox blue
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Genre", Value: fmt.Sprintf("`%s`", e.Genre), Inline: true},
			{Name: "Now Playing", Value: fmt.Sprintf("👥 %s", service.FormatCount(e.Playing)), Inline: true},
			{Name: "Total Visits", Value: visitsValue, Inline: true},
			{Name: "Roblox Rating", Value: likeValue, Inline: true},
			{Name: "Max Players", Value: service.FormatCount(e.MaxPlayers), Inline: true},
			{Name: "Creator", Value: e.CreatorName, Inline: true},
		},
		URL: e.RobloxURL,
	}
	if e.ThumbnailURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: e.ThumbnailURL}
	}
	// Footer: created/updated/refresh
	footer := buildFooter(e)
	if footer != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: footer}
	}

	button := discordgo.Button{
		Label: "▶ Play",
		Style: discordgo.LinkButton,
		URL:   e.RobloxURL,
	}

	return embed, button
}

func buildFooter(e model.Experience) string {
	var parts []string
	if !e.RobloxCreated.IsZero() {
		parts = append(parts, "Created "+e.RobloxCreated.Format("2006-01-02"))
	}
	if !e.RobloxUpdated.IsZero() {
		parts = append(parts, "Updated "+formatRelative(e.RobloxUpdated))
	}
	if !e.LastRefreshedAt.IsZero() {
		parts = append(parts, "Refreshed "+formatRelative(e.LastRefreshedAt))
	}
	if len(parts) == 0 {
		return "Datablox"
	}
	s := "Datablox • "
	for i, p := range parts {
		if i > 0 {
			s += " • "
		}
		s += p
	}
	return s
}

func formatRelative(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// autocompleteText builds an autocomplete Choice from a name string.
func autocompleteText(s string) *discordgo.ApplicationCommandOptionChoice {
	return &discordgo.ApplicationCommandOptionChoice{Name: s, Value: s}
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
