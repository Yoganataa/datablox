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

	"datablox/internal/config"
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
	Expires   time.Time
}

type Server struct {
	cfg     *config.Config
	store   store.Store
	log     *slog.Logger
	discord *discordgo.Session
	verify  *service.VerifyService
	mux     *http.ServeMux
	pending map[string]pendingOAuth
	mu      sync.Mutex
}

func New(cfg *config.Config, st store.Store, log *slog.Logger, discord *discordgo.Session) (*Server, error) {
	rc := roblox.New(cfg.RobloxTimeout)
	var verify *service.VerifyService
	if discord != nil {
		verify = &service.VerifyService{Store: st, Discord: discord, Client: rc}
	}
	s := &Server{cfg: cfg, store: st, log: log, discord: discord, verify: verify, mux: http.NewServeMux(), pending: make(map[string]pendingOAuth)}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/dashboard/guilds", s.handleGuilds)
	s.mux.HandleFunc("/dashboard/guild/", s.handleGuildDetail)
	s.mux.HandleFunc("/verify", s.handleVerifyPage)
	s.mux.HandleFunc("/privacy", s.handlePrivacy)
	s.mux.HandleFunc("/privacy-policy", s.handlePrivacy)
	s.mux.HandleFunc("/terms", s.handleTerms)
	s.mux.HandleFunc("/terms-of-service", s.handleTerms)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/guilds", s.handleListGuilds)
	s.mux.HandleFunc("/api/bindings", s.handleBindings)
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

func (s *Server) isAdmin(r *http.Request) bool {
	c, err := r.Cookie("discord_id")
	if err != nil || c.Value == "" {
		return false
	}
	return s.cfg.AdminDiscordIDs[c.Value]
}

func (s *Server) discordName(r *http.Request) string {
	if c, err := r.Cookie("discord_name"); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) cookieSecure() bool {
	return strings.HasPrefix(s.cfg.WebURL, "https://")
}
func (s *Server) isGuildAdmin(r *http.Request, guildID string) bool {
	// Super admin bypass
	if s.isAdmin(r) {
		return true
	}
	tok := ""
	if c, err := r.Cookie("discord_token"); err == nil {
		tok = c.Value
	}
	if tok == "" {
		return false
	}
	guilds := s.fetchUserGuilds(tok)
	for _, g := range guilds {
		if g.ID == guildID && (g.Permissions&0x20 != 0 || g.Permissions&0x8 != 0) {
			return true
		}
	}
	return false
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


func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_ = pages.Landing(s.isAdmin(r), s.discordName(r)).Render(r.Context(), w)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	guildCount, _ := s.store.CountGuilds(r.Context())
	expCount, _ := s.store.Count(r.Context())
	voteCount, _ := s.store.CountVotes(r.Context())
	verifiedCount, _ := s.store.CountVerifiedUsers(r.Context())
	_ = pages.Dashboard(guildCount, expCount, voteCount, verifiedCount, s.isAdmin(r), s.discordName(r), s.cfg.DiscordClientID).Render(r.Context(), w)
}

func (s *Server) handleGuilds(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("discord_id"); err != nil {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	tok := ""
	if c, err := r.Cookie("discord_token"); err == nil {
		tok = c.Value
	}
	userGuilds := s.fetchUserGuilds(tok)
	// Build admin guild IDs map
	adminIDs := make(map[string]bool)
	var adminGuilds []model.DiscordGuild
	for _, g := range userGuilds {
		if g.Permissions&0x20 != 0 || g.Permissions&0x8 != 0 || g.Owner {
			adminIDs[g.ID] = true
			adminGuilds = append(adminGuilds, g)
		}
	}
	// Fetch configs for admin guilds only (batched)
	configs, _ := s.store.ListGuildConfigsByIDs(r.Context(), keys(adminIDs))
	cfgMap := make(map[string]model.GuildConfig, len(configs))
	for _, c := range configs {
		cfgMap[c.GuildID] = c
	}
	_ = pages.Guilds(adminGuilds, cfgMap, s.isAdmin(r), s.discordName(r)).Render(r.Context(), w)
}

func (s *Server) handleGuildDetail(w http.ResponseWriter, r *http.Request) {
	guildID := strings.TrimPrefix(r.URL.Path, "/dashboard/guild/")
	if idx := strings.Index(guildID, "/"); idx >= 0 {
		guildID = guildID[:idx]
	}
	if guildID == "" {
		http.NotFound(w, r)
		return
	}
	if _, err := r.Cookie("discord_id"); err != nil {
		http.Redirect(w, r, "/auth/discord/login", http.StatusFound)
		return
	}
	if !s.isGuildAdmin(r, guildID) {
		http.Error(w, "you are not admin of this server", http.StatusForbidden)
		return
	}
	cfg, _ := s.store.GetGuildConfig(r.Context(), guildID)
	bindings, _ := s.store.ListBindings(r.Context(), guildID)
	// Fetch guild name via Discord bot if available
	guildName := guildID
	if s.discord != nil {
		if g, err := s.discord.Guild(guildID); err == nil && g != nil {
			guildName = g.Name
		}
	}
	_ = pages.GuildDetail(guildID, guildName, cfg, bindings, s.isAdmin(r), s.discordName(r)).Render(r.Context(), w)
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
		if c, err := r.Cookie("discord_id"); err == nil && c.Value != "" {
			if _, err := s.store.GetVerifiedUser(r.Context(), c.Value); err == nil {
				verified = true
			}
		}
	}
	_ = pages.Verify(s.isAdmin(r), s.discordName(r), verified).Render(r.Context(), w)
}

func (s *Server) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	_ = pages.Privacy(s.isAdmin(r), s.discordName(r)).Render(r.Context(), w)
}

func (s *Server) handleTerms(w http.ResponseWriter, r *http.Request) {
	_ = pages.Terms(s.isAdmin(r), s.discordName(r)).Render(r.Context(), w)
}

func (s *Server) handleListGuilds(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "admin only", http.StatusUnauthorized)
		return
	}
	guilds, err := s.store.ListGuildConfigs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(guilds)
}

func (s *Server) handleBindings(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		http.Error(w, "admin only", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		guildID := r.URL.Query().Get("guild_id")
		if guildID == "" {
			http.Error(w, "guild_id required", http.StatusBadRequest)
			return
		}
		bindings, _ := s.store.ListBindings(r.Context(), guildID)
		_ = json.NewEncoder(w).Encode(bindings)
	case http.MethodPost:
		var b model.GuildBinding
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.store.CreateBinding(r.Context(), b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]int64{"id": id})
	case http.MethodDelete:
		guildID := r.URL.Query().Get("guild_id")
		idStr := r.URL.Query().Get("id")
		var id int64
		fmt.Sscan(idStr, &id)
		if err := s.store.DeleteBinding(r.Context(), id, guildID); err != nil {
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
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
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
	}
	_ = json.NewDecoder(resp2.Body).Decode(&user)
	http.SetCookie(w, &http.Cookie{Name: "discord_id", Value: user.ID, Path: "/", MaxAge: 86400 * 7, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_name", Value: user.Username, Path: "/", MaxAge: 86400 * 7, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_token", Value: tok.AccessToken, Path: "/", MaxAge: 86400 * 7, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/dashboard?discord=ok", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "discord_id", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_name", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "discord_token", Value: "", Path: "/", MaxAge: -1, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) handleRobloxLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RobloxClientID == "" {
		http.Error(w, "ROBLOX_CLIENT_ID not set. Create app at https://create.roblox.com/dashboard/credentials (redirect_uri = "+s.cfg.WebURL+"/auth/roblox/callback)", http.StatusNotImplemented)
		return
	}
	discordID := ""
	if c, err := r.Cookie("discord_id"); err == nil {
		discordID = c.Value
	}
	if discordID == "" {
		discordID = r.URL.Query().Get("discord_id")
	}
	if discordID != "" {
		// Establish the visitor's Discord identity via the bot-provided private link
		// (or a prior login) so the Roblox callback can link both accounts.
		http.SetCookie(w, &http.Cookie{Name: "discord_id", Value: discordID, Path: "/", MaxAge: 86400 * 7, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
	}
	verifier, _ := roblox.GenerateCodeVerifier()
	challenge := roblox.CodeChallenge(verifier)
	state, _ := roblox.GenerateState()
	redirectURI := strings.TrimRight(s.cfg.WebURL, "/") + "/auth/roblox/callback"
	s.mu.Lock()
	s.pending[state] = pendingOAuth{State: state, Verifier: verifier, DiscordID: discordID, Expires: time.Now().Add(5 * time.Minute)}
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
	s.mu.Lock()
	val, ok := s.pending[state]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
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

	verifier := val.Verifier
	if c, err := r.Cookie("code_verifier"); err == nil && verifier == "" {
		verifier = c.Value
	}
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
	if c, err := r.Cookie("discord_id"); err == nil && discordID == "" {
		discordID = c.Value
	}
	if discordID == "" {
		http.Error(w, "Discord not linked. Open this link from the Discord Verify button (it includes your Discord ID).", http.StatusBadRequest)
		return
	}
	var robloxID int64
	fmt.Sscan(info.Sub, &robloxID)
	_ = s.store.UpsertVerifiedUser(r.Context(), model.VerifiedUser{DiscordID: discordID, RobloxID: robloxID, RobloxUsername: info.Username})
	go s.syncVerifiedUser(discordID, robloxID)
	http.Redirect(w, r, "/verify?verified=1", http.StatusFound)
}

func (s *Server) syncVerifiedUser(discordID string, robloxID int64) {
	if s.verify != nil {
		s.verify.SyncAllGuilds(nil, discordID, robloxID)
	} else {
		s.log.Info("verified sync triggered", "discord", discordID, "roblox", robloxID)
	}
}
