package presenceprojectionhttp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// A checkout to class is no pickup: the detail names who recorded it only once
// the child leaves for the day (#5710).
func TestDetailActorsLeavePickupEmptyForCheckoutToClass(t *testing.T) {
	t.Parallel()
	checkIn := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(4 * time.Hour)
	class := studentpresence.CheckoutReasonClass
	actor := &studentpresence.AttendanceActor{Kind: studentpresence.AttendanceActorStaff, Name: "Paul Abholung"}
	status := &studentpresence.DailyAttendanceStatus{
		Status: "checked_out", CheckInTime: &checkIn, CheckOutTime: &checkOut, CheckOutReason: &class,
		CheckedInActor: actor, CheckedOutActor: actor,
	}

	inClass := &StudentDetailResponse{}
	applyAttendanceActorsToDetail(inClass, status)
	assert.NotNil(t, inClass.ActualArrivalBy)
	assert.Nil(t, inClass.ActualPickupBy)

	status.CheckOutReason = nil
	home := &StudentDetailResponse{}
	applyAttendanceActorsToDetail(home, status)
	assert.Equal(t, &AttendanceActorResponse{Kind: "staff", Name: "Paul Abholung"}, home.ActualPickupBy)
}
