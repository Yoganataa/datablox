package service

import (
	"context"
	"fmt"
	"strings"

	"datablox/internal/model"
	"datablox/internal/roblox"
	"datablox/internal/store"

	"github.com/bwmarrin/discordgo"
)

// VerifyService handles Bloxlink-like role/nickname sync.
type VerifyService struct {
	Store   store.Store
	Discord *discordgo.Session
	Client  *roblox.Client
}

// SyncMember syncs a verified user to all applicable guild bindings for a guild.
func (s *VerifyService) SyncMember(ctx context.Context, guildID, discordID string, robloxID int64) error {
	if s.Discord == nil {
		return nil
	}
	bindings, err := s.Store.ListBindings(ctx, guildID)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}
	groups, err := s.Client.GetUserGroups(ctx, robloxID)
	if err != nil {
		return err
	}
	groupMap := make(map[int64]roblox.UserGroup)
	for _, g := range groups {
		groupMap[g.GroupID] = g
	}

	var rolesToAdd []string
	var nickname string
	for _, b := range bindings {
		g, ok := groupMap[b.GroupID]
		if !ok {
			continue
		}
		matched := false
		if b.RobloxRoleID != nil && int64(g.RoleID) == *b.RobloxRoleID {
			matched = true
		} else if b.RankMin != nil && b.RankMax != nil {
			if g.Rank >= *b.RankMin && g.Rank <= *b.RankMax {
				matched = true
			}
		} else if b.RankMin == nil && b.RankMax == nil && b.RobloxRoleID == nil {
			matched = true
		}
		if matched {
			rolesToAdd = append(rolesToAdd, b.DiscordRoleID)
			if b.NicknameTemplate != "" && nickname == "" {
				nickname = strings.ReplaceAll(b.NicknameTemplate, "{robloxName}", g.GroupName)
				nickname = strings.ReplaceAll(nickname, "{groupRank}", fmt.Sprintf("%d", g.Rank))
				nickname = strings.ReplaceAll(nickname, "{robloxUsername}", g.GroupName)
			}
		}
	}

	if len(rolesToAdd) > 0 {
		for _, roleID := range rolesToAdd {
			_ = s.Discord.GuildMemberRoleAdd(guildID, discordID, roleID)
		}
	}
	if nickname != "" {
		_ = s.Discord.GuildMemberNickname(guildID, discordID, nickname)
	}
	return nil
}

// SyncAllGuilds syncs a verified user across all guilds they are in (best effort).
func (s *VerifyService) SyncAllGuilds(ctx context.Context, discordID string, robloxID int64) {
	if s.Discord == nil {
		return
	}
	guilds, _ := s.Store.ListGuildConfigs(ctx)
	for _, g := range guilds {
		if _, err := s.Discord.GuildMember(g.GuildID, discordID); err != nil {
			continue
		}
		_ = s.SyncMember(ctx, g.GuildID, discordID, robloxID)
	}
}

func (s *VerifyService) EnsureVerified(ctx context.Context, discordID string, robloxID int64, username string) error {
	_ = s.Store.UpsertVerifiedUser(ctx, model.VerifiedUser{DiscordID: discordID, RobloxID: robloxID, RobloxUsername: username})
	s.SyncAllGuilds(ctx, discordID, robloxID)
	return nil
}
