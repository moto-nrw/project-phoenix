package presence_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	active "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type pausedCheckinWrite struct {
	active.StudentPresence
	beforeWrite func(context.Context) error
}

func (p pausedCheckinWrite) RecordVisit(ctx context.Context, visit studentpresence.Visit) (studentpresence.Visit, error) {
	if err := p.beforeWrite(ctx); err != nil {
		return studentpresence.Visit{}, err
	}
	return p.StudentPresence.RecordVisit(ctx, visit)
}

// Both writers use the real tenant runtime and native Presence/People reads.
// The port pause is after the guard, not a SQL hook. pg_blocking_pids proves
// the membership UPDATE actually waits, including after CreateVisit returns.
func TestCreateVisitLockedGroupRoutesBeforeConcurrentMembershipChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	student := testpkg.CreateTestStudent(t, db, "Locked", "Routing", "3a")
	oldGroup := testpkg.CreateTestEducationGroup(t, db, "Checkin old group")
	newGroup := testpkg.CreateTestEducationGroup(t, db, "Checkin new group")
	assignStudentToEducationGroup(t, db, testpkg.Ctx(t), student.ID, oldGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "Locked", "Staff")
	device := testpkg.CreateTestDevice(t, db, "locked-checkin-routing")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	t.Cleanup(cancel)
	ctx = api.WithAttendanceDevice(ctx, device.ID, device.TenantID)
	ctx = api.WithAttendanceStaff(ctx, staff.ID, staff.TenantID)
	guarded := make(chan int, 1)
	releaseWrite, releaseCommit := make(chan struct{}), make(chan struct{})
	var writeOnce, commitOnce sync.Once
	unpauseWrite := func() { writeOnce.Do(func() { close(releaseWrite) }) }
	unpauseCommit := func() { commitOnce.Do(func() { close(releaseCommit) }) }
	t.Cleanup(unpauseCommit)
	t.Cleanup(unpauseWrite)
	module := testSchoolPresence(t, db)
	svc, broadcaster := newServiceWithBroadcaster(t, db, pausedCheckinWrite{
		StudentPresence: module,
		beforeWrite: func(txCtx context.Context) error {
			tx, ok := tenant.TransactionFromContext(txCtx)
			if !ok {
				return fmt.Errorf("missing tenant transaction")
			}
			var scope struct {
				PID    int
				Role   string
				Tenant string
				RLS    bool
			}
			if err := tx.(bun.Tx).NewRaw(`SELECT pg_backend_pid() AS pid, current_user AS role,
				current_setting('app.current_tenant_id') AS tenant,
				row_security_active('users.student_school_memberships'::regclass) AS rls`).Scan(txCtx, &scope); err != nil {
				return err
			}
			if scope.Role != "phoenix_tenant" || scope.Tenant != strconv.FormatInt(student.TenantID, 10) || !scope.RLS {
				return fmt.Errorf("unexpected tenant scope: %+v", scope)
			}
			guarded <- scope.PID
			select {
			case <-releaseWrite:
				return nil
			case <-txCtx.Done():
				return txCtx.Err()
			}
		},
	})
	waker := &recordingGuardianWaker{}
	active.ConfigureForTest(svc, active.WithGuardianWaker(waker))
	created, committed := make(chan error, 1), make(chan error, 1)
	visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}
	go func() {
		committed <- testpkg.WithTenantTx(t, ctx, db, student.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			err := svc.CreateVisit(txCtx, visit)
			created <- err
			if err != nil {
				return err
			}
			select {
			case <-releaseCommit:
				return nil
			case <-txCtx.Done():
				return txCtx.Err()
			}
		})
	}()
	var holderPID int
	select {
	case holderPID = <-guarded:
	case <-ctx.Done():
		t.Fatal("checkin never reached the post-guard write port")
	}
	updated := make(chan error, 1)
	go func() {
		updated <- testpkg.WithTenantTx(t, ctx, db, student.TenantID, func(txCtx context.Context, tx bun.Tx) error {
			result, err := tx.NewUpdate().Table("users.student_school_memberships").Set("group_id = ?", newGroup.ID).
				Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(txCtx)
			if err != nil {
				return err
			}
			rows, err := result.RowsAffected()
			if err == nil && rows != 1 {
				return fmt.Errorf("updated %d memberships, want one", rows)
			}
			return err
		})
	}()
	awaitMembershipUpdateBlocked(t, ctx, db, holderPID)
	unpauseWrite()
	require.NoError(t, awaitResult(t, created))
	awaitMembershipUpdateBlocked(t, ctx, db, holderPID)
	assert.Empty(t, broadcaster.Calls(), "SSE waits for the outer tenant commit")
	assert.Empty(t, waker.studentIDs, "guardian wake waits for commit too")
	unpauseCommit()
	require.NoError(t, awaitResult(t, committed))
	require.NoError(t, awaitResult(t, updated))
	assert.Equal(t, []int64{student.ID}, waker.studentIDs)
	checkins := broadcaster.EventsOfType(active.EventStudentCheckIn)
	require.Len(t, checkins, 2, "one active-group and one education-group delivery")
	for _, event := range checkins {
		require.NotNil(t, event.Data.GroupIDs)
		assert.Equal(t, []string{strconv.FormatInt(oldGroup.ID, 10)}, *event.Data.GroupIDs)
	}
	refresh := tenantCallsOfType(broadcaster, active.EventDashboardCountsChanged)
	require.Len(t, refresh, 1)
	require.NotNil(t, refresh[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{strconv.FormatInt(oldGroup.ID, 10)}, *refresh[0].Event.Data.GroupIDs)
	var actualGroup int64
	require.NoError(t, db.NewRaw("SELECT group_id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL", student.ID).Scan(ctx, &actualGroup))
	assert.Equal(t, newGroup.ID, actualGroup, "the blocked writer commits after checkin")
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 1)
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, attendance, 1)
}

func awaitMembershipUpdateBlocked(t *testing.T, ctx context.Context, db *bun.DB, holderPID int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE datname = current_database() AND ?::int = ANY(pg_blocking_pids(pid))
			AND query LIKE 'UPDATE "users"."student_school_memberships"%')`, holderPID).Scan(ctx, &blocked)
		require.NoError(t, err)
		return blocked
	}, 5*time.Second, 10*time.Millisecond, "membership writer must wait on the checkin transaction")
}
