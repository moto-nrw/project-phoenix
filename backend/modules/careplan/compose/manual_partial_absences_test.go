package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

type partialAbsenceRows func(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)

func (read partialAbsenceRows) ListPickupExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	return read(ctx, filter)
}

func TestManualPartialAbsenceDatesExcludeAutomaticExcusals(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	const first, last careplan.Date = "2026-03-29", "2026-03-31"
	query := careplan.StudentScheduleFilter{StudentIDs: []int64{7}, From: first, To: last}
	readErr := errors.New("care plan unavailable")
	for _, test := range []struct {
		name string
		err  error
	}{{name: "dates"}, {name: "read failure", err: readErr}} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			reader := NewManualPartialAbsences(partialAbsenceRows(func(actual context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
				calls++
				require.Same(t, ctx, actual)
				require.Equal(t, query, filter)
				return []careplan.PickupException{
					{ExceptionDate: first},
					{ExceptionDate: "2026-03-30", ExcusedFrom: &clock, ExcusedAuto: true},
					{ExceptionDate: last, ExcusedFrom: &clock},
				}, test.err
			}))
			dates, err := reader.Dates(ctx, query.StudentIDs[0], first, last)
			require.Equal(t, 1, calls)
			if test.err != nil {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, dates)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []careplan.Date{last}, dates)
		})
	}
}
