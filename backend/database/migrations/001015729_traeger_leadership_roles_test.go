package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestTraegerLeadershipRolesKeepEveryRight runs the migration of #5471 over
// the role set before it: every person and open invitation with Verwaltung
// gets Leitung and Schulzugang, the other roles stay as they were, and the
// down path restores the old role set.
func TestTraegerLeadershipRolesKeepEveryRight(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, traegerLeadershipRolesDown(ctx, db))

	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Leitung", Slug: fmt.Sprintf("traeger-leitung-%d", testpkg.UniqueTestTenantID(t)),
	})
	office := testpkg.CreateTestAccount(t, db, "traeger-verwaltung")
	billing := testpkg.CreateTestAccount(t, db, "traeger-abrechnung")
	testpkg.CreateTestTraegerMember(t, db, organization.ID, office.ID, "administration", "billing")
	testpkg.CreateTestTraegerMember(t, db, organization.ID, billing.ID, "billing")
	invite := func(email string, roles string) {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
			VALUES (?, ?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), ?::VARCHAR(32)[], NOW() + INTERVAL '1 day')`,
			organization.ID, email, roles)
		require.NoError(t, err)
	}
	invite("verwaltung@test.local", "{administration}")
	invite("personal@test.local", "{personnel}")

	_, err := db.ExecContext(ctx, `INSERT INTO auth.organization_account_roles (organization_id, account_id, role) VALUES (?, ?, 'leadership')`,
		organization.ID, billing.ID)
	require.ErrorContains(t, err, "chk_organization_account_roles_role", "before the migration Leitung does not exist")

	roles := func(accountID int64) []string {
		var held []string
		require.NoError(t, db.NewRaw(`SELECT role FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ? ORDER BY role`,
			organization.ID, accountID).Scan(ctx, &held))
		return held
	}
	invited := func(email string) []string {
		var held []string
		require.NoError(t, db.NewRaw(`SELECT unnest(roles) FROM auth.traeger_invitations WHERE organization_id = ? AND email = ?`,
			organization.ID, email).Scan(ctx, &held))
		return held
	}

	require.NoError(t, traegerLeadershipRolesUp(ctx, db))
	assert.Equal(t, []string{"administration", "billing", "leadership", "school_access"}, roles(office.ID),
		"Verwaltung keeps its rights through Leitung and Schulzugang")
	assert.Equal(t, []string{"billing"}, roles(billing.ID), "the other roles stay")
	assert.Equal(t, []string{"administration", "leadership", "school_access"}, invited("verwaltung@test.local"),
		"an open invitation with Verwaltung gets the same addition")
	assert.Equal(t, []string{"personnel"}, invited("personal@test.local"))

	// The Träger-Büro gives Leitung alone to a new person; down keeps the
	// rights through Verwaltung.
	_, err = db.ExecContext(ctx, `INSERT INTO auth.organization_account_roles (organization_id, account_id, role) VALUES (?, ?, 'leadership')`,
		organization.ID, billing.ID)
	require.NoError(t, err, "after the migration Leitung exists")
	invite("leitung@test.local", "{leadership}")

	require.NoError(t, traegerLeadershipRolesDown(ctx, db))
	assert.Equal(t, []string{"administration", "billing"}, roles(office.ID), "down restores the role set before")
	assert.Equal(t, []string{"administration", "billing"}, roles(billing.ID), "a later Leitung falls back to Verwaltung")
	assert.Equal(t, []string{"administration"}, invited("verwaltung@test.local"))
	assert.Equal(t, []string{"administration"}, invited("leitung@test.local"))
	assert.Equal(t, []string{"personnel"}, invited("personal@test.local"))
	_, err = db.ExecContext(ctx, `INSERT INTO auth.organization_account_roles (organization_id, account_id, role) VALUES (?, ?, 'school_access')`,
		organization.ID, billing.ID)
	require.ErrorContains(t, err, "chk_organization_account_roles_role", "down restores the role check")

	require.NoError(t, traegerLeadershipRolesUp(ctx, db))
}
