package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TestOfferingPhasesOutsidePeriod (#5344): a phase whose active offerings a
// planning period leaves out is named, one that fits or lies in another
// period is not. The full-containment rule itself stays unchanged.
func TestOfferingPhasesOutsidePeriod(t *testing.T) {
	t.Parallel()
	period := &careplan.LinkedPeriod{ID: 7, StartDate: calendar.NewDate(2026, 8, 1), EndDate: calendar.NewDate(2027, 7, 7)}
	phase := func(id int64, start, end calendar.Date) careplan.OfferingPhase {
		return careplan.OfferingPhase{ID: id, Name: "Phase", ServiceStart: start, ServiceEnd: end}
	}
	for _, tc := range []struct {
		name     string
		phase    careplan.OfferingPhase
		inactive bool
		noPeriod bool
		excluded bool
	}{
		{name: "end after the period", phase: phase(1, period.StartDate, calendar.NewDate(2027, 7, 31)), excluded: true},
		{name: "start before the period", phase: phase(1, calendar.NewDate(2026, 7, 20), period.EndDate), excluded: true},
		{name: "phase around the period", phase: phase(1, calendar.NewDate(2026, 7, 1), calendar.NewDate(2027, 7, 31)), excluded: true},
		{name: "exact bounds fit", phase: phase(1, period.StartDate, period.EndDate)},
		{name: "control fixture: end moved inside", phase: phase(1, period.StartDate, calendar.NewDate(2027, 7, 7))},
		{name: "phase inside the period", phase: phase(1, calendar.NewDate(2026, 10, 12), calendar.NewDate(2026, 10, 23))},
		{name: "phase after the period", phase: phase(1, calendar.NewDate(2027, 7, 8), calendar.NewDate(2027, 7, 31))},
		{name: "phase before the period", phase: phase(1, calendar.NewDate(2025, 8, 1), calendar.NewDate(2026, 7, 31))},
		{name: "touches the last period day", phase: phase(1, period.EndDate, calendar.NewDate(2027, 7, 31)), excluded: true},
		{name: "only inactive offerings", phase: phase(1, period.StartDate, calendar.NewDate(2027, 7, 31)), inactive: true},
		{name: "no period excludes nothing", phase: phase(1, period.StartDate, calendar.NewDate(2027, 7, 31)), noPeriod: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checked := period
			if tc.noPeriod {
				checked = nil
			}
			offerings := []careplan.CareOffering{
				{ID: 11, PhaseID: tc.phase.ID, IsActive: !tc.inactive},
				{ID: 12, PhaseID: tc.phase.ID, IsActive: !tc.inactive},
				{ID: 13, PhaseID: tc.phase.ID, IsActive: false},
				{ID: 14, PhaseID: 99, IsActive: true},
			}
			got := offeringPhasesOutsidePeriod([]careplan.OfferingPhase{tc.phase}, offerings, checked)
			if !tc.excluded {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			assert.Equal(t, tc.phase, got[0].Phase)
			assert.Equal(t, 2, got[0].OfferingCount, "only the phase's active offerings count")
			assert.Equal(t, *period, got[0].Period)
		})
	}
}

// TestOfferingPhasesOutsidePeriodOrder: several excluded phases come sorted
// by service start, a phase without offerings is left out.
func TestOfferingPhasesOutsidePeriodOrder(t *testing.T) {
	t.Parallel()
	period := &careplan.LinkedPeriod{StartDate: calendar.NewDate(2026, 11, 16), EndDate: calendar.NewDate(2026, 11, 27)}
	later := careplan.OfferingPhase{ID: 1, Name: "B", ServiceStart: calendar.NewDate(2026, 9, 1), ServiceEnd: calendar.NewDate(2027, 7, 31)}
	earlier := careplan.OfferingPhase{ID: 2, Name: "A", ServiceStart: calendar.NewDate(2026, 8, 1), ServiceEnd: calendar.NewDate(2027, 7, 31)}
	empty := careplan.OfferingPhase{ID: 3, Name: "C", ServiceStart: calendar.NewDate(2026, 8, 1), ServiceEnd: calendar.NewDate(2027, 7, 31)}
	got := offeringPhasesOutsidePeriod(
		[]careplan.OfferingPhase{later, earlier, empty},
		[]careplan.CareOffering{{PhaseID: 1, IsActive: true}, {PhaseID: 2, IsActive: true}},
		period,
	)
	require.Len(t, got, 2)
	assert.Equal(t, earlier.ID, got[0].Phase.ID)
	assert.Equal(t, later.ID, got[1].Phase.ID)
}
