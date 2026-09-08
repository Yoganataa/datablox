package core

import (
	"sort"

	"datablox/internal/model"
)

// ModuleDef defines a module for UI — general, not hero.
type ModuleDef struct {
	Slug        string
	Name        string
	Icon        string
	Description string
}

var AllModules = []ModuleDef{
	{Slug: "automod", Name: "AutoMod", Icon: "shield-alert", Description: "Automated moderation filters"},
	{Slug: "bindings", Name: "Bindings", Icon: "link", Description: "Roblox group role bindings"},
	{Slug: "feed", Name: "Feed", Icon: "rss", Description: "Roblox experience catalog feed"},
	{Slug: "leveling", Name: "Leveling", Icon: "trophy", Description: "XP and level system"},
	{Slug: "moderation", Name: "Moderation", Icon: "gavel", Description: "Warn, mute, ban and purge"},
	{Slug: "reaction_roles", Name: "Reaction Roles", Icon: "smile-plus", Description: "Emoji role assignment"},
	{Slug: "verify", Name: "Verify", Icon: "badge-check", Description: "Roblox verification"},
	{Slug: "welcome", Name: "Welcome", Icon: "hand", Description: "Welcome messages and auto-role"},
}

// SortedModules returns modules sorted: enabled first (A-Z) then disabled A-Z.
// guildModules map slug->enabled, if missing treat as disabled.
func SortedModules(guildModules []model.GuildModule) []ModuleDef {
	enabled := map[string]bool{}
	for _, gm := range guildModules {
		enabled[gm.Slug] = gm.Enabled
	}
	out := make([]ModuleDef, len(AllModules))
	copy(out, AllModules)
	sort.Slice(out, func(i, j int) bool {
		ei, ej := enabled[out[i].Slug], enabled[out[j].Slug]
		if ei != ej {
			return ei && !ej // enabled first
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// SortedModulesAlpha returns alphabet only (landing without guild context)
func SortedModulesAlpha() []ModuleDef {
	out := make([]ModuleDef, len(AllModules))
	copy(out, AllModules)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
