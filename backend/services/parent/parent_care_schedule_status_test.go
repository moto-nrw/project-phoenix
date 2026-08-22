package parent

import (
	"testing"
	"time"

	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
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
		want        scheduleService.CareDayStatus
	}{
		{name: "scheduled by arrival", hasArrival: true, arrival: "08:00", want: scheduleService.CareDayScheduled},
		{name: "scheduled by care day without time", hasArrival: true, want: scheduleService.CareDayScheduled},
		{name: "scheduled by pickup", pickup: "15:30", want: scheduleService.CareDayScheduled},
		{name: "off day in existing plan", hasCarePlan: true, want: scheduleService.CareDayNotScheduled},
		{name: "plan unknown", want: scheduleService.CareDayUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := careDayStatus(test.hasCarePlan, test.hasArrival, test.arrival, test.pickup); got != test.want {
				t.Fatalf("careDayStatus() = %q, want %q", got, test.want)
			}
		})
	}
}
