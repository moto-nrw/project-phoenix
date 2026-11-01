package careplan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TestClipPhaseToPeriod: a phase counts for the part of its service window
// that overlaps the planning period; only a phase without overlap is refused.
func TestClipPhaseToPeriod(t *testing.T) {
	t.Parallel()
	period := &careplan.LinkedPeriod{ID: 9, StartDate: calendar.NewDate(2026, 8, 1), EndDate: calendar.NewDate(2027, 7, 7)}
	for _, tc := range []struct {
		name               string
		start, end         calendar.Date
		wantStart, wantEnd calendar.Date
		refused            bool
	}{
		{name: "inside", start: calendar.NewDate(2026, 9, 1), end: calendar.NewDate(2027, 6, 30),
			wantStart: calendar.NewDate(2026, 9, 1), wantEnd: calendar.NewDate(2027, 6, 30)},
		{name: "ends after the period", start: calendar.NewDate(2026, 8, 1), end: calendar.NewDate(2027, 7, 31),
			wantStart: calendar.NewDate(2026, 8, 1), wantEnd: calendar.NewDate(2027, 7, 7)},
		{name: "starts before the period", start: calendar.NewDate(2026, 7, 1), end: calendar.NewDate(2027, 6, 30),
			wantStart: calendar.NewDate(2026, 8, 1), wantEnd: calendar.NewDate(2027, 6, 30)},
		{name: "covers the period", start: calendar.NewDate(2026, 7, 1), end: calendar.NewDate(2027, 7, 31),
			wantStart: calendar.NewDate(2026, 8, 1), wantEnd: calendar.NewDate(2027, 7, 7)},
		{name: "touches the last day", start: calendar.NewDate(2027, 7, 7), end: calendar.NewDate(2027, 7, 31),
			wantStart: calendar.NewDate(2027, 7, 7), wantEnd: calendar.NewDate(2027, 7, 7)},
		{name: "after the period", start: calendar.NewDate(2027, 7, 8), end: calendar.NewDate(2027, 7, 31), refused: true},
		{name: "before the period", start: calendar.NewDate(2026, 6, 1), end: calendar.NewDate(2026, 7, 31), refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			phase := careplan.OfferingPhase{ID: 3, Name: "Schuljahr", ServiceStart: tc.start, ServiceEnd: tc.end, SchoolYear: true}
			clipped, err := careplan.ClipPhaseToPeriod(phase, period)
			if tc.refused {
				require.ErrorIs(t, err, careplan.ErrCareOfferingTemplatePeriodMismatch)
				require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
				return
			}
			require.NoError(t, err)
			want := phase
			want.ServiceStart, want.ServiceEnd = tc.wantStart, tc.wantEnd
			assert.Equal(t, want, clipped)
		})
	}

	t.Run("without a period", func(t *testing.T) {
		t.Parallel()
		phase := careplan.OfferingPhase{ID: 3, ServiceStart: calendar.NewDate(2026, 8, 1), ServiceEnd: calendar.NewDate(2027, 7, 31)}
		clipped, err := careplan.ClipPhaseToPeriod(phase, nil)
		require.NoError(t, err)
		assert.Equal(t, phase, clipped)
	})
}
