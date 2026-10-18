package identityaccess_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The school portal invites teachers; elsewhere a school lead gets the
// lead article and everyone else the caregiver one.
func TestStaffHelpRoleFollowsTheSidebar(t *testing.T) {
	t.Parallel()
	assert.Equal(t, identityaccess.HelpRoleTeacher, identityaccess.StaffHelpRole(true, true))
	assert.Equal(t, identityaccess.HelpRoleTeacher, identityaccess.StaffHelpRole(true, false))
	assert.Equal(t, identityaccess.HelpRoleLead, identityaccess.StaffHelpRole(false, true))
	assert.Equal(t, identityaccess.HelpRoleCaregiver, identityaccess.StaffHelpRole(false, false))
}

// Resolved settings join the link in buildHelpHref's order; unresolved ones
// stay out.
func TestWelcomeHelpLinkLeavesUnresolvedSettingsOut(t *testing.T) {
	t.Parallel()
	nfc, presence, group := true, "binary", "open_care"
	assert.Equal(t, "https://app.test/help/einladung-annehmen-und-konto-einrichten?role=lead&nfc_enabled=true&presence_mode=binary&group_mode=open_care",
		identityaccess.WelcomeHelpLink("https://app.test", identityaccess.HelpRoleLead, identityaccess.WelcomeHelpSettings{
			NFCEnabled: &nfc, PresenceMode: &presence, GroupMode: &group,
		}))
	assert.Equal(t, "https://eltern.test/help/eltern-konto-einrichten?role=parent&presence_mode=binary",
		identityaccess.WelcomeHelpLink("https://eltern.test", identityaccess.HelpRoleParent, identityaccess.WelcomeHelpSettings{PresenceMode: &presence}))
	assert.Equal(t, "https://app.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver",
		identityaccess.WelcomeHelpLink("https://app.test", identityaccess.HelpRoleCaregiver, identityaccess.WelcomeHelpSettings{}))
}
