package postgres

// Every lock the Timetable adapter takes is admitted through the unit of
// work's lock order before PostgreSQL can make the statement wait (#6702).
// Each case holds the conflicting lock in a second transaction, runs the
// real adapter method in a ledger transaction, waits until PostgreSQL
// reports it blocked by that transaction (pg_blocking_pids) and then
// requires the ledger to hold the admitted lock already. Without the
// tenant.OrderLock call the ledger is still empty while the statement waits.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

type admissionFixture struct {
	db           *bun.DB
	tenantID     int64
	staffID      int64
	studentID    int64
	groupID      int64
	categoryID   int64
	categoryName string
	instanceID   int64
	oldInstance  int64
	participant  int64
	supervisorID int64
	enrollmentID int64
	trackID      int64
	taskID       int64
	protocolID   int64
}

func newAdmissionFixture(t *testing.T) admissionFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := admissionFixture{db: db, tenantID: testpkg.Tenant(t)}
	staff := testpkg.CreateTestStaff(t, db, "Admission", "Staff")
	student := testpkg.CreateTestStudent(t, db, "Admission", "Child", "3a")
	room := testpkg.CreateTestRoom(t, db, "Admission room")
	group := testpkg.CreateTestActivityGroup(t, db, "Admission group")
	category := testpkg.CreateTestActivityCategory(t, db, "Admission category")
	instance := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2099, time.June, 1), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID})
	old := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2001, time.June, 1), room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	participant := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "expected")
	f.staffID, f.studentID, f.groupID, f.instanceID, f.oldInstance, f.participant = staff.ID, student.ID, group.ID, instance.ID, old.ID, participant.ID
	f.categoryID, f.categoryName = category.ID, category.Name
	insert := func(query string, args ...any) int64 {
		var id int64
		require.NoError(t, db.NewRaw(query+" RETURNING id", args...).Scan(ctx, &id))
		return id
	}
	f.supervisorID = insert(`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, TRUE, '2099-01-01')`, f.tenantID, staff.ID, group.ID)
	f.enrollmentID = insert(`INSERT INTO activities.student_enrollments (tenant_id, student_id, activity_group_id, valid_from) VALUES (?, ?, ?, '2099-01-01')`, f.tenantID, student.ID, group.ID)
	f.trackID = insert(`INSERT INTO schedule.planning_tracks (tenant_id, name, color, sort_order) VALUES (?, 'Admission track', '#112233', 0)`, f.tenantID)
	f.taskID = insert(`INSERT INTO schedule.pickup_extension_tasks (tenant_id, student_id, weekday, effective_from, previous_pickup_time, pickup_time) VALUES (?, ?, 1, '2099-01-05', '14:00', '15:00')`, f.tenantID, student.ID)
	f.protocolID = insert(`INSERT INTO schedule.group_week_protocols (tenant_id, scope, group_name, week_start) VALUES (?, 'school', 'Admission', '2099-06-01')`, f.tenantID)
	return f
}

// lockAdmissionCase names one adapter lock site: the statement another
// transaction holds, the adapter call and the ledger entry it admits.
type lockAdmissionCase struct {
	name  string
	hold  func(f admissionFixture) (string, []any)
	run   func(ctx context.Context, store *Store, f admissionFixture) error
	class tenant.LockClass
	key   string
}

func rowHold(table string, id func(admissionFixture) int64) func(admissionFixture) (string, []any) {
	return func(f admissionFixture) (string, []any) {
		return fmt.Sprintf("SELECT id FROM %s WHERE id = ? FOR UPDATE", table), []any{id(f)}
	}
}

func TestTimetableLocksAreAdmittedBeforeTheyWait(t *testing.T) {
	t.Parallel()
	f := newAdmissionFixture(t)
	instance := func(f admissionFixture) int64 { return f.instanceID }
	for _, tc := range []lockAdmissionCase{
		{name: "block row", hold: rowHold("schedule.activity_instances", instance), class: tenant.LockLeaf, key: "schedule.activity_instances",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.LockActivityInstance(ctx, f.instanceID, true)
				return err
			}},
		{name: "blocks by id", hold: rowHold("schedule.activity_instances", instance), class: tenant.LockLeaf, key: "schedule.activity_instances",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, err := s.SetActivityInstanceListKind(ctx, []int64{f.instanceID}, nil, time.Now())
				return err
			}},
		{name: "group row", hold: rowHold("activities.groups", func(f admissionFixture) int64 { return f.groupID }), class: tenant.LockReference, key: "activities.groups",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.FindGroup(ctx, f.groupID, "NO KEY UPDATE")
				return err
			}},
		{name: "block attendance", hold: rowHold("schedule.instance_students", func(f admissionFixture) int64 { return f.participant }), class: tenant.LockLeaf, key: "schedule.instance_students",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, err := s.LockInstanceStudentAssignments(ctx, f.instanceID)
				return err
			}},
		{name: "participants by id", hold: rowHold("schedule.instance_students", func(f admissionFixture) int64 { return f.participant }), class: tenant.LockLeaf, key: "schedule.instance_students",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, err := s.LockInstanceStudentsByID(ctx, []int64{f.participant})
				return err
			}},
		{name: "offboarding blocks", hold: rowHold("schedule.activity_instances", instance), class: tenant.LockLeaf, key: "schedule.activity_instances",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.PreviewStaffOffboarding(ctx, f.staffID, "2099-01-01")
				return err
			}},
		{name: "offboarding supervisors", hold: rowHold("activities.supervisors", func(f admissionFixture) int64 { return f.supervisorID }), class: tenant.LockLeaf, key: "activities.supervisors",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.PreviewStaffOffboarding(ctx, f.staffID, "2099-01-01")
				return err
			}},
		{name: "pickup extension task", hold: rowHold("schedule.pickup_extension_tasks", func(f admissionFixture) int64 { return f.taskID }), class: tenant.LockLeaf, key: "schedule.pickup_extension_tasks",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.FindPickupExtensionTaskForUpdate(ctx, f.taskID)
				return err
			}},
		{name: "planning track", hold: rowHold("schedule.planning_tracks", func(f admissionFixture) int64 { return f.trackID }), class: tenant.LockReference, key: "schedule.planning_tracks",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.FindPlanningTrack(ctx, f.trackID, "SHARE")
				return err
			}},
		{name: "planning track order", hold: rowHold("schedule.planning_tracks", func(f admissionFixture) int64 { return f.trackID }), class: tenant.LockReference, key: "schedule.planning_tracks",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, err := s.ReorderPlanningTracks(ctx, []int64{f.trackID})
				return err
			}},
		{name: "category", hold: rowHold("activities.categories", func(f admissionFixture) int64 { return f.categoryID }), class: tenant.LockReference, key: "activities.categories",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.FindCategory(ctx, f.categoryID, "SHARE")
				return err
			}},
		{name: "category by name", hold: rowHold("activities.categories", func(f admissionFixture) int64 { return f.categoryID }), class: tenant.LockReference, key: "activities.categories",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, _, err := s.FindCategoryByName(ctx, f.categoryName, false, "SHARE")
				return err
			}},
		{name: "care exit enrollments", hold: rowHold("activities.student_enrollments", func(f admissionFixture) int64 { return f.enrollmentID }), class: tenant.LockLeaf, key: "activities.student_enrollments",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, err := s.LockStudentEnrollmentsForCareExit(ctx, []int64{f.studentID}, "2099-01-01")
				return err
			}},
		{name: "planned supervisor table", class: tenant.LockLeaf, key: "activities.supervisors",
			hold: func(admissionFixture) (string, []any) {
				return "LOCK TABLE activities.supervisors IN ROW EXCLUSIVE MODE", nil
			},
			run: func(ctx context.Context, s *Store, _ admissionFixture) error {
				_, err := s.LockPlannedSupervisors(ctx)
				return err
			}},
		{name: "week protocol", hold: rowHold("schedule.group_week_protocols", func(f admissionFixture) int64 { return f.protocolID }), class: tenant.LockLeaf, key: "schedule.group_week_protocols",
			run: func(ctx context.Context, s *Store, f admissionFixture) error {
				_, _, err := s.FindWeekProtocol(ctx, f.protocolID, true)
				return err
			}},
		{name: "class arrival plan", class: tenant.LockLeaf, key: "advisory:class-arrival",
			hold: func(f admissionFixture) (string, []any) {
				return "SELECT pg_advisory_xact_lock(hashtext(?))", []any{fmt.Sprintf("class-arrival:%d:%s", f.tenantID, "3a")}
			},
			run: func(ctx context.Context, s *Store, _ admissionFixture) error {
				_, err := s.LockClassArrivalPlan(ctx, "3a")
				return err
			}},
		// Last: the history cleanup deletes the old block it waited for.
		{name: "history cleanup", hold: rowHold("schedule.activity_instances", func(f admissionFixture) int64 { return f.oldInstance }), class: tenant.LockLeaf, key: "schedule.activity_instances",
			run: func(ctx context.Context, s *Store, _ admissionFixture) error {
				_, _, err := s.DeleteActivityInstancesBefore(ctx, "2002-01-01")
				return err
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAdmittedBeforeWait(t, f, tc)
		})
	}
}

// requireAdmittedBeforeWait runs the case's adapter call while another
// transaction holds the conflicting lock and checks the ledger once
// PostgreSQL reports the call waiting for that transaction.
func requireAdmittedBeforeWait(t *testing.T, f admissionFixture, tc lockAdmissionCase) {
	t.Helper()
	store := New(DatabaseRuntime(f.db))
	query, args := tc.hold(f)
	holder, release := make(chan int64, 1), make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithTenantTx(t, context.Background(), f.db, f.tenantID, func(ctx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return err
			}
			holder <- pid
			<-release
			return nil
		})
	}()
	var holderPID int64
	select {
	case holderPID = <-holder:
	case err := <-holderDone:
		require.FailNow(t, "the holding transaction failed", "%v", err)
	}
	type started struct {
		ctx context.Context
		pid int64
	}
	path, pathDone := make(chan started, 1), make(chan error, 1)
	go func() {
		pathDone <- testpkg.WithTenantTx(t, context.Background(), f.db, f.tenantID, func(ctx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			path <- started{ctx: ctx, pid: pid}
			return tc.run(ctx, store, f)
		})
	}()
	var running started
	select {
	case running = <-path:
	case err := <-pathDone:
		close(release)
		require.FailNow(t, "the adapter transaction failed before it started", "%v", err)
	}
	awaitWaitingFor(t, f.db, running.pid, holderPID, pathDone, release)
	held := tenant.HoldsLock(running.ctx, tc.class, tc.key)
	close(release)
	require.NoError(t, <-holderDone)
	require.NoError(t, <-pathDone, "the adapter call completes once the lock is free")
	require.True(t, held, "the ledger admits %s %q before the statement waits", tc.class, tc.key)
}

func awaitWaitingFor(t *testing.T, db *bun.DB, waiter, holder int64, done <-chan error, release chan struct{}) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, holder, waiter).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
		select {
		case err := <-done:
			close(release)
			require.FailNow(t, "the adapter call finished instead of waiting", "%v", err)
		case <-poll.C:
		}
	}
	close(release)
	require.FailNow(t, "the adapter call never waited for the holding transaction")
}
