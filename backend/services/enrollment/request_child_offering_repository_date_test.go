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
