package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// The "Unterricht fällt aus" preset (#2970): the first non-cancelled block
// of the date that addresses the class, matched on the template's own
// target, its dynamic targets, or its offering class filter — the same three
// fields the OGS dialog reads.

func classBlockInstance(id, groupID int64, start string, status string) *scheduleModel.ActivityInstance {
	parsed, err := time.Parse("15:04", start)
	if err != nil {
		panic(err)
	}
	return &scheduleModel.ActivityInstance{
		Model:           scheduleModel.Model{ID: id},
		Date:            timezone.NewDate(2026, 9, 7),
		ActivityGroupID: &groupID,
		StartTime:       parsed,
		Status:          status,
	}
}

func TestEarliestPlannedBlockStartForClassPicksTheFirstBlockThatAddressesTheClass(t *testing.T) {
	t.Parallel()
	deps := newTimetableOpsDeps()
	date := timezone.NewDate(2026, 9, 7)

	// Matching is LOWER(BTRIM(...)) like every school_class join: "4A" and
	// " 4a " are the same class, "Klasse 4a" is not.
	klasse4a := "4A"
	klasse3b := "3b"
	deps.activityGroups.byID[1] = &activitiesModel.Group{Model: activitiesModel.Model{ID: 1}, TargetSchoolClass: &klasse4a}
	deps.activityGroups.byID[2] = &activitiesModel.Group{Model: activitiesModel.Model{ID: 2}, TargetSchoolClass: &klasse3b}
	deps.activityGroups.byID[3] = &activitiesModel.Group{Model: activitiesModel.Model{ID: 3}}
	deps.activityGroups.targetsByGroup[3] = []*activitiesModel.GroupTarget{{TargetGroupType: activitiesModel.TargetGroupTypeKlasse, TargetSchoolClass: &klasse4a}}
	deps.activityGroups.byID[4] = &activitiesModel.Group{Model: activitiesModel.Model{ID: 4}, SourceSchoolClasses: []string{"4a"}}

	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		// Another class's earlier block must not win.
		classBlockInstance(10, 2, "11:00", scheduleModel.InstanceStatusPlanned),
		// A cancelled block of the class does not count.
		classBlockInstance(11, 1, "11:30", scheduleModel.InstanceStatusCancelled),
		classBlockInstance(12, 1, "13:15", scheduleModel.InstanceStatusPlanned),
		classBlockInstance(13, 3, "12:45", scheduleModel.InstanceStatusPlanned),
		classBlockInstance(14, 4, "14:00", scheduleModel.InstanceStatusPlanned),
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
	date := timezone.NewDate(2026, 9, 7)

	// Spontaneous blocks carry no template and therefore no class.
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{
		{Model: scheduleModel.Model{ID: 20}, Date: date, StartTime: time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC), Status: scheduleModel.InstanceStatusPlanned},
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

	_, err := deps.service.EarliestPlannedBlockStartForClass(context.Background(), "4a", timezone.NewDate(2026, 9, 7))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
