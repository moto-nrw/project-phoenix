package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The autofill of the enrollment form reads the guardian profile of the
// calling account in the school, with the linked children and the
// relationship's submit permission, through the reader the root binds
// (#2734).
func TestGuardianAutofillReadsTheGuardiansProfileAndChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	profile, err := testutil.NewEnrollmentGuardianAutofill(db).
		LoadForTenant(testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t)), chain.AccountID, chain.TenantID)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "Sabine", profile.FirstName)
	assert.Equal(t, "Schneider", profile.LastName)
	require.NotNil(t, profile.Email)
	assert.Equal(t, chain.Email, *profile.Email)
	require.Len(t, profile.Children, 1)
	child := profile.Children[0]
	assert.Equal(t, chain.StudentID, child.StudentID)
	assert.Equal(t, "Felix", child.FirstName)
	assert.Equal(t, "Schneider", child.LastName)
	assert.Equal(t, "1a", child.SchoolClass)
	assert.True(t, child.EnrollmentSubmit, "a primary guardian may re-enroll the child")
	assert.NotEmpty(t, child.Status)
}

// An account without a profile in the school reads as no profile, so the
// route falls back to the session's claims.
func TestGuardianAutofillWithoutProfileIsNil(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "autofill-no-profile")

	profile, err := testutil.NewEnrollmentGuardianAutofill(db).
		LoadForTenant(testpkg.WithTestTenantRuntime(t, testpkg.Ctx(t)), account.ID, testpkg.Tenant(t))

	require.NoError(t, err)
	assert.Nil(t, profile)
}
