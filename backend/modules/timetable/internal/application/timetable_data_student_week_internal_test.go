package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

// A child's week reads the regular arrival through Care Plan's native
// baseline; a composition without it fails instead of guessing.
func TestArrivalReadsRequireNativeBaseline(t *testing.T) {
	t.Parallel()
	data := &timetableData{deps: ports.TimetableDataDependencies{RunInTx: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }}}
	date := calendar.NewDate(2026, 9, 21)
	var studentID int64
	week := &timetable.StudentWeek{ArrivalByDate: map[string]timetable.StudentWeekTimes{}, PickupByDate: map[string]timetable.StudentWeekTimes{}}

	err := data.loadStudentWeekBaselines(context.Background(), week, studentID, date, date)

	require.EqualError(t, err, "load arrival schedules: baseline projection is not configured")
}
