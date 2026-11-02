package behavior_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_OperatorLogout_RevokesOnlyTheFamily pins #5194 for the
// operator portal: logout revokes the family of the access token, so its access tokens
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

	require.ErrorIs(t, identity.LogoutOperator(ctx, operator.ID, ""), identityaccess.ErrOperatorRefreshTokenInvalid,
		"a token without family revokes nothing")

	require.NoError(t, identity.LogoutOperator(ctx, operator.ID, family))

	_, err = identity.LatestOperatorSessionInFamily(ctx, family)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound, "the logged-out family is gone")
	_, err = identity.LatestOperatorSessionInFamily(ctx, otherFamily)
	require.NoError(t, err, "the other device keeps its family")
	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, refreshHandle(t, refresh))
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid)
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "token_revoked"))

	require.NoError(t, identity.LogoutOperator(ctx, operator.ID, family), "a second logout is a successful no-op")
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "token_revoked"), "a no-op logout records nothing")
}

// TestIntegration_OperatorLogout_CatchesAConcurrentRotation pins the
// refresh/logout race for operators (#5194). The open transaction stands in
// for a rotation: it holds the newest row of the family and writes its
// successor. The first logout pass waits for that row and misses the
// successor; a later pass must revoke it, so no fresh session survives.
func TestIntegration_OperatorLogout_CatchesAConcurrentRotation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-logout-race"), testPassword)
	access, refresh, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	family := accessSessionOf(t, access).FamilyID

	rotation, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = rotation.Rollback() }()
	var rotationPID int
	require.NoError(t, rotation.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&rotationPID))
	_, err = rotation.ExecContext(ctx, `SELECT id FROM platform.operator_refresh_tokens WHERE token = ? FOR UPDATE`, sha256Hex(refreshHandle(t, refresh)))
	require.NoError(t, err)
	_, err = rotation.ExecContext(ctx, `
		INSERT INTO platform.operator_refresh_tokens (operator_id, token, expiry, family_id, generation)
		VALUES (?, ?, NOW() + INTERVAL '1 hour', ?, 1)`, operator.ID, sha256Hex("successor-"+family), family)
	require.NoError(t, err)

	logout := make(chan error, 1)
	go func() { logout <- identity.LogoutOperator(context.WithoutCancel(ctx), operator.ID, family) }()
	require.Eventually(t, func() bool {
		var blocked int
		err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, rotationPID).Scan(&blocked)
		return err == nil && blocked > 0
	}, 5*time.Second, 10*time.Millisecond, "the first pass must wait for the rotation's row")
	require.NoError(t, rotation.Commit())

	select {
	case err := <-logout:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("logout did not finish after the rotation committed")
	}
	_, err = identity.LatestOperatorSessionInFamily(ctx, family)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound, "the successor committed during logout is revoked too")
}
