package behavior_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionCapEvictsTheWholeFamily pins AU-7 (#6289): the cap picks the
// session to evict by its head, but a rotated family still holds its
// predecessor within the rotation grace. Deleting only the head left that
// predecessor as the family's latest generation, so the family's access
// tokens stayed valid, the predecessor's refresh token renewed it and a
// school switch minted a new family out of it. Eviction ends the family.
func TestSessionCapEvictsTheWholeFamily(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "session-cap-family")
	ctx := testpkg.Ctx(t)
	evicted := accessSessionOf(t, f.access)

	// The rotation leaves the first generation behind as the predecessor.
	_, _, err := f.service.RefreshToken(ctx, f.refresh)
	require.NoError(t, err)
	require.EqualValues(t, 2, familyRowCount(t, f.db, evicted.FamilyID), "head and predecessor")

	// Four further logins fill the cap; the fifth evicts the family whose
	// expiry is closest, the one rotated before the others signed in.
	others := fillSessionCap(t, f, 0)
	others = append(others, newOtherDeviceSession(t, f))
	require.Len(t, liveTokens(t, f.db, f.accountID), 5)

	assert.Zero(t, familyRowCount(t, f.db, evicted.FamilyID), "no row of the evicted family survives")
	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, evicted), identityaccess.ErrInvalidToken,
		"the evicted family's access token is refused")
	_, _, err = f.switchToTarget(ctx, evicted.FamilyID)
	require.Error(t, err, "a school switch mints nothing out of the evicted family")
	_, _, err = f.service.RefreshToken(ctx, f.refresh)
	require.Error(t, err, "the predecessor's refresh token renews nothing")
	assert.Len(t, liveTokens(t, f.db, f.accountID), 5, "the refused attempts minted no session")
	for _, other := range others {
		require.NoError(t, f.module.VerifyAccessSession(ctx, other), "the sessions within the cap stay")
	}
}
