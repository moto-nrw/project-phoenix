package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMealBillingAttendanceCutoffMatchesCleanup(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		days   int
		cutoff calendar.Date
	}{{30, "2026-10-09"}, {365, "2025-11-08"}} {
		find := mealBillingAvailableFrom(func(context.Context) (int, error) { return scenario.days, nil },
			func() calendar.Date { return "2026-11-08" })
		cutoff, err := find(context.Background())
		require.NoError(t, err)
		assert.Equal(t, scenario.cutoff, cutoff)
	}
	unavailable := errors.New("setting unavailable")
	find := mealBillingAvailableFrom(func(context.Context) (int, error) { return 0, unavailable },
		func() calendar.Date { return "2026-11-08" })
	_, err := find(context.Background())
	require.ErrorIs(t, err, unavailable)
}
