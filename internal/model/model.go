package model

import "time"

type Experience struct {
	UniverseID      int64
	PlaceID         int64
	Name            string
	Description     string
	CreatorID       int64
	CreatorName     string
	Genre           string
	MaxPlayers      int
	Playing         int
	Visits          int64
	UpVotes         int64
	DownVotes       int64
	ThumbnailURL    string
	RobloxURL       string
	RobloxCreated   time.Time
	RobloxUpdated   time.Time
	LastRefreshedAt time.Time
	CreatedAt       time.Time
}

type GuildConfig struct {
	GuildID          string
	ChannelID        string // feed channel (1 channel untuk vote #1 + top10 #2 + feed #3+)
	VoteMessageID    string
	TopMessageID     string
	VerifyChannelID  string // verify channel (terpisah dari feed)
	VerifyMessageID  string
	DefaultGenre     string
}

type Vote struct {
	MessageID  string
	UserID     string
	UniverseID int64
	Emoji      string
	CreatedAt  time.Time
}

type VerifiedUser struct {
	DiscordID      string
	RobloxID       int64
	RobloxUsername string
	VerifiedAt     time.Time
	UpdatedAt      time.Time
}

type DiscordGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Permissions int64  `json:"permissions"`
	Owner       bool   `json:"owner"`
}

type GuildBinding struct {
	ID               int64
	GuildID          string
	GroupID          int64
	RobloxRoleID     *int64
	RankMin          *int
	RankMax          *int
	DiscordRoleID    string
	NicknameTemplate string
	CreatedAt        time.Time
}
