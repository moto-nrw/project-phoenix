package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestTraegerDocumentsRoleKeepsEveryRight runs the migration of #5992 over
// the role set before it: every person and open invitation with Verwaltung
// gets Dokumente at the same Träger and nowhere else, the other roles stay,
// and the down path restores the old role set without widening a right.
// The migration runs as the table owner without RLS, so the Träger match of
// the grant stands on its own.
func TestTraegerDocumentsRoleKeepsEveryRight(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, traegerDocumentsRoleDown(ctx, db))

	suffix := testpkg.UniqueTestTenantID(t)
	north := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Dokumente Nord", Slug: fmt.Sprintf("traeger-dokumente-nord-%d", suffix),
	})
	south := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Dokumente Süd", Slug: fmt.Sprintf("traeger-dokumente-sued-%d", suffix),
	})
	office := testpkg.CreateTestAccount(t, db, "traeger-dokumente-verwaltung")
	billing := testpkg.CreateTestAccount(t, db, "traeger-dokumente-abrechnung")
	testpkg.CreateTestTraegerMember(t, db, north.ID, office.ID, "administration", "billing")
	testpkg.CreateTestTraegerMember(t, db, south.ID, office.ID, "billing")
	testpkg.CreateTestTraegerMember(t, db, north.ID, billing.ID, "billing")
	invite := func(email string, roles string) {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
			VALUES (?, ?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), ?::VARCHAR(32)[], NOW() + INTERVAL '1 day')`,
			north.ID, email, roles)
		require.NoError(t, err)
	}
	invite("verwaltung@test.local", "{administration,leadership}")
	invite("personal@test.local", "{personnel}")

	grant := func(accountID int64) error {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.organization_account_roles (organization_id, account_id, role) VALUES (?, ?, 'documents')`,
			north.ID, accountID)
		return err
	}
	require.ErrorContains(t, grant(billing.ID), "chk_organization_account_roles_role", "before the migration Dokumente does not exist")

	roles := func(organizationID, accountID int64) []string {
		var held []string
		require.NoError(t, db.NewRaw(`SELECT role FROM auth.organization_account_roles WHERE organization_id = ? AND account_id = ? ORDER BY role`,
			organizationID, accountID).Scan(ctx, &held))
		return held
	}
	invited := func(email string) []string {
		var held []string
		require.NoError(t, db.NewRaw(`SELECT unnest(roles) FROM auth.traeger_invitations WHERE organization_id = ? AND email = ?`,
			north.ID, email).Scan(ctx, &held))
		return held
	}

	require.NoError(t, traegerDocumentsRoleUp(ctx, db))
	assert.Equal(t, []string{"administration", "billing", "documents"}, roles(north.ID, office.ID),
		"Verwaltung keeps the documents through Dokumente")
	assert.Equal(t, []string{"billing"}, roles(south.ID, office.ID),
		"the grant stays at the Träger of the Verwaltung")
	assert.Equal(t, []string{"billing"}, roles(north.ID, billing.ID), "the other roles stay")
	assert.Equal(t, []string{"administration", "documents", "leadership"}, invited("verwaltung@test.local"),
		"an open invitation with Verwaltung gets the same addition")
	assert.Equal(t, []string{"personnel"}, invited("personal@test.local"))
	require.NoError(t, traegerDocumentsRoleUp(ctx, db), "a second run adds nothing twice")
	assert.Equal(t, []string{"administration", "billing", "documents"}, roles(north.ID, office.ID))

	// The Träger-Büro gives Dokumente alone to a new person; down takes it
	// away instead of handing out Verwaltung.
	require.NoError(t, grant(billing.ID), "after the migration Dokumente exists")
	invite("dokumente@test.local", "{documents}")

	require.NoError(t, traegerDocumentsRoleDown(ctx, db))
	assert.Equal(t, []string{"administration", "billing"}, roles(north.ID, office.ID), "down restores the role set before")
	assert.Equal(t, []string{"billing"}, roles(north.ID, billing.ID), "a later Dokumente never becomes Verwaltung")
	assert.Equal(t, []string{"administration", "leadership"}, invited("verwaltung@test.local"))
	assert.Empty(t, invited("dokumente@test.local"), "an invitation that would grant nothing is withdrawn")
	assert.Equal(t, []string{"personnel"}, invited("personal@test.local"))
	require.ErrorContains(t, grant(billing.ID), "chk_organization_account_roles_role", "down restores the role check")

	require.NoError(t, traegerDocumentsRoleUp(ctx, db))
}
