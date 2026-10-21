package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Every session end (kiosk, timeout, nightly close) completes through this
// path, so it is the one place the spontaneous block learns its real end.
func TestEndedSessionCompletionRecordsSpontaneousEnd(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 10, 9)
	calls := make([]string, 0, 3)
	instances := &endedSessionInstancesStub{
		instances: []*scheduleModels.ActivityInstance{
			spontaneousBlock(7711, day, 15, 16, true),
			spontaneousBlock(7712, day, 15, 0, false),
		},
		completed: 2,
		calls:     &calls,
	}
	completion := newEndedSessionCompletion(t, ports.EndedSessionCompletionDependencies{
		Instances:    fakeInstanceRows(instances),
		Participants: legacyParticipantRows(&endedSessionParticipantsStub{calls: &calls}),
	})

	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{8811, 8812}, berlinAt(day, 20, 32))
	require.NoError(t, err)
	assert.EqualValues(t, 2, completed)
	require.Len(t, instances.updated, 1, "only the spontaneous block gets a new end")
	assert.Equal(t, int64(7711), instances.updated[0].instanceID)
	assert.Equal(t, []string{"end_time"}, instances.updated[0].columns)
	assert.Equal(t, "20:32", instances.updated[0].endTime.Format("15:04"))
}

func TestEndedSessionCompletionKeepsExistingSpontaneousEnd(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 10, 9)
	instances := &endedSessionInstancesStub{
		instances: []*scheduleModels.ActivityInstance{
			spontaneousBlock(7713, day, 15, 16, true),
		},
		calls: &[]string{},
	}
	completion := newEndedSessionCompletion(t, ports.EndedSessionCompletionDependencies{
		Instances:    fakeInstanceRows(instances),
		Participants: legacyParticipantRows(&endedSessionParticipantsStub{calls: &[]string{}}),
	})

	completed, err := completion.CompleteActiveByActiveGroupIDs(context.Background(), []int64{8813}, berlinAt(day, 21, 7))
	require.NoError(t, err)
	assert.Zero(t, completed)
	assert.Empty(t, instances.updated, "an already completed session keeps its recorded end")
}
