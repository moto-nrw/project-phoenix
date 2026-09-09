package users_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	authSvcPkg "github.com/moto-nrw/project-phoenix/services/auth"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func offboardingCredential(prefix string, suffix string) string {
	return prefix + suffix + "!"
}

type offboardingScenario struct {
	db      *bun.DB
	repos   *repositories.Factory
	authSvc authSvcPkg.AuthService
	svc     *offboardingTestRunner
	deps    offboardingcompose.Dependencies
	ctx     context.Context
}

func newOffboardingScenario(t *testing.T, databases ...*bun.DB) *offboardingScenario {
	t.Helper()

	var db *bun.DB
	if len(databases) == 0 {
		db = testpkg.SetupTestDB(t)
	} else {
		db = databases[0]
	}

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	repos.SetConfigRuntime(testpkg.ConfigRuntime(db))

	authCfg, err := authSvcPkg.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	authCfg.Audit = testpkg.NewAuthEventCommand(repos.AuthEvent)
	authService, err := authSvcPkg.NewService(repos, authCfg, db, nil)
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, authService, db)

	actorAccount := testpkg.CreateTestAccount(t, db, "offboarding-audit-actor@example.org")
	deps := offboardingcompose.Dependencies{
		DB: db, Access: authService,
		Authorize: func(ctx context.Context) (staffoffboarding.Actor, error) {
			actor, _ := ctx.Value(offboardingTestActorKey{}).(staffoffboarding.Actor)
			actor.AccountID = actorAccount.ID
			actor.TenantID = tenant.FromContext(ctx)
			return actor, nil
		},
		Cleanup: func(context.Context, int64) error { return nil },
	}
	svc := &offboardingTestRunner{deps: deps}

	return &offboardingScenario{
		db:      db,
		repos:   repos,
		authSvc: authService,
		svc:     svc,
		deps:    deps,
		ctx:     testpkg.Ctx(t),
	}
}

// This test-only adapter preserves the scenario call shape while invoking
// the production owner-workflow composition. It is not a runtime provider.
type offboardingTestActorKey struct{}
type offboardingTestRunner struct {
	deps offboardingcompose.Dependencies
}

func (r *offboardingTestRunner) OffboardStaff(ctx context.Context, staffID, actorID int64, username string) error {
	workflow, err := offboardingcompose.New(r.deps)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actorID, Username: username})
	_, err = workflow.Offboard(ctx, staffID)
	if errors.Is(err, staffoffboarding.ErrInUse) {
		return usersSvc.ErrStaffInUse
	}
	return err
}

type failingOffboardingAudit struct{}

func (failingOffboardingAudit) Append(context.Context, any) error {
	return errors.New("forced audit failure")
}

func TestOffboardStaff_ConcurrentSupervisionCannotAttachAfterRetirement(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"create", "replace"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			assertConcurrentSupervisionRejected(t, operation)
		})
	}
}

func assertConcurrentSupervisionRejected(t *testing.T, operation string) {
	t.Helper()
	sc := newOffboardingScenario(t, testpkg.SetupIsolatedTestDB(t))
	active, err := services.NewActiveTestModule(sc.db, testpkg.TenantRuntime(t, sc.db))
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, sc.db, "Concurrent", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, sc.db, "Concurrent supervision")
	room := testpkg.CreateTestRoom(t, sc.db, "Concurrent supervision")
	group := testpkg.CreateTestActiveGroup(t, sc.db, activity.ID, room.ID)
	workflow, err := offboardingcompose.New(sc.deps)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(sc.ctx, 15*time.Second)
	defer cancel()
	actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: staff.ID, Username: "test-admin"})
	writerDone := make(chan error, 1)
	err = tenant.WithinCurrentTenant(actorCtx, func(txCtx context.Context) error {
		preview, err := workflow.Preview(txCtx, staff.ID)
		if err != nil {
			return err
		}
		go func() {
			writerDone <- tenant.WithinCurrentTenant(ctx, func(writerCtx context.Context) error {
				if operation == "replace" {
					_, err := active.Active.UpdateActiveGroupSupervisors(writerCtx, group.ID, []int64{staff.ID})
					return err
				}
				return active.Active.CreateGroupSupervisor(writerCtx, &activeModels.GroupSupervisor{
					StaffID: staff.ID, GroupID: group.ID, Role: "supervisor", StartDate: timezone.TodayDate(),
				})
			})
		}()
		require.Eventually(t, func() bool {
			var waiting int
			err := sc.db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
			return err == nil && waiting > 0
		}, 5*time.Second, 10*time.Millisecond)
		_, err = workflow.Execute(txCtx, staff.ID, preview.Revision)
		return err
	})
	require.NoError(t, err)
	select {
	case err := <-writerDone:
		require.Error(t, err, "supervision writer must recheck live staff after waiting for offboarding")
	case <-ctx.Done():
		t.Fatal("supervision writer did not finish after retirement")
	}
	var supervisors int
	require.NoError(t, sc.db.NewSelect().TableExpr("active.group_supervisors").ColumnExpr("count(*)").Where("staff_id = ?", staff.ID).Scan(ctx, &supervisors))
	require.Zero(t, supervisors)
}

func TestOffboardStaff_CleanupIntentFailureRestoresAccessAndAllOwnerWrites(t *testing.T) {
	t.Parallel()
	sc := newOffboardingScenario(t)
	staff, account := testpkg.CreateTestCalendarStaff(t, sc.db, "Atomic", "Offboarding")
	token := testpkg.CreateTestToken(t, sc.db, account.ID, "refresh")
	shift := testpkg.CreateTestStaffShift(t, sc.db, staff.ID, testpkg.TodayDate().AddDays(1), testpkg.StaffShiftOpts{})
	room := testpkg.CreateTestRoom(t, sc.db, "Atomic offboarding")
	instance := testpkg.CreateTestActivityInstance(t, sc.db, testpkg.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{})
	assignment := testpkg.CreateTestInstanceStaff(t, sc.db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	failure := errors.New("cleanup intent unavailable")
	sc.svc.deps.Cleanup = func(context.Context, int64) error { return failure }
	err := sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin")
	require.ErrorIs(t, err, failure)
	_, err = sc.repos.Staff.FindByID(sc.ctx, staff.ID)
	require.NoError(t, err)
	_, err = sc.repos.StaffShift.FindByID(sc.ctx, shift.ID)
	require.NoError(t, err)
	_, err = sc.repos.InstanceStaff.FindByID(sc.ctx, assignment.ID)
	require.NoError(t, err)
	person, err := sc.repos.Person.FindByID(sc.ctx, staff.PersonID)
	require.NoError(t, err)
	require.NotNil(t, person.AccountID)
	require.Equal(t, account.ID, *person.AccountID)
	roles, err := sc.authSvc.GetAccountRoles(sc.ctx, int(account.ID))
	require.NoError(t, err)
	require.NotEmpty(t, roles)
	active, err := sc.authSvc.VerifyAccountTenantMembership(sc.ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	tokens, err := sc.authSvc.GetActiveTokens(sc.ctx, int(account.ID))
	require.NoError(t, err)
	require.Len(t, tokens, 1, "rolled-back account cleanup must not revoke credentials after commit")
	require.Equal(t, token.ID, tokens[0].ID)
	var auditRows int
	require.NoError(t, sc.db.NewSelect().TableExpr("audit.data_deletions").ColumnExpr("count(*)").Where("staff_id = ?", staff.ID).Scan(sc.ctx, &auditRows))
	require.Zero(t, auditRows, "the deletion audit and owner writes must roll back together")

	sc.svc.deps.Cleanup = func(context.Context, int64) error { return nil }
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))
	tokens, err = sc.authSvc.GetActiveTokens(sc.ctx, int(account.ID))
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestOffboardStaff_ConcurrentPlanningCannotAttachAfterRetirement(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"timetable", "shift", "move_history"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			assertConcurrentPlanningRejected(t, operation)
		})
	}
}

func assertConcurrentPlanningRejected(t *testing.T, operation string) {
	t.Helper()
	sc := newOffboardingScenario(t, testpkg.SetupIsolatedTestDB(t))
	staff := testpkg.CreateTestStaff(t, sc.db, "Concurrent", "Timetable")
	room := testpkg.CreateTestRoom(t, sc.db, "Concurrent timetable")
	date := testpkg.TodayDate().AddDays(1)
	if operation == "move_history" {
		date = testpkg.TodayDate().AddDays(-1)
	}
	instance := testpkg.CreateTestActivityInstance(t, sc.db, date, room.ID, testpkg.ActivityInstanceOpts{})
	if operation == "move_history" {
		testpkg.CreateTestInstanceStaff(t, sc.db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
	}
	workflow, err := offboardingcompose.New(sc.deps)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(sc.ctx, 15*time.Second)
	defer cancel()
	actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: staff.ID, Username: "test-admin"})
	writerDone := make(chan error, 1)
	err = tenant.WithinCurrentTenant(actorCtx, func(txCtx context.Context) error {
		preview, err := workflow.Preview(txCtx, staff.ID)
		if err != nil {
			return err
		}
		go func() {
			writerDone <- tenant.WithinCurrentTenant(ctx, func(writerCtx context.Context) error {
				if operation == "shift" {
					return sc.repos.StaffShift.Create(writerCtx, &scheduleModels.StaffShift{StaffID: staff.ID, Date: scheduleModels.Date(testpkg.TodayDate().AddDays(1)), StartTime: testpkg.WallClock(8, 0), EndTime: testpkg.WallClock(12, 0), CreatedBy: staff.ID})
				}
				if operation == "move_history" {
					instance.Date = scheduleModels.Date(testpkg.TodayDate().AddDays(1))
					return sc.repos.ActivityInstance.Update(writerCtx, instance)
				}
				return sc.repos.InstanceStaff.Create(writerCtx, &scheduleModels.InstanceStaff{InstanceID: instance.ID, StaffID: staff.ID})
			})
		}()
		require.Eventually(t, func() bool {
			var waiting int
			err := sc.db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(ctx, &waiting)
			return err == nil && waiting > 0
		}, 5*time.Second, 10*time.Millisecond)
		_, err = workflow.Execute(txCtx, staff.ID, preview.Revision)
		return err
	})
	require.NoError(t, err)
	select {
	case err := <-writerDone:
		require.Error(t, err, "assignment must revalidate live staff after retirement releases its lock")
	case <-ctx.Done():
		t.Fatal("assignment writer did not finish")
	}
	rows, err := sc.repos.InstanceStaff.FindByInstanceID(ctx, instance.ID)
	require.NoError(t, err)
	if operation == "move_history" {
		require.Len(t, rows, 1, "the historical assignment is retained")
		persisted, err := sc.repos.ActivityInstance.FindByID(ctx, instance.ID)
		require.NoError(t, err)
		require.Equal(t, scheduleModels.Date(date), persisted.Date, "history cannot be moved back into an operational assignment")
	} else {
		require.Empty(t, rows)
	}
	var shifts int
	require.NoError(t, sc.db.NewRaw("SELECT count(*) FROM schedule.staff_shifts WHERE tenant_id = ? AND staff_id = ?", testpkg.Tenant(t), staff.ID).Scan(ctx, &shifts))
	require.Zero(t, shifts)
}

func TestOffboardingRuntimeEvidence(t *testing.T) {
	t.Parallel()
	sc := newOffboardingScenario(t, testpkg.SetupIsolatedTestDB(t))
	actor := testpkg.CreateTestStaff(t, sc.db, "Runtime", "Actor")
	queue, err := repositories.NewStaffDocumentCleanup(sc.db, nil)
	require.NoError(t, err)
	sc.deps.Cleanup = queue.Enqueue
	workflow, err := offboardingcompose.New(sc.deps)
	require.NoError(t, err)
	ctx := context.WithValue(sc.ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "runtime-actor"})
	counter := testpkg.CaptureQueriesForContext(t, sc.db)
	testpkg.AttachLockWaitEvidence(sc.db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(ctx))
	var version string
	require.NoError(t, sc.db.NewRaw("SHOW server_version").Scan(sc.ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, sc.db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(sc.ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()
	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := sc.db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := sc.db.Stats()
		require.NoError(t, err)
		if iteration < 5 {
			return
		}
		writes := counter.WriteRows()
		rows, statements := counter.Rows()
		samples[operation] = append(samples[operation], testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			WriteRowsAffected: &writes, RowsAffected: rows, StatementsWithRows: statements,
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		})
	}
	room := testpkg.CreateTestRoom(t, sc.db, "Offboarding runtime")
	for iteration := range 35 {
		staff, account := testpkg.CreateTestCalendarStaff(t, sc.db, "Runtime", "Subject")
		testpkg.CreateTestToken(t, sc.db, account.ID, "refresh")
		testpkg.CreateTestStaffShift(t, sc.db, staff.ID, testpkg.TodayDate().AddDays(1), testpkg.StaffShiftOpts{})
		instance := testpkg.CreateTestActivityInstance(t, sc.db, testpkg.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{})
		testpkg.CreateTestInstanceStaff(t, sc.db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
		var preview staffoffboarding.Preview
		measure("preview", iteration, func() error { var err error; preview, err = workflow.Preview(ctx, staff.ID); return err })
		measure("execute", iteration, func() error { _, err := workflow.Execute(ctx, staff.ID, preview.Revision); return err })
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("offboarding-runtime %s", raw)
}

// assignTenantRole links the account to a role inside the fixture tenant.
func assignTenantRole(t *testing.T, db *bun.DB, accountID int64, roleID int64) {
	t.Helper()
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	accountRole.SetTenantID(testpkg.Tenant(t))
	err := db.NewInsert().
		Model(accountRole).
		ModelTableExpr(`auth.account_roles`).
		Scan(testpkg.Ctx(t))
	require.NoError(t, err)
}

// TestOffboardStaff_WithAttendanceHistory is the headline regression for
// issue #695: staff with attendance records could never be deleted because of
// the ON DELETE RESTRICT FK. With soft delete the offboarding must succeed and
// the attendance history must survive untouched.
func TestOffboardStaff_WithAttendanceHistory(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Attendance", "History")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	student := testpkg.CreateTestStudent(t, sc.db, "Offboard", "Student", "1a")
	device := testpkg.CreateTestDevice(t, sc.db, fmt.Sprintf("offb-dev-%d", time.Now().UnixNano()))
	attendance := testpkg.CreateTestAttendance(t, sc.db, student.ID, staff.ID, device.ID, time.Now(), nil)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.attendance WHERE id = ?`, attendance.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	var attendanceCount int
	err := sc.db.NewSelect().
		TableExpr(`active.attendance`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ? AND checked_in_by = ?`, attendance.ID, staff.ID).
		Scan(context.Background(), &attendanceCount)
	require.NoError(t, err)
	assert.Equal(t, 1, attendanceCount, "attendance history must survive offboarding")

	var deletedAt *time.Time
	err = sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt, "staff row must be soft-deleted, not hard-deleted")

	var auditCount int
	err = sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`COUNT(*)`).
		Where(`staff_id = ? AND deletion_type = 'manual' AND deleted_by = 'test-admin'`, staff.ID).
		Scan(context.Background(), &auditCount)
	require.NoError(t, err)
	assert.Equal(t, 1, auditCount, "offboarding must write an audit.data_deletions record")
}

// TestOffboardStaff_RevokesAccountAccess covers bug 1 of issue #695: after
// deletion the Betreuer must no longer be able to log in.
func TestOffboardStaff_RevokesAccountAccess(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-login-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Off", "Boarded", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	role := testpkg.GetOrCreateTestRole(t, sc.db, "user")
	assignTenantRole(t, sc.db, account.ID, role.ID)
	testpkg.CreateTestToken(t, sc.db, account.ID, "refresh")

	_, _, err := sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.NoError(t, err, "login must work before offboarding")

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.False(t, exists, "account-tenant mapping must be inactive after offboarding")

	var active bool
	err = sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active)
	require.NoError(t, err)
	assert.False(t, active, "single-tenant account must be deactivated")

	var tokenCount int
	err = sc.db.NewSelect().
		TableExpr(`auth.tokens`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ?`, account.ID).
		Scan(context.Background(), &tokenCount)
	require.NoError(t, err)
	assert.Zero(t, tokenCount, "refresh tokens must be revoked")

	var personAccountID *int64
	err = sc.db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`account_id`).
		Where(`id = ?`, person.ID).
		Scan(context.Background(), &personAccountID)
	require.NoError(t, err)
	assert.Nil(t, personAccountID, "person must be unlinked from the account")

	var roleCount int
	err = sc.db.NewSelect().
		TableExpr(`auth.account_roles`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ?`, account.ID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Zero(t, roleCount, "tenant roles must be removed")

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.Error(t, err, "login must fail after offboarding")
}

// TestOffboardStaff_MultiTenantAccountKeepsOtherSchool: offboarding at school A
// must not lock the account out of school B.
func TestOffboardStaff_MultiTenantAccountKeepsOtherSchool(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-multi-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Multi", "Tenant", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, sc.db, otherTenant)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, otherTenant)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	var active bool
	err := sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active)
	require.NoError(t, err)
	assert.True(t, active, "account with another active school mapping must stay active")

	existsA, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.False(t, existsA, "offboarded school mapping must be inactive")

	existsB, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, otherTenant)
	require.NoError(t, err)
	assert.True(t, existsB, "other school mapping must stay active")

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.NoError(t, err, "login at the other school must keep working")
}

// TestOffboardStaff_ReinviteSameEmailSameSchool covers bug 2 of issue #695:
// the same email must be re-invitable at the same school after offboarding,
// but a token alone must not reactivate the globally disabled account.
func TestOffboardStaff_ReinviteSameEmailSameSchool(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	invSvc := authSvcPkg.NewInvitationService(authSvcPkg.InvitationServiceConfig{
		InvitationRepo:    sc.repos.InvitationToken,
		AccountRepo:       sc.repos.Account,
		AccountTenantRepo: sc.repos.AccountTenant,
		RoleRepo:          sc.repos.Role,
		AccountRoleRepo:   sc.repos.AccountRole,
		PersonRepo:        sc.repos.Person,
		StaffRepo:         sc.repos.Staff,
		TeacherRepo:       sc.repos.Teacher,
		// Accepting onto an existing person checks that person is not a child's
		// record, so the repository is required, not optional (#2222).
		StudentRepo:      sc.repos.Student,
		SchoolRepo:       sc.repos.School,
		Mailer:           email.NewMockMailer(),
		FrontendURL:      "http://localhost:3000",
		InvitationExpiry: time.Hour,
		DB:               sc.db,
	})
	testpkg.SetTenantRuntime(t, invSvc, sc.db)

	oldCredential := offboardingCredential("Offboard", "123")
	newCredential := offboardingCredential("Reinvited", "456")
	emailAddr := fmt.Sprintf("offboard-reinvite-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, oldCredential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Re", "Invited", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	role := testpkg.CreateTestSystemRole(t, sc.db, "user")

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM auth.invitation_tokens WHERE email = ?`, emailAddr)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM users.staff WHERE person_id IN (SELECT id FROM users.persons WHERE account_id = ?)`, account.ID)
	})

	// Before offboarding the re-invite is blocked (current production behavior).
	_, err := invSvc.CreateInvitation(sc.ctx, authSvcPkg.InvitationRequest{
		Email:     emailAddr,
		RoleID:    role.ID,
		TenantID:  testpkg.Tenant(t),
		CreatedBy: account.ID,
	})
	require.Error(t, err, "re-invite must be blocked while the mapping is active")
	require.ErrorIs(t, err, authSvcPkg.ErrAccountAlreadyHasTenantAccess)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	invitation, err := invSvc.CreateInvitation(sc.ctx, authSvcPkg.InvitationRequest{
		Email:     emailAddr,
		RoleID:    role.ID,
		TenantID:  testpkg.Tenant(t),
		CreatedBy: account.ID,
	})
	require.NoError(t, err, "re-invite must succeed after offboarding")

	reactivated, err := invSvc.AcceptInvitation(context.Background(), invitation.Token, authSvcPkg.UserRegistrationData{
		FirstName:       "Re",
		LastName:        "Invited",
		Password:        newCredential,
		ConfirmPassword: newCredential,
	})
	require.ErrorIs(t, err, authSvcPkg.ErrInvitationOwnerRequired)
	require.Nil(t, reactivated)

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.False(t, exists, "token-only acceptance must not reactivate the tenant mapping")

	var staffCount int
	err = sc.db.NewSelect().
		TableExpr(`users.staff AS "staff"`).
		ColumnExpr(`COUNT(*)`).
		Join(`JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Where(`"person".account_id = ? AND "staff".deleted_at IS NULL AND "person".deleted_at IS NULL`, account.ID).
		Scan(context.Background(), &staffCount)
	require.NoError(t, err)
	assert.Zero(t, staffCount, "rejected acceptance must not recreate a live staff record")
	stored, err := sc.repos.Account.FindByID(context.Background(), account.ID)
	require.NoError(t, err)
	assert.False(t, stored.Active)
	assert.Equal(t, account.PasswordHash, stored.PasswordHash)

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, newCredential)
	require.Error(t, err, "the proposed invitation password must grant no access")
}

// TestOffboardStaff_ActiveSupervisionBlocks: an active room supervision still
// blocks offboarding (no FK safety net remains, the pre-check is the guard).
func TestOffboardStaff_ActiveSupervisionBlocks(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Supervising", "Staff")
	activityGroup := testpkg.CreateTestActivityGroup(t, sc.db, "OffboardSupervision")
	room := testpkg.CreateTestRoom(t, sc.db, fmt.Sprintf("OffbRoom-%d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, sc.db, activityGroup.ID, room.ID)
	supervisor := testpkg.CreateTestGroupSupervisor(t, sc.db, staff.ID, activeGroup.ID, "supervisor")

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.group_supervisors WHERE id = ?`, supervisor.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.groups WHERE id = ?`, activeGroup.ID)
	})

	err := sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin")
	require.Error(t, err)
	assert.True(t, errors.Is(err, usersSvc.ErrStaffInUse), "active supervision must block offboarding")

	var deletedAt *time.Time
	scanErr := sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt)
	require.NoError(t, scanErr)
	assert.Nil(t, deletedAt, "blocked offboarding must not soft-delete the staff row")
}

func TestOffboardStaff_ActiveGroupHandoverBlocks(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)
	staff := testpkg.CreateTestStaff(t, sc.db, "Handover", "Target")
	group := testpkg.CreateTestEducationGroup(t, sc.db, "OffboardHandover")
	today := timezone.TodayDate()
	handover := testpkg.CreateTestGroupSubstitution(
		t, sc.db, group.ID, nil, staff.ID, today, today.AddDays(1),
	)

	err := sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin")
	require.ErrorIs(t, err, usersSvc.ErrStaffInUse)

	_, err = sc.repos.GroupSubstitution.FindByID(sc.ctx, handover.ID)
	require.NoError(t, err)
	_, err = sc.repos.Staff.FindByID(sc.ctx, staff.ID)
	require.NoError(t, err)
}

// TestOffboardStaff_CleansUpAssignments: planned assignments the old
// ON DELETE CASCADE used to remove must be cleaned up explicitly.
func TestOffboardStaff_CleansUpAssignments(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	teacher := testpkg.CreateTestTeacher(t, sc.db, "Assigned", "Teacher")
	staffID := teacher.StaffID
	educationGroup := testpkg.CreateTestEducationGroup(t, sc.db, "OffboardGroup")
	groupTeacher := testpkg.CreateTestGroupTeacher(t, sc.db, educationGroup.ID, teacher.ID)

	otherStaff := testpkg.CreateTestStaff(t, sc.db, "Other", "Substitute")
	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, sc.db, educationGroup.ID, &staffID, otherStaff.ID,
		today.AddDays(1), today.AddDays(8))

	var personID int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`person_id`).
		Where(`id = ?`, staffID).
		Scan(context.Background(), &personID))

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_substitution WHERE id = ?`, substitution.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_teacher WHERE id = ?`, groupTeacher.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.groups WHERE id = ?`, educationGroup.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staffID, staffID, "test-admin"))

	var gtCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`education.group_teacher`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, groupTeacher.ID).
		Scan(context.Background(), &gtCount))
	assert.Zero(t, gtCount, "group-teacher assignment must be removed")

	var subCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`education.group_substitution`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, substitution.ID).
		Scan(context.Background(), &subCount))
	assert.Zero(t, subCount, "future substitution must be removed")

	var teacherDeletedAt *time.Time
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.teachers`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, teacher.ID).
		Scan(context.Background(), &teacherDeletedAt))
	assert.NotNil(t, teacherDeletedAt, "teacher row must be soft-deleted")
}

// TestOffboardStaff_BroadcastsGroupAccessChanged verifies that the direct
// assignment cleanup invalidates open group views after the transaction
// commits. Offboarding does not pass through the education service.
func TestOffboardStaff_BroadcastsGroupAccessChanged(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	sc.svc.deps.Broadcaster = broadcaster

	teacher := testpkg.CreateTestTeacher(t, sc.db, "Broadcast", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, sc.db, "OffboardBroadcastGroup")
	groupTeacher := testpkg.CreateTestGroupTeacher(t, sc.db, group.ID, teacher.ID)
	today := timezone.TodayDate()
	regularStaffID := teacher.StaffID
	otherStaff := testpkg.CreateTestStaff(t, sc.db, "Broadcast", "Substitute")
	substitution := testpkg.CreateTestGroupSubstitution(
		t, sc.db, group.ID, &regularStaffID, otherStaff.ID, today, today.AddDays(1),
	)

	var personID int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`person_id`).
		Where(`id = ?`, teacher.StaffID).
		Scan(context.Background(), &personID))

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_substitution WHERE id = ?`, substitution.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_teacher WHERE id = ?`, groupTeacher.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.groups WHERE id = ?`, group.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, teacher.StaffID, teacher.StaffID, "test-admin"))

	event := testpkg.AssertSingleTenantEvent(t, broadcaster, realtime.EventGroupAccessChanged, testpkg.Tenant(t))
	require.NotNil(t, event.Data.Source)
	assert.Equal(t, "staff_offboarding", *event.Data.Source)
}

func TestOffboardStaff_RollbackBroadcastsNothing(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	sc.deps.Audit = failingOffboardingAudit{}
	svc := &offboardingTestRunner{deps: sc.deps}
	svc.deps.Broadcaster = broadcaster

	teacher := testpkg.CreateTestTeacher(t, sc.db, "Rollback", "Broadcast")
	group := testpkg.CreateTestEducationGroup(t, sc.db, "OffboardRollbackGroup")
	groupTeacher := testpkg.CreateTestGroupTeacher(t, sc.db, group.ID, teacher.ID)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_teacher WHERE id = ?`, groupTeacher.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.groups WHERE id = ?`, group.ID)
	})

	err := svc.OffboardStaff(sc.ctx, teacher.StaffID, teacher.StaffID, "test-admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced audit failure")
	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged),
		"rolled-back assignment cleanup must not publish group access changes")

	var assignmentCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`education.group_teacher`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, groupTeacher.ID).
		Scan(context.Background(), &assignmentCount))
	assert.Equal(t, 1, assignmentCount, "group assignment must roll back with the failed audit")
}

// TestOffboardStaff_Idempotent: deleting a non-existent staff member stays a
// no-op so the HTTP handler keeps returning 200.
func TestOffboardStaff_EmptyDisplayNameKeepsLegacyAuditAttribution(t *testing.T) {
	t.Parallel()
	sc := newOffboardingScenario(t)
	staff := testpkg.CreateTestStaff(t, sc.db, "UnnamedActor", "Offboarding")
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, ""))
	var deletedBy string
	require.NoError(t, sc.db.NewRaw("SELECT deleted_by FROM audit.data_deletions WHERE staff_id = ? AND tenant_id = ?", staff.ID, testpkg.Tenant(t)).Scan(sc.ctx, &deletedBy))
	require.Equal(t, "system", deletedBy)
}

func TestOffboardStaff_Idempotent(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, 999999, 999999, "test-admin"))
}

// TestOffboardStaff_ExcludedFromListsAndPIN: offboarded staff must vanish from
// operational queries.
func TestOffboardStaff_ExcludedFromListsAndPIN(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Listed", "Staff")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	listed, err := sc.repos.Staff.ListAllWithPerson(sc.ctx)
	require.NoError(t, err)
	for _, s := range listed {
		assert.NotEqual(t, staff.ID, s.ID, "offboarded staff must not appear in the staff list")
	}

	found, err := sc.repos.Staff.FindByID(sc.ctx, staff.ID)
	assert.Error(t, err, "FindByID must not return soft-deleted staff")
	assert.Nil(t, found)
}

// TestOffboardStaff_ClearsDirectPermissions: direct tenant-scoped permission
// grants must not survive offboarding — re-invitation reuses the account ID,
// so leftover grants would restore old elevated permissions.
func TestOffboardStaff_ClearsDirectPermissions(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Direct", "Permission")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	perm := testpkg.CreateTestPermission(t, sc.db,
		fmt.Sprintf("offb-perm-%d", time.Now().UnixNano()), "students", "read")
	require.NoError(t, sc.repos.AccountPermission.GrantPermission(sc.ctx, account.ID, perm.ID))

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM auth.account_permissions WHERE account_id = ?`, account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	var permCount int
	err := sc.db.NewSelect().
		TableExpr(`auth.account_permissions`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).
		Scan(context.Background(), &permCount)
	require.NoError(t, err)
	assert.Zero(t, permCount, "direct permission grants must be removed on offboarding")
}

// TestOffboardStaff_PreservesGuardianAccess: a dual-role teacher/guardian
// account loses staff access but must keep the guardian role and an active
// tenant mapping so the parents portal keeps working.
func TestOffboardStaff_PreservesGuardianAccess(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-guardian-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Dual", "Role", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	staffRole := testpkg.GetOrCreateTestRole(t, sc.db, "user")
	assignTenantRole(t, sc.db, account.ID, staffRole.ID)
	guardianRole := testpkg.GetOrCreateTestRole(t, sc.db, "guardian")
	assignTenantRole(t, sc.db, account.ID, guardianRole.ID)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	countRole := func(roleID int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`auth.account_roles`).
			ColumnExpr(`COUNT(*)`).
			Where(`account_id = ? AND role_id = ?`, account.ID, roleID).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countRole(staffRole.ID), "staff role must be removed")
	assert.Equal(t, 1, countRole(guardianRole.ID), "guardian role must be kept")

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.True(t, exists, "tenant mapping must stay active for the guardian")

	var active bool
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active))
	assert.True(t, active, "guardian account must stay active")

	var personAccountID *int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`account_id`).
		Where(`id = ?`, person.ID).
		Scan(context.Background(), &personAccountID))
	assert.Nil(t, personAccountID, "staff person must still be unlinked from the account")

	// Staff portal login is refused for the now guardian-only account; the
	// parents portal remains the entry point.
	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.Error(t, err, "staff login must be refused for the guardian-only account")
	assert.ErrorIs(t, err, authSvcPkg.ErrParentMustUseParentPortal)
}

// TestOffboardStaff_RemovesSameDayPlannedInstanceAssignments: same-day
// timetable assignments on instances that have not started yet must be
// removed (instance Start would copy them into active supervisors), while
// already-completed same-day instances keep their rows as history.
func TestOffboardStaff_RemovesSameDayPlannedInstanceAssignments(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "SameDay", "Planned")
	room := testpkg.CreateTestRoom(t, sc.db, fmt.Sprintf("OffbInstRoom-%d", time.Now().UnixNano()))
	today := timezone.TodayDate()

	makeInstance := func(title string) *scheduleModels.ActivityInstance {
		inst := &scheduleModels.ActivityInstance{
			Date:          scheduleModels.Date(today),
			Title:         title,
			StartTime:     time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			RoomID:        room.ID,
			Status:        scheduleModels.InstanceStatusPlanned,
			IsSpontaneous: true,
		}
		inst.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, sc.repos.ActivityInstance.Create(sc.ctx, inst))
		return inst
	}
	plannedInst := makeInstance(fmt.Sprintf("offb-planned-%d", time.Now().UnixNano()))
	completedInst := makeInstance(fmt.Sprintf("offb-completed-%d", time.Now().UnixNano()))
	_, err := sc.db.ExecContext(context.Background(),
		`UPDATE schedule.activity_instances SET status = 'completed' WHERE id = ?`, completedInst.ID)
	require.NoError(t, err)

	makeAssignment := func(instanceID int64) *scheduleModels.InstanceStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staff.ID}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, sc.repos.InstanceStaff.Create(sc.ctx, row))
		return row
	}
	plannedRow := makeAssignment(plannedInst.ID)
	completedRow := makeAssignment(completedInst.ID)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	countAssignment := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`schedule.instance_staff`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countAssignment(plannedRow.ID),
		"same-day planned assignment must be removed so instance Start cannot copy it")
	assert.Equal(t, 1, countAssignment(completedRow.ID),
		"same-day completed assignment must stay as history")
}

// TestOffboardStaff_RemovesPendingAndFutureAbsences: pending requests and
// not-yet-over absences must disappear from operational queries; past decided
// absences stay as history.
func TestOffboardStaff_RemovesPendingAndFutureAbsences(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Absent", "Staff")
	actor := testpkg.CreateTestStaff(t, sc.db, "Offboarding", "Admin")
	today := timezone.TodayDate()

	makeAbsence := func(absenceType, status string, start, end timezone.Date) *activeModels.StaffAbsence {
		absence := &activeModels.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: absenceType,
			DateStart:   start,
			DateEnd:     end,
			Status:      status,
			CreatedBy:   staff.ID,
		}
		require.NoError(t, sc.repos.StaffAbsence.Create(sc.ctx, absence))
		return absence
	}
	pendingRequest := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusRequested,
		today.AddDays(3), today.AddDays(5))
	futureApproved := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved,
		today.AddDays(10), today.AddDays(12))
	pastApproved := makeAbsence(activeModels.AbsenceTypeSick, activeModels.AbsenceStatusApproved,
		today.AddDays(-10), today.AddDays(-8))
	pastQuestion := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusQuestion,
		today.AddDays(-6), today.AddDays(-4))

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, actor.ID, "test-admin"))

	countAbsence := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`active.staff_absences`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countAbsence(pendingRequest.ID), "pending request must be removed")
	assert.Zero(t, countAbsence(futureApproved.ID), "future approved absence must be removed")
	assert.Zero(t, countAbsence(pastQuestion.ID), "past questioned request must be removed")
	assert.Equal(t, 1, countAbsence(pastApproved.ID), "past absence must stay as history")

	var auditedCount string
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`metadata->>'staff_absences'`).
		Where(`staff_id = ?`, staff.ID).
		Scan(context.Background(), &auditedCount))
	assert.Equal(t, "3", auditedCount, "audit record must count the deleted absences")

	var tombstones []*auditModels.TimeTrackingDeletion
	require.NoError(t, sc.db.NewSelect().
		Model(&tombstones).
		ModelTableExpr(`audit.time_tracking_deletions AS "time_tracking_deletion"`).
		Where(`"time_tracking_deletion".staff_id = ?`, staff.ID).
		OrderExpr(`"time_tracking_deletion".source_id ASC`).
		Scan(sc.ctx))
	require.Len(t, tombstones, 3, "every deleted absence must leave a tombstone")
	for _, tombstone := range tombstones {
		assert.Equal(t, actor.ID, tombstone.DeletedBy)
		assert.Equal(t, auditModels.TimeTrackingDeletionSourceAbsence, tombstone.Source)
		assert.Equal(t, "Personal-Offboarding", tombstone.Note)
		var payload activeModels.StaffAbsence
		require.NoError(t, json.Unmarshal(tombstone.Payload, &payload))
		assert.Equal(t, tombstone.SourceID, payload.ID)
	}
}

func TestOffboardStaff_AbsenceAuditFailureRollsBackOffboarding(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Audit", "Rollback")
	today := timezone.TodayDate()
	absence := &activeModels.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: activeModels.AbsenceTypeVacation,
		DateStart:   today.AddDays(1),
		DateEnd:     today.AddDays(2),
		Status:      activeModels.AbsenceStatusApproved,
		CreatedBy:   staff.ID,
	}
	require.NoError(t, sc.repos.StaffAbsence.Create(sc.ctx, absence))

	sc.svc.deps.Audit = failingOffboardingAudit{}
	err := sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced audit failure")

	var absenceCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`active.staff_absences`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, absence.ID).
		Scan(context.Background(), &absenceCount))
	assert.Equal(t, 1, absenceCount, "absence deletion must roll back when its tombstone fails")

	var deletedAt *time.Time
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt))
	assert.Nil(t, deletedAt, "staff offboarding must roll back with the failed tombstone")
}

func TestOffboardStaff_RemovesUpcomingStaffShifts(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Shifted", "Offboard")
	today := timezone.TodayDate()

	makeShift := func(date timezone.Date, startHour int) *scheduleModels.StaffShift {
		shift := &scheduleModels.StaffShift{
			StaffID:   staff.ID,
			Date:      scheduleModels.Date(date),
			StartTime: time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
			EndTime:   time.Date(1, 1, 1, startHour+4, 0, 0, 0, time.UTC),
			CreatedBy: staff.ID,
		}
		require.NoError(t, sc.repos.StaffShift.Create(sc.ctx, shift))
		return shift
	}
	past := makeShift(today.AddDays(-1), 8)
	sameDay := makeShift(today, 8)
	future := makeShift(today.AddDays(1), 8)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	countShift := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`schedule.staff_shifts`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Equal(t, 1, countShift(past.ID), "past staff shift must stay as history")
	assert.Zero(t, countShift(sameDay.ID), "same-day staff shift must be removed")
	assert.Zero(t, countShift(future.ID), "future staff shift must be removed")

	var auditedCount string
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`metadata->>'staff_shifts'`).
		Where(`staff_id = ?`, staff.ID).
		Scan(context.Background(), &auditedCount))
	assert.Equal(t, "2", auditedCount, "audit record must count the deleted staff shifts")
}

// TestOffboardStaff_ClearsWorkTimeModelAssignment: the soft-deleted staff row
// must not keep its work_time_model_id, or the RESTRICT FK blocks model
// deletion while the live-staff pre-check reports zero assignments.
func TestOffboardStaff_ClearsWorkTimeModelAssignment(t *testing.T) {
	t.Parallel()

	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Modeled", "Staff")
	model := &configModel.WorkTimeModel{
		Name:               fmt.Sprintf("offb-model-%d", time.Now().UnixNano()),
		RotationLength:     1,
		RotationAnchorDate: configModel.NewCalendarDate(2026, time.January, 5),
	}
	require.NoError(t, sc.repos.WorkTimeModel.Create(sc.ctx, model, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 300},
	}))
	_, err := sc.db.ExecContext(context.Background(),
		`UPDATE users.staff SET work_time_model_id = ? WHERE id = ?`, model.ID, staff.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM config.work_time_model_entries WHERE work_time_model_id = ?`, model.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM config.work_time_models WHERE id = ?`, model.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))

	var workTimeModelID *int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`work_time_model_id`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &workTimeModelID))
	assert.Nil(t, workTimeModelID, "work_time_model_id must be cleared on offboarding")

	require.NoError(t, sc.repos.WorkTimeModel.Delete(sc.ctx, model.ID),
		"work-time-model deletion must succeed once the offboarded staff no longer references it")
}

// TestOffboardStaff_LocksAccountBeforeRevokingRoles pins the LOCK ORDER the
// school-portal mint guard depends on (#2207): auth.accounts is taken FOR
// UPDATE before any role, permission or mapping row of that account is
// touched. The guard walks the same rows account-first because it has to
// decide against a locked account; offboarding used to walk them in the
// opposite direction and only update auth.accounts at the very end, which is
// the textbook setup for a PostgreSQL deadlock (40P01) between an admin
// offboarding a Lehrkraft and that same person logging into the school portal.
//
// The proof is indirect but exact: while another transaction holds the account
// row, offboarding must not get as far as deleting the role mapping.
func TestOffboardStaff_LocksAccountBeforeRevokingRoles(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "789")
	emailAddr := fmt.Sprintf("offboard-lockorder-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Lock", "Order", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, testpkg.Tenant(t))
	role := testpkg.GetOrCreateTestRole(t, sc.db, "user")
	assignTenantRole(t, sc.db, account.ID, role.ID)

	holder, err := sc.db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, err = holder.ExecContext(context.Background(),
		`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID)
	require.NoError(t, err)

	blockedCtx, cancel := context.WithTimeout(sc.ctx, 2*time.Second)
	defer cancel()
	err = sc.svc.OffboardStaff(blockedCtx, staff.ID, staff.ID, "test-admin")
	require.Error(t, err, "offboarding must block on the account row, not walk past it")

	var roleCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`auth.account_roles`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ?`, account.ID).
		Scan(context.Background(), &roleCount))
	assert.Equal(t, 1, roleCount,
		"role removal must happen AFTER the account lock: with the account held elsewhere, no role may be gone")

	require.NoError(t, holder.Rollback())

	// With the lock released the very same offboarding runs to completion —
	// the block above was contention, not a broken code path.
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, staff.ID, "test-admin"))
}
