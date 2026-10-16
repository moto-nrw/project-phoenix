package presenceprojection

import (
	"database/sql"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListManualPlanningOccurrencesRejectsInvalidTenantBeforeSQL(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Group")
	date := calendar.NewDate(2026, 9, 4)
	ctx := testpkg.Ctx(t)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx = counter.Context(ctx)
	// An ambient valid tenant must not rescue an invalid explicit argument.
	for _, invalid := range []int64{0, -1} {
		result, err := Reader{}.ListManualPlanningOccurrences(ctx, tx, invalid, group.ID, date, date)
		assert.ErrorIs(t, err, ErrInvalidTenantID)
		assert.Empty(t, result)
	}
	assert.Empty(t, counter.Queries(), "invalid tenants must fail before SQL")
}

func TestListManualPlanningOccurrencesPreservesDatabaseErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Group")
	date := calendar.NewDate(2026, 9, 4)
	ctx := testpkg.Ctx(t)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	_, err = Reader{}.ListManualPlanningOccurrences(ctx, tx, testpkg.Tenant(t), group.ID, date, date)
	assert.ErrorIs(t, err, sql.ErrTxDone)
	assert.ErrorContains(t, err, "presence projection:")
}
