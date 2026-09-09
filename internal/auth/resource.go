package auth

type Resource struct {
	GuildID string
}

type Target struct {
	UserID    string
	GuildRole GuildRole
	BotRole   BotRole
}
