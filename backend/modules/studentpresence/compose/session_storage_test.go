package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The presence cutover (#2762) made active.activity_sessions and
// active.activity_session_attendance the authoritative storage of a block's
// execution and of its participants' attendance. The tests below hold the
// owner to the cutover contract: one session per block, tenant-isolated
// reads and writes on both targets, and writes that join the caller's
// transaction so a failure after any command rolls all of them back.

type sessionStorageFixture struct {
	module      *studentpresence.Module
	instanceID  int64
	groupID     int64
	participant int64
}

func newSessionStorageFixture(t *testing.T, db *bun.DB, title string) sessionStorageFixture {
	t.Helper()
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, db, title)
	room := testpkg.CreateTestRoom(t, db, title)
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2026, 9, 22), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activity.ID, Title: title})
	student := testpkg.CreateTestStudent(t, db, "Session", title, "2a")
	participant := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "")
	return sessionStorageFixture{module: module, instanceID: instance.ID, groupID: group.ID, participant: participant.ID}
}

func TestActivitySessionStartsOncePerBlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newSessionStorageFixture(t, db, "Once")
	startedAt := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)

	session, err := f.module.StartActivitySession(ctx, studentpresence.ActivitySessionStart{InstanceID: f.instanceID, ActiveGroupID: f.groupID, StartedAt: startedAt})
	require.NoError(t, err)
	assert.Equal(t, studentpresence.ActivitySessionActive, session.Status)
	assert.Equal(t, f.instanceID, session.InstanceID)

	_, err = f.module.StartActivitySession(ctx, studentpresence.ActivitySessionStart{InstanceID: f.instanceID, ActiveGroupID: f.groupID, StartedAt: startedAt})
	require.ErrorIs(t, err, studentpresence.ErrActivitySessionExists, "the second start of the same block is refused")

	found, err := f.module.FindActivitySession(ctx, f.instanceID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, found.ID)
	completed, notScheduled, err := sessionExecution(ctx, f.module, []int64{f.instanceID}, []int64{f.participant})
	require.NoError(t, err)
	assert.Empty(t, completed, "a running block has not ended")
	assert.Empty(t, notScheduled)
}

func sessionExecution(ctx context.Context, module *studentpresence.Module, instanceIDs, participantIDs []int64) ([]int64, []int64, error) {
	result, err := module.SessionExecution(ctx, studentpresence.SessionExecutionFilter{InstanceIDs: instanceIDs, ParticipantIDs: participantIDs})
	return result.CompletedInstanceIDs, result.NotScheduledParticipantIDs, err
}

func TestSessionStorageIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	own := newSessionStorageFixture(t, db, "Own")
	at := time.Date(2026, 9, 22, 14, 5, 0, 0, time.UTC)
	_, err := own.module.StartActivitySession(ctx, studentpresence.ActivitySessionStart{InstanceID: own.instanceID, ActiveGroupID: own.groupID, StartedAt: at})
	require.NoError(t, err)
	rows, err := own.module.CheckInParticipants(ctx, []int64{own.participant}, at)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)

	var foreignCtx context.Context
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreignCtx = testpkg.Ctx(t)
		newSessionStorageFixture(t, db, "Foreign")
	})

	// Reads of the other school's block see nothing.
	_, err = own.module.FindActivitySession(foreignCtx, own.instanceID)
	require.ErrorIs(t, err, studentpresence.ErrActivitySessionNotFound)
	sessions, err := own.module.ListActivitySessions(foreignCtx, studentpresence.ActivitySessionFilter{InstanceIDs: []int64{own.instanceID}})
	require.NoError(t, err)
	assert.Empty(t, sessions)
	attendance, err := own.module.ListSessionAttendance(foreignCtx, []int64{own.participant})
	require.NoError(t, err)
	assert.Empty(t, attendance)
	completed, notScheduled, err := sessionExecution(foreignCtx, own.module, []int64{own.instanceID}, []int64{own.participant})
	require.NoError(t, err)
	assert.Empty(t, completed)
	assert.Empty(t, notScheduled)

	// Writes against the other school's rows change nothing there.
	rows, err = own.module.CheckOutParticipants(foreignCtx, []int64{own.participant}, at.Add(time.Hour))
	require.NoError(t, err)
	assert.Zero(t, rows)
	_, err = own.module.MarkParticipantsNotScheduled(foreignCtx, []int64{own.participant})
	require.Error(t, err, "the tenant-scoped foreign key refuses a marker row for a participant of another school")
	_, err = own.module.CompleteActivitySession(foreignCtx, studentpresence.ActivitySessionCompletion{InstanceID: own.instanceID, CompletedAt: at.Add(time.Hour)})
	require.Error(t, err, "the other school cannot end a block it cannot see")
	err = own.module.PatchSessionAttendance(foreignCtx, own.participant, studentpresence.SessionAttendancePatch{Status: ptr("absent")})
	require.Error(t, err, "RLS refuses an attendance row for a participant of another school")

	kept, err := own.module.ListSessionAttendance(ctx, []int64{own.participant})
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, "present", kept[0].Status)
	assert.Nil(t, kept[0].CheckedOutAt)
	assert.False(t, kept[0].NotScheduled)
	session, err := own.module.FindActivitySession(ctx, own.instanceID)
	require.NoError(t, err)
	assert.Equal(t, studentpresence.ActivitySessionActive, session.Status)
}

func ptr[T any](value T) *T { return &value }

func TestSessionStorageWritesRollBackWithTheCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newSessionStorageFixture(t, db, "Rollback")
	at := time.Date(2026, 9, 22, 14, 10, 0, 0, time.UTC)

	abort := errors.New("abort after the owner commands")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := f.module.StartActivitySession(txCtx, studentpresence.ActivitySessionStart{InstanceID: f.instanceID, ActiveGroupID: f.groupID, StartedAt: at})
		require.NoError(t, err)
		rows, err := f.module.CheckInParticipants(txCtx, []int64{f.participant}, at)
		require.NoError(t, err)
		require.EqualValues(t, 1, rows)
		require.NoError(t, f.module.PatchSessionAttendance(txCtx, f.participant, studentpresence.SessionAttendancePatch{Note: ptr("bis 15 Uhr")}))
		_, err = f.module.CompleteActivitySession(txCtx, studentpresence.ActivitySessionCompletion{InstanceID: f.instanceID, CompletedAt: at.Add(time.Hour)})
		require.NoError(t, err)
		completed, _, err := sessionExecution(txCtx, f.module, []int64{f.instanceID}, nil)
		require.NoError(t, err)
		require.Equal(t, []int64{f.instanceID}, completed, "inside the transaction the block has ended")
		return abort
	})
	require.ErrorIs(t, err, abort)

	_, err = f.module.FindActivitySession(ctx, f.instanceID)
	require.ErrorIs(t, err, studentpresence.ErrActivitySessionNotFound, "the session rolled back with the caller")
	attendance, err := f.module.ListSessionAttendance(ctx, []int64{f.participant})
	require.NoError(t, err)
	assert.Empty(t, attendance, "the attendance rows rolled back with the caller")

	// The same commands succeed again afterwards: nothing half-written blocks a retry.
	_, err = f.module.StartActivitySession(ctx, studentpresence.ActivitySessionStart{InstanceID: f.instanceID, ActiveGroupID: f.groupID, StartedAt: at})
	require.NoError(t, err)
	rows, err := f.module.CheckInParticipants(ctx, []int64{f.participant}, at)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
}
