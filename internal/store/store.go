package store

import (
	"context"

	"datablox/internal/model"
)

type Store interface {
	UpsertExperience(ctx context.Context, exp model.Experience) error
	GetByUniverseID(ctx context.Context, universeID int64) (model.Experience, error)
	GetByPlaceID(ctx context.Context, placeID int64) (model.Experience, error)
	Search(ctx context.Context, query, genre string, limit int, offset int) ([]model.Experience, int, error)
	ListByGenre(ctx context.Context, genre string, limit, offset int) ([]model.Experience, int, error)
	Random(ctx context.Context, genre string) (model.Experience, error)
	Trending(ctx context.Context, limit int) ([]model.Experience, error)
	AllUniverseIDs(ctx context.Context) ([]int64, error)
	Count(ctx context.Context) (int, error)
	CountGuilds(ctx context.Context) (int, error)
	CountVotes(ctx context.Context) (int, error)
	CountVerifiedUsers(ctx context.Context) (int, error)
	UpdatePlayingAndThumbnail(ctx context.Context, universeID int64, playing, maxPlayers int, thumbnailURL string) error
	GetGuildConfig(ctx context.Context, guildID string) (model.GuildConfig, error)
	SetGuildConfig(ctx context.Context, cfg model.GuildConfig) error
	ListGuildConfigs(ctx context.Context) ([]model.GuildConfig, error)
	// Feed / vote (1 channel, 1 vote message paginated 25/halaman, top10 dari vote)
	HasVote(ctx context.Context, messageID, userID string, universeID int64) (bool, error)
	GetUserVotes(ctx context.Context, messageID, userID string) ([]int64, error)
	GetVoteCounts(ctx context.Context, messageID string) (map[int64]int, error)
	GetTopVoted(ctx context.Context, messageID string, limit int) ([]model.Experience, error)
	AddVote(ctx context.Context, v model.Vote) error
	RemoveVote(ctx context.Context, messageID, userID string, universeID int64) error
	ToggleVote(ctx context.Context, v model.Vote) (bool, error)
	GetVotesByMessage(ctx context.Context, messageID string) ([]model.Vote, error)
	CountVotesByUniverse(ctx context.Context, messageID string, universeID int64) (int, error)
	// Bloxlink-like verification + bindings
	UpsertVerifiedUser(ctx context.Context, u model.VerifiedUser) error
	GetVerifiedUser(ctx context.Context, discordID string) (model.VerifiedUser, error)
	ListVerifiedUsers(ctx context.Context, limit int) ([]model.VerifiedUser, error)
	CreateBinding(ctx context.Context, b model.GuildBinding) (int64, error)
	ListBindings(ctx context.Context, guildID string) ([]model.GuildBinding, error)
	DeleteBinding(ctx context.Context, id int64, guildID string) error
	Close() error
}
