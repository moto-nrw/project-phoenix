package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestRequestChildOfferingRepository_ListAtDate_DoesNotReturnFutureSelection(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	fx := setupOfferingChangeFixture(t, env, "FutureSelection")

	require.NoError(t, env.repos.RequestChildOffering.ReplaceForRequestChild(ctx, fx.childID, nil))
	futureStart := env.sourcePhase.ServiceStartDate.AddDays(30)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		fx.childID,
		futureStart,
		[]*enrollmentModels.RequestChildOffering{{CareOfferingID: fx.oldOffering.ID}},
	))

	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(
		ctx,
		fx.childID,
		env.sourcePhase.ServiceStartDate,
	)
	require.NoError(t, err)
	require.Empty(t, links)
}

// The branch a hand-written predicate misses (#2185): asked for a date
// BEFORE the phase's service start, the repository does not answer "nothing
// booked" — it returns the next interval instead. Any reader that re-derives
// the point-in-time predicate in Go disagrees here, and disagreeing means an
// editor seeded empty while the save still finds a selection to replace.
// Whoever removes this fallback must also revisit ListChildOfferings.
func TestRequestChildOfferingRepo_AtDateBeforeServiceStart_ReturnsNextInterval(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	fx := setupOfferingChangeFixture(t, env, "PreStartFallback")

	beforeStart := env.sourcePhase.ServiceStartDate.AddDays(-10)

	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, beforeStart)
	require.NoError(t, err)
	require.NotEmpty(t, links,
		"before the service start the repository reports the upcoming interval, not 'nothing booked'")
	for _, link := range links {
		require.NotNil(t, link.ValidFrom)
		assert.True(t, link.ValidFrom.After(beforeStart),
			"the fallback returns rows a plain point-in-time predicate would reject")
	}
}

func TestRequestChildOfferingRepository_ListAtDates_DoesNotReturnHistoricalSelection(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	fx := setupOfferingChangeFixture(t, env, "HistoricalSelection")

	futureStart := env.sourcePhase.ServiceStartDate.AddDays(30)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		fx.childID,
		futureStart,
		[]*enrollmentModels.RequestChildOffering{{CareOfferingID: fx.newOffering.ID}},
	))

	links, err := env.repos.RequestChildOffering.ListByRequestChildIDsAtDates(ctx, map[int64]timezone.Date{
		fx.childID: futureStart,
	})
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, fx.newOffering.ID, links[0].CareOfferingID)
}
