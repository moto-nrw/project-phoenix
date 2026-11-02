package owners

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Care Plan's booking materialization announces a changed offering pickup
// projection through this binding once its transaction committed (#3560):
// staff clients refetch the pickup schedule. Care Plan decides when and to
// whom (modules/careplan/internal/application).
func TestPickupScheduleBroadcaster_WakesStaffPickupViews(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	require.NoError(t, PickupScheduleBroadcaster{Broadcaster: broadcaster}.PickupScheduleChanged(7))

	testpkg.AssertSingleTenantEvent(t, broadcaster, "pickup_schedule_changed", 7)
}

// Only a phase of the kind Ferienbetreuung marks its offerings as holiday
// care for the Regeltermin suggestion (#5139); school-year and other phases
// do not.
func TestBookingOfferingPhaseMarksHolidayPhases(t *testing.T) {
	t.Parallel()

	for kind, holiday := range map[string]bool{
		enrollmentOwner.PhaseKindHoliday:    true,
		enrollmentOwner.PhaseKindSchoolYear: false,
		enrollmentOwner.PhaseKindCustom:     false,
	} {
		assert.Equal(t, holiday, BookingOfferingPhase(&enrollmentOwner.Phase{Kind: kind}).Holiday, kind)
	}
}

func TestStudentDayBookings_KeepTheExclusiveBookingEnd(t *testing.T) {
	t.Parallel()

	until := calendar.NewDate(2026, 11, 19)
	assert.Equal(t, []timetable.StudentDayBooking{{
		Weekday: timetable.WeekdayThursday, CountsAsCare: true, PickupTime: "16:00", Until: &until,
	}}, studentDayBookings([]careplan.RemainingDayBooking{{
		Weekday: timetable.WeekdayThursday, CountsAsCare: true, PickupTime: "16:00", Until: &until,
	}}))
}
