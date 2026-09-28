package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestQueueActivityUpdates_BroadcastsOnlyAfterCommit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	broadcaster := &lifecycleEventRecorder{}
	svc := &InstanceLifecycleService{deps: InstanceLifecycleDependencies{Broadcaster: broadcaster}}
	tenantID := tenant.FromContext(testpkg.Ctx(t))
	touched := timetable.TouchedActivities{123: {
		InstanceID: 456,
		Date:       timezone.NewDate(2026, 7, 15),
		StartTime:  time.Date(1970, 1, 1, 9, 30, 0, 0, time.UTC),
	}}

	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		svc.QueueActivityUpdates(ctx, touched)
		assert.Empty(t, broadcaster.Calls(), "event must not escape an uncommitted transaction")
		return nil
	})
	require.NoError(t, err)

	calls := broadcaster.GroupCallsForTopic("123")
	require.Len(t, calls, 1)
	assert.Equal(t, tenantID, calls[0].TenantID)
	assert.Equal(t, LifecycleEventActivityUpdate, calls[0].Event.Type)
	require.NotNil(t, calls[0].Event.InstanceID)
	assert.Equal(t, "456", *calls[0].Event.InstanceID)
}

func TestQueueActivityUpdates_DropsEventOnRollback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	broadcaster := &lifecycleEventRecorder{}
	svc := &InstanceLifecycleService{deps: InstanceLifecycleDependencies{Broadcaster: broadcaster}}
	tenantID := tenant.FromContext(testpkg.Ctx(t))
	touched := timetable.TouchedActivities{123: {Date: timezone.NewDate(2026, 7, 15)}}
	rollback := errors.New("rollback")

	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		svc.QueueActivityUpdates(ctx, touched)
		return rollback
	})

	require.ErrorIs(t, err, rollback)
	assert.Empty(t, broadcaster.Calls())
}
