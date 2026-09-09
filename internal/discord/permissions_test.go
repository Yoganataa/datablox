package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestExtractPermissionsModerateMembers(t *testing.T) {
	p := ExtractPermissions(int64(discordgo.PermissionModerateMembers))
	if !p.ModerateMembers {
		t.Errorf("ModerateMembers bit should map to Permissions.ModerateMembers")
	}
	if p.BanMembers || p.ManageGuild {
		t.Errorf("single bit must not imply other permissions: %+v", p)
	}
	all := ExtractPermissions(int64(discordgo.PermissionAdministrator))
	if !(all.Administrator && all.ManageGuild && all.ManageRoles && all.ManageChannels &&
		all.ManageMessages && all.ModerateMembers && all.BanMembers && all.KickMembers) {
		t.Errorf("Administrator should imply all modeled permissions: %+v", all)
	}
	zero := ExtractPermissions(0)
	if zero.Administrator || zero.ManageGuild || zero.ManageRoles || zero.ManageChannels ||
		zero.ManageMessages || zero.ModerateMembers || zero.BanMembers || zero.KickMembers {
		t.Errorf("zero bits should map to no permissions: %+v", zero)
	}
}
