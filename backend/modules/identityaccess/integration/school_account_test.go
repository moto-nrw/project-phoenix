package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolAccountLookupRequiresActiveMappingInCurrentTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "school-account-lookup")
	var observations []identityCompose.Observation
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	got, err := access.FindSchoolAccountByEmail(testpkg.Ctx(t), " "+strings.ToUpper(account.Email)+" ")
	require.NoError(t, err)
	require.Equal(t, account.ID, got.ID)
	require.EqualValues(t, 2, observations[0].Stats.Queries, "resolve the account before checking its tenant mapping")

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, err = access.FindSchoolAccountByEmail(testpkg.ContextForTenant(testpkg.Ctx(t), otherTenant), account.Email)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)

	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	_, err = access.FindSchoolAccountByEmail(testpkg.Ctx(t), account.Email)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	_, err = access.FindSchoolAccountByEmail(testpkg.Ctx(t), "not-found@example.test")
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	_, err = access.FindSchoolAccountByEmail(context.Background(), account.Email)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	require.Len(t, observations, 5)
	require.ErrorIs(t, observations[4].Err, identityaccess.ErrTenantRequired)
	require.Zero(t, observations[4].Stats.Queries)
}
