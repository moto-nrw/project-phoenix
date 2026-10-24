package workforce

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestWholeDayAssignmentNamesEachOfItsDays(t *testing.T) {
	t.Parallel()

	days, err := ScheduleWholeDayAssignment{Dates: []string{"2026-03-02", "2026-03-03"}}.Days()
	require.NoError(t, err)
	assert.Equal(t, []calendar.Date{"2026-03-02", "2026-03-03"}, days)

	days, err = ScheduleWholeDayAssignment{}.Days()
	require.NoError(t, err)
	assert.Empty(t, days)

	for _, dates := range [][]string{{"2026-03-02", ""}, {"02.03.2026"}} {
		_, err := ScheduleWholeDayAssignment{Dates: dates}.Days()
		var refusal *SubstitutionOperationError
		require.True(t, errors.As(err, &refusal), "dates %v", dates)
		assert.ErrorIs(t, err, ErrSubstitutionInvalidPeriod)
		assert.Equal(t, "Der Zeitraum ist ungültig.", refusal.Message)
	}
}
