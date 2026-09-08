package core

import (
	"github.com/bwmarrin/discordgo"
)

// Command bundles a slash command with routing metadata — specter-go pattern.
// Group ties to access gate, RequiredPerm surfaces to Discord as default_member_permissions.
type Command struct {
	Def          *discordgo.ApplicationCommand
	Group        string
	RequiredPerm int64
	Handler      func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// Router dispatches interactions with panic recovery (specter-go style).
// Lightweight shim: Datablox still uses Bot.onInteraction switch, this is forward-compat.
type Router struct {
	commands   map[string]Command
	components map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

func NewRouter() *Router {
	return &Router{commands: map[string]Command{}, components: map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}}
}

func (r *Router) Register(cmd Command) {
	if cmd.Def == nil || cmd.Handler == nil {
		return
	}
	if cmd.RequiredPerm != 0 && cmd.Def.DefaultMemberPermissions == nil {
		perm := cmd.RequiredPerm
		cmd.Def.DefaultMemberPermissions = &perm
	}
	r.commands[cmd.Def.Name] = cmd
}

func (r *Router) RegisterComponent(prefix string, h func(s *discordgo.Session, i *discordgo.InteractionCreate)) {
	r.components[prefix] = h
}

func (r *Router) Definitions() []*discordgo.ApplicationCommand {
	defs := make([]*discordgo.ApplicationCommand, 0, len(r.commands))
	for _, c := range r.commands {
		defs = append(defs, c.Def)
	}
	return defs
}

func (r *Router) Lookup(name string) (Command, bool) { c, ok := r.commands[name]; return c, ok }
