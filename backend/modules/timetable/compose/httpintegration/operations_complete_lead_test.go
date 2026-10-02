package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The roster announces the school's lead before the planned end (#3809), so
// "Beenden ab" and the write path name the same time: a block ending 15:00
// with a 15-minute lead completes from 14:45.
func TestTimetableOperationsRosterAppliesCompleteLead(t *testing.T) {
	t.Parallel()

	instanceID, activeGroupID := int64(3809), int64(3810)
	completeAt := time.Date(2026, time.May, 10, 14, 45, 0, 0, calendar.Berlin)

	for _, tc := range []struct {
		name string
		now  time.Time
		lead int
		want bool
		at   time.Time
	}{
		{name: "one second before the lead", now: completeAt.Add(-time.Second), lead: 15, want: false, at: completeAt},
		{name: "at the lead", now: completeAt, lead: 15, want: true, at: completeAt},
		{name: "no lead keeps the planned end", now: completeAt, lead: 0, want: false, at: completeAt.Add(15 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := newTimetableOpsDeps()
			deps.settings.enforcePlannedEnd = true
			deps.settings.completeLead = tc.lead
			deps.now = func() time.Time { return tc.now }
			deps.instanceRepo.byID[instanceID] = activeInstance(instanceID, activeGroupID)

			roster, err := deps.service.Roster(context.Background(), 700, true, instanceID)

			require.NoError(t, err)
			assert.Equal(t, tc.want, roster.Instance.CanComplete)
			assert.Equal(t, tc.at.Format(time.RFC3339), roster.Instance.CompleteAvailableAt)
		})
	}
}

func TestTimetableOperationsRosterSurfacesCompleteLeadFailure(t *testing.T) {
	t.Parallel()

	deps := newTimetableOpsDeps()
	deps.settings.enforcePlannedEnd = true
	deps.settings.completeLeadErr = errors.New("settings down")
	deps.instanceRepo.byID[3811] = activeInstance(3811, 3812)

	_, err := deps.service.Roster(context.Background(), 700, true, 3811)

	require.ErrorIs(t, err, timetable.ErrLifecycleSettings)
}
