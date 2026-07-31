package schedule

import (
	"context"
	"errors"
	"testing"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type legacyWeekendCleanerTestRepo struct {
	scheduleModel.ActivityInstanceRepository
	calls     int
	template  int64
	weekdays  []int
	deleteErr error
}

func (r *legacyWeekendCleanerTestRepo) DeletePlannedMaterializedWeekendInstances(_ context.Context, templateID int64, weekdays []int) (int64, error) {
	r.calls++
	r.template = templateID
	r.weekdays = append([]int(nil), weekdays...)
	return 0, r.deleteErr
}

func TestDeleteRemovedLegacyWeekendInstances(t *testing.T) {
	templateID := int64(921)
	previous := []*activitiesModel.Schedule{
		{Weekday: activitiesModel.WeekdayFriday},
		{Weekday: activitiesModel.WeekdaySaturday},
		{Weekday: activitiesModel.WeekdaySunday},
	}

	t.Run("deletes only removed legacy weekend days", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{}
		svc := NewTimetableDataService(TimetableDataDependencies{ActivityInstanceRepo: repo})

		err := svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activitiesModel.WeekdayFriday, activitiesModel.WeekdaySunday})

		require.NoError(t, err)
		assert.Equal(t, 1, repo.calls)
		assert.Equal(t, templateID, repo.template)
		assert.Equal(t, []int{activitiesModel.WeekdaySaturday}, repo.weekdays)
	})

	t.Run("skips cleanup when all legacy days remain", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{}
		svc := NewTimetableDataService(TimetableDataDependencies{ActivityInstanceRepo: repo})

		require.NoError(t, svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activitiesModel.WeekdayFriday, activitiesModel.WeekdaySaturday, activitiesModel.WeekdaySunday}))
		assert.Zero(t, repo.calls)
	})

	t.Run("wraps cleanup failures", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{deleteErr: errors.New("delete failed")}
		svc := NewTimetableDataService(TimetableDataDependencies{ActivityInstanceRepo: repo})

		err := svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activitiesModel.WeekdayFriday})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete removed legacy weekend instances")
	})
}
