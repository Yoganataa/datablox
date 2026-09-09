package auth

type BotRole uint8

const (
	BotNone BotRole = iota
	BotAdmin
	BotOwner
)

func (r BotRole) String() string {
	switch r {
	case BotOwner:
		return "BotOwner"
	case BotAdmin:
		return "BotAdmin"
	default:
		return "BotNone"
	}
}

type GuildRole uint8

const (
	GuildMember GuildRole = iota
	GuildAdmin
	GuildOwner
)

func (r GuildRole) String() string {
	switch r {
	case GuildOwner:
		return "GuildOwner"
	case GuildAdmin:
		return "GuildAdmin"
	default:
		return "GuildMember"
	}
}
