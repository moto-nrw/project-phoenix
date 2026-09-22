package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
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

func presenceCutoverInstance(t *testing.T, db *bun.DB, title string, activityID *int64, roomID, activeGroupID int64, startHour int) *scheduleModels.ActivityInstance {
	t.Helper()
	instance := &scheduleModels.ActivityInstance{
		Date: scheduleModels.NewDate(2026, 9, 22), ActivityGroupID: activityID, Title: title, RoomID: roomID,
		StartTime: time.Date(2000, 1, 1, startHour, 0, 0, 0, time.UTC), EndTime: time.Date(2000, 1, 1, startHour+1, 0, 0, 0, time.UTC),
		Status: scheduleModels.InstanceStatusActive, ActiveGroupID: &activeGroupID,
	}
	instance.SetTenantID(testpkg.Tenant(t))
	return instance
}

func countInstancesTitled(t *testing.T, db *bun.DB, ctx context.Context, title string) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.activity_instances WHERE tenant_id = ? AND title = ?`, testpkg.Tenant(t), title).Scan(ctx, &count))
	return count
}

func TestActivityInstanceCreateInExecutionStateJoinsTheCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	presence, err := repositories.NewStudentPresence(db, func(presenceCompose.Observation) {})
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, "Cutover UoW")
	room := testpkg.CreateTestRoom(t, db, "Cutover UoW")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	abort := errors.New("abort after both owners wrote")
	var createdID int64
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		instance := presenceCutoverInstance(t, db, "UoW abort", &activity.ID, room.ID, group.ID, 14)
		require.NoError(t, factory.ActivityInstance.Create(txCtx, instance))
		createdID = instance.ID
		require.Equal(t, scheduleModels.InstanceStatusActive, instance.Status, "the reloaded DTO shows the execution")
		session, err := presence.FindActivitySession(txCtx, instance.ID)
		require.NoError(t, err)
		require.Equal(t, group.ID, *session.ActiveGroupID)
		return abort
	})
	require.ErrorIs(t, err, abort)
	assert.Zero(t, countInstancesTitled(t, db, ctx, "UoW abort"), "the plan rolled back with the caller")
	_, err = presence.FindActivitySession(ctx, createdID)
	require.ErrorIs(t, err, studentpresence.ErrActivitySessionNotFound, "the session rolled back with the caller")
}

func TestActivityInstanceCreateRollsBackThePlanWhenTheSessionCannotStart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	activity := testpkg.CreateTestActivityGroup(t, db, "Cutover failure")
	room := testpkg.CreateTestRoom(t, db, "Cutover failure")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	// The live group already runs one block; a second block cannot bind it.
	first := presenceCutoverInstance(t, db, "Cutover first", &activity.ID, room.ID, group.ID, 14)
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return factory.ActivityInstance.Create(txCtx, first)
	}))

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		second := presenceCutoverInstance(t, db, "Cutover second", &activity.ID, room.ID, group.ID, 15)
		return factory.ActivityInstance.Create(txCtx, second)
	})
	require.Error(t, err, "the Presence command fails after the Timetable command")
	assert.Equal(t, 1, countInstancesTitled(t, db, ctx, "Cutover first"))
	assert.Zero(t, countInstancesTitled(t, db, ctx, "Cutover second"), "the plan written before the failing command is gone")

	// The retry with a free live group succeeds: nothing half-written remains.
	other := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return factory.ActivityInstance.Create(txCtx, presenceCutoverInstance(t, db, "Cutover second", &activity.ID, room.ID, other.ID, 15))
	}))
	assert.Equal(t, 1, countInstancesTitled(t, db, ctx, "Cutover second"))
}
