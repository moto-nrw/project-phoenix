package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The "Unterricht fällt aus" preset (#2970): the first non-cancelled block
// of the date that addresses the class, matched on the template's own
// target, its dynamic targets, or its offering class filter — the same three
// fields the OGS dialog reads.

func classBlockInstance(id, groupID int64, start string, status string) *scheduleModels.ActivityInstance {
	parsed, err := time.Parse("15:04", start)
	if err != nil {
		panic(err)
	}
	return &scheduleModels.ActivityInstance{
		Model:           scheduleModels.Model{ID: id},
		Date:            scheduleModels.NewDate(2026, 9, 7),
		ActivityGroupID: &groupID,
		StartTime:       parsed,
		Status:          status,
	}
}

func TestEarliestPlannedBlockStartForClassPicksTheFirstBlockThatAddressesTheClass(t *testing.T) {
	t.Parallel()
	deps := newTimetableOpsDeps()
	date := calendar.NewDate(2026, 9, 7)

	// Matching is LOWER(BTRIM(...)) like every school_class join: "4A" and
	// " 4a " are the same class, "Klasse 4a" is not.
	klasse4a := "4A"
	klasse3b := "3b"
	deps.activityGroups.byID[1] = &activitiesModels.Group{Model: activitiesModels.Model{ID: 1}, TargetSchoolClass: &klasse4a}
	deps.activityGroups.byID[2] = &activitiesModels.Group{Model: activitiesModels.Model{ID: 2}, TargetSchoolClass: &klasse3b}
	deps.activityGroups.byID[3] = &activitiesModels.Group{Model: activitiesModels.Model{ID: 3}}
	deps.activityGroups.targetsByGroup[3] = []*activitiesModels.GroupTarget{{TargetGroupType: activitiesModels.TargetGroupTypeKlasse, TargetSchoolClass: &klasse4a}}
	deps.activityGroups.byID[4] = &activitiesModels.Group{Model: activitiesModels.Model{ID: 4}, SourceSchoolClasses: []string{"4a"}}

	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		// Another class's earlier block must not win.
		classBlockInstance(10, 2, "11:00", scheduleModels.InstanceStatusPlanned),
		// A cancelled block of the class does not count.
		classBlockInstance(11, 1, "11:30", scheduleModels.InstanceStatusCancelled),
		classBlockInstance(12, 1, "13:15", scheduleModels.InstanceStatusPlanned),
		classBlockInstance(13, 3, "12:45", scheduleModels.InstanceStatusPlanned),
		classBlockInstance(14, 4, "14:00", scheduleModels.InstanceStatusPlanned),
	}

	start, err := deps.service.EarliestPlannedBlockStartForClass(context.Background(), " 4A ", date)
	require.NoError(t, err)
	assert.Equal(t, "12:45", start, "dynamic klasse target counts, cancelled block and other classes do not")

	start, err = deps.service.EarliestPlannedBlockStartForClass(context.Background(), "3b", date)
	require.NoError(t, err)
	assert.Equal(t, "11:00", start)
}

func TestEarliestPlannedBlockStartForClassIsEmptyWithoutABlock(t *testing.T) {
	t.Parallel()
	deps := newTimetableOpsDeps()
	date := calendar.NewDate(2026, 9, 7)

	// Spontaneous blocks carry no template and therefore no class.
	deps.instanceRepo.byDate = []*scheduleModels.ActivityInstance{
		{Model: scheduleModels.Model{ID: 20}, Date: scheduleModels.Date(date), StartTime: time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC), Status: scheduleModels.InstanceStatusPlanned},
	}

	start, err := deps.service.EarliestPlannedBlockStartForClass(context.Background(), "4a", date)
	require.NoError(t, err)
	assert.Empty(t, start)

	start, err = deps.service.EarliestPlannedBlockStartForClass(context.Background(), "   ", date)
	require.NoError(t, err)
	assert.Empty(t, start, "an empty class never matches anything")
}

func TestEarliestPlannedBlockStartForClassSurfacesRepositoryErrors(t *testing.T) {
	t.Parallel()
	deps := newTimetableOpsDeps()
	deps.instanceRepo.findByDateErr = errors.New("boom")

	_, err := deps.service.EarliestPlannedBlockStartForClass(context.Background(), "4a", calendar.NewDate(2026, 9, 7))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
