package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/require"
)

// A child's week reads the regular arrival through Care Plan's native
// baseline; a composition without it fails instead of guessing.
func TestArrivalReadsRequireNativeBaseline(t *testing.T) {
	t.Parallel()
	data := &timetableData{deps: TimetableDataDependencies{}}
	date := timezone.NewDate(2026, 9, 21)
	var studentID int64
	week := &timetable.StudentWeek{ArrivalByDate: map[string]timetable.StudentWeekTimes{}, PickupByDate: map[string]timetable.StudentWeekTimes{}}

	err := data.loadStudentWeekBaselines(context.Background(), week, studentID, date, date)

	require.EqualError(t, err, "load arrival schedules: baseline projection is not configured")
}
