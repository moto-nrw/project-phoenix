package compose

import (
	"context"
	"log/slog"
	"testing"

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
