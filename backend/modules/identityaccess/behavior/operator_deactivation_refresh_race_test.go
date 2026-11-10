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

// TestOperatorDeactivationCatchesAConcurrentRefresh pins AU-8 (#6289). A
// refresh held its session row and only then read the operator, while the
// deactivation locks the operator and then deletes the sessions: the two
// took the same rows in opposite order and deadlocked (40P01), failing one
// of them. Refresh now locks the operator row first, so the deactivation
// waits for the rotation, then deletes its successor with the rest.
func TestOperatorDeactivationCatchesAConcurrentRefresh(t *testing.T) {
	t.Parallel()
	db := lockRacePool(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	actor := testpkg.CreateTestOperator(t, db)
	target := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-deactivation-race"), testPassword)
	_, refresh, err := identity.IssueTokensForAuthenticatedOperator(ctx, target.ID, "127.0.0.1", "ua")
	require.NoError(t, err)

	// The holder stands in for the moment the refresh holds its session row:
	// the refresh queues on it, the deactivation behind the refresh.
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID))
	_, err = holder.ExecContext(ctx, `SELECT id FROM platform.operator_refresh_tokens WHERE operator_id = ? FOR UPDATE`, target.ID)
	require.NoError(t, err)

	refreshed := make(chan error, 1)
	go func() {
		_, _, refreshErr := identity.RefreshOperatorToken(ctx, refresh, "")
		refreshed <- refreshErr
	}()
	refreshPID := waitForBlockedBackend(t, db, []int{holderPID}, "the refresh must wait for its session row")

	deactivated := make(chan error, 1)
	go func() {
		_, statusErr := identity.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{OperatorID: target.ID, ActorID: actor.ID})
		deactivated <- statusErr
	}()
	waitForBlockedBackend(t, db, []int{holderPID, refreshPID}, "the deactivation must queue behind the refresh")
	require.NoError(t, holder.Commit())

	for name, done := range map[string]chan error{"refresh": refreshed, "deactivation": deactivated} {
		select {
		case err := <-done:
			if name == "deactivation" {
				require.NoError(t, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not finish after the row was released", name)
		}
	}

	assert.Zero(t, countRows(t, db, `SELECT count(*) FROM platform.operator_refresh_tokens WHERE operator_id = ?`, target.ID),
		"no session of the deactivated operator survives, not even one a concurrent refresh committed")
}
