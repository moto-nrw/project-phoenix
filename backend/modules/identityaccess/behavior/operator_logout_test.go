package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_OperatorLogout_RevokesOnlyTheFamily pins #5194 for the
// operator portal: logout revokes the presented family, so its access tokens
// fail the per-request family check (RequiresActiveOperator), while another
// device's family stays valid.
func TestIntegration_OperatorLogout_RevokesOnlyTheFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-logout"), testPassword)

	access, refresh, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	otherAccess, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	family, otherFamily := accessSessionOf(t, access).FamilyID, accessSessionOf(t, otherAccess).FamilyID
	require.NotEqual(t, family, otherFamily)

	require.ErrorIs(t, identity.LogoutOperator(ctx, operator.ID+1_000_000, refreshHandle(t, refresh)), identityaccess.ErrOperatorRefreshTokenInvalid,
		"another operator's handle revokes nothing")
	_, err = identity.LatestOperatorSessionInFamily(ctx, family)
	require.NoError(t, err)

	require.NoError(t, identity.LogoutOperator(ctx, operator.ID, refreshHandle(t, refresh)))

	_, err = identity.LatestOperatorSessionInFamily(ctx, family)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound, "the logged-out family is gone")
	_, err = identity.LatestOperatorSessionInFamily(ctx, otherFamily)
	require.NoError(t, err, "the other device keeps its family")
	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, refreshHandle(t, refresh))
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid)
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "token_revoked"))

	require.NoError(t, identity.LogoutOperator(ctx, operator.ID, refreshHandle(t, refresh)), "a second logout is a successful no-op")
}
