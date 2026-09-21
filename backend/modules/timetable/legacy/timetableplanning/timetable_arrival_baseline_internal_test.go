package timetableplanning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestArrivalReadsRequireNativeBaseline(t *testing.T) {
	t.Parallel()
	svc := NewTimetableDataService(TimetableDataDependencies{})
	date := timezone.NewDate(2026, 9, 21)
	var studentID int64
	err := svc.preloadArrivalSchedules(context.Background(), &StudentWeekPreload{}, studentID, date, date)
	require.EqualError(t, err, "load arrival schedules: baseline projection is not configured")

	err = svc.fillArrivalSchedules(context.Background(), &arrivalPreload{},
		map[string]map[int64]struct{}{date.String(): {studentID: {}}},
		map[string]timezone.Date{date.String(): date})
	require.EqualError(t, err, "load arrival schedules: baseline projection is not configured")

	// No affected students means no projection is needed.
	require.NoError(t, svc.fillArrivalSchedules(context.Background(), &arrivalPreload{}, nil, nil))
}
