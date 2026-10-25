package defaults

import (
	"sort"
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
)

// TestTraegerLevelKeysOfVersionOne pins the first Träger classification (ADR
// 0086 section 4, ADR 0107 point 7): the legal texts of the enrollment and
// the parent portal functions without a lock are "beide", everything else
// stays at the school. A change of this list is a product decision.
func TestTraegerLevelKeysOfVersionOne(t *testing.T) {
	t.Parallel()
	want := []string{
		config.KeyEnrollmentLegalAGBText, config.KeyEnrollmentLegalDSGVOEnabled, config.KeyEnrollmentLegalDSGVOText,
		config.KeyEnrollmentLegalEmailContactEnabled, config.KeyEnrollmentLegalEmailContactText,
		config.KeyEnrollmentLegalPhotoEnabled, config.KeyEnrollmentLegalPhotoText, config.KeyEnrollmentLegalTermsEnabled,
		config.KeyGuardianParentCanRemove, config.KeyGuardianParentInviteMode, config.KeyParentCareModeRequestEnabled,
		config.KeyParentCarePickupRequestEnabled, config.KeyParentGuardianManagementEnabled,
		config.KeyParentMasterDataEditEnabled, config.KeyParentMasterDataRequestEnabled,
		config.KeyParentMessageStaffNameVisible, config.KeyParentNewsEnabled,
	}
	sort.Strings(want)
	var got []string
	for key, definition := range config.DefaultRegistry().AllDefinitions() {
		switch definition.Levels {
		case config.LevelBoth:
			got = append(got, key)
			assert.Equal(t, config.TraegerRightSettings, definition.TraegerWriteRight, "%s is written with the right Einstellungen", key)
		case config.LevelSchool:
		default:
			t.Errorf("%s has level %q; v1 has no Träger-only setting", key, definition.Levels)
		}
	}
	sort.Strings(got)
	assert.Equal(t, want, got)
}
