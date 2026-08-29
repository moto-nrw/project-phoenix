package config

import (
	"context"
	"errors"
	"testing"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type broadcastWorkTimeModelRepo struct {
	configModels.WorkTimeModelRepository
	updated    bool
	refreshed  bool
	refreshErr error
}

func (r *broadcastWorkTimeModelRepo) Update(context.Context, *configModels.WorkTimeModel, []*configModels.WorkTimeModelEntry) error {
	r.updated = true
	return nil
}

func (r *broadcastWorkTimeModelRepo) RefreshAssignedStaffSchedules(context.Context, int64) error {
	r.refreshed = true
	return r.refreshErr
}

func (r *broadcastWorkTimeModelRepo) FindByID(_ context.Context, id int64) (*configModels.WorkTimeModel, error) {
	return &configModels.WorkTimeModel{ID: id}, nil
}

func TestWorkTimeModelUpdateBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	t.Parallel()

	repo := &broadcastWorkTimeModelRepo{}
	service := NewWorkTimeModelService(repo)
	notified := false
	service.SetChangeNotifier(func(context.Context) { notified = true })
	model := &configModels.WorkTimeModel{
		ID:                 7,
		Name:               "Vollzeit",
		RotationLength:     1,
		RotationAnchorDate: configModels.CalendarDate{Year: 2026, Month: time.July, Day: 1},
	}

	updated, err := service.UpdateModel(context.Background(), model, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, repo.updated)
	assert.True(t, repo.refreshed)
	assert.True(t, notified)
}

func TestWorkTimeModelUpdateDoesNotNotifyWhenRefreshFails(t *testing.T) {
	sentinel := errors.New("refresh failed")
	repo := &broadcastWorkTimeModelRepo{refreshErr: sentinel}
	service := NewWorkTimeModelService(repo)
	notified := false
	service.SetChangeNotifier(func(context.Context) { notified = true })
	model := &configModels.WorkTimeModel{
		ID:                 7,
		Name:               "Vollzeit",
		RotationLength:     1,
		RotationAnchorDate: configModels.CalendarDate{Year: 2026, Month: time.July, Day: 1},
	}

	_, err := service.UpdateModel(context.Background(), model, nil)
	require.ErrorIs(t, err, sentinel)
	assert.False(t, notified)
}
