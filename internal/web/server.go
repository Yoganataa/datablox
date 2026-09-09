package web

import (
	"encoding/json"
	"time"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"datablox/internal/auth"
	"datablox/internal/auth/adapters"
	"datablox/internal/config"
	discordpkg "datablox/internal/discord"
	"datablox/internal/model"
	"datablox/internal/roblox"
	"datablox/internal/service"
	"datablox/internal/store"
	"datablox/internal/web/templates/pages"
	"datablox/utils"

	"github.com/bwmarrin/discordgo"
)

type pendingOAuth struct {
	State     string
	Verifier  string
	DiscordID string
	GuildID   string
	Expires   time.Time
}

type Server struct {
	cfg      *config.Config
	store    store.Store
	log      *slog.Logger
	discord  *discordgo.Session
	verify   *service.VerifyService
	mux      *http.ServeMux
	pending  map[string]pendingOAuth
	mu       sync.Mutex
	sessions map[string]webSession
	sessMu   sync.Mutex
	botRoles *adapters.BotRoleResolver
}

func New(cfg *config.Config, st store.Store, log *slog.Logger, discord *discordgo.Session) (*Server, error) {
	rc := roblox.New(cfg.RobloxTimeout)
	var verify *service.VerifyService
	if discord != nil {
		verify = &service.VerifyService{Store: st, Discord: discord, Client: rc}
	}
	s := &Server{cfg: cfg, store: st, log: log, discord: discord, verify: verify, mux: http.NewServeMux(), pending: make(map[string]pendingOAuth), sessions: make(map[string]webSession), botRoles: adapters.NewBotRoleResolverFromBoolMap(cfg.OwnerDiscordIDs, cfg.AdminDiscordIDs)}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/guilds", s.handleGuilds)
	s.mux.HandleFunc("/guild/", s.handleGuildDetail)
	s.mux.HandleFunc("/dashboard/guilds", s.handleGuilds)
	s.mux.HandleFunc("/dashboard/guild/", s.handleGuildDetail)
	s.mux.HandleFunc("/verify", s.handleVerifyPage)
	s.mux.HandleFunc("/privacy", s.handlePrivacy)
	s.mux.HandleFunc("/privacy/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/privacy", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/privacy-policy", s.handlePrivacy)
	s.mux.HandleFunc("/privacy-policy/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/privacy-policy", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/terms", s.handleTerms)
	s.mux.HandleFunc("/terms/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/terms", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/terms-of-service", s.handleTerms)
	s.mux.HandleFunc("/terms-of-service/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/terms-of-service", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/guide", s.handleGuide)
	s.mux.HandleFunc("/guide/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/guide", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/status", http.StatusMovedPermanently) })
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/guilds", s.handleListGuilds)
	s.mux.HandleFunc("/api/modules", s.handleModules)
	s.mux.HandleFunc("/api/guild-modules", s.handleGuildModules)
	s.mux.HandleFunc("/api/bindings", s.handleBindings)
	s.mux.HandleFunc("/api/reaction-roles", s.handleReactionRoles)
	s.mux.HandleFunc("/api/automod", s.handleAutomod)
	s.mux.HandleFunc("/api/welcome", s.handleWelcome)
	s.mux.HandleFunc("/api/levels", s.handleLevels)
	s.mux.HandleFunc("/auth/discord/login", s.handleDiscordLogin)
	s.mux.HandleFunc("/auth/discord/callback", s.handleDiscordCallback)
	s.mux.HandleFunc("/auth/roblox/login", s.handleRobloxLogin)
	s.mux.HandleFunc("/auth/roblox/callback", s.handleRobloxCallback)
	s.mux.HandleFunc("/logout", s.handleLogout)
	// Assets (Tailwind output + templui JS) — CLI workflow
	s.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	// templui embedded scripts (import workflow fallback)
	utils.SetupScriptRoutes(s.mux, s.cfg.LogLevel == "debug")
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) Start() error {
	addr := ":" + s.cfg.WebPort
	s.log.Info("web dashboard listening", "addr", addr, "url", s.cfg.WebURL)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) discordName(r *http.Request) string {
	if c, err := r.Cookie("discord_name"); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) discordAvatar(r *http.Request) string {
	if c, err := r.Cookie("discord_avatar"); err == nil {
		return c.Value
	}
	return ""
}

// navBotStaff reports whether the signed-in user is bot staff (BotOwner/BotAdmin).
// Display-only: backs the template isBotStaff flag (super-admin banner). Never a gate;
// all authorization decisions go through auth.Can()/CanTarget().
func (s *Server) navBotStaff(r *http.Request) bool {
	sess, ok := s.getSession(r)
	if !ok {
		return false
	}
	br := s.botRoles.ResolveBotRole(sess.DiscordID)
	return br == auth.BotOwner || br == auth.BotAdmin
}

func (s *Server) cookieSecure() bool {
	return strings.HasPrefix(s.cfg.WebURL, "https://")
}
func (s *Server) principalForGuild(r *http.Request, guildID string) (auth.Principal, bool) {
	sess, ok := s.getSession(r)
	if !ok {
		return auth.Principal{}, false
	}
	userID := sess.DiscordID
	// BotRole from config (cached resolver, config immutable after startup)
	botRole := s.botRoles.ResolveBotRole(userID)
	// GuildRole + Permissions from Discord
	var guildOwnerID string
	var permsBits int64
	if sess.DiscordToken != "" {
		for _, g := range s.fetchUserGuilds(sess.DiscordToken) {
			if g.ID == guildID {
				guildOwnerID = ""
				if g.Owner {
					// OwnerID is userID itself if owner
					guildOwnerID = userID
				}
				permsBits = g.Permissions
				break
			}
		}
	}
	// Fallback: try fetch via bot for ownerID
	if guildOwnerID == "" && s.discord != nil {
		if g, err := s.discord.Guild(guildID); err == nil && g != nil {
			guildOwnerID = g.OwnerID
		}
	}
	perms := discordpkg.ExtractPermissions(permsBits)
	guildResolver := adapters.NewGuildResolver()
	guildRole := guildResolver.ResolveGuildRole(guildOwnerID, userID, perms)
	return adapters.ResolvePrincipal(userID, guildID, botRole, guildRole, perms), true
}

type discordGuild = model.DiscordGuild

func (s *Server) fetchUserGuilds(token string) []model.DiscordGuild {
	if token == "" {
		return nil
	}
	req, _ := http.NewRequest("GET", "https://discord.com/api/users/@me/guilds", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var guilds []model.DiscordGuild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil
	}
	return guilds
}

func (s *Server) fetchGuildViaBot(guildID string) (name, icon string, memberCount int) {
	if s.cfg.DiscordToken == "" || guildID == "" {
		return "", "", 0
	}
	req, _ := http.NewRequest("GET", "https://discord.com/api/v10/guilds/"+guildID+"?with_counts=true", nil)
	req.Header.Set("Authorization", "Bot "+s.cfg.DiscordToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return "", "", 0
	}
	defer resp.Body.Close()
	var g struct {
		Name            string `json:"name"`
		Icon            string `json:"icon"`
		MemberCount     int    `json:"member_count"`
		ApproxMemberCount int `json:"approximate_member_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return "", "", 0
	}
	if g.MemberCount == 0 {
		g.MemberCount = g.ApproxMemberCount
	}
	return g.Name, g.Icon, g.MemberCount
}

func (s *Server) fetchChannelViaBot(channelID string) string {
	if s.cfg.DiscordToken == "" || channelID == "" {
		return ""
	}
	req, _ := http.NewRequest("GET", "https://discord.com/api/v10/channels/"+channelID, nil)
	req.Header.Set("Authorization", "Bot "+s.cfg.DiscordToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var ch struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return ""
	}
	if ch.Name != "" {
		return "#" + ch.Name
	}
	return ""
}


func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	guildCount, _ := s.store.CountGuilds(r.Context())
	expCount, _ := s.store.Count(r.Context())
	voteCount, _ := s.store.CountVotes(r.Context())
	_ = pages.Landing(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r), s.cfg.DiscordClientID, guildCount, expCount, voteCount).Render(r.Context(), w)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.getSession(r); !ok {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/guilds", http.StatusFound)
}

func (s *Server) handleGuilds(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.getSession(r)
	if !ok {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	tok := sess.DiscordToken
	userGuilds := s.fetchUserGuilds(tok)
	// Build guilds where principal can view panel (per-guild Can, not a global gate)
	adminIDs := make(map[string]bool)
	var adminGuilds []model.DiscordGuild
	for _, g := range userGuilds {
		p, ok := s.principalForGuild(r, g.ID)
		if !ok {
			continue
		}
		if auth.Can(p, auth.GuildPanelView, auth.Resource{GuildID: g.ID}) {
			adminIDs[g.ID] = true
			adminGuilds = append(adminGuilds, g)
		}
	}
	// Fetch configs + guild_modules for admin guilds (general)
	configs, _ := s.store.ListGuildConfigsByIDs(r.Context(), keys(adminIDs))
	cfgMap := make(map[string]model.GuildConfig, len(configs))
	for _, c := range configs {
		cfgMap[c.GuildID] = c
	}
	gmMap := make(map[string][]model.GuildModule, len(adminGuilds))
	for _, g := range adminGuilds {
		if gms, err := s.store.ListGuildModules(r.Context(), g.ID); err == nil {
			gmMap[g.ID] = gms
		}
	}
	_ = pages.Guilds(adminGuilds, cfgMap, gmMap, s.navBotStaff(r), s.discordName(r), s.discordAvatar(r)).Render(r.Context(), w)
}

func (s *Server) handleGuildDetail(w http.ResponseWriter, r *http.Request) {
	guildID := r.URL.Path
	if strings.HasPrefix(guildID, "/dashboard/guild/") {
		guildID = strings.TrimPrefix(guildID, "/dashboard/guild/")
	} else {
		guildID = strings.TrimPrefix(guildID, "/guild/")
	}
	if idx := strings.Index(guildID, "/"); idx >= 0 {
		guildID = guildID[:idx]
	}
	if guildID == "" {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.getSession(r); !ok {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	p, ok := s.principalForGuild(r, guildID)
	if !ok {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	if !auth.Can(p, auth.GuildPanelView, auth.Resource{GuildID: guildID}) {
		http.Error(w, "you are not admin of this server", http.StatusForbidden)
		return
	}
	cfg, _ := s.store.GetGuildConfig(r.Context(), guildID)
	bindings, _ := s.store.ListBindings(r.Context(), guildID)
	reactionRoles, _ := s.store.ListReactionRoles(r.Context(), guildID)
	guildModules, _ := s.store.ListGuildModules(r.Context(), guildID)
	// Fetch guild details for header (name, icon, members) — try bot cache, fallback to Bot REST, then user's guilds
	guildName := guildID
	guildIcon := ""
	memberCount := 0
	if s.discord != nil {
		if g, err := s.discord.Guild(guildID); err == nil && g != nil {
			guildName = g.Name
			guildIcon = g.Icon
			memberCount = g.MemberCount
		}
	}
	if guildName == guildID {
		if n, ic, mc := s.fetchGuildViaBot(guildID); n != "" {
			guildName = n
			guildIcon = ic
			memberCount = mc
		}
	}
	if guildName == guildID {
		if sess, ok := s.getSession(r); ok && sess.DiscordToken != "" {
			for _, g := range s.fetchUserGuilds(sess.DiscordToken) {
				if g.ID == guildID {
					guildName = g.Name
					guildIcon = g.Icon
					break
				}
			}
		}
	}
	// Resolve channel names for display (bot REST fallback, no raw ID leak)
	feedChannelName := ""
	verifyChannelName := ""
	if cfg.ChannelID != "" {
		if name := s.fetchChannelViaBot(cfg.ChannelID); name != "" {
			feedChannelName = name
		} else if s.discord != nil {
			if ch, err := s.discord.Channel(cfg.ChannelID); err == nil && ch != nil && ch.Name != "" {
				feedChannelName = "#" + ch.Name
			} else {
				feedChannelName = "#" + cfg.ChannelID
			}
		} else {
			feedChannelName = "#" + cfg.ChannelID
		}
	}
	if cfg.VerifyChannelID != "" {
		if name := s.fetchChannelViaBot(cfg.VerifyChannelID); name != "" {
			verifyChannelName = name
		} else if s.discord != nil {
			if ch, err := s.discord.Channel(cfg.VerifyChannelID); err == nil && ch != nil && ch.Name != "" {
				verifyChannelName = "#" + ch.Name
			} else {
				verifyChannelName = "#" + cfg.VerifyChannelID
			}
		} else {
			verifyChannelName = "#" + cfg.VerifyChannelID
		}
	}
	if memberCount == 0 {
		memberCount = -1 // unknown in web HMR, hide
	}
	_ = pages.GuildDetail(guildID, guildName, guildIcon, memberCount, feedChannelName, verifyChannelName, cfg, bindings, reactionRoles, guildModules, s.navBotStaff(r), s.discordName(r), s.discordAvatar(r)).Render(r.Context(), w)
}

func keys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func (s *Server) handleVerifyPage(w http.ResponseWriter, r *http.Request) {
	verifiedParam := r.URL.Query().Get("verified") == "1"
	verified := false
	if verifiedParam {
		if sess, ok := s.getSession(r); ok {
			if _, err := s.store.GetVerifiedUser(r.Context(), sess.DiscordID); err == nil {
				verified = true
			}
		}
	}
	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		if c, err := r.Cookie("verify_guild_id"); err == nil {
			guildID = c.Value
		}
	}
	_ = pages.Verify(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r), verified, guildID).Render(r.Context(), w)
}

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request) {
	_ = pages.Guide(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r)).Render(r.Context(), w)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	guildCount, _ := s.store.CountGuilds(r.Context())
	expCount, _ := s.store.Count(r.Context())
	voteCount, _ := s.store.CountVotes(r.Context())
	botOnline := s.discord != nil
	_ = pages.Status(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r), botOnline, guildCount, expCount, voteCount).Render(r.Context(), w)
}

func (s *Server) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	_ = pages.Privacy(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r)).Render(r.Context(), w)
}

func (s *Server) handleTerms(w http.ResponseWriter, r *http.Request) {
	_ = pages.Terms(s.navBotStaff(r), s.discordName(r), s.discordAvatar(r)).Render(r.Context(), w)
}

func (s *Server) handleListGuilds(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.getSession(r); !ok {
		http.Error(w, "admin only", http.StatusUnauthorized)
		return
	}
	guilds, err := s.store.ListGuildConfigs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Discovery: include only guilds where principal can view the panel.
	visible := make([]model.GuildConfig, 0, len(guilds))
	for _, g := range guilds {
		p, ok := s.principalForGuild(r, g.GuildID)
		if !ok {
			continue
		}
		if auth.Can(p, auth.GuildPanelView, auth.Resource{GuildID: g.GuildID}) {
			visible = append(visible, g)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(visible)
}

func (s *Server) handleBindings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		guildID := r.URL.Query().Get("guild_id")
		if guildID == "" {
			http.Error(w, "guild_id required", http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, guildID)
		if !ok || !auth.Can(p, auth.GuildBindingsView, auth.Resource{GuildID: guildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		bindings, _ := s.store.ListBindings(r.Context(), guildID)
		_ = json.NewEncoder(w).Encode(bindings)
	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		var b model.GuildBinding
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, b.GuildID)
		if !ok || !auth.Can(p, auth.GuildBindingsManage, auth.Resource{GuildID: b.GuildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		id, err := s.store.CreateBinding(r.Context(), b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]int64{"id": id})
	case http.MethodDelete:
		if !requireCSRF(w, r) {
			return
		}
		guildID := r.URL.Query().Get("guild_id")
		idStr := r.URL.Query().Get("id")
		var id int64
		fmt.Sscan(idStr, &id)
		p, ok := s.principalForGuild(r, guildID)
		if !ok || !auth.Can(p, auth.GuildBindingsManage, auth.Resource{GuildID: guildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteBinding(r.Context(), id, guildID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleReactionRoles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		guildID := r.URL.Query().Get("guild_id")
		if guildID == "" {
			http.Error(w, "guild_id required", http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, guildID)
		if !ok || !auth.Can(p, auth.GuildReactionView, auth.Resource{GuildID: guildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		roles, _ := s.store.ListReactionRoles(r.Context(), guildID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(roles)
	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		var rr model.ReactionRole
		if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, rr.GuildID)
		if !ok || !auth.Can(p, auth.GuildReactionManage, auth.Resource{GuildID: rr.GuildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		if rr.Mode == "" {
			rr.Mode = "normal"
		}
		id, err := s.store.CreateReactionRole(r.Context(), rr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"id": id})
	case http.MethodDelete:
		if !requireCSRF(w, r) {
			return
		}
		guildID := r.URL.Query().Get("guild_id")
		idStr := r.URL.Query().Get("id")
		var id int64
		fmt.Sscan(idStr, &id)
		p, ok := s.principalForGuild(r, guildID)
		if !ok || !auth.Can(p, auth.GuildReactionManage, auth.Resource{GuildID: guildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteReactionRole(r.Context(), id, guildID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAutomod(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gid := r.URL.Query().Get("guild_id")
		p, ok := s.principalForGuild(r, gid)
		if !ok || !auth.Can(p, auth.GuildAutomodView, auth.Resource{GuildID: gid}) {
			http.Error(w, "not admin", http.StatusForbidden)
			return
		}
		cfg, _ := s.store.GetAutomodConfig(r.Context(), gid)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		var cfg model.AutomodConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, cfg.GuildID)
		if !ok || !auth.Can(p, auth.GuildAutomodManage, auth.Resource{GuildID: cfg.GuildID}) {
			http.Error(w, "not admin", http.StatusForbidden)
			return
		}
		_ = s.store.SetAutomodConfig(r.Context(), cfg)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWelcome(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gid := r.URL.Query().Get("guild_id")
		p, ok := s.principalForGuild(r, gid)
		if !ok || !auth.Can(p, auth.GuildWelcomeView, auth.Resource{GuildID: gid}) {
			http.Error(w, "not admin", http.StatusForbidden)
			return
		}
		cfg, _ := s.store.GetWelcomeConfig(r.Context(), gid)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		var cfg model.WelcomeConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, cfg.GuildID)
		if !ok || !auth.Can(p, auth.GuildWelcomeManage, auth.Resource{GuildID: cfg.GuildID}) {
			http.Error(w, "not admin", http.StatusForbidden)
			return
		}
		_ = s.store.SetWelcomeConfig(r.Context(), cfg)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLevels(w http.ResponseWriter, r *http.Request) {
	gid := r.URL.Query().Get("guild_id")
	if gid == "" {
		http.Error(w, "guild_id required", http.StatusBadRequest)
		return
	}
	levels, _ := s.store.Leaderboard(r.Context(), gid, 20)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(levels)
}

func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	mods, _ := s.store.ListModules(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mods)
}

func (s *Server) handleGuildModules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gid := r.URL.Query().Get("guild_id")
		if gid == "" {
			http.Error(w, "guild_id required", http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, gid)
		if !ok || !auth.Can(p, auth.GuildModulesView, auth.Resource{GuildID: gid}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		gms, _ := s.store.ListGuildModules(r.Context(), gid)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gms)
	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		var req struct {
			GuildID string `json:"guild_id"`
			Slug    string `json:"slug"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.GuildID == "" || req.Slug == "" {
			http.Error(w, "guild_id and slug required", http.StatusBadRequest)
			return
		}
		p, ok := s.principalForGuild(r, req.GuildID)
		if !ok || !auth.Can(p, auth.GuildModulesManage, auth.Resource{GuildID: req.GuildID}) {
			http.Error(w, "not admin of this guild", http.StatusForbidden)
			return
		}
		if err := s.store.SetGuildModuleEnabled(r.Context(), req.GuildID, req.Slug, req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDiscordLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DiscordClientID == "" {
		http.Error(w, "DISCORD_CLIENT_ID not set. Set in .env (Discord Developer Portal → OAuth2)", http.StatusNotImplemented)
		return
	}
	state, _ := roblox.GenerateState()
	redirectURI := strings.TrimRight(s.cfg.WebURL, "/") + "/auth/discord/callback"
	s.mu.Lock()
	s.pending[state] = pendingOAuth{State: state, Expires: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	time.AfterFunc(5*time.Minute, func() {
		s.mu.Lock()
		delete(s.pending, state)
		s.mu.Unlock()
	})
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	url := roblox.DiscordAuthorizeURL(s.cfg.DiscordClientID, redirectURI, state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state/code", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	val, ok := s.pending[state]
	s.mu.Unlock()
	if !ok {
		// HMR restart or separate process: fallback to cookie state (web HMR loses pending map but cookie survives)
		if c, err := r.Cookie("oauth_state"); err != nil || c.Value != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
	} else if time.Now().After(val.Expires) {
		s.mu.Lock()
		delete(s.pending, state)
		s.mu.Unlock()
		http.Error(w, "state expired", http.StatusBadRequest)
		return
	} else {
		s.mu.Lock()
		delete(s.pending, state)
		s.mu.Unlock()
	}

	redirectURI := strings.TrimRight(s.cfg.WebURL, "/") + "/auth/discord/callback"
	data := url.Values{}
	data.Set("client_id", s.cfg.DiscordClientID)
	data.Set("client_secret", s.cfg.DiscordSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, _ := http.NewRequest("POST", "https://discord.com/api/oauth2/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req2, _ := http.NewRequest("GET", "https://discord.com/api/users/@me", nil)
	req2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp2.Body.Close()
	var user struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&user)
	avatarURL := ""
	if user.Avatar != "" {
		ext := "png"
		if len(user.Avatar) > 0 && user.Avatar[0:2] == "a_" {
			ext = "gif"
		}
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s", user.ID, user.Avatar, ext)
	}
	sessID, err := s.createSession(user.ID, tok.AccessToken)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, sessID)
	http.SetCookie(w, &http.Cookie{Name: "discord_name", Value: user.Username, Path: "/", MaxAge: 86400 * 7, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_avatar", Value: avatarURL, Path: "/", MaxAge: 86400 * 7, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	// Clear any legacy identity cookies from older versions.
	http.SetCookie(w, &http.Cookie{Name: "discord_id", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/dashboard?discord=ok", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.destroySession(w, r)
	http.SetCookie(w, &http.Cookie{Name: "discord_name", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_token", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_avatar", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) handleRobloxLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RobloxClientID == "" {
		http.Error(w, "ROBLOX_CLIENT_ID not set. Create app at https://create.roblox.com/dashboard/credentials (redirect_uri = "+s.cfg.WebURL+"/auth/roblox/callback)", http.StatusNotImplemented)
		return
	}
	discordID := ""
	if sess, ok := s.getSession(r); ok {
		discordID = sess.DiscordID
	}
	if discordID == "" {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	guildID := r.URL.Query().Get("guild_id")
	if guildID == "" {
		if c, err := r.Cookie("verify_guild_id"); err == nil {
			guildID = c.Value
		}
	}
	if guildID != "" {
		http.SetCookie(w, &http.Cookie{Name: "verify_guild_id", Value: guildID, Path: "/", MaxAge: 86400 * 7, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	}
	verifier, _ := roblox.GenerateCodeVerifier()
	challenge := roblox.CodeChallenge(verifier)
	state, _ := roblox.GenerateState()
	redirectURI := strings.TrimRight(s.cfg.WebURL, "/") + "/auth/roblox/callback"
	s.mu.Lock()
	s.pending[state] = pendingOAuth{State: state, Verifier: verifier, DiscordID: discordID, GuildID: guildID, Expires: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	time.AfterFunc(5*time.Minute, func() {
		s.mu.Lock()
		delete(s.pending, state)
		s.mu.Unlock()
	})
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	http.SetCookie(w, &http.Cookie{Name: "code_verifier", Value: verifier, Path: "/", HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	url := roblox.AuthorizeURL(s.cfg.RobloxClientID, redirectURI, state, challenge)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleRobloxCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state/code", http.StatusBadRequest)
		return
	}
	var verifier string
	var discordIDFromState string
	s.mu.Lock()
	val, ok := s.pending[state]
	s.mu.Unlock()
	if ok {
		if time.Now().After(val.Expires) {
			s.mu.Lock()
			delete(s.pending, state)
			s.mu.Unlock()
			http.Error(w, "state expired", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		delete(s.pending, state)
		s.mu.Unlock()
		verifier = val.Verifier
		discordIDFromState = val.DiscordID
	} else {
		if c, err := r.Cookie("oauth_state"); err != nil || c.Value != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		if c, err := r.Cookie("code_verifier"); err == nil {
			verifier = c.Value
		}
	}
	if verifier == "" {
		if c, err := r.Cookie("code_verifier"); err == nil {
			verifier = c.Value
		}
	}
	// make val available for discordID fallback below
	val = pendingOAuth{Verifier: verifier, DiscordID: discordIDFromState}

	redirectURI := strings.TrimRight(s.cfg.WebURL, "/") + "/auth/roblox/callback"
	data := url.Values{}
	data.Set("client_id", s.cfg.RobloxClientID)
	data.Set("client_secret", s.cfg.RobloxSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", verifier)

	req, _ := http.NewRequest("POST", "https://apis.roblox.com/oauth/v1/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("token exchange failed %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req2, _ := http.NewRequest("GET", "https://apis.roblox.com/oauth/v1/userinfo", nil)
	req2.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp2.Body.Close()
	var info struct {
		Sub      string `json:"sub"`
		Username string `json:"preferred_username"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&info); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	discordID := val.DiscordID
	if sess, ok := s.getSession(r); ok && sess.DiscordID != "" {
		discordID = sess.DiscordID
	}
	if discordID == "" {
		http.Error(w, "Discord not linked. Open this link from the Discord Verify button (it includes your Discord ID).", http.StatusBadRequest)
		return
	}
	var robloxID int64
	fmt.Sscan(info.Sub, &robloxID)
	_ = s.store.UpsertVerifiedUser(r.Context(), model.VerifiedUser{DiscordID: discordID, RobloxID: robloxID, RobloxUsername: info.Username})
	go s.syncVerifiedUser(discordID, robloxID)
	guildID := val.GuildID
	if guildID == "" {
		if c, err := r.Cookie("verify_guild_id"); err == nil {
			guildID = c.Value
		}
	}
	if guildID == "" {
		guildID = r.URL.Query().Get("guild_id")
	}
	if guildID != "" {
		http.SetCookie(w, &http.Cookie{Name: "verify_guild_id", Value: guildID, Path: "/", MaxAge: 86400*7, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, "/verify?verified=1&guild_id="+url.QueryEscape(guildID), http.StatusFound)
	} else {
		http.Redirect(w, r, "/verify?verified=1", http.StatusFound)
	}
}

func (s *Server) syncVerifiedUser(discordID string, robloxID int64) {
	if s.verify != nil {
		s.verify.SyncAllGuilds(nil, discordID, robloxID)
	} else {
		s.log.Info("verified sync triggered", "discord", discordID, "roblox", robloxID)
	}
}
