package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type legacyWeekendCleanerTestRepo struct {
	ports.ActivityInstanceRecords
	calls     int
	template  int64
	weekdays  []int
	deleted   int64
	deleteErr error
}

func (r *legacyWeekendCleanerTestRepo) DeletePlannedMaterializedWeekendInstances(_ context.Context, templateID int64, weekdays []int) (int64, error) {
	r.calls++
	r.template = templateID
	r.weekdays = append([]int(nil), weekdays...)
	return r.deleted, r.deleteErr
}

func TestDeleteRemovedLegacyWeekendInstances(t *testing.T) {
	t.Parallel()

	templateID := int64(921)
	previous := []*activityContract.Schedule{
		{Weekday: activityContract.WeekdayFriday},
		{Weekday: activityContract.WeekdaySaturday},
		{Weekday: activityContract.WeekdaySunday},
	}

	t.Run("deletes only removed legacy weekend days", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{}
		svc := NewTemplateService(TemplateServiceDependencies{ActivityInstanceRepo: repo})

		err := svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activityContract.WeekdayFriday, activityContract.WeekdaySunday})

		require.NoError(t, err)
		assert.Equal(t, 1, repo.calls)
		assert.Equal(t, templateID, repo.template)
		assert.Equal(t, []int{activityContract.WeekdaySaturday}, repo.weekdays)
	})

	t.Run("broadcasts after deleting materialized instances", func(t *testing.T) {
		announcer := &recordingStaffingAnnouncer{}
		repo := &legacyWeekendCleanerTestRepo{deleted: 2}
		svc := NewTemplateService(TemplateServiceDependencies{
			ActivityInstanceRepo: repo,
			Staffing:             announcer,
			Logger:               slog.Default(),
		})
		ctx := tenant.WithTenantID(context.Background(), 314)

		require.NoError(t, svc.deleteRemovedLegacyWeekendInstances(ctx, templateID, previous, []int{activityContract.WeekdayFriday}))

		require.Equal(t, []staffingAnnouncement{{tenantID: 314, source: "template_legacy_weekend_cleanup"}}, announcer.calls)
	})

	t.Run("skips cleanup when all legacy days remain", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{}
		svc := NewTemplateService(TemplateServiceDependencies{ActivityInstanceRepo: repo})

		require.NoError(t, svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activityContract.WeekdayFriday, activityContract.WeekdaySaturday, activityContract.WeekdaySunday}))
		assert.Zero(t, repo.calls)
	})

	t.Run("wraps cleanup failures", func(t *testing.T) {
		repo := &legacyWeekendCleanerTestRepo{deleteErr: errors.New("delete failed")}
		svc := NewTemplateService(TemplateServiceDependencies{ActivityInstanceRepo: repo})

		err := svc.deleteRemovedLegacyWeekendInstances(context.Background(), templateID, previous, []int{activityContract.WeekdayFriday})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete removed legacy weekend instances")
	})
}

func TestValidateLegacyTemplateWeekdays(t *testing.T) {
	t.Parallel()

	existing := []*activityContract.Schedule{
		{Weekday: activityContract.WeekdayFriday},
		{Weekday: activityContract.WeekdaySaturday},
	}

	assert.NoError(t, validateLegacyTemplateWeekdays(existing, []int{
		activityContract.WeekdayFriday,
		activityContract.WeekdaySaturday,
	}))
	assert.ErrorIs(t, validateLegacyTemplateWeekdays(existing, []int{
		activityContract.WeekdayFriday,
		activityContract.WeekdaySunday,
	}), timetable.ErrTemplateWeekendWeekday)
}
