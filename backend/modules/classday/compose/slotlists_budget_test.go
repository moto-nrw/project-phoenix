package compose_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestBuildList_QueryBudget records the projection's statement count for a
// slot reconciliation and pins it against the register (#2701, #2940): every
// read over the owner facades is a bulk load by ID set, so the count must not
// move when the roster grows from three to eight children.
func TestBuildList_QueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := buildMensaFixtureOn(t, db)
	ctx := testpkg.Ctx(t)
	counter := testpkg.CaptureQueries(t, db)

	build := func(rows int) int {
		counter.Reset()
		result, err := f.svc.BuildList(ctx, classday.Params{
			Date:   classday.Date(listDate.String()),
			Target: classday.TargetSlots,
			Source: classday.SourceReconciliation,
		})
		require.NoError(t, err)
		require.Len(t, result.Rows, rows)
		return counter.Total()
	}

	small := build(3)
	f.addPlannedPresent(t, 5)
	large := build(8)
	assert.Equal(t, small, large, "query count must not grow with the roster")
	testpkg.AssertQueryBudget(t, "modules.classday.slot_list.build", counter.Queries())
}
