package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The tenant-portal guard of the school cutover (#2207 PR 3) hangs entirely on
// this predicate, and getting it wrong locks a real person out of BOTH portals:
// the tenant login refuses them here, the school login refuses everything that
// is not the SYSTEM lehrkraft role. Moved from services/auth's
// TestIsSchoolPortalOnlyForTenant with the staff preview guard (#3225).
func TestIsSchoolPortalOnly(t *testing.T) {
	t.Parallel()

	systemLehrkraft := RoleAssignment{RoleID: 1, Name: "lehrkraft", IsSystem: true}
	tenantScoped := int64(42)
	customLehrkraft := RoleAssignment{RoleID: 2, Name: "Lehrkraft", TenantID: &tenantScoped}
	caregiver := RoleAssignment{RoleID: 3, Name: "user", IsSystem: true}
	admin := RoleAssignment{RoleID: 4, Name: "admin", IsSystem: true}

	tests := []struct {
		name  string
		roles []RoleAssignment
		want  bool
	}{
		{"system lehrkraft alone", []RoleAssignment{systemLehrkraft}, true},
		{"lehrkraft plus caregiver", []RoleAssignment{systemLehrkraft, caregiver}, false},
		{"lehrkraft plus admin", []RoleAssignment{admin, systemLehrkraft}, false},
		{"caregiver alone", []RoleAssignment{caregiver}, false},
		// A tenant-scoped custom role that merely SHARES the name carries
		// arbitrary permissions and keeps its tenant-portal access. Matching on
		// the name would refuse it here while the school login refuses it too.
		{"custom role named lehrkraft", []RoleAssignment{customLehrkraft}, false},
		{"custom lehrkraft plus system lehrkraft", []RoleAssignment{customLehrkraft, systemLehrkraft}, false},
		// No roles at all is a different problem and stays on the existing path.
		{"no roles", nil, false},
		// The old nil role pointer: an assignment without role facts.
		{"assignment without role facts", []RoleAssignment{{}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsSchoolPortalOnly(tc.roles))
		})
	}
}
