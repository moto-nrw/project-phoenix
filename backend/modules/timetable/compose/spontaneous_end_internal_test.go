package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spontaneousBlock(id int64, date timezone.Date, startHour, startMinute int, spontaneous bool) *scheduleModels.ActivityInstance {
	instance := endedInstance(id, date)
	instance.IsSpontaneous = spontaneous
	instance.StartTime = time.Date(1, time.January, 1, startHour, startMinute, 0, 0, time.UTC)
	instance.EndTime = instance.StartTime.Add(time.Hour)
	return instance
}

func berlinAt(date timezone.Date, hour, minute int) time.Time {
	return date.BerlinMidnight().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// A spontaneous block ends when its session ends, not at the placeholder hour
// the kiosk mirror wrote (#3921).
func TestSpontaneousCompletionEnd(t *testing.T) {
	t.Parallel()
	day := timezone.NewDate(2026, 10, 9)
	cases := []struct {
		name        string
		instance    *scheduleModels.ActivityInstance
		completedAt time.Time
		want        string
		ok          bool
	}{
		{"completion clock", spontaneousBlock(1, day, 15, 16, true), berlinAt(day, 20, 32), "20:32", true},
		{"never before the start", spontaneousBlock(2, day, 15, 16, true), berlinAt(day, 15, 16), "15:17", true},
		{"later day ends at 23:59", spontaneousBlock(3, day, 22, 0, true), berlinAt(day.AddDays(1), 0, 30), "23:59", true},
		{"planned blocks keep their end", spontaneousBlock(4, day, 15, 16, false), berlinAt(day, 20, 32), "", false},
		{"no minute left after 23:59", spontaneousBlock(5, day, 23, 59, true), berlinAt(day, 23, 59), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			end, ok := spontaneousCompletionEnd(tc.instance, tc.completedAt)
			require.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, end.Format("15:04"))
			}
		})
	}
}

// Every session end (kiosk, timeout, nightly close) completes through this
// path, so it is the one place the spontaneous block learns its real end.
func TestEndedSessionCompletionRecordsSpontaneousEnd(t *testing.T) {
	t.Parallel()
	day := timezone.NewDate(2026, 10, 9)
	calls := make([]string, 0, 3)
	instances := &endedSessionInstancesStub{
		instances: []*scheduleModels.ActivityInstance{
			spontaneousBlock(7711, day, 15, 16, true),
			spontaneousBlock(7712, day, 15, 0, false),
		},
		completed: 2,
		calls:     &calls,
	}
	completion := newEndedSessionCompletion(t, EndedSessionCompletionDependencies{
		Instances:    instances,
		Participants: &endedSessionParticipantsStub{calls: &calls},
	})

	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{8811, 8812}, berlinAt(day, 20, 32))
	require.NoError(t, err)
	assert.EqualValues(t, 2, completed)
	require.Len(t, instances.updated, 1, "only the spontaneous block gets a new end")
	assert.Equal(t, int64(7711), instances.updated[0].instanceID)
	assert.Equal(t, []string{"end_time"}, instances.updated[0].columns)
	assert.Equal(t, "20:32", instances.updated[0].endTime.Format("15:04"))
}
