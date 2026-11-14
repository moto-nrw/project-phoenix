package grouplive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func activitiesOn(context.Context) (bool, error) { return true, nil }

// plannedActivityFakes serves a group with two children and records which
// children the timetable was asked for.
func plannedActivityFakes(caller func(context.Context) (Caller, error), restricted bool, asked *[][]int64, activities map[int64][]PlannedActivity) *fakes {
	return &fakes{
		caller:           caller,
		restricted:       restricted,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		members: func(context.Context, int64) ([]RosterStudent, error) {
			return []RosterStudent{{ID: 11, FirstName: "Ada"}, {ID: 12, FirstName: "Bob"}}, nil
		},
		activitiesEnabled: activitiesOn,
		activities: func(_ context.Context, ids []int64, date Date) (map[int64][]PlannedActivity, error) {
			*asked = append(*asked, ids)
			if date != today {
				return nil, errors.New("asked for another day")
			}
			return activities, nil
		},
	}
}

// TestLiveGroupAttachesPlannedActivities pins that today's planned offerings
// of a child reach its card (#6487) when the school switched them on, and
// only with planning access, like arrival and pickup.
func TestLiveGroupAttachesPlannedActivities(t *testing.T) {
	t.Parallel()
	pottery := PlannedActivity{Title: "Töpfer-AG", StartTime: "14:00", EndTime: "15:00"}
	handball := PlannedActivity{Title: "Handball", StartTime: "15:15", EndTime: "16:00"}
	activities := map[int64][]PlannedActivity{11: {pottery, handball}}

	var asked [][]int64
	got, err := newService(plannedActivityFakes(fullCaller, false, &asked, activities)).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got.Students, 2)
	assert.Equal(t, []PlannedActivity{pottery, handball}, got.Students[0].PlannedActivities)
	assert.Empty(t, got.Students[1].PlannedActivities)
	assert.Equal(t, [][]int64{{11, 12}}, asked, "one read for the whole group")

	for name, f := range map[string]func(*[][]int64) *fakes{
		"without users:read": func(asked *[][]int64) *fakes {
			return plannedActivityFakes(func(context.Context) (Caller, error) { return Caller{CanReadGroups: true}, nil }, false, asked, activities)
		},
		"without staff eligibility": func(asked *[][]int64) *fakes {
			return plannedActivityFakes(fullCaller, true, asked, activities)
		},
		"switched off": func(asked *[][]int64) *fakes {
			f := plannedActivityFakes(fullCaller, false, asked, activities)
			f.activitiesEnabled = func(context.Context) (bool, error) { return false, nil }
			return f
		},
	} {
		var asked [][]int64
		got, err := newService(f(&asked)).LiveGroup(context.Background(), 1)
		require.NoError(t, err, name)
		for _, student := range got.Students {
			assert.Empty(t, student.PlannedActivities, name)
		}
		assert.Empty(t, asked, "%s: the timetable is not asked", name)
	}
}

// TestLiveGroupPlannedActivitiesFailures pins that a failed read fails the
// projection instead of silently hiding the offerings.
func TestLiveGroupPlannedActivitiesFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("unavailable")

	var asked [][]int64
	f := plannedActivityFakes(fullCaller, false, &asked, nil)
	f.activities = func(context.Context, []int64, Date) (map[int64][]PlannedActivity, error) { return nil, failure }
	_, err := newService(f).LiveGroup(context.Background(), 1)
	require.ErrorIs(t, err, failure)

	f = plannedActivityFakes(fullCaller, false, &asked, nil)
	f.activitiesEnabled = func(context.Context) (bool, error) { return false, failure }
	_, err = newService(f).LiveGroup(context.Background(), 1)
	require.ErrorIs(t, err, failure)
}
