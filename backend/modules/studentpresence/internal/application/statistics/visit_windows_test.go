package statistics

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestVisitWindowsUseEligibilityAndBerlinDayBoundaries(t *testing.T) {
	t.Parallel()
	first := timezone.NewDate(2026, 3, 28)
	today := first.AddDays(3)
	student := &ports.StatisticsStudent{ID: 7, EnrolledOn: func(day, reportToday timezone.Date) bool {
		require.Equal(t, today, reportToday)
		return day != first.AddDays(2)
	}}
	windows := visitWindows([]*ports.StatisticsStudent{nil, {}, student}, first, today, today)
	require.Equal(t, []ports.StudentVisitWindow{
		{StudentID: 7, StartAt: first.BerlinMidnight(), EndAt: first.AddDays(2).BerlinMidnight()},
		{StudentID: 7, StartAt: today.BerlinMidnight(), EndAt: today.AddDays(1).BerlinMidnight()},
	}, windows)
	require.Equal(t, 47*time.Hour, windows[0].EndAt.Sub(windows[0].StartAt), "spring DST loses one elapsed hour, not a calendar day")
}
