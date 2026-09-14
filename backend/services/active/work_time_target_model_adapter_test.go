package active_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/stretchr/testify/require"
)

type targetModelRecords struct {
	row *config.WorkTimeModel
	err error
}

func (r targetModelRecords) FindByID(context.Context, int64) (*config.WorkTimeModel, error) {
	return r.row, r.err
}
func (r targetModelRecords) FindByIDs(context.Context, []int64) ([]*config.WorkTimeModel, error) {
	return []*config.WorkTimeModel{nil, r.row}, r.err
}

func TestWorkTimeTargetModelPreservesRotationAndPartialReadError(t *testing.T) {
	t.Parallel()
	anchor := timezone.NewDate(2026, 3, 23)
	row := &config.WorkTimeModel{ID: 7, RotationLength: 2, RotationAnchorDate: config.CalendarDate(anchor), Entries: []*config.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 360},
		{WeekIndex: 1, DayOfWeek: 0, TargetMinutes: 420},
	}}
	readErr := errors.New("partial model read")
	reader := services.NewWorkTimeTargetModels(targetModelRecords{row: row, err: readErr})
	model, err := reader.FindByID(context.Background(), row.ID)
	require.ErrorIs(t, err, readErr)
	require.Equal(t, row.ID, model.ID)
	require.Equal(t, anchor, model.RotationAnchorDate)
	for _, test := range []struct {
		date   timezone.Date
		target int
	}{
		{anchor.AddDays(-7), 420}, {anchor, 360}, {anchor.AddDays(7), 420}, {anchor.AddDays(14), 360},
	} {
		target, matched := model.DailyTarget(anchor, test.date)
		require.True(t, matched)
		require.Equal(t, test.target, target)
	}
	models, err := reader.FindByIDs(context.Background(), []int64{row.ID})
	require.ErrorIs(t, err, readErr)
	require.Len(t, models, 2)
	require.Nil(t, models[0])
	require.Equal(t, row.ID, models[1].ID)
}
