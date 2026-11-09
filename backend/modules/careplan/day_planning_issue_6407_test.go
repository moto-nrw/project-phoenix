package careplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// Ort hat Vorrang (#6371, #6407): Ist ein krank gemeldetes, entschuldigtes oder
// auf Klassenfahrt gemeldetes Kind eingecheckt, gilt der Betreuungsplan des
// Tages weiter. Die Meldung ist nur ein Hinweis; die geplante Gehzeit bleibt
// der Grund des Tages, damit Abholzeit und Verspätung sichtbar bleiben.
func TestResolveDayPlanningKeepsThePlanOfACheckedInReportedChild(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, time.November, 9)
	pickupAt := time.Date(2026, time.November, 9, 15, 30, 0, 0, time.UTC)
	arrivalAt := time.Date(2026, time.November, 9, 11, 45, 0, 0, time.UTC)

	reports := map[string]DayPlanningInputs{
		"sick":       {Sick: true},
		"excused":    {Excused: true},
		"class_trip": {ClassTrip: true},
	}
	for name, report := range reports {
		t.Run(name+"_pickup_schedule", func(t *testing.T) {
			t.Parallel()
			in := report
			in.HasActualAttendance = true
			in.Pickup = &EffectivePickupTime{Date: day, PickupTime: &pickupAt}

			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonPickupSchedule, got.Reason)
		})
		t.Run(name+"_arrival_and_pickup_exception", func(t *testing.T) {
			t.Parallel()
			in := report
			in.HasActualAttendance = true
			in.Arrival = &EffectiveArrivalTime{Date: day, ArrivalTime: &arrivalAt}
			in.Pickup = &EffectivePickupTime{Date: day, PickupTime: &pickupAt, IsException: true}

			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonArrivalSchedule, got.Reason)
		})
		t.Run(name+"_without_check_in_stays_absent", func(t *testing.T) {
			t.Parallel()
			in := report
			in.Pickup = &EffectivePickupTime{Date: day, PickupTime: &pickupAt}

			got := ResolveDayPlanning(in)
			assert.False(t, got.ComesToday)
			assert.Equal(t, name, got.Reason)
		})
		t.Run(name+"_checked_in_without_plan_is_unplanned", func(t *testing.T) {
			t.Parallel()
			in := report
			in.HasActualAttendance = true

			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonUnplanned, got.Reason)
		})
	}

	t.Run("timeless_exception_still_reads_unplanned", func(t *testing.T) {
		t.Parallel()
		got := ResolveDayPlanning(DayPlanningInputs{
			HasActualAttendance: true,
			Sick:                true,
			Pickup:              &EffectivePickupTime{Date: day, IsException: true},
		})
		assert.True(t, got.ComesToday)
		assert.Equal(t, DayPlanningReasonUnplanned, got.Reason)
	})
}
