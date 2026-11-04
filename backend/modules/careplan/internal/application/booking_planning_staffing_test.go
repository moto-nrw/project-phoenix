package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingPlanningStaffing struct{ calls int }

func (s *recordingPlanningStaffing) AnnounceBookingChange(context.Context) { s.calls++ }

// Every booking write runs ReconcileOfferingPickupForStudents in its
// transaction; that step hands the booking change to the hints that compare
// the planning with the bookings (#5498).
func TestReconcileOfferingPickupForStudentsAnnouncesBookingChange(t *testing.T) {
	t.Parallel()
	staffing := &recordingPlanningStaffing{}
	m := &BookingMaterialization{deps: BookingMaterializationDependencies{PlanningStaffing: staffing}}

	require.NoError(t, m.ReconcileOfferingPickupForStudents(context.Background(), nil))
	assert.Zero(t, staffing.calls, "no student, no booking change")

	require.NoError(t, m.ReconcileOfferingPickupForStudents(context.Background(), []int64{fakeStudentID}))
	assert.Equal(t, 1, staffing.calls)
}

func TestReconcileOfferingPickupForStudentsWithoutPlanningStaffing(t *testing.T) {
	t.Parallel()
	m := &BookingMaterialization{}
	assert.NoError(t, m.ReconcileOfferingPickupForStudents(context.Background(), []int64{fakeStudentID}))
}
