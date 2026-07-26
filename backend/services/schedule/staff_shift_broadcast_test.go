package schedule

import (
	"context"
	"errors"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffShiftCreateBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	service := NewStaffShiftService(
		&shiftMockRepo{},
		&shiftMockStaffRepo{},
		nil,
		nil,
		nil,
	).(*staffShiftService)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service.SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	_, err := service.CreateShift(ctx, validShift(7))
	require.NoError(t, err)
	assert.Empty(t, broadcaster.Events())

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestStaffShiftCreateDoesNotBroadcastAfterFailure(t *testing.T) {
	service := NewStaffShiftService(
		&shiftMockRepo{createFunc: func(context.Context, *scheduleModels.StaffShift) error {
			return errors.New("insert failed")
		}},
		&shiftMockStaffRepo{},
		nil,
		nil,
		nil,
	).(*staffShiftService)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service.SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	_, err := service.CreateShift(ctx, validShift(7))
	require.Error(t, err)

	commit()
	assert.Empty(t, broadcaster.Events())
}

func TestStaffShiftSeriesCreateBroadcastsTimeTrackingChangeAfterCommit(t *testing.T) {
	service := newSeriesServiceForTest(seriesServiceMocks{}).(*staffShiftSeriesService)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service.SetBroadcaster(broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	_, err := service.CreateSeries(ctx, unitSeries(t))
	require.NoError(t, err)
	assert.Empty(t, broadcaster.Events())

	commit()

	events := broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged)
	require.Len(t, events, 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}
