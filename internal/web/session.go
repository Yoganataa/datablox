package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const sessionCookieName = "dbx_session"

// sessionTTL matches the previous discord_* cookie lifetime (7 days).
const sessionTTL = 7 * 24 * time.Hour

// webSession is server-side authentication state. The browser only holds an
// opaque random ID; Discord user ID and OAuth token never come from the client.
type webSession struct {
	DiscordID    string
	DiscordToken string
	Expires      time.Time
}

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createSession stores a new server-side session and returns its opaque ID.
// Expired sessions are swept opportunistically.
func (s *Server) createSession(discordID, token string) (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	for k, v := range s.sessions {
		if !now.Before(v.Expires) {
			delete(s.sessions, k)
		}
	}
	s.sessions[id] = webSession{DiscordID: discordID, DiscordToken: token, Expires: now.Add(sessionTTL)}
	return id, nil
}

// getSession resolves the opaque session cookie to server-side state.
// Unknown, missing, or expired sessions are rejected.
func (s *Server) getSession(r *http.Request) (webSession, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return webSession{}, false
	}
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	sess, ok := s.sessions[c.Value]
	if !ok || !time.Now().Before(sess.Expires) {
		delete(s.sessions, c.Value)
		return webSession{}, false
	}
	return sess, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: id, Path: "/", MaxAge: int(sessionTTL.Seconds()), HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
}

func (s *Server) destroySession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		s.sessMu.Lock()
		delete(s.sessions, c.Value)
		s.sessMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cookieSecure(), SameSite: http.SameSiteLaxMode})
}

// requireCSRF guards state-changing endpoints against cross-site request
// forgery. Our own pages call these endpoints with fetch/XMLHttpRequest and
// send X-Requested-With; a cross-origin attacker cannot attach custom headers
// without a CORS preflight, which this server never grants. This complements
// SameSite=Lax (which already strips cookies on cross-site POST in modern
// browsers) for older clients and same-site-subdomain scenarios.
func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		http.Error(w, "missing X-Requested-With", http.StatusForbidden)
		return false
	}
	return true
}
