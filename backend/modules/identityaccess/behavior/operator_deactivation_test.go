package behavior_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Accounts of team members who left are deactivated in the operator
// dashboard and deleted once their ledger entries are gone (#4047).

func insertOperatorSession(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO platform.operator_refresh_tokens
		(operator_id, token, expiry, family_id) VALUES (?, ?, now() + interval '1 day', ?)`,
		operatorID, uniqueOperatorEmail("session-token"), uniqueOperatorEmail("session-family"))
	require.NoError(t, err)
}

func countRows(t *testing.T, db *bun.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(query, args...).Scan(context.Background(), &count))
	return count
}

func TestOperatorDeactivationLocksTheAccountAndCanBeUndone(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	operators := buildServiceFactory(t, db)
	ctx := context.Background()
	actor := testpkg.CreateTestOperator(t, db)
	target := testpkg.CreateTestOperator(t, db)
	insertOperatorSession(t, db, target.ID)

	deactivated, err := operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{
		OperatorID: target.ID, ActorID: actor.ID, IPAddress: "10.0.0.7",
	})
	require.NoError(t, err)
	assert.False(t, deactivated.Active)
	require.NotNil(t, deactivated.DeactivatedAt)
	assert.WithinDuration(t, time.Now(), *deactivated.DeactivatedAt, time.Minute)
	assert.Zero(t, countRows(t, db, `SELECT count(*) FROM platform.operator_refresh_tokens WHERE operator_id = ?`, target.ID),
		"the sessions of a deactivated operator end at once")
	assert.Equal(t, 1, countRows(t, db, `SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'operator_deactivated' AND resource_id = ?`, actor.ID, target.ID),
		"the ledger names who deactivated whom")

	stored, err := operators.FindOperator(ctx, target.ID)
	require.NoError(t, err)
	assert.False(t, stored.Active)
	assert.Equal(t, deactivated.DeactivatedAt.Unix(), stored.DeactivatedAt.Unix())

	reactivated, err := operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{
		OperatorID: target.ID, ActorID: actor.ID, IPAddress: "10.0.0.7", Active: true,
	})
	require.NoError(t, err)
	assert.True(t, reactivated.Active)
	assert.Nil(t, reactivated.DeactivatedAt, "a reactivated account no longer counts toward deletion")
	assert.Equal(t, 1, countRows(t, db, `SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'operator_reactivated' AND resource_id = ?`, actor.ID, target.ID))
}

func TestOperatorCannotDeactivateTheirOwnAccount(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	operators := buildServiceFactory(t, db)
	ctx := context.Background()
	actor := testpkg.CreateTestOperator(t, db)

	_, err := operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{OperatorID: actor.ID, ActorID: actor.ID})
	require.ErrorIs(t, err, identityaccess.ErrOperatorSelfDeactivation)
	stored, err := operators.FindOperator(ctx, actor.ID)
	require.NoError(t, err)
	assert.True(t, stored.Active)

	missing := testpkg.CreateTestOperator(t, db)
	_, err = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, missing.ID)
	require.NoError(t, err)
	_, err = operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{OperatorID: missing.ID, ActorID: actor.ID})
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
	_, err = operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{OperatorID: missing.ID, ActorID: actor.ID, Active: true})
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
}

// An account goes once it was deactivated before 1 January of last year: by
// then the hourly cleanup has removed its ledger entries, which all predate
// the deactivation. Its announcements stay without an author.
func TestDeactivatedOperatorsAreDeletedAfterTheLedgerWindow(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	operators := buildServiceFactory(t, db)
	ctx := context.Background()
	// Three years back is always before 1 January of last year; half a year
	// back never is. The exact cutoff is pinned in the application tests.
	now := time.Now()
	deactivate := func(operatorID int64, at time.Time) {
		_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false, deactivated_at = ? WHERE id = ?`, at, operatorID)
		require.NoError(t, err)
	}
	expired := testpkg.CreateTestOperator(t, db)
	deactivate(expired.ID, now.AddDate(-3, 0, 0))
	var announcementID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.announcements (title, content, type, created_by)
		VALUES ('Neue Version', 'Text', 'release', ?) RETURNING id`, expired.ID).Scan(ctx, &announcementID))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM platform.announcements WHERE id = ?`, announcementID) })
	young := testpkg.CreateTestOperator(t, db)
	deactivate(young.ID, now.AddDate(0, -6, 0))
	active := testpkg.CreateTestOperator(t, db)

	deleted, err := operators.DeleteDeactivatedOperators(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)

	_, err = operators.FindOperator(ctx, expired.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
	for _, kept := range []int64{young.ID, active.ID} {
		_, err = operators.FindOperator(ctx, kept)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, countRows(t, db, `SELECT count(*) FROM platform.announcements WHERE id = ? AND created_by IS NULL`, announcementID),
		"the announcement stays without its author")
}
