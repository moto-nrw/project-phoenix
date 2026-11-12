package enrollmenthttp_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// gatedFutureRenewal is setupFutureRenewal with the tenant recurrence gate
// wired the way the root binds it, for the decision, its bookings and the
// phase administration: the phase-start edit, the approval and the
// correction then meet on the real advisory lock.
func gatedFutureRenewal(t *testing.T) (*futureRenewal, enrollmentNative.PhaseAdministration) {
	t.Helper()
	testpkg.OwnTenant(t)
	r := setupFutureRenewal(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, r.env.db)).LockRecurrenceWrites
	bookings := newBookingsForTest(r.env.rolloverTestEnv, nil, lock, nil)
	decision := newDecisionServiceForTestWithBookings(t, r.env.rolloverTestEnv, bookings, nil, lock, nil, nil)
	r.env = &decisionTestEnv{rolloverTestEnv: r.env.rolloverTestEnv, decision: decision, bookings: bookings}
	phases := NewTestPhases(TestPhaseDependencies{
		Records:                r.env.repos.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(r.env.reposCare.CarePlan)},
		LockTemplateRecurrence: lock,
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Logger:                 slog.Default(),
	})
	return r, phases
}

// whilePhaseStartMoves runs write on a second connection while a phase edit
// that moves the start by days holds the recurrence gate, waits until
// PostgreSQL reports write blocked by the edit and commits the edit. With
// moveAfterBlock the edit first saves the unchanged phase, so it holds only
// the gate, and moves the start, rewriting pending plans, once write waits.
func whilePhaseStartMoves(
	t *testing.T,
	r *futureRenewal,
	phases enrollmentNative.PhaseAdministration,
	days int,
	moveAfterBlock bool,
	write func(context.Context) error,
) (editErr, writeErr error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	moved := enrollmentAPI.OwnerPhaseForTest(r.env.sourcePhase)
	moved.ServiceStartDate = enrollmentNative.Date(r.start.AddDays(days))
	first := moved
	if moveAfterBlock {
		unchanged := *moved
		unchanged.ServiceStartDate = enrollmentNative.Date(r.start)
		first = &unchanged
	}
	done := make(chan error, 1)
	editErr = testpkg.WithTenantTx(t, ctx, r.env.db, testpkg.Tenant(t), func(editCtx context.Context, tx bun.Tx) error {
		if err := phaseEdit(phases)(editCtx, first); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(editCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, r.env.db, testpkg.Tenant(t), write)
		}()
		if finished := waitForBlockedWriter(t, editCtx, tx, pid, done); finished != nil {
			return fmt.Errorf("the concurrent write finished without waiting for the phase edit: %w", *finished)
		}
		if !moveAfterBlock {
			return nil
		}
		return phaseEdit(phases)(editCtx, moved)
	})
	select {
	case writeErr = <-done:
	case <-ctx.Done():
		t.Fatalf("the concurrent write did not finish; phase edit returned %v", editErr)
	}
	return editErr, writeErr
}

// waitForBlockedWriter waits until a statement of another connection waits
// on a lock held by the connection pid. It returns the write's result when
// the write finished first instead.
func waitForBlockedWriter(t *testing.T, ctx context.Context, tx bun.Tx, pid int, done <-chan error) *error {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		var waiting bool
		err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(ctx, &waiting)
		require.NoError(t, err)
		if waiting {
			return nil
		}
		select {
		case writeErr := <-done:
			return &writeErr
		case <-deadline:
			require.FailNow(t, "the concurrent write must wait for the phase edit")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// An approval that waits for a concurrent phase-start edit must plan the
// weekly plan for the start that edit committed. Reading the phase before
// the recurrence gate kept the old start: the edit rescheduled only the
// plans that already existed, and the new pickup times replaced the running
// ones a week early (#6043 review).
func TestDecisionService_Decide_FutureRenewalPlansCommittedPhaseStart(t *testing.T) {
	t.Parallel()

	r, phases := gatedFutureRenewal(t)
	editErr, approveErr := whilePhaseStartMoves(t, r, phases, 7, false, func(ctx context.Context) error {
		_, err := r.env.decision.Decide(ctx, enrollmentAPI.DecideInput{
			RequestID: r.requestID, ChildID: r.childID, Status: enrollmentNative.DecisionApproved, ReviewedBy: r.reviewerAccountID,
		})
		return err
	})
	require.NoError(t, editErr)
	require.NoError(t, approveErr)

	plan, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().ChildWeeklyPlan, r.childID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, enrollmentNative.Date(r.start.AddDays(7)), plan.On, "the plan follows the committed phase start")
	assert.Zero(t, r.applyDue(t, r.start), "nothing applies on the old start")
	r.assertRunningPlan(t, "on the old start")
}

// A direct correction of an approved child with a pending weekly plan and a
// phase-start edit take the gates before the request and child rows. The
// edit holds the recurrence gate while it rewrites the pending plans; a
// correction that locked the child first and the gate later closed a 40P01
// cycle with it (#6043 review).
func TestChangeRequestService_CorrectApprovedChildData_WaitsForPhaseStartEdit(t *testing.T) {
	t.Parallel()

	r, phases := gatedFutureRenewal(t)
	r.approve(t)
	svc := newChangeRequestServiceWithDecisionForTest(t, r.env)
	child, err := enrollmentAPI.ReadOwnerChildForTest(testpkg.Ctx(t), r.env.repos.Enrollment(), r.childID)
	require.NoError(t, err)
	require.NotNil(t, child.TargetGradeLevel)

	editErr, correctErr := whilePhaseStartMoves(t, r, phases, 7, true, func(ctx context.Context) error {
		_, err := svc.CorrectApprovedChildData(ctx, enrollmentAPI.CorrectApprovedChildDataInput{
			RequestID: r.requestID, ChildID: r.childID,
			FirstName: child.FirstName, LastName: "Berichtigt", DateOfBirth: child.DateOfBirth, TargetGradeLevel: child.TargetGradeLevel,
			Reason: "Nachname berichtigt", ActorAccountID: r.reviewerAccountID,
		})
		return err
	})
	require.NoError(t, editErr, "the phase edit must not deadlock")
	require.NoError(t, correctErr, "the correction must finish after the phase edit")

	plan, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.env.db, r.env.repos.Enrollment().ChildWeeklyPlan, r.childID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, enrollmentNative.Date(r.start.AddDays(7)), plan.On)
	r.assertRunningPlan(t, "after the correction")
}
