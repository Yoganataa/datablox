package adapters

import "datablox/internal/auth"

// BotRoleResolver resolves BotRole from config OWNER_IDS/ADMIN_IDS.
// OWNER_IDS wins if user in both.
type BotRoleResolver struct {
	owners map[string]struct{}
	admins map[string]struct{}
}

func NewBotRoleResolver(owners, admins map[string]struct{}) *BotRoleResolver {
	return &BotRoleResolver{owners: owners, admins: admins}
}

func NewBotRoleResolverFromBoolMap(owners, admins map[string]bool) *BotRoleResolver {
	o := make(map[string]struct{}, len(owners))
	for k := range owners {
		o[k] = struct{}{}
	}
	a := make(map[string]struct{}, len(admins))
	for k := range admins {
		a[k] = struct{}{}
	}
	return NewBotRoleResolver(o, a)
}

func (r *BotRoleResolver) ResolveBotRole(userID string) auth.BotRole {
	if _, ok := r.owners[userID]; ok {
		return auth.BotOwner
	}
	if _, ok := r.admins[userID]; ok {
		return auth.BotAdmin
	}
	return auth.BotNone
}
