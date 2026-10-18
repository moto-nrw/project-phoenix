package parentaudience

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// noApplicants is a school without undecided enrollment applications.
type noApplicants struct{}

func (noApplicants) PendingAnnouncementApplicants(context.Context) ([]contract.PendingAnnouncementApplicant, error) {
	return nil, nil
}

func (noApplicants) PendingAnnouncementApplicantsForSchools(context.Context, []int64) ([]contract.PendingAnnouncementApplicant, error) {
	return nil, nil
}

func TestDeclarationSignersAnswerOnlyTheNamedSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	projection := New(func(context.Context) (bun.IDB, int64, error) { return db, 0, nil }, noApplicants{}, testpkg.GuardianLinks{})

	signers, err := projection.DeclarationSigners(ctx, chain.TenantID, []int64{chain.StudentID})
	require.NoError(t, err)
	require.Len(t, signers, 1)
	assert.Equal(t, chain.StudentID, signers[0].StudentID)
	assert.Equal(t, chain.GuardianProfileID, signers[0].GuardianProfileID)
	assert.Equal(t, chain.AccountID, signers[0].AccountID)
	assert.Equal(t, "Sabine", signers[0].FirstName)
	assert.Equal(t, "Schneider", signers[0].LastName)

	foreign, err := projection.DeclarationSigners(ctx, otherSchool, []int64{chain.StudentID})
	require.NoError(t, err)
	assert.Empty(t, foreign, "another school must not reach the child's guardians")
}
