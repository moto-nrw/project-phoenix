package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// The tenant-portal guard of the school cutover (#2207 PR 3) hangs entirely on
// this predicate, and getting it wrong locks a real person out of BOTH portals:
// the tenant login refuses them here, the school login refuses everything that
// is not the SYSTEM lehrkraft role.
func TestIsSchoolPortalOnlyForTenant(t *testing.T) {
	t.Parallel()

	systemLehrkraft := &authModels.Role{Name: "lehrkraft", IsSystem: true}
	tenantScoped := int64(42)
	customLehrkraft := &authModels.Role{Name: "Lehrkraft", IsSystem: false, TenantID: &tenantScoped}
	caregiver := &authModels.Role{Name: "user", IsSystem: true}
	admin := &authModels.Role{Name: "admin", IsSystem: true}

	tests := []struct {
		name  string
		roles []*authModels.Role
		want  bool
	}{
		{"system lehrkraft alone", []*authModels.Role{systemLehrkraft}, true},
		{"lehrkraft plus caregiver", []*authModels.Role{systemLehrkraft, caregiver}, false},
		{"lehrkraft plus admin", []*authModels.Role{admin, systemLehrkraft}, false},
		{"caregiver alone", []*authModels.Role{caregiver}, false},
		// A tenant-scoped custom role that merely SHARES the name carries
		// arbitrary permissions and keeps its tenant-portal access. Matching on
		// the name would refuse it here while the school login refuses it too.
		{"custom role named lehrkraft", []*authModels.Role{customLehrkraft}, false},
		{"custom lehrkraft plus system lehrkraft", []*authModels.Role{customLehrkraft, systemLehrkraft}, false},
		// No roles at all is a different problem and stays on the existing path.
		{"no roles", nil, false},
		{"nil entry", []*authModels.Role{nil}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsSchoolPortalOnlyForTenant(tc.roles))
		})
	}
}
