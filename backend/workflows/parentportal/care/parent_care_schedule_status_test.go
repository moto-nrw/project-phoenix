package care

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
		want        careplan.CareDayStatus
	}{
		{name: "scheduled by arrival", hasArrival: true, arrival: "08:00", want: careplan.CareDayScheduled},
		{name: "scheduled by care day without time", hasArrival: true, want: careplan.CareDayScheduled},
		{name: "scheduled by pickup", pickup: "15:30", want: careplan.CareDayScheduled},
		{name: "off day in existing plan", hasCarePlan: true, want: careplan.CareDayNotScheduled},
		{name: "plan unknown", want: careplan.CareDayUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := careDayStatus(test.hasCarePlan, test.hasArrival, test.arrival, test.pickup); got != test.want {
				t.Fatalf("careDayStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
