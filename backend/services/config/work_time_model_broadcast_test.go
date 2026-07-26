package config

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type broadcastWorkTimeModelRepo struct {
	configModels.WorkTimeModelRepository
	updated   bool
	refreshed bool
}

func (r *broadcastWorkTimeModelRepo) Update(context.Context, *configModels.WorkTimeModel, []*configModels.WorkTimeModelEntry) error {
	r.updated = true
	return nil
}

func (r *broadcastWorkTimeModelRepo) RefreshAssignedStaffSchedules(context.Context, int64) error {
	r.refreshed = true
	return nil
}

func (r *broadcastWorkTimeModelRepo) FindByID(_ context.Context, id int64) (*configModels.WorkTimeModel, error) {
	return &configModels.WorkTimeModel{ID: id}, nil
}

func TestWorkTimeModelUpdateBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	repo := &broadcastWorkTimeModelRepo{}
	service := NewWorkTimeModelService(repo)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service.SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)
	model := &configModels.WorkTimeModel{
		ID:                 7,
		Name:               "Vollzeit",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.July, 1),
	}

	updated, err := service.UpdateModel(ctx, model, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, repo.updated)
	assert.True(t, repo.refreshed)
	assert.Empty(t, broadcaster.Events())

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestWorkTimeModelUpdateLogsBroadcastFailureAfterCommit(t *testing.T) {
	repo := &broadcastWorkTimeModelRepo{}
	service := NewWorkTimeModelService(repo)
	broadcaster := testpkg.NewRecordingBroadcaster()
	broadcaster.Err = errors.New("hub unavailable")
	service.SetBroadcaster(broadcaster)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)
	model := &configModels.WorkTimeModel{
		ID:                 7,
		Name:               "Vollzeit",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.July, 1),
	}

	_, err := service.UpdateModel(ctx, model, nil)
	require.NoError(t, err)
	assert.Empty(t, logs.String())

	commit()

	logOutput := logs.String()
	assert.Contains(t, logOutput, "SSE staff time-tracking broadcast failed")
	assert.Contains(t, logOutput, "hub unavailable")
	assert.Contains(t, logOutput, "tenant_id=42")
}
