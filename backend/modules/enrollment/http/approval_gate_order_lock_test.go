package enrollmenthttp_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// whileHoldingGates takes the tenant gates with hold on one connection, runs
// write on a second connection, waits until PostgreSQL reports write blocked
// by the first connection, and then runs then on the first. The project-wide
// order is class-writes, recurrence, row locks (ADR 0162): a write that locked
// an enrollment row before it waited for a gate closes a cycle with then.
func whileHoldingGates(
	t *testing.T,
	db *bun.DB,
	hold, write, then func(context.Context) error,
) (thenErr, writeErr error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if err := hold(holdCtx); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(holdCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), write)
		}()
		if finished := waitForBlockedWriter(t, holdCtx, tx, pid, done); finished != nil {
			done <- *finished
			return fmt.Errorf("the concurrent write finished without waiting for the gates: %w", *finished)
		}
		thenErr = then(holdCtx)
		return thenErr
	})
	select {
	case writeErr = <-done:
	case <-ctx.Done():
		t.Fatalf("the concurrent write did not finish; the holder returned %v", holdErr)
	}
	if thenErr == nil {
		thenErr = holdErr
	}
	return thenErr, writeErr
}

// gatedDecisionTest is setupDecisionTest with the tenant recurrence gate
// wired the way the root binds it for the decision and its bookings.
func gatedDecisionTest(t *testing.T) (*decisionTestEnv, func(context.Context) error) {
	t.Helper()
	testpkg.OwnTenant(t)
	base := setupDecisionTest(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, base.db)).LockRecurrenceWrites
	bookings := newBookingsForTest(base.rolloverTestEnv, nil, lock, nil)
	decision := newDecisionServiceForTestWithBookings(t, base.rolloverTestEnv, bookings, nil, lock, nil, nil)
	return &decisionTestEnv{rolloverTestEnv: base.rolloverTestEnv, decision: decision, bookings: bookings}, lock
}

// A change-request approval and the approval of a sibling child of the same
// request take the gates before the request row. The change request locked
// the parent request first and the recurrence gate later, while Decide
// (approved) holds the gates and then waits for that parent row: 40P01
// (#6043 review round 2). Now the change request waits for the gates and then
// sees the committed sibling decision.
func TestChangeRequestService_Approve_WaitsForSiblingApprovalGates(t *testing.T) {
	t.Parallel()

	env, _ := gatedDecisionTest(t)
	ctx := testpkg.Ctx(t)
	gradeOne, gradeTwo := int16(1), int16(2)
	submission := enrollmentAPI.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Sperre",
		GuardianEmail:     "change-request-gate-order@example.com",
		ConsentFlags: testFormValues(map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		}),
		Children: []enrollmentAPI.SubmitChild{
			{FirstName: "Lina", LastName: "Sperre", DateOfBirth: calendar.NewDate(2018, 4, 15), TargetGradeLevel: &gradeOne},
			{FirstName: "Emil", LastName: "Sperre", DateOfBirth: calendar.NewDate(2019, 5, 20), TargetGradeLevel: &gradeOne},
		},
	}
	result, err := env.requestSvc.Submit(ctx, submission)
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	first, sibling := result.Children[0], result.Children[1]
	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: result.Request.ID, ChildID: first.ID, Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	proposed := submission
	proposed.Children = append([]enrollmentAPI.SubmitChild(nil), submission.Children...)
	proposed.Children[0].ID, proposed.Children[1].ID = first.ID, sibling.ID
	proposed.Children[0].TargetGradeLevel = &gradeTwo
	svc := newChangeRequestServiceWithDecisionForTest(t, env)
	restoreTakeover := liftTakeoverStamp(t, env, result.Request.ID)
	created, err := svc.Create(ctx, result.Request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
		Submission: proposed, ParentNote: "Klassenstufe korrigieren.",
	})
	require.NoError(t, err)
	restoreTakeover()

	decideErr, approveErr := whileHoldingGates(t, env.db,
		env.bookings.LockOfferingDerivedWrites,
		func(writeCtx context.Context) error {
			_, err := svc.Approve(writeCtx, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
				Note: "Freigegeben.", ActorAccountID: env.creatorID, ActorRole: "admin",
			})
			return err
		},
		func(holdCtx context.Context) error {
			_, err := env.decision.Decide(holdCtx, enrollmentAPI.DecideInput{
				RequestID: result.Request.ID, ChildID: sibling.ID, Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
			})
			return err
		},
	)
	require.NoError(t, decideErr, "the sibling approval must not deadlock")
	// The approval waited for the gates, not on the request row, so it read
	// the sibling's committed approval and refuses the now stale proposal.
	require.ErrorIs(t, approveErr, enrollmentNative.ErrChangeRequestConflict,
		"the change-request approval must finish after the sibling approval, without 40P01")

	children, err := enrollmentAPI.ReadOwnerRequestChildrenForTest(testpkg.TenantContext(env.sourcePhase.TenantID), env.repos.Enrollment(), result.Request.ID)
	require.NoError(t, err)
	statuses := map[int64]string{}
	for _, child := range children {
		statuses[child.ID] = child.Status
	}
	assert.Equal(t, enrollmentNative.ChildStatusApproved, statuses[sibling.ID])
}

// The rollover tick takes the gates before its bulk transitions. It moved
// auto-renewed children to submitted first and waited for the recurrence
// gate of the class-switch step later, while a concurrent approval holds the
// gates and waits for such a child: 40P01 rolled back the tenant tick
// (#6043 review round 2).
func TestRolloverService_RunDeadlineWorker_WaitsForApprovalGates(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, db)).LockRecurrenceWrites
	env, decision := setupAutoApproveIntegrationEnvWithDecision(t, nil, lock)
	ctx := testpkg.Ctx(t)
	seedApprovedChildWithStudent(t, env, "Anna", "Sperre", "rollover-gate-order@example.com", "Lina", "Sperre", int16(1))
	req := validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, true)
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "rollover-gate-order-target"
	rolled, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	renewed, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, rolled.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, renewed, 1)

	studentEnrollment := testStudentEnrollment(env.db)
	approveErr, tickErr := whileHoldingGates(t, env.db,
		func(holdCtx context.Context) error {
			if err := studentEnrollment.LockEnrollmentClassWrites(holdCtx); err != nil {
				return err
			}
			return lock(holdCtx)
		},
		func(writeCtx context.Context) error {
			_, err := env.rolloverSvc.RunDeadlineWorker(writeCtx, time.Now())
			return err
		},
		func(holdCtx context.Context) error {
			_, err := decision.Decide(holdCtx, enrollmentAPI.DecideInput{
				RequestID: renewed[0].RequestID, ChildID: renewed[0].ID, Status: enrollmentNative.DecisionApproved,
			})
			return err
		},
	)
	require.NoError(t, approveErr, "the approval must not deadlock")
	require.NoError(t, tickErr, "the tick must finish after the approval")

	approved, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, rolled.Phase.ID, []string{enrollmentNative.ChildStatusApproved})
	require.NoError(t, err)
	assert.Len(t, approved, 1, "the approval wins; the tick leaves the decided child alone")
}
