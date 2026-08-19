package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestQueueActivityUpdates_BroadcastsOnlyAfterCommit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &instanceService{deps: InstanceServiceDependencies{Broadcaster: broadcaster}}
	tenantID := tenant.FromContext(testpkg.Ctx(t))
	instance := &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(2026, 7, 15),
		StartTime: time.Date(1970, 1, 1, 9, 30, 0, 0, time.UTC),
	}
	instance.ID = 456

	err := tenant.WithTenantTx(context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		svc.QueueActivityUpdates(ctx, map[int64]*scheduleModel.ActivityInstance{123: instance})
		assert.Empty(t, broadcaster.Calls(), "event must not escape an uncommitted transaction")
		return nil
	})
	require.NoError(t, err)

	calls := broadcaster.GroupCallsForTopic("123")
	require.Len(t, calls, 1)
	assert.Equal(t, tenantID, calls[0].TenantID)
	assert.Equal(t, realtime.EventActivityUpdate, calls[0].Event.Type)
	require.NotNil(t, calls[0].Event.Data.InstanceID)
	assert.Equal(t, "456", *calls[0].Event.Data.InstanceID)
}

func TestQueueActivityUpdates_DropsEventOnRollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &instanceService{deps: InstanceServiceDependencies{Broadcaster: broadcaster}}
	tenantID := tenant.FromContext(testpkg.Ctx(t))
	instance := &scheduleModel.ActivityInstance{Date: timezone.NewDate(2026, 7, 15)}
	rollback := errors.New("rollback")

	err := tenant.WithTenantTx(context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		svc.QueueActivityUpdates(ctx, map[int64]*scheduleModel.ActivityInstance{123: instance})
		return rollback
	})

	require.ErrorIs(t, err, rollback)
	assert.Empty(t, broadcaster.Calls())
}
