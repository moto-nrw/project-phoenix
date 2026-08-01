package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

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
