package auth

// Principal is a snapshot of identity + authority for a (user, guild) context.
// No discordgo, http, config, or database types are referenced here.
type Principal struct {
	UserID      string
	GuildID     string
	BotRole     BotRole
	GuildRole   GuildRole
	Permissions Permissions
}
