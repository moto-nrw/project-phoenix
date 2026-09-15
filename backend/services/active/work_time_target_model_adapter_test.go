package active_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/config/settingstest"
	"github.com/stretchr/testify/require"
)

func TestWorkTimeTargetModelPreservesRotationAndPartialReadError(t *testing.T) {
	t.Parallel()
	anchor := timezone.NewDate(2026, 3, 23)
	template := settingstest.Template{ID: 7, RotationLength: 2, Anchor: anchor, Entries: []settingstest.TemplateEntry{
		{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 360},
		{WeekIndex: 1, DayOfWeek: 0, TargetMinutes: 420},
	}}
	readErr := errors.New("partial model read")
	reader := services.NewWorkTimeTargetModels(settingstest.Templates(template, readErr))
	model, err := reader.FindByID(context.Background(), template.ID)
	require.ErrorIs(t, err, readErr)
	require.Equal(t, template.ID, model.ID)
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
	models, err := reader.FindByIDs(context.Background(), []int64{template.ID})
	require.ErrorIs(t, err, readErr)
	require.Len(t, models, 2)
	require.Nil(t, models[0])
	require.Equal(t, template.ID, models[1].ID)
}
