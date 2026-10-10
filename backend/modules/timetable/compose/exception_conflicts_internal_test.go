package compose

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestExceptionArrivalReadsRequireNativeBaseline(t *testing.T) {
	t.Parallel()
	detection := &conflictDetection{logger: slog.Default()}
	date := timezone.NewDate(2026, 9, 21)
	var studentID int64

	err := detection.fillArrivalSchedules(context.Background(), &arrivalPreload{},
		map[timezone.Date]map[int64]struct{}{date: {studentID: {}}})
	require.EqualError(t, err, "load arrival schedules: baseline projection is not configured")

	// No affected students means no projection is needed.
	require.NoError(t, detection.fillArrivalSchedules(context.Background(), &arrivalPreload{}, nil))
}

func TestTemplatePreloadUsesFridayAsWeekendOriginOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	friday := timezone.NewDate(2026, time.May, 8)
	pre := &templatePreload{byKey: map[groupWeekdayKey][]time.Time{
		{GroupID: 7, Weekday: int(time.Friday)}:   {time.Date(0, 1, 1, 14, 0, 0, 0, time.UTC)},
		{GroupID: 7, Weekday: int(time.Saturday)}: {time.Date(0, 1, 1, 11, 0, 0, 0, time.UTC)},
	}}

	start, ok := pre.resolveOriginalStart(7, friday.AddDays(1), true, slog.Default())
	assert.True(t, ok)
	assert.Equal(t, "14:00", start)

	start, ok = pre.resolveOriginalStart(7, friday.AddDays(1), false, slog.Default())
	assert.True(t, ok)
	assert.Equal(t, "11:00", start)
}
