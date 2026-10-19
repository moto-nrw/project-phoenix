package application

import (
	"encoding/json"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/stretchr/testify/assert"

	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// presetTable stands in for the Security Runtime policy: "pickup_only" grants
// one permission, everything else normalizes to "primary_guardian" with two.
func presetTable() ports.GuardianRolePresetFuncs {
	return ports.GuardianRolePresetFuncs{
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

	link := &peopleValues.GuardianRelationshipSnapshot{}
	link.GuardianRole = "custom"
	link.Permissions = []byte(`{"parent_portal.sick_note.submit":true}`)

	ApplyGuardianRolePreset(presetTable(), link, "pickup_only")

	_, role, _, _, _, permissions := guardianAuthorizationData(link)
	assert.Equal(t, "pickup_only", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true}, permissions)
}

func TestApplyDefaultGuardianRoleUsesTheRelationshipFacts(t *testing.T) {
	t.Parallel()

	pickupOnly := &peopleValues.GuardianRelationshipSnapshot{RelationshipType: "other", CanPickup: true}
	ApplyDefaultGuardianRole(presetTable(), pickupOnly)
	_, role, _, _, _, permissions := guardianAuthorizationData(pickupOnly)
	assert.Equal(t, "pickup_only", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true}, permissions)

	primary := &peopleValues.GuardianRelationshipSnapshot{RelationshipType: "parent", IsPrimary: true}
	ApplyDefaultGuardianRole(presetTable(), primary)
	_, role, _, _, _, permissions = guardianAuthorizationData(primary)
	assert.Equal(t, "primary_guardian", role)
	assert.Equal(t, map[string]interface{}{"parent_portal.access": true, "parent_portal.notes.write": true}, permissions)
}

func guardianAuthorizationData(link *peopleValues.GuardianRelationshipSnapshot) (string, string, bool, bool, bool, map[string]any) {
	var permissions map[string]any
	if json.Unmarshal(link.Permissions, &permissions) != nil {
		permissions = nil
	}
	return link.RelationshipType, link.GuardianRole, link.IsPrimary, link.IsEmergencyContact, link.CanPickup, permissions
}
