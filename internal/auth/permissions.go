package auth

// Permissions is the canonical permission model known to application layer.
// It is produced by Discord adapter from raw bits, never by handlers directly.
// Single source of truth — internal/auth is the only package that defines it.
type Permissions struct {
	Administrator   bool
	ManageGuild     bool
	ManageRoles     bool
	ManageChannels  bool
	ManageMessages  bool
	ModerateMembers bool
	BanMembers      bool
	KickMembers     bool
}
