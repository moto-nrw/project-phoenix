package contracttest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hold grade transition's first gate, then wait for the correction to reach
// the class gate. Apply must still obtain recurrence on its own connection.
// The pre-fix correction holds recurrence while waiting here, closing 40P01.
func TestOfferingCorrectionAndGradeTransitionDoNotDeadlock(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	fx := setupOfferingChangeFixture(t, env, "CorrectionLock")
	database := testpkg.NewDatabase(t, env.db)
	recurrence := owners.MustNewTimetableRecurrenceLock(database)
	bookings := newBookingsForTest(env, nil, recurrence.LockRecurrenceWrites, nil)
	decisions := composeDecisions(t, env.db, env.reposEnrollment, env.repos, env.reposPeople, env.reposSchedules, bookings, nil, recurrence.LockRecurrenceWrites, nil)
	membership, err := owners.NewSchoolMembership(database)
	require.NoError(t, err)
	grade, err := api.NewGradeTransitionTestModule(database, nil, func() time.Time { return decisionTestToday.BerlinMidnight() })
	require.NoError(t, err)
	testpkg.CreateTestStudent(t, env.db, "Grade", "Child", "4a")
	transition := testpkg.CreateTestGradeTransition(t, env.db, "2026-2027", env.creatorID)
	nextClass := "5a"
	testpkg.CreateTestGradeTransitionMapping(t, env.db, transition.ID, "4a", &nextClass)
	// Use the production principal middleware rather than importing another
	// owner's authorization internals from this contract suite.
	claimsCtx := context.WithValue(offeringChangeAdminContext(t), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: int(env.creatorID), TenantID: testpkg.Tenant(t), Permissions: []string{"grade_transitions:apply"},
	})
	var authorizedCtx context.Context
	api.SecurityPrincipalMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		authorizedCtx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil).WithContext(claimsCtx))
	require.NotNil(t, authorizedCtx)
	ctx, cancel := context.WithTimeout(authorizedCtx, 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	gradeErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(gradeCtx context.Context, tx bun.Tx) error {
		if err := membership.LockStudentClassWrites(gradeCtx, true); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(gradeCtx, &pid))
		go func() {
			done <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error {
				_, err := decisions.UpdateChildOfferings(writeCtx, enrollment.UpdateChildOfferingsInput{
					RequestID: fx.requestID, ChildID: fx.childID, ActorAccountID: env.creatorID, ActorRole: "admin", Reason: "Correct offering",
					Offerings: []enrollment.OfferingAdjustmentSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
				})
				return err
			})
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE l.locktype = 'advisory'
    AND l.classid = ? AND l.objid = ? AND l.objsubid = 2
    AND NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, int32(0x636c6173), testpkg.Tenant(t), pid).Scan(gradeCtx, &waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond, "correction must wait on the class gate")
		_, err := grade.GradeTransition.Apply(gradeCtx, transition.ID, "")
		return err
	})
	var correctionErr error
	select {
	case correctionErr = <-done:
	case <-ctx.Done():
		t.Fatal("correction did not finish")
	}
	require.NoError(t, gradeErr, "grade transition must not deadlock")
	require.NoError(t, correctionErr, "correction must finish after grade transition")
}

// Care end holds its gate prefix before the offering writer starts. In the
// row interleaving it also holds the child, exposing request -> child cycles
// independently of the recurrence inversion. PostgreSQL, not timing, tells
// us when the competing command is blocked before confirmation continues.
func TestOfferingWritesAndCareExitDoNotDeadlock(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, operation := range []string{"approve", "reset", "approve with locked child", "reject", "mark done", "conflict"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnCtx(t)
			env := setupDecisionTest(t)
			fx := setupOfferingChangeFixture(t, env, "CareLock")
			database := testpkg.NewDatabase(t, env.db)
			recurrence := owners.MustNewTimetableRecurrenceLock(database)
			env.bookings = newBookingsForTest(env, nil, recurrence.LockRecurrenceWrites, nil)
			svc := newOfferingChangeServiceForTest(t, env)
			ctx, cancel := context.WithTimeout(offeringChangeAdminContext(t), 20*time.Second)
			defer cancel()
			fx.oldOffering.PickupTimes = map[string]string{"mon": "14:30"}
			require.NoError(t, newCareOfferingFixtureRecords(env.repos.CarePlan()).Update(ctx, fx.oldOffering))
			request, err := testpkg.TenantQuery1(t, ctx, env.db, svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
				StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
				Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
			})
			require.NoError(t, err)
			if operation == "mark done" {
				_, err := env.db.NewUpdate().TableExpr("enrollment.offering_change_requests").Set("effective_from = ?", decisionTestToday.AddDays(-1)).Where("id = ?", request.ID).Exec(ctx)
				require.NoError(t, err)
			}
			lifecycle := newCareLifecycleServiceWithLockAt(t, env.db, recurrence.LockRecurrenceWrites, func() calendar.Date { return decisionTestToday })
			input := careplan.CareExitInput{StudentIDs: []int64{fx.studentID}, LastCareDay: fx.switchDate.AddDays(10), Reason: careplan.CareExitReasonMovedAway}
			preview, err := testpkg.TenantQuery1(t, ctx, env.db, lifecycle.Preview, input)
			require.NoError(t, err)
			require.False(t, preview.Blocked)
			write := func(writeCtx context.Context) error {
				switch operation {
				case "reset":
					// The reset still resolves a valid baseline before the later care end.
					return env.bookings.ResetStudentPickupDayToOffering(writeCtx, fx.studentID, calendar.NewDate(2026, 9, 7))
				case "mark done":
					return svc.MarkDone(writeCtx, request.ID, "", "Completed externally", env.creatorID)
				case "conflict":
					if err := svc.LockConflictRequest(writeCtx, request.ID); err != nil {
						return err
					}
					return svc.DecideConflictRequest(writeCtx, careplan.OfferingConflictDecision{RequestID: request.ID, Reason: "Resolve conflict", ReviewerID: env.creatorID})
				default:
					return decideOfferingChange(writeCtx, svc, careplan.OfferingChangeDecisionInput{RequestID: request.ID, Approve: operation != "reject", Reason: "Reviewed", ReviewedBy: env.creatorID, ActorRole: "admin"})
				}
			}
			membership, err := owners.NewSchoolMembership(database)
			require.NoError(t, err)
			people, err := owners.NewPeopleDirectory(database)
			require.NoError(t, err)
			done := make(chan error, 1)
			careErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(careCtx context.Context, tx bun.Tx) error {
				if err := membership.LockStudentClassWrites(careCtx, false); err != nil {
					return err
				}
				if err := recurrence.LockRecurrenceWrites(careCtx); err != nil {
					return err
				}
				if operation != "approve" && operation != "reset" {
					if err := people.LockStudent(careCtx, fx.studentID); err != nil {
						return err
					}
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(careCtx, &pid))
				go func() {
					done <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return write(writeCtx) })
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, pid).Scan(careCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "offering writer must be blocked by care end")
				_, err := lifecycle.Confirm(careCtx, preview.Token, input, env.creatorID)
				return err
			})
			var writeErr error
			select {
			case writeErr = <-done:
			case <-ctx.Done():
				t.Fatal("offering writer did not finish")
			}
			require.NoError(t, careErr, "care end must not deadlock against offering writes")
			// This is a scheduled exit after the switch date, not an effective
			// departure: review/reset must still succeed after the care commit.
			require.NoError(t, writeErr, "offering writer must finish after care end")
		})
	}
}

// A request edited while review waits on the gate must not be decided using
// the preliminary snapshot. The authoritative version check is under its lock.
func TestOfferingReviewRechecksVersionAfterWaitingForGate(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env := setupDecisionTest(t)
	fx := setupOfferingChangeFixture(t, env, "VersionLock")
	database := testpkg.NewDatabase(t, env.db)
	recurrence := owners.MustNewTimetableRecurrenceLock(database)
	env.bookings = newBookingsForTest(env, nil, recurrence.LockRecurrenceWrites, nil)
	svc := newOfferingChangeServiceForTest(t, env)
	ctx, cancel := context.WithTimeout(offeringChangeAdminContext(t), 20*time.Second)
	defer cancel()
	request, err := testpkg.TenantQuery1(t, ctx, env.db, svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	done := make(chan error, 1)
	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context, tx bun.Tx) error {
		if err := env.bookings.LockOfferingDerivedWrites(editCtx); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(editCtx, &pid))
		go func() {
			done <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(reviewCtx context.Context, _ bun.Tx) error {
				return decideOfferingChange(reviewCtx, svc, careplan.OfferingChangeDecisionInput{
					RequestID: request.ID, ExpectedVersion: careplan.ParentRequestVersion(request.UpdatedAt),
					Reason: "Reviewed", ReviewedBy: env.creatorID,
				})
			})
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, pid).Scan(editCtx, &waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond, "review must wait before locking the request")
		return env.repos.CarePlan().UpdateOfferingChangeEffectiveFrom(editCtx, request.ID, fx.switchDate.AddDays(1).String())
	})
	require.NoError(t, err)
	select {
	case err := <-done:
		require.ErrorIs(t, err, parentrequests.ErrStale)
	case <-ctx.Done():
		t.Fatal("review did not finish")
	}
	stored, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.CarePlan().FindOfferingChange, request.ID, false)
	require.NoError(t, err)
	require.Equal(t, careplan.OfferingChangePending, stored.Status)
	require.Equal(t, fx.switchDate.AddDays(1).String(), stored.EffectiveFrom)
}
