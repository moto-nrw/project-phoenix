package postgres

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestOrderBatchResultsRestoresInputOrder(t *testing.T) {
	t.Parallel()
	day := calendar.Date("2026-08-31")
	values := []row{
		{ID: 41, Value: "positive", Day: day, Time: time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC), StudentID: 1},
		{ID: 42, Value: "negative", Day: day, Time: time.Date(1, 1, 1, 8, 1, 0, 0, time.UTC), StudentID: 2},
	}
	returned := []row{values[1], values[0]}

	entries, err := orderBatchResults(values, returned)

	require.NoError(t, err)
	require.Equal(t, []int64{41, 42}, []int64{entries[0].ID, entries[1].ID})
	require.Equal(t, []string{"positive", "negative"}, []string{entries[0].Value, entries[1].Value})
}
