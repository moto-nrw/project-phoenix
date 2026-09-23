package httpintegration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Since the presence cutover (#2762) the retained activity-instance DTO is
// written by two owners: Timetable plans the block, Student Presence runs it.
// A DTO created in an execution state therefore issues one Timetable command
// and one or two Presence commands. They share the caller's tenant
// transaction, so an abort after the last command, or a Presence command
// that fails after the plan was written, leaves neither half behind.

func presenceCutoverInstance(t *testing.T, title string, activityID *int64, roomID, activeGroupID int64, startHour int) *scheduleModels.ActivityInstance {
	t.Helper()
	instance := buildInstance(testpkg.Tenant(t), roomID, activityID, scheduleModels.NewDate(2026, 9, 22),
		time.Date(2000, 1, 1, startHour, 0, 0, 0, time.UTC), time.Date(2000, 1, 1, startHour+1, 0, 0, 0, time.UTC), title)
	instance.Status = scheduleModels.InstanceStatusActive
	instance.ActiveGroupID = &activeGroupID
	return instance
}

// countPresenceCutoverRows counts both halves of the blocks titled so; a
// transaction sees its own uncommitted rows, the pool the committed ones.
func countPresenceCutoverRows(t *testing.T, db bun.IDB, ctx context.Context, title string) (instances, sessions int) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.activity_instances WHERE tenant_id = ? AND title = ?`, testpkg.Tenant(t), title).Scan(ctx, &instances))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_sessions AS session
		JOIN schedule.activity_instances AS instance ON instance.tenant_id = session.tenant_id AND instance.id = session.schedule_instance_id
		WHERE session.tenant_id = ? AND instance.title = ?`, testpkg.Tenant(t), title).Scan(ctx, &sessions))
	return instances, sessions
}

func TestActivityInstanceCreateInExecutionStateJoinsTheCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityInstance
	fx := newActivityInstanceFixtures(t, db, "cutover-uow")
	defer fx.cleanup()
	group := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

	abort := errors.New("abort after both owners wrote")
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		instance := presenceCutoverInstance(t, "UoW abort", &fx.activityID, fx.roomID, group.ID, 14)
		require.NoError(t, repo.Create(txCtx, instance))
		require.Equal(t, scheduleModels.InstanceStatusActive, instance.Status, "the reloaded DTO shows the execution")
		require.NotNil(t, instance.ActiveGroupID)
		require.Equal(t, group.ID, *instance.ActiveGroupID)
		instances, sessions := countPresenceCutoverRows(t, tx, txCtx, "UoW abort")
		require.Equal(t, 1, instances)
		require.Equal(t, 1, sessions, "inside the transaction both owners hold their half")
		return abort
	})
	require.ErrorIs(t, err, abort)
	instances, sessions := countPresenceCutoverRows(t, db, ctx, "UoW abort")
	assert.Zero(t, instances, "the plan rolled back with the caller")
	assert.Zero(t, sessions, "the session rolled back with the caller")
}

func TestActivityInstanceCreateRollsBackThePlanWhenTheSessionCannotStart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActivityInstance
	fx := newActivityInstanceFixtures(t, db, "cutover-failure")
	defer fx.cleanup()
	group := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)

	// The live group already runs one block; a second block cannot bind it.
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		return repo.Create(txCtx, presenceCutoverInstance(t, "Cutover first", &fx.activityID, fx.roomID, group.ID, 14))
	}))

	err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		return repo.Create(txCtx, presenceCutoverInstance(t, "Cutover second", &fx.activityID, fx.roomID, group.ID, 15))
	})
	require.Error(t, err, "the Presence command fails after the Timetable command")
	instances, sessions := countPresenceCutoverRows(t, db, ctx, "Cutover first")
	assert.Equal(t, 1, instances)
	assert.Equal(t, 1, sessions)
	instances, sessions = countPresenceCutoverRows(t, db, ctx, "Cutover second")
	assert.Zero(t, instances, "the plan written before the failing command is gone")
	assert.Zero(t, sessions)

	// The retry with a free live group succeeds: nothing half-written remains.
	other := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		return repo.Create(txCtx, presenceCutoverInstance(t, "Cutover second", &fx.activityID, fx.roomID, other.ID, 15))
	}))
	instances, sessions = countPresenceCutoverRows(t, db, ctx, "Cutover second")
	assert.Equal(t, 1, instances)
	assert.Equal(t, 1, sessions)
}
