package identityaccess_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
)

func TestWelcomeHelpURLOpensTheArticleOfTheRole(t *testing.T) {
	t.Parallel()
	query := []string{"role=lead", "nfc_enabled=true"}
	assert.Equal(t, "https://app.test/help/einladung-annehmen-und-konto-einrichten?role=lead&nfc_enabled=true",
		identityaccess.WelcomeHelpURL("https://app.test/", identityaccess.HelpRoleLead, query))
	assert.Equal(t, "https://app.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver",
		identityaccess.WelcomeHelpURL("https://app.test", identityaccess.HelpRoleCaregiver, []string{"role=caregiver"}))
	assert.Equal(t, "https://schule.test/help/zugang-zu-moto-schule-einrichten?role=teacher",
		identityaccess.WelcomeHelpURL("https://schule.test", identityaccess.HelpRoleTeacher, []string{"role=teacher"}))
	assert.Equal(t, "https://eltern.test/help/eltern-konto-einrichten?role=parent",
		identityaccess.WelcomeHelpURL("https://eltern.test", identityaccess.HelpRoleParent, []string{"role=parent"}))
	assert.Equal(t, "https://eltern.test/downloads/moto-elterninfo.pdf", identityaccess.ParentInfoURL(" https://eltern.test/ "))
}
