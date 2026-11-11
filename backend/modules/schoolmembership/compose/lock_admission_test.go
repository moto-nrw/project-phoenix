package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// lockAdmissionCase is one lock School Membership takes: hold takes the
// conflicting lock in another transaction, path is the production operation
// that waits for it, and class and key are the ledger entry the operation
// must admit before it waits (#6702).
type lockAdmissionCase struct {
	name  string
	hold  func(context.Context, bun.Tx) error
	path  func(context.Context) error
	class tenant.LockClass
	key   string
}

// Every lock of School Membership's store is admitted through the unit of
// work's lock order before it waits: while the operation is observed waiting
// for the other transaction (pg_blocking_pids), its ledger already holds the
// lock, so a later gate, day or staff row would be refused instead of
// closing a deadlock cycle.
func TestMembershipLocksAreAdmittedBeforeTheyWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	quotas := newChildQuotas()
	dependencies := Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: quotas,
		Today: func() calendar.Date { return "2099-12-07" }}
	module, err := New(dependencies)
	require.NoError(t, err)
	windows, err := NewPendingWindows(dependencies)
	require.NoError(t, err)
	offboarding, err := NewOffboarding(dependencies)
	require.NoError(t, err)
	school := testpkg.Tenant(t)
	quotas.set(school, 1000)

	staff := createStaff(t, ctx, db, module, "Lock", "Staff", schoolmembership.StaffFields{})
	teacher, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	external, err := module.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{StaffID: staff.ID, Organization: "Verein", ActivityExpertise: "Fußball"}})
	require.NoError(t, err)
	entry, err := module.CreateClassListEntry(ctx, schoolmembership.CreateClassListEntry{ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Lea", LastName: "Lock", SchoolClass: "3a"}})
	require.NoError(t, err)
	leaver := createStaff(t, ctx, db, module, "Lock", "Leaver", schoolmembership.StaffFields{})
	class, err := module.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: leaver.ID, SchoolClass: "4b"})
	require.NoError(t, err)
	pending := testpkg.CreateTestStudent(t, db, "Lock", "Pending", "1a")
	_, err = db.ExecContext(ctx, "DELETE FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?", school, pending.ID)
	require.NoError(t, err)
	_, err = module.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: pending.ID, SchoolClass: "1a", Status: "pending", EnrolledFrom: "2100-08-01", EnrolledUntil: "2101-07-31"})
	require.NoError(t, err)
	newcomer := testpkg.CreateTestPerson(t, db, "Lock", "Newcomer")
	enrolled := testpkg.CreateTestStudent(t, db, "Lock", "Enrolled", "2a")
	_, err = db.ExecContext(ctx, "DELETE FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?", school, enrolled.ID)
	require.NoError(t, err)

	rowLock := func(sql string, id int64) func(context.Context, bun.Tx) error {
		return func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.NewRaw(sql, id).Exec(ctx)
			return err
		}
	}
	for _, tc := range []lockAdmissionCase{
		{
			name: "staff row", class: tenant.LockStaffRows, key: tenant.LockIDKey(staff.ID),
			hold: rowLock(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, staff.ID),
			path: func(ctx context.Context) error { _, err := module.FindStaffForMutation(ctx, staff.ID); return err },
		},
		{
			name: "staff person merge gate", class: tenant.LockStaffPersonMerge, key: tenant.LockIDKey(school),
			hold: func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, staffPersonMergeLockKeyForTest(school)).Exec(ctx)
				return err
			},
			path: func(ctx context.Context) error {
				_, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: newcomer.ID}})
				return err
			},
		},
		{
			name: "teacher", class: tenant.LockLeaf, key: "users.teachers",
			hold: rowLock(`SELECT id FROM users.teachers WHERE id = ? FOR UPDATE`, teacher.ID),
			path: func(ctx context.Context) error {
				_, err := module.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{ID: teacher.ID, TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID, Role: "Erzieherin"}})
				return err
			},
		},
		{
			name: "external caregiver record", class: tenant.LockLeaf, key: "users.external_caregivers",
			hold: rowLock(`SELECT id FROM users.external_caregivers WHERE id = ? FOR UPDATE`, external.ID),
			path: func(ctx context.Context) error {
				_, err := module.UpdateExternalCaregiverRecord(ctx, schoolmembership.UpdateExternalCaregiverRecord{ID: external.ID, ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{StaffID: staff.ID, Organization: "Neuer Verein", ActivityExpertise: "Fußball"}})
				return err
			},
		},
		{
			name: "class list entry", class: tenant.LockLeaf, key: "users.class_list_entries",
			hold: rowLock(`SELECT id FROM users.class_list_entries WHERE id = ? FOR UPDATE`, entry.ID),
			path: func(ctx context.Context) error {
				_, err := module.UpdateClassListEntry(ctx, schoolmembership.UpdateClassListEntry{ID: entry.ID, ClassListEntryFields: schoolmembership.ClassListEntryFields{FirstName: "Lea", LastName: "Lock", SchoolClass: "3b"}})
				return err
			},
		},
		{
			name: "class-write gate", class: tenant.LockStudentClassWrites, key: tenant.LockIDKey(school),
			hold: classGateHold(school),
			path: func(ctx context.Context) error { return module.LockStudentClassWrites(ctx, false) },
		},
		{
			name: "class-write gate of an identity insert", class: tenant.LockStudentClassWrites, key: tenant.LockIDKey(school),
			hold: classGateHold(school),
			path: func(ctx context.Context) error {
				gate, err := StudentClassWriteGateQuery(ctx, db)
				if err != nil {
					return err
				}
				tx, _ := tenant.CurrentTransaction[bun.IDB](ctx)
				_, err = tx.NewRaw(`WITH class_write_gate AS MATERIALIZED (?) SELECT 1 FROM class_write_gate`, gate).Exec(ctx)
				return err
			},
		},
		{
			name: "child quota", class: tenant.LockLeaf, key: "advisory:child-quota",
			hold: func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, int32(0x6b6f6e74), int32(school)).Exec(ctx)
				return err
			},
			path: func(ctx context.Context) error {
				_, err := module.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: enrolled.ID, SchoolClass: "2a", Status: "active"})
				return err
			},
		},
		{
			name: "pending student window", class: tenant.LockLeaf, key: "users.student_school_memberships",
			hold: rowLock(`SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL FOR UPDATE`, pending.ID),
			path: func(ctx context.Context) error {
				_, err := windows.MovePendingWindow(ctx, schoolmembership.PendingWindowMove{StudentID: pending.ID,
					FromBefore: "2100-08-01", FromAfter: "2100-08-08", UntilBefore: "2101-07-31", UntilAfter: "2101-07-24"})
				return err
			},
		},
		{
			name: "group assignments", class: tenant.LockLeaf, key: "education.group_teacher",
			hold: func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(ctx, "LOCK TABLE education.group_teacher IN SHARE ROW EXCLUSIVE MODE")
				return err
			},
			path: func(ctx context.Context) error { return module.LockGroupAssignments(ctx) },
		},
		{
			name: "class assignments of a retirement", class: tenant.LockLeaf, key: "education.class_teachers",
			hold: rowLock(`SELECT id FROM education.class_teachers WHERE id = ? FOR UPDATE`, class.ID),
			path: func(ctx context.Context) error { _, err := offboarding.Preview(ctx, leaver.ID); return err },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAdmittedBeforeWait(t, db, tc)
		})
	}
}

func classGateHold(school int64) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, int32(0x636c6173), int32(school)).Exec(ctx)
		return err
	}
}

// staffPersonMergeLockKeyForTest is the school's merge gate key, as School
// Membership and People Directory build it.
func staffPersonMergeLockKeyForTest(schoolID int64) string {
	return fmt.Sprintf("staff-person-merge:%d", schoolID)
}

// The nightly erasure skips a tombstone another transaction holds and never
// waits; the row it would lock is still admitted as a leaf first.
func TestRetiredStaffErasureAdmitsItsSkipLockedRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	offboarding, err := NewOffboarding(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment})
	require.NoError(t, err)
	retired := createStaff(t, ctx, db, module, "Lock", "Tombstone", schoolmembership.StaffFields{})
	_, err = db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = '2020-01-01T00:00:00Z' WHERE id = ?`, retired.ID).Exec(ctx)
	require.NoError(t, err)
	locked, release := make(chan struct{}), make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, retired.ID).Exec(ctx); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	err = testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context) error {
		_, found, err := offboarding.EraseNextRetiredStaff(ctx, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 0)
		assert.False(t, found, "the held tombstone is skipped, not waited for")
		assert.True(t, tenant.HoldsLock(ctx, tenant.LockLeaf, "users.staff_school_memberships"), "the skipped row is admitted as a leaf")
		assert.ErrorIs(t, tenant.OrderLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(retired.ID)), tenant.ErrLockOrder,
			"no staff row may follow the leaf")
		return err
	})
	close(release)
	require.NoError(t, err)
	require.NoError(t, <-holder)
}

// requireAdmittedBeforeWait runs the case's operation while another
// transaction holds the conflicting lock, observes the operation waiting for
// it, and requires the operation's ledger to hold the lock already.
func requireAdmittedBeforeWait(t *testing.T, db *bun.DB, tc lockAdmissionCase) {
	t.Helper()
	locked, release := make(chan struct{}), make(chan struct{})
	holderPID, holderDone := make(chan int64, 1), make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			holderPID <- pid
			if err := tc.hold(ctx, tx); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	select {
	case <-locked:
	case err := <-holderDone:
		require.FailNow(t, "the holder failed", "%v", err)
	}
	holder := <-holderPID
	pathPID, pathCtx, pathDone := make(chan int64, 1), make(chan context.Context, 1), make(chan error, 1)
	go func() {
		pathDone <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			pathPID <- pid
			pathCtx <- ctx
			return tc.path(ctx)
		})
	}()
	pid, ctx := <-pathPID, <-pathCtx
	for {
		select {
		case err := <-pathDone:
			require.FailNow(t, "the operation finished instead of waiting", "%v", err)
		default:
		}
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, holder, pid).Scan(context.Background(), &blocked))
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.True(t, tenant.HoldsLock(ctx, tc.class, tc.key), "the operation waits for its lock before the ledger admitted it")
	close(release)
	released = true
	require.NoError(t, <-holderDone)
	require.NoError(t, <-pathDone)
}
