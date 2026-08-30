package enrollment_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOfferingChangeRequestService_Create_RequiresAndAuditsCompleteWithdrawalConfirmation(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ParentCompleteWithdrawal")
	input := enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections:    nil,
	}

	_, err := svc.Create(offeringChangeAdminContext(t), input)
	require.ErrorIs(t, err, enrollmentService.ErrCompleteWithdrawalConfirmationRequired)
	pending, err := env.repos.OfferingChangeRequest.GetPendingForStudent(testpkg.Ctx(t), fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, pending, "the first unconfirmed attempt must not store a request")

	input.CompleteWithdrawalConfirmed = true
	row, err := svc.Create(offeringChangeAdminContext(t), input)
	require.NoError(t, err)
	assert.True(t, row.CompleteWithdrawalConfirmed)
	assert.Equal(t, env.creatorID, *row.WithdrawalConfirmedBy)
	assert.False(t, row.WithdrawalConfirmedAt.IsZero())
}

func TestDirectOfferingAdjustment_PreviewRejectsCompleteWithdrawalWhenBookingsAreNotAuthoritative(t *testing.T) {
	t.Parallel()

	authoritative := false
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = false
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	direct, ok := svc.(enrollmentService.DirectOfferingAdjustmentCoordinator)
	require.True(t, ok)
	fx := setupOfferingChangeFixture(t, env, "DirectPreviewNonAuthoritativeWithdrawal")
	env.sourcePhase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionAtLeastOne
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	fx.oldOffering.IsRequired = true
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.oldOffering))

	_, err := direct.PreviewDirectOfferingAdjustment(ctx, enrollmentService.DirectOfferingAdjustmentInput{
		StudentID: fx.studentID, EffectiveFrom: fx.switchDate, Selections: []enrollmentService.OfferingChangeSelection{},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Decide_RequiresStaffWithdrawalConfirmation(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "StaffCompleteWithdrawal")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		CompleteWithdrawalConfirmed: true,
	})
	require.NoError(t, err)

	decision := enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID, ActorRole: "admin",
	}
	err = svc.Decide(ctx, decision)
	require.ErrorIs(t, err, enrollmentService.ErrCompleteWithdrawalConfirmationRequired)
	stillPending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, stillPending.Status)
	pendingCompletions, _, err := env.repos.CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{
		StudentID: fx.studentID, Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	assert.Empty(t, pendingCompletions, "a failed approval must not create its completion task")

	decision.CompleteWithdrawalConfirmed = true
	require.NoError(t, svc.Decide(ctx, decision))
	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusApproved, decided.Status)
	pendingCompletions, _, err = env.repos.CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{
		StudentID: fx.studentID, Page: 1, PageSize: 10,
	})
	require.NoError(t, err)
	require.Len(t, pendingCompletions, 1)
	assert.Equal(t, env.creatorID, *pendingCompletions[0].WithdrawalConfirmedBy)
}

func TestOfferingChangeRequestService_Reject_DoesNotCreateWithdrawalCompletion(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RejectCompleteWithdrawal")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		CompleteWithdrawalConfirmed: true,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: false, Reason: "Nicht freigegeben",
		ReviewedBy: env.creatorID, ActorRole: "admin",
	}))
	assertNoPendingWithdrawal(t, env, fx.studentID)
}

func assertNoPendingWithdrawal(t *testing.T, env *decisionTestEnv, studentID int64) {
	t.Helper()
	pending, _, err := env.repos.CareWithdrawal.ListPending(
		t.Context(),
		userModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1},
	)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestOfferingChangeRequestService_Decide_ReportsAppliedWithdrawalResult(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "WithdrawalStateDrift")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		CompleteWithdrawalConfirmed: true,
	})
	require.NoError(t, err)

	// Simulate a request whose materialized target gains a care offering before
	// review. The guardian confirmation remains an immutable submission audit;
	// the decision result must describe what staff actually applies.
	payload := fmt.Sprintf(`{"offerings":[{"offering_id":%d,"selected_days":["mon"]}]}`, fx.newOffering.ID)
	_, err = env.db.NewUpdate().TableExpr("enrollment.offering_change_requests").
		Set("payload = ?::jsonb", payload).Where("id = ?", row.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID, ActorRole: "admin",
	}))
	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.True(t, decided.CompleteWithdrawalConfirmed)
	assert.False(t, decided.ApprovedCompleteWithdrawal)

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view.LastDecision)
	assert.False(t, view.LastDecision.CompleteWithdrawal)
	_, total, err := env.repos.CareWithdrawal.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{
		StudentID: fx.studentID, Page: 1, PageSize: 1,
	})
	require.NoError(t, err)
	assert.Zero(t, total)
}

func TestOfferingChangeRequestService_Decide_RequiresConfirmationAfterTargetBecomesWithdrawal(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "WithdrawalInverseDrift")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)
	assert.False(t, row.CompleteWithdrawalConfirmed)

	_, err = env.db.NewUpdate().TableExpr("enrollment.offering_change_requests").
		Set(`payload = '{"offerings":[]}'::jsonb`).Where("id = ?", row.ID).Exec(ctx)
	require.NoError(t, err)
	preview, err := svc.PreviewDecision(ctx, row.ID, nil, nil)
	require.NoError(t, err, "the required-care preview must allow staff to reach the confirmation step")
	require.NotEmpty(t, preview.Selections)
	decision := enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID, ActorRole: "admin",
	}
	err = svc.Decide(ctx, decision)
	require.ErrorIs(t, err, enrollmentService.ErrCompleteWithdrawalConfirmationRequired)
	stillPending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, stillPending.Status)

	decision.CompleteWithdrawalConfirmed = true
	require.NoError(t, svc.Decide(ctx, decision))
	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.True(t, decided.ApprovedCompleteWithdrawal)
}

func TestOfferingChangeRequestService_GetForStudent_DropsOldWithdrawalAfterCareResumes(t *testing.T) {
	t.Parallel()

	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "WithdrawalStatusResume")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		CompleteWithdrawalConfirmed: true,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID, ActorRole: "admin",
		CompleteWithdrawalConfirmed: true,
	}))

	oldDecision := time.Now().AddDate(0, 0, -30)
	_, err = env.db.NewUpdate().TableExpr("enrollment.offering_change_requests").
		Set("reviewed_at = ?", oldDecision).
		Set("effective_from = ?", timezone.TodayDate().AddDays(-1)).
		Where("id = ?", row.ID).Exec(ctx)
	require.NoError(t, err)
	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view, "the status remains while the completion task is open")

	changed, err := env.repos.CareWithdrawal.MarkObsoleteForRebooking(
		ctx, fx.studentID, timezone.TodayDate().AddDays(-1), time.Now(),
	)
	require.NoError(t, err)
	require.True(t, changed)
	view, err = svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, view, "a resumed-care child must not keep an old withdrawal status forever")
}

// The staff queue has to say what a decision does to the child's whole booking
// picture, not just to the lines that move (#2434): a request that empties the
// list must be recognizable as a Komplett-Abmeldung, and one that keeps other
// offerings must name them.

func pendingViewForRequest(
	t *testing.T,
	svc enrollmentService.OfferingChangeRequestService,
	requestID int64,
) *enrollmentService.OfferingChangeView {
	t.Helper()
	views, _, err := svc.ListPending(offeringChangeAdminContext(t), modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	for _, view := range views {
		if view.Request != nil && view.Request.ID == requestID {
			return view
		}
	}
	t.Fatal("the queue must contain the new request")
	return nil
}

func TestOfferingChangeRequestService_ListPending_MarksFullWithdrawal(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "FullWithdrawal")

	row, err := svc.Create(offeringChangeAdminContext(t), enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections:    nil,
	})
	require.NoError(t, err)

	view := pendingViewForRequest(t, svc, row.ID)
	assert.True(t, view.FullWithdrawal, "an empty offering list is a Komplett-Abmeldung")
	assert.Empty(t, view.Unchanged, "nothing stays booked after a Komplett-Abmeldung")
}

func TestOfferingChangeRequestService_ListPending_MarksRequiredCareWithdrawal(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RequiredFullWithdrawal")
	env.sourcePhase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionAtLeastOne
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	fx.oldOffering.IsRequired = true
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.oldOffering))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		CompleteWithdrawalConfirmed: true,
	})
	require.NoError(t, err)

	view := pendingViewForRequest(t, svc, row.ID)
	assert.True(t, view.FullWithdrawal)
}

func TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "KeepsOther")
	// A second child that holds both offerings, so dropping one leaves one.
	_, _, studentID := submitApprovedAdjustmentChild(
		t, env, "offer-change-keeps-other@example.com", "OfferChangeKeepsOther",
		[]*enrollmentModels.CareOffering{fx.oldOffering, fx.newOffering},
	)

	row, err := svc.Create(offeringChangeAdminContext(t), enrollmentService.CreateOfferingChangeInput{
		StudentID:     studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	view := pendingViewForRequest(t, svc, row.ID)
	assert.False(t, view.FullWithdrawal, "one offering stays booked, so this is no Komplett-Abmeldung")
	require.Len(t, view.Unchanged, 1)
	assert.Equal(t, fx.oldOffering.ID, view.Unchanged[0].OfferingID)
	assert.Equal(t, []string{"mon"}, view.Unchanged[0].NewDays)
	for _, entry := range view.Diff {
		assert.NotEqual(t, fx.oldOffering.ID, entry.OfferingID,
			"an untouched booking does not belong in the changed lines")
	}
}
