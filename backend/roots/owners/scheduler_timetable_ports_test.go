package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// recordingMaterialization records the source of the materializations the
// Worker asks Timetable for.
type recordingMaterialization struct {
	timetable.MaterializationCapability
	sources []timetable.MaterializationSource
}

func (r *recordingMaterialization) MaterializeForTenant(_ context.Context, from, to calendar.Date, source timetable.MaterializationSource) (*timetable.MaterializationResult, error) {
	r.sources = append(r.sources, source)
	return &timetable.MaterializationResult{From: from, To: to, InstancesCreated: 7, CandidatesSkippedExisting: 3, CandidatesRaced: 2, DurationMS: 123}, nil
}

// The Worker's weekly run is the scheduler's materialization, not a manual
// one, and its counts reach the Worker's log.
func TestSchedulerMaterializerRunsAsTheScheduler(t *testing.T) {
	t.Parallel()

	capability := &recordingMaterialization{}
	materializer := newSchedulerMaterializer(capability)
	from := calendar.NewDate(2026, time.October, 19)

	run, err := materializer.MaterializeScheduledWindow(context.Background(), from, from.AddDays(20))

	require.NoError(t, err)
	assert.Equal(t, []timetable.MaterializationSource{timetable.MaterializationSourceScheduler}, capability.sources,
		"source tag must be scheduler (not manual) for scheduled runs")
	require.NotNil(t, run)
	assert.Equal(t, 7, run.InstancesCreated)
	assert.Equal(t, 3, run.CandidatesSkippedExisting)
	assert.Equal(t, 2, run.CandidatesRaced)
	assert.Equal(t, int64(123), run.DurationMS)
	assert.Nil(t, newSchedulerMaterializer(nil), "a missing owner leaves the port unset")
	assert.Nil(t, newSchedulerTimetableCleanup(nil))
	assert.Nil(t, newSchedulerAutoStart(nil))
	assert.Nil(t, newSchedulerAutoEnd(nil))
}
