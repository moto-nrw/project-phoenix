package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOwnerOfferingSelectionsAtDate_DoesNotReturnFutureSelection(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	fx := setupOfferingChangeFixture(t, env, "FutureSelection")

	require.NoError(t, env.repos.Enrollment().ReplaceRequestChildOfferings(ctx, fx.childID, nil))
	futureStart := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(30)
	require.NoError(t, env.repos.Enrollment().ScheduleRequestChildOfferings(
		ctx,
		fx.childID,
		owner.Date(futureStart),
		[]*owner.RequestChildOffering{{CareOfferingID: fx.oldOffering.ID}},
	))

	links, err := env.repos.Enrollment().RequestChildOfferingsAtDate(
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
func TestOwnerOfferingSelectionsBeforeServiceStart_ReturnsNextInterval(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	fx := setupOfferingChangeFixture(t, env, "PreStartFallback")

	beforeStart := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(-10)

	links, err := env.repos.Enrollment().RequestChildOfferingsAtDate(ctx, fx.childID, owner.Date(beforeStart))
	require.NoError(t, err)
	require.NotEmpty(t, links,
		"before the service start the repository reports the upcoming interval, not 'nothing booked'")
	for _, link := range links {
		require.NotNil(t, link.ValidFrom)
		assert.True(t, owner.Date(beforeStart).Before(*link.ValidFrom),
			"the fallback returns rows a plain point-in-time predicate would reject")
	}
}

func TestOwnerOfferingSelectionsAtDates_DoesNotReturnHistoricalSelection(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	fx := setupOfferingChangeFixture(t, env, "HistoricalSelection")

	futureStart := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(30)
	require.NoError(t, env.repos.Enrollment().ScheduleRequestChildOfferings(
		ctx,
		fx.childID,
		owner.Date(futureStart),
		[]*owner.RequestChildOffering{{CareOfferingID: fx.newOffering.ID}},
	))

	links, err := env.repos.Enrollment().RequestChildOfferingsAtDates(ctx, map[int64]owner.Date{
		fx.childID: owner.Date(futureStart),
	})
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, fx.newOffering.ID, links[0].CareOfferingID)
}
