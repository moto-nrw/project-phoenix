package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Exercise the root-bound rows adapter, not a fake that already returns the
// sentinel. The preliminary unlocked read must preserve the locked read's
// not-found contract, including tenant-hidden requests.
func TestOfferingChangeNotFound(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	ctx := offeringChangeAdminContext(t)
	var missingID int64
	require.NoError(t, env.db.NewRaw("SELECT nextval(pg_get_serial_sequence('enrollment.offering_change_requests', 'id'))").Scan(t.Context(), &missingID))
	var foreignID int64
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign := setupDecisionTest(t)
		changes := newOfferingChangeServiceForTest(t, foreign)
		fx := setupOfferingChangeFixture(t, foreign, "ForeignMissing")
		row, err := changes.SubmitOfferingChange(offeringChangeAdminContext(t), careplan.CreateOfferingChangeInput{
			StudentID: fx.studentID, AccountID: foreign.creatorID, EffectiveFrom: fx.switchDate,
			Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
		})
		require.NoError(t, err)
		foreignID = row.ID
	})
	for name, id := range map[string]int64{"missing": missingID, "foreign": foreignID} {
		t.Run(name, func(t *testing.T) {
			for _, operation := range []struct {
				name string
				call func(context.Context) error
			}{
				{"approve", func(ctx context.Context) error {
					return svc.Decide(ctx, careplan.OfferingChangeDecisionInput{RequestID: id, Approve: true, ReviewedBy: env.creatorID})
				}},
				{"reject", func(ctx context.Context) error {
					return svc.Decide(ctx, careplan.OfferingChangeDecisionInput{RequestID: id, Reason: "Rejected", ReviewedBy: env.creatorID})
				}},
				{"mark done", func(ctx context.Context) error { return svc.MarkDone(ctx, id, "", "", env.creatorID) }},
				{"conflict candidate", func(ctx context.Context) error { _, err := svc.ConflictCandidate(ctx, id); return err }},
				{"lock conflict", func(ctx context.Context) error { return svc.LockConflictRequest(ctx, id) }},
				{"decide conflict", func(ctx context.Context) error {
					return svc.DecideConflictRequest(ctx, careplan.OfferingConflictDecision{RequestID: id, Approve: true, ReviewerID: env.creatorID})
				}},
				{"preview", func(ctx context.Context) error { _, err := svc.PreviewDecision(ctx, id, nil, nil); return err }},
			} {
				t.Run(operation.name, func(t *testing.T) {
					require.ErrorIs(t, operation.call(ctx), careplan.ErrOfferingChangeNotFound)
				})
			}
		})
	}
}

func TestOfferingMissingSubjectsPreserveErrors(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	fx := setupOfferingChangeFixture(t, env, "MissingSubjects")
	ctx := offeringChangeAdminContext(t)
	reserveID := func(table string) int64 {
		var id int64
		require.NoError(t, env.db.NewRaw("SELECT nextval(pg_get_serial_sequence(?, 'id'))", table).Scan(t.Context(), &id))
		return id
	}
	for _, input := range []struct {
		name               string
		requestID, childID int64
		want               error
	}{
		{"missing request", reserveID("enrollment.requests"), fx.childID, careplan.ErrBookingRequestNotFound},
		{"missing child", fx.requestID, reserveID("enrollment.request_children"), careplan.ErrBookingChildNotFound},
	} {
		t.Run(input.name, func(t *testing.T) {
			_, err := testpkg.TenantQuery1(t, ctx, env.db, env.bookings.AdjustOfferings, careplan.OfferingAdjustment{
				RequestID: input.requestID, ChildID: input.childID, Reason: "Corrected", ActorAccountID: env.creatorID,
				Source: careplan.OfferingAdjustmentSourceDirect,
			})
			require.ErrorIs(t, err, input.want)
		})
	}
	missingStudentID := reserveID("users.student_profiles")
	err := testpkg.TenantCommand2(t, ctx, env.db, pickupTimeService(t, env).ResetStudentPickupDayToOffering, missingStudentID, calendar.NewDate(2026, 8, 24))
	require.ErrorIs(t, err, careplan.ErrPickupResetNoOffering)
	t.Run("foreign subjects", func(t *testing.T) {
		foreignCtx := testpkg.OwnCtx(t)
		_, err := testpkg.TenantQuery1(t, foreignCtx, env.db, env.bookings.AdjustOfferings, careplan.OfferingAdjustment{
			RequestID: fx.requestID, ChildID: fx.childID, Reason: "Corrected", ActorAccountID: env.creatorID,
			Source: careplan.OfferingAdjustmentSourceDirect,
		})
		require.ErrorIs(t, err, careplan.ErrBookingRequestNotFound)
		err = testpkg.TenantCommand2(t, foreignCtx, env.db, pickupTimeService(t, env).ResetStudentPickupDayToOffering, fx.studentID, calendar.NewDate(2026, 8, 24))
		require.ErrorIs(t, err, careplan.ErrPickupResetNoOffering)
	})
}

func TestOfferingChangeDeletedWhileWaitingForGate(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, operation := range []string{"approve", "reject", "mark done", "conflict"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupDecisionTest(t)
			recurrence := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db))
			env.bookings = newBookingsForTest(env, nil, recurrence.LockRecurrenceWrites, nil)
			svc := newOfferingChangeServiceForTest(t, env)
			fx := setupOfferingChangeFixture(t, env, "DeletedWhileWaiting")
			ctx, cancel := context.WithTimeout(offeringChangeAdminContext(t), 20*time.Second)
			defer cancel()
			row, err := testpkg.TenantQuery1(t, ctx, env.db, svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
				StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
				Selections: []careplan.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
			})
			require.NoError(t, err)
			people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, env.db))
			done := make(chan error, 1)
			err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
				if err := env.bookings.LockOfferingDerivedWrites(deleteCtx); err != nil {
					return err
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
				go func() {
					done <- testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(reviewCtx context.Context, _ bun.Tx) error {
						switch operation {
						case "mark done":
							return svc.MarkDone(reviewCtx, row.ID, "", "", env.creatorID)
						case "conflict":
							return svc.LockConflictRequest(reviewCtx, row.ID)
						default:
							return svc.Decide(reviewCtx, careplan.OfferingChangeDecisionInput{RequestID: row.ID, Approve: operation == "approve", Reason: "Reviewed", ReviewedBy: env.creatorID})
						}
					})
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(deleteCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "preliminary read must finish before deletion")
				deleted, err := people.DeleteStudent(deleteCtx, fx.studentID)
				require.EqualValues(t, 1, deleted)
				return err
			})
			require.NoError(t, err)
			select {
			case err := <-done:
				require.ErrorIs(t, err, careplan.ErrOfferingChangeNotFound)
			case <-ctx.Done():
				t.Fatal("review did not finish after deletion")
			}
			_, err = testpkg.TenantQuery2(t, ctx, env.db, env.repos.CarePlan().FindOfferingChange, row.ID, false)
			require.ErrorIs(t, err, careplan.ErrOfferingChangeNotFound, "student deletion must cascade to the request")
		})
	}
}
