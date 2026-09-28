package securityruntime

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

type guardianRoleRecord struct {
	relationshipType              string
	primary, emergency, canPickup bool
	role                          string
	permissions                   map[string]interface{}
}

func (r *guardianRoleRecord) GuardianAuthorizationData() (string, string, bool, bool, bool, map[string]interface{}) {
	return r.relationshipType, r.role, r.primary, r.emergency, r.canPickup, r.permissions
}

func (r *guardianRoleRecord) SetGuardianAuthorizationData(role string, permissions map[string]interface{}) {
	r.role, r.permissions = role, permissions
}

// TestDefaultGuardianRolePresetMatchesThePolicy pins the composition that
// gives a new relationship its default role (#2727): the default preset and
// the permissions it grants equal what the authorization policy applies.
func TestDefaultGuardianRolePresetMatchesThePolicy(t *testing.T) {
	t.Parallel()

	for _, relationshipType := range []string{"parent", "guardian", "grandparent", ""} {
		for _, flags := range [][3]bool{{}, {true, false, false}, {false, true, false}, {false, false, true}} {
			want := &guardianRoleRecord{relationshipType: relationshipType, primary: flags[0], emergency: flags[1], canPickup: flags[2]}
			authorize.ApplyDefaultStudentGuardianRole(want)

			role, granted := StudentGuardianRolePreset(DefaultStudentGuardianRole(relationshipType, flags[0], flags[1], flags[2]))
			if role != want.role {
				t.Errorf("%q %v: role %q, policy applies %q", relationshipType, flags, role, want.role)
			}
			if len(granted) != len(want.permissions) {
				t.Errorf("%q %v: %d permissions, policy applies %d", relationshipType, flags, len(granted), len(want.permissions))
			}
			for _, permission := range granted {
				if want.permissions[permission] != true {
					t.Errorf("%q %v: preset grants %q, policy does not", relationshipType, flags, permission)
				}
			}
		}
	}
}

func TestGuardianPermissionGranted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		value any
		want  bool
	}{
		{"true", true, true},
		{"false", false, false},
		{"absent", nil, false},
		{"historical non-boolean value", "yes", true},
	} {
		if got := GuardianPermissionGranted(tc.value); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
