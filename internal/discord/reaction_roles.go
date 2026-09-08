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
	for _, rr := range roles {
		if !emojiMatches(rr.Emoji, r.Emoji) {
			continue
		}
		switch rr.Mode {
		case "drop":
			continue
		case "reverse":
			if hasRole(s, r.GuildID, r.UserID, rr.RoleID) {
				_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, rr.RoleID)
			} else {
				_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, rr.RoleID)
			}
		case "unique":
			for _, other := range roles {
				if other.RoleID != rr.RoleID {
					_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, other.RoleID)
				}
			}
			_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, rr.RoleID)
		case "verify":
			_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, rr.RoleID)
			_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.APIName(), r.UserID)
		default:
			_ = s.GuildMemberRoleAdd(r.GuildID, r.UserID, rr.RoleID)
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
		if rr.Mode == "normal" || rr.Mode == "" || rr.Mode == "unique" {
			_ = s.GuildMemberRoleRemove(r.GuildID, r.UserID, rr.RoleID)
		}
		// verify / reverse / drop do not remove on unreact (one-way)
	}
}

func emojiMatches(stored string, e discordgo.Emoji) bool {
	if stored == e.Name {
		return true
	}
	if stored == e.APIName() {
		return true
	}
	if e.ID != "" {
		if stored == e.Name+":"+e.ID {
			return true
		}
		if stored == "a:"+e.Name+":"+e.ID && e.Animated {
			return true
		}
	}
	stored = strings.Trim(stored, "<>:")
	if e.ID != "" && stored == e.Name+":"+e.ID {
		return true
	}
	// normalized form via NormalizeEmoji
	if NormalizeEmoji(stored) == emojiKey(e) {
		return true
	}
	return false
}

func emojiKey(e discordgo.Emoji) string {
	if e.ID != "" {
		return e.Name + ":" + e.ID
	}
	return e.Name
}

// NormalizeEmoji specter-style: "<:name:id>" or "<a:name:id>" -> "name:id", unicode stays
func NormalizeEmoji(raw string) string {
	if len(raw) > 2 && raw[0] == '<' && raw[len(raw)-1] == '>' {
		inner := raw[1 : len(raw)-1]
		if len(inner) > 0 && inner[0] == ':' {
			inner = inner[1:]
		} else if len(inner) > 2 && inner[0:2] == "a:" {
			inner = inner[2:]
		}
		return inner
	}
	return raw
}

func hasRole(s *discordgo.Session, guildID, userID, roleID string) bool {
	m, err := s.GuildMember(guildID, userID)
	if err != nil || m == nil {
		return false
	}
	for _, r := range m.Roles {
		if r == roleID {
			return true
		}
	}
	return false
}
