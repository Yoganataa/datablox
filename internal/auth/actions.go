package auth

type Action string

const (
	GuildPanelView      Action = "guild.panel.view"
	GuildModulesView    Action = "guild.modules.view"
	GuildModulesManage  Action = "guild.modules.manage"
	GuildAutomodView    Action = "guild.automod.view"
	GuildAutomodManage  Action = "guild.automod.manage"
	GuildWelcomeView    Action = "guild.welcome.view"
	GuildWelcomeManage  Action = "guild.welcome.manage"
	GuildLevelsView     Action = "guild.levels.view"
	GuildLevelsManage   Action = "guild.levels.manage"
	GuildVerifyView     Action = "guild.verify.view"
	GuildVerifyManage   Action = "guild.verify.manage"
	GuildFeedView       Action = "guild.feed.view"
	GuildFeedManage     Action = "guild.feed.manage"
	GuildBindingsView   Action = "guild.bindings.view"
	GuildBindingsManage Action = "guild.bindings.manage"
	GuildReactionView   Action = "guild.reaction_roles.view"
	GuildReactionManage Action = "guild.reaction_roles.manage"
	ModerationWarn      Action = "moderation.warn"
	ModerationMute      Action = "moderation.mute"
	ModerationKick      Action = "moderation.kick"
	ModerationBan       Action = "moderation.ban"
	ModerationPurge     Action = "moderation.purge"
	BotAdminsManage     Action = "bot.admins.manage"
	BotOwnerManage      Action = "bot.owner.manage"
)

type Scope uint8

const (
	ScopeGlobal Scope = iota
	ScopeGuild
)

var actionScope = map[Action]Scope{
	BotAdminsManage:     ScopeGlobal,
	BotOwnerManage:      ScopeGlobal,
	GuildPanelView:      ScopeGuild,
	GuildModulesView:    ScopeGuild,
	GuildModulesManage:  ScopeGuild,
	GuildAutomodView:    ScopeGuild,
	GuildAutomodManage:  ScopeGuild,
	GuildWelcomeView:    ScopeGuild,
	GuildWelcomeManage:  ScopeGuild,
	GuildLevelsView:     ScopeGuild,
	GuildLevelsManage:   ScopeGuild,
	GuildVerifyView:     ScopeGuild,
	GuildVerifyManage:   ScopeGuild,
	GuildFeedView:       ScopeGuild,
	GuildFeedManage:     ScopeGuild,
	GuildBindingsView:   ScopeGuild,
	GuildBindingsManage: ScopeGuild,
	GuildReactionView:   ScopeGuild,
	GuildReactionManage: ScopeGuild,
	ModerationWarn:      ScopeGuild,
	ModerationMute:      ScopeGuild,
	ModerationKick:      ScopeGuild,
	ModerationBan:       ScopeGuild,
	ModerationPurge:     ScopeGuild,
}

func (a Action) Scope() Scope {
	if s, ok := actionScope[a]; ok {
		return s
	}
	return ScopeGuild
}

var knownActions = map[Action]struct{}{
	GuildPanelView: {}, GuildModulesView: {}, GuildModulesManage: {},
	GuildAutomodView: {}, GuildAutomodManage: {},
	GuildWelcomeView: {}, GuildWelcomeManage: {},
	GuildLevelsView: {}, GuildLevelsManage: {},
	GuildVerifyView: {}, GuildVerifyManage: {},
	GuildFeedView: {}, GuildFeedManage: {},
	GuildBindingsView: {}, GuildBindingsManage: {},
	GuildReactionView: {}, GuildReactionManage: {},
	ModerationWarn: {}, ModerationMute: {}, ModerationKick: {}, ModerationBan: {}, ModerationPurge: {},
	BotAdminsManage: {}, BotOwnerManage: {},
}

func (a Action) IsKnown() bool {
	_, ok := knownActions[a]
	return ok
}
