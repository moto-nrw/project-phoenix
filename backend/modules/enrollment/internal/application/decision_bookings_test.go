package application

import (
	"context"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type decisionBookingsRecorder struct {
	DecisionBookings
	studentIDs [][]int64
}

func (r *decisionBookingsRecorder) ReconcileOfferingPickupForStudents(_ context.Context, studentIDs []int64) error {
	r.studentIDs = append(r.studentIDs, studentIDs)
	return nil
}

func TestAfterDecisionReconcilesFutureApprovedBooking(t *testing.T) {
	t.Parallel()

	bookings := &decisionBookingsRecorder{}
	decisions := NewDecisions(DecisionDependencies{
		Settings: unusedDecisionSettings{},
		Bookings: bookings,
		Today:    func() calendar.Date { return calendar.NewDate(2026, 11, 2) },
	})
	studentID := int64(42)

	err := decisions.afterDecision(context.Background(), &decisionTarget{child: &RequestChild{CreatedStudentID: &studentID}},
		&enrollmentNative.Phase{ServiceStartDate: enrollmentNative.Date("2027-08-01")}, enrollmentNative.DecideInput{
			Status: enrollmentNative.DecisionApproved, SuppressParentEmail: true,
		})

	require.NoError(t, err)
	assert.Equal(t, [][]int64{{studentID}}, bookings.studentIDs)
}
