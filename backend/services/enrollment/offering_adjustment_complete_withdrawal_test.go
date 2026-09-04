package enrollment_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type failingCareWithdrawalReconciler struct{}

func pendingWithdrawalForStudent(
	ctx context.Context,
	repo userModels.CareWithdrawalCompletionRepository,
	studentID int64,
) (*userModels.CareWithdrawalCompletion, error) {
	rows, _, err := repo.ListPending(ctx, userModels.CareWithdrawalCompletionFilter{StudentID: studentID, Page: 1, PageSize: 1})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (failingCareWithdrawalReconciler) ReconcileAuthoritativeBookingChange(
	context.Context,
	userModels.CareWithdrawalBookingChange,
) error {
	return errors.New("forced durable completion failure")
}

type completeWithdrawalFixture struct {
	t                  *testing.T
	env                *decisionTestEnv
	ctx                context.Context
	requestID, childID int64
	studentID          int64
	careID, lunchID    int64
	baseInput          enrollmentService.UpdateChildOfferingsInput
}

func TestDecisionService_CompleteWithdrawalRequiresConfirmationAndPersistsTaskAtomically(t *testing.T) {
	t.Parallel()
	fixture := newCompleteWithdrawalFixture(t)
	fixture.assertUnconfirmedChangeRollsBack()
	fixture.assertTaskFailureRollsBack()
	pending := fixture.completeWithdrawal()
	fixture.assertCompletionAudit()
	fixture.assertCareExitCompletesSource(pending)
}

func newCompleteWithdrawalFixture(t *testing.T) *completeWithdrawalFixture {
	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	t.Cleanup(cleanup)
	ctx := testpkg.Ctx(t)
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-10))
	requestID, childID := submitOneChild(t, env, "complete-withdrawal@example.com", "Lina", "Abmeldung")
	care := createWithdrawalOffering(t, env, "Ganztag", true)
	lunch := createWithdrawalOffering(t, env, "Mittagessen", false)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	fixture := &completeWithdrawalFixture{t: t, env: env, ctx: ctx, requestID: requestID, childID: childID,
		studentID: *outcome.Child.CreatedStudentID, careID: care.ID, lunchID: lunch.ID}
	fixture.baseInput = enrollmentService.UpdateChildOfferingsInput{RequestID: requestID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin", Reason: "Betreuung geändert"}
	withCare := fixture.baseInput
	withCare.Offerings = withdrawalSelection(care.ID)
	require.NoError(t, fixture.apply(env.decision, withCare))
	return fixture
}

func createWithdrawalOffering(t *testing.T, env *decisionTestEnv, name string, countsAsCare bool) *enrollmentModels.CareOffering {
	return createAdjustmentCareOfferingWith(t, env, name, func(offering *enrollmentModels.CareOffering) {
		offering.CountsAsCare = countsAsCare
		offering.CountsAsCareSet = true
	})
}

func withdrawalSelection(offeringID int64) []enrollmentService.OfferingAdjustmentSelection {
	return []enrollmentService.OfferingAdjustmentSelection{{OfferingID: offeringID, SelectedDays: []string{"mon"}}}
}

func (f *completeWithdrawalFixture) apply(decision enrollmentService.DecisionService, input enrollmentService.UpdateChildOfferingsInput) error {
	return testpkg.WithTenantTx(f.t, f.ctx, f.env.db, testpkg.Tenant(f.t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := decision.UpdateChildOfferings(txCtx, input)
		return err
	})
}

func (f *completeWithdrawalFixture) withoutCareInput() enrollmentService.UpdateChildOfferingsInput {
	input := f.baseInput
	input.Offerings = withdrawalSelection(f.lunchID)
	return input
}

func (f *completeWithdrawalFixture) assertUnconfirmedChangeRollsBack() {
	err := f.apply(f.env.decision, f.withoutCareInput())
	require.ErrorIs(f.t, err, enrollmentService.ErrCompleteWithdrawalConfirmationRequired)
	links, err := f.env.repos.RequestChildOffering.ListByRequestChildID(f.ctx, f.childID)
	require.NoError(f.t, err)
	require.Len(f.t, links, 1, "the warning response must roll the booking mutation back")
	pending, err := pendingWithdrawalForStudent(f.ctx, f.env.repos.CareWithdrawal, f.studentID)
	require.NoError(f.t, err)
	assert.Nil(f.t, pending)
}

func (f *completeWithdrawalFixture) assertTaskFailureRollsBack() {
	authoritative := true
	failing := newDecisionServiceForTestWithCareWithdrawal(f.env.rolloverTestEnv,
		stubActivationSettings{bookingsAuthoritative: &authoritative}, nil, failingCareWithdrawalReconciler{})
	input := f.withoutCareInput()
	input.CompleteWithdrawalConfirmed = true
	err := f.apply(failing, input)
	require.ErrorContains(f.t, err, "forced durable completion failure")
	links, err := f.env.repos.RequestChildOffering.ListByRequestChildID(f.ctx, f.childID)
	require.NoError(f.t, err)
	require.Len(f.t, links, 1, "a failed durable task must roll the booking mutation back")
	assert.Equal(f.t, f.careID, links[0].CareOfferingID)
}

func (f *completeWithdrawalFixture) completeWithdrawal() *userModels.CareWithdrawalCompletion {
	input := f.withoutCareInput()
	input.CompleteWithdrawalConfirmed = true
	require.NoError(f.t, f.apply(f.env.decision, input))
	links, err := f.env.repos.RequestChildOffering.ListByRequestChildID(f.ctx, f.childID)
	require.NoError(f.t, err)
	require.Len(f.t, links, 1)
	assert.Equal(f.t, f.lunchID, links[0].CareOfferingID)
	pending, err := pendingWithdrawalForStudent(f.ctx, f.env.repos.CareWithdrawal, f.studentID)
	require.NoError(f.t, err)
	require.NotNil(f.t, pending)
	assert.Equal(f.t, decisionTestToday, pending.FirstBookinglessDay)
	assert.Equal(f.t, "admin", pending.WithdrawalConfirmedRole)
	require.NotNil(f.t, pending.SourceRequestChildID)
	assert.Equal(f.t, f.childID, *pending.SourceRequestChildID)
	return pending
}

func (f *completeWithdrawalFixture) assertCompletionAudit() {
	rows, err := f.env.repos.EnrollmentOfferingAdjustment.ListByRequestChildID(f.ctx, f.childID)
	require.NoError(f.t, err)
	require.Len(f.t, rows, 2)
	assert.True(f.t, rows[0].CompleteWithdrawalConfirmed)
}

func (f *completeWithdrawalFixture) assertCareExitCompletesSource(pending *userModels.CareWithdrawalCompletion) {
	lifecycle := newWithdrawalLifecycle(f.env)
	input := usersService.CareExitInput{LastCareDay: decisionTestToday.AddDays(-1), Reason: userModels.CareExitReasonNoCareNeed}
	preview, err := lifecycle.PreviewWithdrawalCareEnd(f.ctx, pending.ID, input)
	require.NoError(f.t, err)
	require.Len(f.t, preview.Students, 1)
	require.Len(f.t, preview.Students[0].SourceOfferings, 2)
	assert.Equal(f.t, "Ganztag", preview.Students[0].SourceOfferings[0].Name)
	assert.Equal(f.t, "Mittagessen", preview.Students[0].SourceOfferings[1].Name)
	_, err = lifecycle.ConfirmWithdrawalCareEnd(f.ctx, pending.ID, preview.Token, input, f.env.creatorID)
	require.NoError(f.t, err)
	var validUntil *timezone.Date
	require.NoError(f.t, f.env.db.NewRaw(`SELECT valid_until FROM enrollment.request_child_offerings
		WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?`,
		testpkg.Tenant(f.t), f.childID, f.lunchID).Scan(f.ctx, &validUntil))
	require.NotNil(f.t, validUntil)
	assert.Equal(f.t, decisionTestToday, *validUntil)
}

func newWithdrawalLifecycle(env *decisionTestEnv) usersService.CareLifecycleService {
	return usersService.NewCareLifecycleService(usersService.CareLifecycleDependencies{
		StudentRepo: env.repos.Student, PersonRepo: env.repos.Person,
		CareExitRepo: env.repos.CareExit, CleanupRepo: env.repos.CareExitCleanup,
		WithdrawalRepo: env.repos.CareWithdrawal, TagReleaser: env.repos.GradeTransition,
		AuditService: usersService.NewStudentAuditService(env.repos.StudentFieldEdit, slog.Default()),
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return env.settings.ResolveBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative)
		},
		DB: env.db, Logger: slog.Default(),
	})
}

func TestDecisionService_CompleteWithdrawalIsDisabledWhenBookingsAreNotAuthoritative(t *testing.T) {
	t.Parallel()
	fixture := newNonAuthoritativeWithdrawalFixture(t)
	require.NoError(t, fixture.apply(withdrawalSelection(fixture.careID)))
	require.NoError(t, fixture.apply(nil), "the ordinary correction does not require withdrawal confirmation")
	pending, err := pendingWithdrawalForStudent(fixture.ctx, fixture.env.repos.CareWithdrawal, fixture.studentID)
	require.NoError(t, err)
	assert.Nil(t, pending)
}

func TestDecisionService_ExplicitConfirmationCannotBypassBookingAuthority(t *testing.T) {
	t.Parallel()
	fixture := newNonAuthoritativeWithdrawalFixture(t)
	require.NoError(t, fixture.apply(withdrawalSelection(fixture.careID)))
	require.NoError(t, fixture.applyConfirmed(nil))
	pending, err := pendingWithdrawalForStudent(fixture.ctx, fixture.env.repos.CareWithdrawal, fixture.studentID)
	require.NoError(t, err)
	assert.Nil(t, pending)
}

type nonAuthoritativeWithdrawalFixture struct {
	t                  *testing.T
	env                *decisionTestEnv
	ctx                context.Context
	requestID, childID int64
	studentID, careID  int64
}

func newNonAuthoritativeWithdrawalFixture(t *testing.T) *nonAuthoritativeWithdrawalFixture {
	authoritative := false
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	t.Cleanup(cleanup)
	ctx := testpkg.Ctx(t)
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-10))
	email := fmt.Sprintf("non-authoritative-withdrawal-%d@example.com", testpkg.UniqueSuffix())
	requestID, childID := submitOneChild(t, env, email, "Toni", "Optional")
	care := createWithdrawalOffering(t, env, "Ganztag", true)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	return &nonAuthoritativeWithdrawalFixture{t: t, env: env, ctx: ctx,
		requestID: requestID, childID: childID, studentID: *outcome.Child.CreatedStudentID, careID: care.ID}
}

func (f *nonAuthoritativeWithdrawalFixture) apply(offerings []enrollmentService.OfferingAdjustmentSelection) error {
	return f.applyInput(offerings, false)
}

func (f *nonAuthoritativeWithdrawalFixture) applyConfirmed(offerings []enrollmentService.OfferingAdjustmentSelection) error {
	return f.applyInput(offerings, true)
}

func (f *nonAuthoritativeWithdrawalFixture) applyInput(
	offerings []enrollmentService.OfferingAdjustmentSelection, confirmed bool,
) error {
	return testpkg.WithTenantTx(f.t, f.ctx, f.env.db, testpkg.Tenant(f.t), func(ctx context.Context, _ bun.Tx) error {
		_, err := f.env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
			RequestID: f.requestID, ChildID: f.childID, Offerings: offerings,
			ActorAccountID: f.env.creatorID, ActorRole: "admin", Reason: "Betreuung geändert",
			CompleteWithdrawalConfirmed: confirmed,
		})
		return err
	})
}

type withdrawalRaceFixture struct {
	t                 *testing.T
	env               *decisionTestEnv
	ctx               context.Context
	lifecycle         usersService.CareLifecycleService
	decision          enrollmentService.DecisionService
	recurrenceGate    func(context.Context) error
	completionWaiting chan struct{}
	studentID, careID int64
	originalEnd       *timezone.Date
	input             enrollmentService.UpdateChildOfferingsInput
}

func TestDecisionService_RebookingWinsAgainstConcurrentWithdrawalCompletion(t *testing.T) {
	t.Parallel()
	fixture := newWithdrawalRaceFixture(t)
	pending, preview, exitInput := fixture.prepareWithdrawalRace()
	fixture.runRebookingRace(pending, preview, exitInput)
	fixture.assertRebookingWon()
}

func newWithdrawalRaceFixture(t *testing.T) *withdrawalRaceFixture {
	authoritative := true
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{bookingsAuthoritative: &authoritative})
	t.Cleanup(cleanup)
	fixture := &withdrawalRaceFixture{t: t, env: env, ctx: testpkg.Ctx(t), completionWaiting: make(chan struct{})}
	setSourcePhaseServiceStartDate(t, env, decisionTestToday.AddDays(-10))
	fixture.wireRaceServices(&authoritative)
	fixture.seedRaceStudent()
	return fixture
}

func (f *withdrawalRaceFixture) wireRaceServices(authoritative *bool) {
	f.recurrenceGate = func(ctx context.Context) error {
		return scheduleService.LockTenantRecurrenceWrites(ctx, f.env.db)
	}
	repos := f.env.repos
	f.lifecycle = usersService.NewCareLifecycleService(usersService.CareLifecycleDependencies{
		StudentRepo: repos.Student, PersonRepo: repos.Person,
		CareExitRepo: repos.CareExit, CleanupRepo: repos.CareExitCleanup,
		WithdrawalRepo: repos.CareWithdrawal, TagReleaser: repos.GradeTransition,
		AuditService: usersService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
		LockCareBookingWrites: func(ctx context.Context) error {
			close(f.completionWaiting)
			return f.recurrenceGate(ctx)
		},
		BookingsAuthoritative: func(context.Context) (bool, error) { return *authoritative, nil },
		DB:                    f.env.db, Logger: slog.Default(),
	})
	f.decision = newDecisionServiceForTestWithCareWithdrawal(f.env.rolloverTestEnv,
		stubActivationSettings{bookingsAuthoritative: authoritative}, f.recurrenceGate, f.lifecycle)
}

func (f *withdrawalRaceFixture) seedRaceStudent() {
	requestID, childID := submitOneChild(f.t, f.env, "concurrent-withdrawal@example.com", "Mara", "Wiederbuchung")
	care := createWithdrawalOffering(f.t, f.env, "Ganztag", true)
	outcome, err := f.env.decision.Decide(f.ctx, enrollmentService.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionApproved, ReviewedBy: f.env.creatorID,
	})
	require.NoError(f.t, err)
	f.studentID, f.careID = *outcome.Child.CreatedStudentID, care.ID
	student, err := f.env.repos.Student.FindByID(f.ctx, f.studentID)
	require.NoError(f.t, err)
	f.originalEnd = student.EnrolledUntil
	f.input = enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: f.env.creatorID,
		ActorRole: "admin", Reason: "Betreuung geändert", Offerings: withdrawalSelection(care.ID),
	}
}

func (f *withdrawalRaceFixture) apply(ctx context.Context) error {
	_, err := f.decision.UpdateChildOfferings(ctx, f.input)
	return err
}

func (f *withdrawalRaceFixture) applyInTenantTx() error {
	return testpkg.WithTenantTx(f.t, f.ctx, f.env.db, testpkg.Tenant(f.t), func(ctx context.Context, _ bun.Tx) error {
		return f.apply(ctx)
	})
}

func (f *withdrawalRaceFixture) prepareWithdrawalRace() (
	*userModels.CareWithdrawalCompletion, *usersService.CareExitPreview, usersService.CareExitInput,
) {
	require.NoError(f.t, f.applyInTenantTx())
	f.input.Offerings = nil
	f.input.CompleteWithdrawalConfirmed = true
	require.NoError(f.t, f.applyInTenantTx())
	pending, err := pendingWithdrawalForStudent(f.ctx, f.env.repos.CareWithdrawal, f.studentID)
	require.NoError(f.t, err)
	require.NotNil(f.t, pending)
	exitInput := usersService.CareExitInput{
		LastCareDay: decisionTestToday.AddDays(-1), Reason: userModels.CareExitReasonNoCareNeed,
	}
	preview, err := f.lifecycle.PreviewWithdrawalCareEnd(f.ctx, pending.ID, exitInput)
	require.NoError(f.t, err)
	return pending, preview, exitInput
}

func (f *withdrawalRaceFixture) runRebookingRace(
	pending *userModels.CareWithdrawalCompletion, preview *usersService.CareExitPreview, input usersService.CareExitInput,
) {
	confirmErr := make(chan error, 1)
	err := testpkg.WithTenantTx(f.t, f.ctx, f.env.db, testpkg.Tenant(f.t), func(ctx context.Context, _ bun.Tx) error {
		if err := f.recurrenceGate(ctx); err != nil {
			return err
		}
		go func() {
			_, err := f.lifecycle.ConfirmWithdrawalCareEnd(f.ctx, pending.ID, preview.Token, input, f.env.creatorID)
			confirmErr <- err
		}()
		<-f.completionWaiting
		f.input.Offerings = withdrawalSelection(f.careID)
		f.input.CompleteWithdrawalConfirmed = false
		return f.apply(ctx)
	})
	require.NoError(f.t, err)
	require.ErrorIs(f.t, <-confirmErr, userModels.ErrCareWithdrawalAlreadyResolved)
}

func (f *withdrawalRaceFixture) assertRebookingWon() {
	student, err := f.env.repos.Student.FindByID(f.ctx, f.studentID)
	require.NoError(f.t, err)
	assert.Equal(f.t, f.originalEnd, student.EnrolledUntil, "the stale exit must not land after care was rebooked")
	pending, err := pendingWithdrawalForStudent(f.ctx, f.env.repos.CareWithdrawal, f.studentID)
	require.NoError(f.t, err)
	assert.Nil(f.t, pending)
}
