package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Restore-flow integration tests (#2157). Same env + fixture strategy as
// the Decide tests: real test DB, submissions created through the public
// Submit flow, withdraw through the parent-facing Withdraw flow — so the
// restore undoes exactly the state production produces.

func listRestorationAuditRows(t *testing.T, env *decisionTestEnv, requestID int64) []*auditModels.EnrollmentRestoration {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var rows []*auditModels.EnrollmentRestoration
	err := env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
		Where(`"enrollment_restoration".request_id = ?`, requestID).
		Scan(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			Model((*auditModels.EnrollmentRestoration)(nil)).
			ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
			Where(`"enrollment_restoration".request_id = ?`, requestID).
			Exec(testpkg.Ctx(t))
	})
	return rows
}

func TestDecisionService_RestoreWithdrawn_HappyPath(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	result := submitDecisionSiblings(t, env, "restore-happy@example.com")
	reqID := result.Request.ID
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	// Precondition: the withdraw stamped everything the restore must undo.
	withdrawnReq, err := enrollmentService.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, err)
	require.NotNil(t, withdrawnReq.WithdrawnAt)

	outcome, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 2)

	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status)
		assert.Nil(t, child.ReviewedAt, "restored child must look un-reviewed")
		assert.Nil(t, child.ReviewedBy)
		assert.Nil(t, child.StatusReason)
	}

	restoredReq, err := enrollmentService.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, err)
	assert.Nil(t, restoredReq.WithdrawnAt, "withdrawn_at must be cleared")

	auditRows := listRestorationAuditRows(t, env, reqID)
	require.Len(t, auditRows, 1)
	require.NotNil(t, auditRows[0].ActorAccountID)
	assert.Equal(t, env.creatorID, *auditRows[0].ActorAccountID)
	assert.ElementsMatch(t, outcome.RestoredChildIDs, auditRows[0].ChildIDs)
	assert.False(t, auditRows[0].RestoredAt.IsZero())
}

func TestDecisionService_RestoreWithdrawn_TerminalSiblingUntouched(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	result := submitDecisionSiblings(t, env, "restore-partial@example.com")
	reqID := result.Request.ID
	rejectedChildID := result.Children[0].ID

	// Admin rejects the first child (terminal), then the parent withdraws
	// the whole request — the withdraw skips the rejected child.
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    rejectedChildID,
		Status:     enrollmentService.DecisionRejected,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	outcome, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 1)
	assert.NotContains(t, outcome.RestoredChildIDs, rejectedChildID)

	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	for _, child := range children {
		if child.ID == rejectedChildID {
			assert.Equal(t, enrollmentModels.ChildStatusRejected, child.Status,
				"pre-withdraw terminal child must stay untouched")
			require.NotNil(t, child.ReviewedBy)
			assert.Equal(t, env.creatorID, *child.ReviewedBy)
			continue
		}
		assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status)
		assert.Nil(t, child.ReviewedAt)
	}

	restoredReq, err := enrollmentService.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, err)
	assert.Nil(t, restoredReq.WithdrawnAt)

	listRestorationAuditRows(t, env, reqID) // registers audit cleanup
}

func TestDecisionService_RestoreWithdrawn_NothingWithdrawn(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, _ := submitOneChild(t, env, "restore-nothing@example.com", "Nia", "Nichts")

	_, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestoreNothingWithdrawn))
}

func TestDecisionService_RestoreWithdrawn_NotFound(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := env.decision.RestoreWithdrawn(ctx, 99_999_999, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound))
}

func TestDecisionService_RestoreWithdrawn_LoadFailurePreservesError(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	// A failed request load (here: canceled context) must keep its cause
	// instead of collapsing into the 404 sentinel — the handler's
	// transient-503/default-500 mapping depends on seeing the real error.
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := env.decision.RestoreWithdrawn(ctx, 1, env.creatorID)
	require.Error(t, err)
	assert.False(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound),
		"a query failure must not surface as request-not-found")
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestDecisionService_RestoreWithdrawn_PhaseInactive(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	result := submitDecisionSiblings(t, env, "restore-inactive@example.com")
	reqID := result.Request.ID
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	env.sourcePhase.IsActive = false
	require.NoError(t, env.repos.Enrollment().UpdatePhase(ctx, enrollmentService.OwnerPhaseForTest(env.sourcePhase)))
	t.Cleanup(func() {
		env.sourcePhase.IsActive = true
		_ = env.repos.Enrollment().UpdatePhase(testpkg.Ctx(t), enrollmentService.OwnerPhaseForTest(env.sourcePhase))
	})

	_, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestorePhaseInactive))

	// Nothing changed: children stay withdrawn, withdrawn_at stays set.
	children, listErr := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), reqID)
	require.NoError(t, listErr)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, child.Status)
	}
}

// newRestoreDecisionServiceForRequestEnv wires the minimal DecisionService
// the restore flow needs on top of the request-test env, so the capacity
// tests can drive Submit/Withdraw through the real request service and then
// restore through the real decision service against the same DB.
func newRestoreDecisionServiceForRequestEnv(t *testing.T, env *requestTestEnv) enrollmentService.DecisionService {
	t.Helper()
	repoFactory := repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db))
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		Requests:             repoFactory.Enrollment(),
		Children:             repoFactory.Enrollment(),
		ApprovedOfferings:    approvedOfferingTestProjection(repoFactory),
		CareOfferingRepo:     repoFactory.CareOffering,
		Phases:               repoFactory.Enrollment(),
		RestorationAuditRepo: repoFactory.EnrollmentRestorationAudit,
		Logger:               slog.Default(),
	})
}

func TestDecisionService_RestoreWithdrawn_WaitlistsOverCapacityChildren(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 1)

	// Family A takes the only slot, then withdraws.
	first := validSubmission(t, env.phaseID)
	first.GuardianEmail = "restore-cap-a@example.com"
	first.Children[0].OfferingIDs = []int64{offering.ID}
	resA, err := env.svc.Submit(ctx, first)
	require.NoError(t, err)
	require.Equal(t, enrollmentModels.ChildStatusSubmitted, resA.Children[0].Status)
	require.NoError(t, env.svc.Withdraw(ctx, resA.Request.StatusToken, 0))

	// Family B claims the freed slot in the meantime.
	second := validSubmission(t, env.phaseID)
	second.GuardianEmail = "restore-cap-b@example.com"
	second.Children[0].FirstName = "Bela"
	second.Children[0].OfferingIDs = []int64{offering.ID}
	resB, err := env.svc.Submit(ctx, second)
	require.NoError(t, err)
	require.Equal(t, enrollmentModels.ChildStatusSubmitted, resB.Children[0].Status)

	decision := newRestoreDecisionServiceForRequestEnv(t, env)
	outcome, err := decision.RestoreWithdrawn(ctx, resA.Request.ID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 1)
	assert.Equal(t, []int64{resA.Children[0].ID}, outcome.WaitlistedChildIDs,
		"the offering is full again, so the restored child must come back waitlisted, not submitted")

	repoFactory := repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db))
	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repoFactory.Enrollment(), resA.Request.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusWaitlisted, children[0].Status)
	assert.Nil(t, children[0].ReviewedAt)
	assert.Nil(t, children[0].ReviewedBy)

	// Family B's claim is untouched.
	othersChildren, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repoFactory.Enrollment(), resB.Request.ID)
	require.NoError(t, err)
	require.Len(t, othersChildren, 1)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, othersChildren[0].Status)
}

func TestDecisionService_RestoreWithdrawn_FreeSlotComesBackSubmitted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 1)

	first := validSubmission(t, env.phaseID)
	first.GuardianEmail = "restore-freeslot@example.com"
	first.Children[0].OfferingIDs = []int64{offering.ID}
	resA, err := env.svc.Submit(ctx, first)
	require.NoError(t, err)
	require.NoError(t, env.svc.Withdraw(ctx, resA.Request.StatusToken, 0))

	// Nobody took the slot: the child's own (withdrawn) claim must not
	// count against itself, so the restore lands on submitted.
	decision := newRestoreDecisionServiceForRequestEnv(t, env)
	outcome, err := decision.RestoreWithdrawn(ctx, resA.Request.ID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 1)
	assert.Empty(t, outcome.WaitlistedChildIDs)

	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db)).Enrollment(), resA.Request.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, children[0].Status)
}

// setOfferingInterval stamps a dated validity interval on a child's
// offering selections, the state an approved dated offering switch leaves
// behind (ValidUntil exclusive).
func setOfferingInterval(t *testing.T, db *bun.DB, requestChildID int64, validFrom, validUntil *timezone.Date) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr(`enrollment.request_child_offerings AS "request_child_offering"`).
		Set(`valid_from = ?`, validFrom).
		Set(`valid_until = ?`, validUntil).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// TestDecisionService_RestoreWithdrawn_DisjointIntervalNotWaitlisted pins
// the capacity gate to each claim's own validity interval (review #2159):
// family A's claim ends (exclusively) on the split date, family B's only
// starts there. The intervals never overlap, so B's full slot must not
// waitlist A's restore even though the offering's whole-window peak sits at
// capacity.
func TestDecisionService_RestoreWithdrawn_DisjointIntervalNotWaitlisted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 1)
	splitDate := timezone.NewDate(2027, 2, 1)

	// Family A holds the slot only until the split date, then withdraws.
	first := validSubmission(t, env.phaseID)
	first.GuardianEmail = "restore-dated-a@example.com"
	first.Children[0].OfferingIDs = []int64{offering.ID}
	resA, err := env.svc.Submit(ctx, first)
	require.NoError(t, err)
	setOfferingInterval(t, env.db, resA.Children[0].ID, nil, &splitDate)
	require.NoError(t, env.svc.Withdraw(ctx, resA.Request.StatusToken, 0))

	// Family B claims the offering from the split date onwards.
	second := validSubmission(t, env.phaseID)
	second.GuardianEmail = "restore-dated-b@example.com"
	second.Children[0].FirstName = "Bela"
	second.Children[0].OfferingIDs = []int64{offering.ID}
	resB, err := env.svc.Submit(ctx, second)
	require.NoError(t, err)
	setOfferingInterval(t, env.db, resB.Children[0].ID, &splitDate, nil)

	decision := newRestoreDecisionServiceForRequestEnv(t, env)
	outcome, err := decision.RestoreWithdrawn(ctx, resA.Request.ID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 1)
	assert.Empty(t, outcome.WaitlistedChildIDs,
		"the claims never overlap, so the restore must come back submitted")

	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db)).Enrollment(), resA.Request.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, children[0].Status)
}

// TestDecisionService_RestoreWithdrawn_OverlappingIntervalsShareCapacity pins
// the follow-up from review #2159: two restored siblings whose validity
// intervals only partially overlap must queue against each other on the
// overlap. Each claim alone fits the free slot, so a gate that keys queued
// counts by identical windows would restore both and overbook the offering
// where the intervals meet — the second child must come back waitlisted.
func TestDecisionService_RestoreWithdrawn_OverlappingIntervalsShareCapacity(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)
	offering := setupCareOfferingForCapacity(t, env, 2)
	overlapFrom := timezone.NewDate(2027, 1, 1)
	overlapUntil := timezone.NewDate(2027, 3, 1)

	// One family, two children on the same offering; their dated intervals
	// overlap only in [overlapFrom, overlapUntil).
	req := validSubmission(t, env.phaseID)
	req.GuardianEmail = "restore-overlap@example.com"
	req.Children[0].OfferingIDs = []int64{offering.ID}
	req.Children = append(req.Children, enrollmentService.SubmitChild{
		FirstName:        "Bela",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2019, 5, 20),
		TargetGradeLevel: testpkg.Int16Ptr(1),
		OfferingIDs:      []int64{offering.ID},
	})
	res, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	require.Len(t, res.Children, 2)
	setOfferingInterval(t, env.db, res.Children[0].ID, nil, &overlapUntil)
	setOfferingInterval(t, env.db, res.Children[1].ID, &overlapFrom, nil)
	require.NoError(t, env.svc.Withdraw(ctx, res.Request.StatusToken, 0))

	// The admin lowered the capacity in the meantime: one slot left, which
	// either interval fits alone but the overlap cannot.
	one := 1
	offering.Capacity = &one
	require.NoError(t, repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db)).CareOffering.Update(ctx, offering))

	decision := newRestoreDecisionServiceForRequestEnv(t, env)
	outcome, err := decision.RestoreWithdrawn(ctx, res.Request.ID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 2)
	require.Len(t, outcome.WaitlistedChildIDs, 1,
		"the overlapping restored claims share the single slot, so exactly one child must be waitlisted")

	children, err := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db)).Enrollment(), res.Request.ID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	statuses := map[string]int{}
	for _, child := range children {
		statuses[child.Status]++
	}
	assert.Equal(t, map[string]int{
		enrollmentModels.ChildStatusSubmitted:  1,
		enrollmentModels.ChildStatusWaitlisted: 1,
	}, statuses)
}

func TestDecisionService_RestoreWithdrawn_RejectModeFailsWhenFull(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowReject)
	offering := setupCareOfferingForCapacity(t, env, 1)

	first := validSubmission(t, env.phaseID)
	first.GuardianEmail = "restore-reject-a@example.com"
	first.Children[0].OfferingIDs = []int64{offering.ID}
	resA, err := env.svc.Submit(ctx, first)
	require.NoError(t, err)
	require.NoError(t, env.svc.Withdraw(ctx, resA.Request.StatusToken, 0))

	second := validSubmission(t, env.phaseID)
	second.GuardianEmail = "restore-reject-b@example.com"
	second.Children[0].FirstName = "Bela"
	second.Children[0].OfferingIDs = []int64{offering.ID}
	_, err = env.svc.Submit(ctx, second)
	require.NoError(t, err)

	decision := newRestoreDecisionServiceForRequestEnv(t, env)
	_, err = decision.RestoreWithdrawn(ctx, resA.Request.ID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingFull),
		"reject-mode phase must refuse the restore instead of overbooking")

	// Nothing changed on the withdrawn request.
	children, listErr := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, repositories.NewFactory(env.db, repositories.NewUnobservedTimetableDependencies(env.db)).Enrollment(), resA.Request.ID)
	require.NoError(t, listErr)
	require.Len(t, children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, children[0].Status)
}

func TestDecisionService_RestoreWithdrawn_BlockedByActiveDuplicate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// The production scenario from #2156: withdraw, then re-submit the same
	// child. Restoring the withdrawn request would create a second active
	// request for the same child in the phase.
	first := submitDecisionSiblings(t, env, "restore-dup@example.com")
	require.NoError(t, env.requestSvc.Withdraw(ctx, first.Request.StatusToken, 0))
	_, _ = submitOneChild(t, env, "restore-dup@example.com", "Lina", "Digest")

	_, err := env.decision.RestoreWithdrawn(ctx, first.Request.ID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestoreDuplicateActive))

	// The withdrawn request is untouched.
	children, listErr := enrollmentService.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), first.Request.ID)
	require.NoError(t, listErr)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, child.Status)
	}
	req, findErr := enrollmentService.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), first.Request.ID)
	require.NoError(t, findErr)
	assert.NotNil(t, req.WithdrawnAt)
}
