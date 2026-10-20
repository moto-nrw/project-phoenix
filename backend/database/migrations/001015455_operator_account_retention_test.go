package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorDeactivationTimeFollowsTheActiveFlag(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)
	deactivatedAt := func() *time.Time {
		var at *time.Time
		require.NoError(t, db.NewRaw(`SELECT deactivated_at FROM platform.operators WHERE id = ?`, operator.ID).Scan(ctx, &at))
		return at
	}

	_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false WHERE id = ?`, operator.ID)
	require.NoError(t, err)
	stamped := deactivatedAt()
	require.NotNil(t, stamped, "a deactivation by hand starts the deletion window too")
	assert.WithinDuration(t, time.Now(), *stamped, time.Minute)

	past := time.Date(2024, time.March, 1, 10, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `UPDATE platform.operators SET deactivated_at = ? WHERE id = ?`, past, operator.ID)
	require.NoError(t, err)
	assert.True(t, past.Equal(*deactivatedAt()), "a given deactivation time stays")

	_, err = db.ExecContext(ctx, `UPDATE platform.operators SET active = true WHERE id = ?`, operator.ID)
	require.NoError(t, err)
	assert.Nil(t, deactivatedAt(), "a reactivation clears the deactivation time")
}

func TestOperatorAccountRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)
	_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false, deactivated_at = now() WHERE id = ?`, operator.ID)
	require.NoError(t, err)
	columnExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'platform' AND table_name = 'operators' AND column_name = 'deactivated_at')`).Scan(ctx, &exists))
		return exists
	}

	require.NoError(t, operatorAccountRetentionDown(ctx, db))
	assert.False(t, columnExists())

	var lastUpdate, deactivatedAt time.Time
	require.NoError(t, db.NewRaw(`SELECT updated_at FROM platform.operators WHERE id = ?`, operator.ID).Scan(ctx, &lastUpdate))
	require.NoError(t, operatorAccountRetentionUp(ctx, db))
	assert.True(t, columnExists())
	require.NoError(t, db.NewRaw(`SELECT deactivated_at FROM platform.operators WHERE id = ?`, operator.ID).Scan(ctx, &deactivatedAt))
	assert.True(t, lastUpdate.Equal(deactivatedAt), "an account deactivated by hand counts from its last update")

	var announcementID int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.announcements (title, content, created_by)
		VALUES ('Hinweis', 'Text', NULL) RETURNING id`).Scan(ctx, &announcementID))
	assert.ErrorContains(t, operatorAccountRetentionDown(ctx, db), "announcements have none",
		"the author cannot become required again over announcements without one")
}

// Only the operator ledger may refuse the deletion of a deactivated account
// (#4047); every other reference to platform.operators cascades or lets go.
func TestOnlyTheOperatorLedgerRefusesAnOperatorDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var refusing []string
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || c.relname FROM pg_constraint AS con
		JOIN pg_class AS c ON c.oid = con.conrelid JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE con.contype = 'f' AND con.confrelid = 'platform.operators'::regclass AND con.confdeltype IN ('a', 'r')
		ORDER BY 1`).Scan(ctx, &refusing))
	assert.Equal(t, []string{"platform.operator_audit_log"}, refusing)
}
