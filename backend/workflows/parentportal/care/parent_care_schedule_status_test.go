package care

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
)

func TestHHMMLeavesMissingTimeEmpty(t *testing.T) {
	t.Parallel()

	if got := hhmm(time.Time{}); got != "" {
		t.Fatalf("hhmm(zero) = %q, want empty", got)
	}
}

func TestCareDayStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		hasCarePlan bool
		hasArrival  bool
		arrival     string
		pickup      string
		want        careschedule.CareDayStatus
	}{
		{name: "scheduled by arrival", hasArrival: true, arrival: "08:00", want: careschedule.CareDayScheduled},
		{name: "scheduled by care day without time", hasArrival: true, want: careschedule.CareDayScheduled},
		{name: "scheduled by pickup", pickup: "15:30", want: careschedule.CareDayScheduled},
		{name: "off day in existing plan", hasCarePlan: true, want: careschedule.CareDayNotScheduled},
		{name: "plan unknown", want: careschedule.CareDayUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := careDayStatus(test.hasCarePlan, test.hasArrival, test.arrival, test.pickup); got != test.want {
				t.Fatalf("careDayStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
