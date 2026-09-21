package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func TestResolveCareDay(t *testing.T) {
	t.Parallel()
	pickup := time.Date(0, time.January, 1, 16, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		facts careplan.CareDayFacts
		want  careplan.CareDayStatus
	}{
		{"no plan is unknown", careplan.CareDayFacts{}, careplan.CareDayUnknown},
		{"plan on another weekday", careplan.CareDayFacts{HasPlan: true}, careplan.CareDayNotScheduled},
		{"arrival day without a clock", careplan.CareDayFacts{HasPlan: true, HasArrivalSchedule: true}, careplan.CareDayScheduled},
		{"pickup plans the day", careplan.CareDayFacts{Pickup: &careplan.EffectivePickupTime{PickupTime: &pickup}}, careplan.CareDayScheduled},
		{"arrival cancellation beats pickup", careplan.CareDayFacts{Arrival: &careplan.EffectiveArrivalTime{IsException: true}, Pickup: &careplan.EffectivePickupTime{PickupTime: &pickup}}, careplan.CareDayCancelled},
		{"pickup cancellation", careplan.CareDayFacts{Pickup: &careplan.EffectivePickupTime{IsException: true}}, careplan.CareDayCancelled},
		{"pickup cannot create an unbooked day", careplan.CareDayFacts{BookingsAuthoritative: true, Pickup: &careplan.EffectivePickupTime{PickupTime: &pickup}}, careplan.CareDayNotScheduled},
		{"booked day without a clock", careplan.CareDayFacts{BookingsAuthoritative: true, HasBookedCareDay: true, HasPlan: true, HasArrivalSchedule: true}, careplan.CareDayScheduled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := careplan.ResolveCareDay(tc.facts); got != tc.want {
				t.Fatalf("ResolveCareDay() = %q, want %q", got, tc.want)
			}
		})
	}
}
