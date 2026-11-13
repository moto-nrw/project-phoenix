package simulate

import (
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A full-day run acts out one school day, Runtime.Day: the Berlin day of the
// shared clock when the run starts. The server keeps attendance, shifts and
// blocks per Berlin day and reads the same clock, so every action of the run
// belongs to that day. Real time can cross midnight during a run; from then
// on the server no longer finds the day's attendance (#5511). The run stops
// before its next action instead of acting on another day.

// errSimulatedDayEnded marks a run whose shared clock left its day.
var errSimulatedDayEnded = errors.New("the simulated day ended at Berlin midnight")

type dayEndedError struct{ day, clockDay calendar.Date }

func (e dayEndedError) Error() string {
	return fmt.Sprintf("%v: the run simulates %s, the shared clock reads %s, and the server keeps attendance per Berlin day."+
		" Start the run on a clock that stays on one day, e.g. moved to the morning and driven with --clock-command",
		errSimulatedDayEnded, e.day, e.clockDay)
}

func (dayEndedError) Is(target error) bool { return target == errSimulatedDayEnded }

// checkDay reports a shared clock that left the simulated day. A runtime
// built without a day (a scenario test) holds none.
func (rt *Runtime) checkDay() error {
	if rt.Day.IsZero() {
		return nil
	}
	if clockDay := calendar.DateFromTime(rt.Options.now()); clockDay != rt.Day {
		return dayEndedError{day: rt.Day, clockDay: clockDay}
	}
	return nil
}
