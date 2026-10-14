package compose

import (
	"testing"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/stretchr/testify/assert"
)

// presetTable stands in for the Security Runtime policy: "pickup_only" grants
// one permission, everything else normalizes to "primary_guardian" with two.
func presetTable() GuardianRolePresetFuncs {
	return GuardianRolePresetFuncs{
		Default: func(relationshipType string, isPrimary, _, canPickup bool) string {
			if canPickup && !isPrimary && relationshipType == "other" {
				return "pickup_only"
			}
			return "primary_guardian"
		},
		Presets: func(role string) (string, []string) {
			if role == "pickup_only" {
				return "pickup_only", []string{"parent_portal.access"}
			}
			return "primary_guardian", []string{"parent_portal.access", "parent_portal.notes.write"}
		},
	}
}

func TestApplyGuardianRolePresetReplacesRoleAndPermissions(t *testing.T) {
	t.Parallel()

	link := &users.StudentGuardian{}
	link.SetGuardianAuthorizationData("custom", map[string]interface{}{"parent_portal.sick_note.submit": true})

	ApplyGuardianRolePreset(presetTable(), link, "pickup_only")

	_, role, _, _, _, permissions := link.GuardianAuthorizationData()
	assert.Equal(t, "pickup_only", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true}, permissions)
}

func TestApplyDefaultGuardianRoleUsesTheRelationshipFacts(t *testing.T) {
	t.Parallel()

	pickupOnly := &users.StudentGuardian{RelationshipType: "other", CanPickup: true}
	ApplyDefaultGuardianRole(presetTable(), pickupOnly)
	_, role, _, _, _, permissions := pickupOnly.GuardianAuthorizationData()
	assert.Equal(t, "pickup_only", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true}, permissions)

	primary := &users.StudentGuardian{RelationshipType: "parent", IsPrimary: true}
	ApplyDefaultGuardianRole(presetTable(), primary)
	_, role, _, _, _, permissions = primary.GuardianAuthorizationData()
	assert.Equal(t, "primary_guardian", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true, "parent_portal.notes.write": true}, permissions)
}
