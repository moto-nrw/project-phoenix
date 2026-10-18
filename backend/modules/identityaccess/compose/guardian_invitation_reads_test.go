package compose

import (
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The guardian invitation reads need the database alone and answer for the
// tenant in context like the module's reads; without the database the
// constructor refuses.
func TestNewGuardianInvitationReads(t *testing.T) {
	t.Parallel()

	_, err := NewGuardianInvitationReads(nil)
	require.ErrorContains(t, err, "need the database")

	db := testpkg.SetupTestDB(t)
	reads, err := NewGuardianInvitationReads(db)
	require.NoError(t, err)
	profile := testpkg.CreateTestGuardianProfile(t, db, "invitation-reads@test.local")
	invitations, err := reads.ListGuardianInvitations(testpkg.Ctx(t), profile.ID)
	require.NoError(t, err)
	require.Empty(t, invitations)
	open, err := reads.ListOpenGuardianInvitations(testpkg.Ctx(t), []int64{profile.ID})
	require.NoError(t, err)
	require.Empty(t, open)
}
