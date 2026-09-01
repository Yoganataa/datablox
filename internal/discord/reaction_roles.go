package discord

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) onReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}
	ctx := context.Background()
	roles, err := b.svc.Store.ListReactionRolesByMessage(ctx, r.GuildID, r.MessageID)
	if err != nil || len(roles) == 0 {
		return
	}
	emojiKey := r.Emoji.Name
	if r.Emoji.ID != "" {
		emojiKey = r.Emoji.Name + ":" + r.Emoji.ID
		if r.Emoji.Animated {
			emojiKey = "a:" + emojiKey
		}
	}
	for _, rr := range roles {
		if !emojiMatches(rr.Emoji, r.Emoji) {
			continue
		}
		// Verify-only or normal: give role
		if rr.Mode == "drop" {
			continue
		}
		_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, rr.RoleID)
		if rr.Mode == "verify" {
			// remove reaction to keep counter 1 (verification gate)
			_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.APIName(), r.UserID)
		}
		if rr.Mode == "unique" {
			// remove other roles from same message
			for _, other := range roles {
				if other.RoleID != rr.RoleID {
					_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, other.RoleID)
				}
			}
		}
	}
}

func (b *Bot) onReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}
	ctx := context.Background()
	roles, err := b.svc.Store.ListReactionRolesByMessage(ctx, r.GuildID, r.MessageID)
	if err != nil || len(roles) == 0 {
		return
	}
	for _, rr := range roles {
		if !emojiMatches(rr.Emoji, r.Emoji) {
			continue
		}
		if rr.Mode == "normal" || rr.Mode == "" {
			_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, rr.RoleID)
		}
		if rr.Mode == "unique" {
			_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, rr.RoleID)
		}
		// verify/drop do not remove on unreact
	}
}

func emojiMatches(stored string, e discordgo.Emoji) bool {
	// stored can be unicode emoji like "✅" or custom "name:id" or "a:name:id"
	if stored == e.Name {
		return true
	}
	if stored == e.APIName() {
		return true
	}
	// custom without animated prefix
	if e.ID != "" {
		if stored == e.Name+":"+e.ID {
			return true
		}
		if stored == "a:"+e.Name+":"+e.ID && e.Animated {
			return true
		}
	}
	// case where stored is like "<:name:id>"
	stored = strings.Trim(stored, "<>:")
	if stored == e.Name+":"+e.ID {
		return true
	}
	return false
}
