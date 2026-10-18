package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type careDayRecordsStub struct{ CareDayRecords }
type pickupBaselinesStub struct{ careplan.PickupBaselineReader }

// The care-day derivation built with its participation reads it at call
// time without a later wiring step; without it the constructor refuses.
func TestNewCareDayQuery_RequiresItsBoundaries(t *testing.T) {
	t.Parallel()

	participation := fixedCareParticipation{int64(len(t.Name())): true}
	query, err := NewCareDayQuery(CareDayDependencies{
		Records: careDayRecordsStub{}, PickupBaselines: pickupBaselinesStub{}, CareParticipation: participation,
	})
	require.NoError(t, err)
	got, err := query.(*careDayService).ParticipatingStudentIDsByDate(context.Background(), []int64{int64(len(t.Name()))}, careDayMonday, careDayMonday)
	require.NoError(t, err)
	require.Equal(t, map[int64]bool(participation), got[careDayMonday])

	for name, deps := range map[string]CareDayDependencies{
		"participation":    {Records: careDayRecordsStub{}, PickupBaselines: pickupBaselinesStub{}},
		"records":          {PickupBaselines: pickupBaselinesStub{}, CareParticipation: participation},
		"pickup baselines": {Records: careDayRecordsStub{}, CareParticipation: participation},
	} {
		_, err := NewCareDayQuery(deps)
		require.ErrorContains(t, err, "are required", name)
	}
}
