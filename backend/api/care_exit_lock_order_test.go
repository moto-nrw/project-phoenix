package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hold the first grade-transition gate before starting care end. Once PostgreSQL
// reports care end waiting on it, Apply must still be able to obtain recurrence.
// The old recurrence -> class order closes a real two-connection deadlock here;
// no sleep or scheduler-dependent race is needed to reach that interleaving.
func TestCareExitAndGradeTransitionDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	today := calendar.NewDate(2026, 10, 23)
	for _, operation := range []string{"confirm", "cancel", "due effects", "due effects in ambient transaction"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testpkg.OwnCtx(t), 20*time.Second)
			defer cancel()
			database := testpkg.NewDatabase(t, db)
			actorID := testpkg.CreateTestAccount(t, db, "care-actor").ID
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
				AccountID: actorID, TenantID: testpkg.Tenant(t), Permissions: []string{permissions.GradeTransitionsApply},
			})
			require.NoError(t, err)
			ctx = permissions.WithPrincipal(ctx, principal)
			student := testpkg.CreateTestStudent(t, db, "Care", "Child", "2a")
			testpkg.CreateTestStudent(t, db, "Grade", "Child", "4a")
			transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", actorID)
			nextClass := "5a"
			testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "4a", &nextClass)
			grade, err := NewGradeTransitionTestModule(database, nil, func() time.Time { return today.BerlinMidnight() })
			require.NoError(t, err)
			membership, err := owners.NewSchoolMembership(database)
			require.NoError(t, err)
			recurrence := owners.MustNewTimetableRecurrenceLock(database)
			repos, err := NewCareLifecycleTestRepositories(database, nil)
			require.NoError(t, err)
			svc, err := repos.NewCareLifecycle(CareLifecycleTestConfig{
				Audit:                 NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, NewStudentAudit(database)),
				LockCareBookingWrites: recurrence.LockRecurrenceWrites,
				BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil },
				Today:                 func() calendar.Date { return today },
			})
			require.NoError(t, err)
			input := careplan.CareExitInput{
				StudentIDs: []int64{student.ID}, LastCareDay: today.AddDays(1), Reason: careplan.CareExitReasonMovedAway,
			}
			preview, err := testpkg.TenantQuery1(t, ctx, db, svc.Preview, input)
			require.NoError(t, err)
			require.False(t, preview.Blocked)
			care := func(careCtx context.Context) error {
				_, confirmErr := svc.Confirm(careCtx, preview.Token, input, actorID)
				return confirmErr
			}
			if operation != "confirm" {
				_, err = testpkg.TenantQuery3(t, ctx, db, svc.Confirm, preview.Token, input, actorID)
				require.NoError(t, err)
				care = func(careCtx context.Context) error {
					_, cancelErr := svc.Cancel(careCtx, input.StudentIDs, actorID)
					return cancelErr
				}
			}
			if operation == "due effects" || operation == "due effects in ambient transaction" {
				_, err = db.NewUpdate().TableExpr("users.student_school_memberships").
					Set("enrolled_until = ?", today.AddDays(-1)).
					Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(ctx)
				require.NoError(t, err)
				care = func(careCtx context.Context) error {
					_, dueErr := svc.ApplyDueEffects(careCtx, today)
					return dueErr
				}
			}
			if operation == "due effects in ambient transaction" {
				apply := care
				care = func(careCtx context.Context) error {
					return testpkg.WithTenantTx(t, careCtx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
						return apply(txCtx)
					})
				}
			}

			careDone := make(chan error, 1)
			gradeErr := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(gradeCtx context.Context, tx bun.Tx) error {
				if err := membership.LockStudentClassWrites(gradeCtx, true); err != nil {
					return err
				}
				var gradePID int
				if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(gradeCtx, &gradePID); err != nil {
					return err
				}
				go func() { careDone <- care(testpkg.WithTenantRuntime(t, ctx, db)) }()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw(`SELECT EXISTS (
						SELECT 1 FROM pg_locks l WHERE l.locktype = 'advisory'
						AND l.classid = ? AND l.objid = ? AND l.objsubid = 2
						AND NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid))
					)`, int32(0x636c6173), testpkg.Tenant(t), gradePID).Scan(gradeCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "care end must be waiting on the grade transition's class gate")
				_, applyErr := grade.GradeTransition.Apply(gradeCtx, transition.ID, "")
				return applyErr
			})
			select {
			case careErr := <-careDone:
				for _, err := range []error{gradeErr, careErr} {
					var pgErr interface{ Field(byte) string }
					if errors.As(err, &pgErr) {
						t.Logf("PostgreSQL SQLSTATE: %s", pgErr.Field('C'))
					}
				}
				require.NoError(t, gradeErr, "grade transition must not deadlock against care end")
				require.NoError(t, careErr, "care end must finish after the grade transition commits")
			case <-ctx.Done():
				t.Fatal("care end did not finish after the grade transaction released its locks")
			}
		})
	}
}
