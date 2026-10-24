package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelatedAccountsManagePermissionGrantsFullGuardiansOnly pins the backfill
// of #4603: full guardians keep inviting and removing accounts after the
// deploy, restricted roles gain nothing, and an explicit refusal survives.
func TestRelatedAccountsManagePermissionGrantsFullGuardiansOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Konten", "Kind", "1a")
	link := func(name, role string) int64 {
		profile := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, name, "Konten", name)
		return testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, profile.ID, role).ID
	}
	cases := []struct {
		name        string
		role        string
		permissions string
		want        any
	}{
		{"primary", "primary_guardian", `{"parent_portal.access": true}`, true},
		{"legal", "legal_guardian", `{"parent_portal.access": true}`, true},
		{"co", "co_guardian", `{"parent_portal.access": true}`, true},
		{"refused", "legal_guardian", `{"parent_portal.access": true, "parent_portal.related_accounts.manage": false}`, false},
		{"emergency", "emergency_contact", `{"parent_portal.access": true}`, nil},
		{"pickup", "pickup_only", `{}`, nil},
	}
	ids := make([]int64, len(cases))
	for i, c := range cases {
		ids[i] = link(c.name, c.role)
		// The fixture applies today's presets; put each row back into the
		// shape it had before this migration.
		_, err := db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = ?::jsonb
			WHERE tenant_id = ? AND relationship_id = ?`, c.permissions, tenantID, ids[i])
		require.NoError(t, err)
	}

	require.NoError(t, relatedAccountsManagePermissionUp(ctx, db))

	for i, c := range cases {
		var value *bool
		require.NoError(t, db.NewRaw(`SELECT (permissions -> 'parent_portal.related_accounts.manage')::boolean
			FROM auth.guardian_student_access WHERE tenant_id = ? AND relationship_id = ?`, tenantID, ids[i]).
			Scan(ctx, &value), c.name)
		if c.want == nil {
			assert.Nil(t, value, c.name)
			continue
		}
		require.NotNil(t, value, c.name)
		assert.Equal(t, c.want, *value, c.name)
	}
}
