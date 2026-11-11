package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolEmailIdentitiesAreBatchedAndTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "batch-email-account")
	role := testpkg.CreateTestRole(t, db, "batch-email-role")
	person := testpkg.CreateTestPerson(t, db, "Batch", "Invitee")
	invitation := testpkg.CreateTestInvitationToken(t, db, "batch-email-invitation", role.ID, 0, time.Now().Add(-time.Hour))
	_, err := db.NewRaw(`UPDATE auth.invitation_tokens SET person_id = ? WHERE id = ? AND tenant_id = ?`, person.ID, invitation.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, TenantRuntime: testpkg.TenantRuntime(t, db), Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	emails := []string{" " + strings.ToUpper(account.Email) + " ", strings.ToUpper(invitation.Email), "missing@example.test"}
	accounts, invited, err := access.(identityaccess.SchoolEmailQuery).FindSchoolEmailIdentities(counter.Context(testpkg.Ctx(t)), emails)
	require.NoError(t, err)
	require.Equal(t, map[string]int64{account.Email: account.ID}, accounts)
	require.Equal(t, map[string][]int64{invitation.Email: {person.ID}}, invited, "expired unused invitations remain identity links")
	require.Len(t, counter.Matching(func(sql string) bool { return strings.Contains(sql, "union all") }), 1)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	accounts, invited, err = access.(identityaccess.SchoolEmailQuery).FindSchoolEmailIdentities(testpkg.ContextForTenant(testpkg.Ctx(t), otherTenant), emails)
	require.NoError(t, err)
	require.Empty(t, accounts)
	require.Empty(t, invited)
	_, _, err = access.(identityaccess.SchoolEmailQuery).FindSchoolEmailIdentities(context.Background(), emails)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)

	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET used_at = NOW() WHERE id = ? AND tenant_id = ?`, invitation.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	accounts, invited, err = access.(identityaccess.SchoolEmailQuery).FindSchoolEmailIdentities(testpkg.Ctx(t), emails)
	require.NoError(t, err)
	require.Empty(t, accounts)
	require.Empty(t, invited)
	counter.Reset()
	accounts, invited, err = access.(identityaccess.SchoolEmailQuery).FindSchoolEmailIdentities(counter.Context(testpkg.Ctx(t)), nil)
	require.NoError(t, err)
	require.Empty(t, accounts)
	require.Empty(t, invited)
	require.Empty(t, counter.Matching(func(sql string) bool { return strings.Contains(sql, "auth.accounts") }))
}
