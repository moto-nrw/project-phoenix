package contracttest_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Every write that copies the phase's bounds reads them with Enrollment's
// one phase bounds reader: in its transaction, behind the recurrence gate
// the phase edit holds (#6657). A writer that read the phase before an edit
// and writes after it copies the new bounds; an edit that starts while the
// writer is open waits and then moves what the writer wrote. The care exit
// restore derives the bounds that were the phase's from the phase as it is
// now. The env's phase runs from 2026-09-01 to 2027-07-31.

// recurrenceGate is the Timetable recurrence gate the root binds.
func recurrenceGate(t *testing.T, env *decisionTestEnv) func(context.Context) error {
	t.Helper()
	return owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites
}

// gatedIntake is the env's intake with the recurrence gate, as the root
// binds it.
func gatedIntake(t *testing.T, env *decisionTestEnv) *enrollmentTest.Intake {
	t.Helper()
	return gatedIntakeWith(t, env, nil)
}

// gatedIntakeWith is gatedIntake whose parent-change mark, the last write
// of a form edit, runs through parentChanges.
func gatedIntakeWith(t *testing.T, env *decisionTestEnv, parentChanges parentChangeMarks) *enrollmentTest.Intake {
	t.Helper()
	schools := newFactorySchools(t, env.db)
	offerings := enrollmentTest.NewCareOfferingRecords(env.repos.CarePlan())
	return owners.NewEnrollmentIntake(owners.EnrollmentIntakeSources{
		Bookings:               api.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:               env.reposEnrollment.Enrollment(),
		Children:               env.reposEnrollment.Enrollment(),
		CareOfferingRepo:       offerings,
		Capacity:               owners.NewEnrollmentOfferingCapacity(offerings, env.reposEnrollment.Enrollment(), env.settings),
		Catalog:                env.reposEnrollment.Enrollment(),
		SchoolRepo:             schools,
		Notifications:          testNotifications(env.reposEnrollment.Enrollment(), env.settings, schools),
		RateLimitRepo:          env.reposEnrollment.Enrollment(),
		LateInviteRepo:         env.reposEnrollment.Enrollment(),
		OutboxEnqueuer:         &recordingMailOutbox{},
		Settings:               env.settings,
		FrontendURL:            "http://localhost:3000",
		Logger:                 slog.Default(),
		LockTemplateRecurrence: recurrenceGate(t, env),
		ParentChanges:          parentChanges,
	})
}

// parentChangeMarks is the intake's port that marks a request as changed by
// the parents.
type parentChangeMarks interface {
	MarkRequestParentChanged(ctx context.Context, requestID int64) error
}

// pausedParentChanges marks the request as changed and then holds the form
// edit's transaction open: it reports the transaction's server process and
// waits for release.
type pausedParentChanges struct {
	inner   parentChangeMarks
	reached chan int
	release chan struct{}
}

func (p pausedParentChanges) MarkRequestParentChanged(ctx context.Context, requestID int64) error {
	if err := p.inner.MarkRequestParentChanged(ctx, requestID); err != nil {
		return err
	}
	ambient, _ := tenant.TransactionFromContext(ctx)
	idb, ok := ambient.(bun.IDB)
	if !ok {
		return errors.New("the form edit runs without a transaction")
	}
	var pid int
	if err := idb.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
		return err
	}
	p.reached <- pid
	<-p.release
	return nil
}

// gatedChangeRequests are the env's change requests with the gates the
// root binds: the approval's booking gates and the phase bounds reader's.
func gatedChangeRequests(t *testing.T, env *decisionTestEnv) enrollmentTest.ChangeRequests {
	t.Helper()
	gate := recurrenceGate(t, env)
	bookings := newBookingsForTest(env, nil, gate, nil)
	decisions := composeDecisions(t, env.db, env.reposEnrollment, env.repos, env.reposPeople, env.reposSchedules, bookings, nil, gate, nil)
	schools := newFactorySchools(t, env.db)
	offerings := enrollmentTest.NewCareOfferingRecords(env.repos.CarePlan())
	return owners.NewEnrollmentChangeRequests(owners.EnrollmentChangeRequestSources{
		Bookings:               api.NewEnrollmentCareBookingCommands(carePlanTest.NewOfferingBookings()),
		Requests:               env.reposEnrollment.Enrollment(),
		Children:               env.reposEnrollment.Enrollment(),
		Guardians:              env.reposEnrollment.Enrollment(),
		LateInviteRepo:         env.reposEnrollment.Enrollment(),
		CareOfferingRepo:       offerings,
		Capacity:               owners.NewEnrollmentOfferingCapacity(offerings, env.reposEnrollment.Enrollment(), env.settings),
		Catalog:                env.reposEnrollment.Enrollment(),
		Notifications:          testNotifications(env.reposEnrollment.Enrollment(), env.settings, schools),
		GuardianProfileRepo:    env.reposPeople.GuardianProfile,
		GuardianPhoneRepo:      env.reposPeople.GuardianPhoneNumber,
		StudentRepo:            env.reposPeople.Student,
		GuardianHold:           owners.NewEnrollmentGuardianHold(testpkg.MustDatabase(env.db), env.reposPeople.StudentGuardian, env.reposPeople.Student, env.reposPeople.GuardianProfile, nil),
		Decisions:              decisions,
		BookingGates:           bookings,
		Settings:               env.settings,
		OutboxEnqueuer:         &recordingMailOutbox{},
		FrontendURL:            "http://localhost:3000",
		ParentsURL:             "http://parents.localhost:3000",
		Logger:                 slog.Default(),
		LockTemplateRecurrence: gate,
	})
}

// boundsSubmission is a family's submission of one child with the offering.
func boundsSubmission(t *testing.T, env *decisionTestEnv, offeringID int64, email, firstName string) enrollmentTest.SubmitRequest {
	t.Helper()
	grade := int16(2)
	return enrollmentTest.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Quelle", GuardianEmail: email,
		ConsentFlags: consentFlags(t),
		Children: []enrollmentTest.SubmitChild{{
			FirstName: firstName, LastName: "Quelle", DateOfBirth: calendar.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade, OfferingIDs: []int64{offeringID},
		}},
	}
}

// approveIntoList approves the submitted child and derives the offering's
// sourced list from its bookings, the way approveSourcedChild does.
func approveIntoList(t *testing.T, env *decisionTestEnv, offering *offeringContract.CareOfferingRecord, requestID, childID int64, name string) *sourcedChild {
	t.Helper()
	ctx := testpkg.Ctx(t)
	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t), uniqueSchemaName(t.Name()+name), start.AddDays(-31), end.AddDays(31))
	outcome, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, enrollmentTest.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentTest.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	template := createSourcedTemplate(t, env, name+"Termin", offering.ID, nil, period.ID)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID: template.ID, OfferingIDs: []int64{offering.ID}, CalendarPeriodID: &period.ID, EffectiveFrom: offeringResyncToday,
	}))
	return &sourcedChild{env: env, offering: offering, studentID: *outcome.Child.CreatedStudentID, childID: childID, template: template, start: start, end: end}
}

// backendPID is the server process of the transaction.
func backendPID(t *testing.T, ctx context.Context, tx bun.Tx) int {
	t.Helper()
	var pid int
	require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid))
	return pid
}

// editHeldOpen runs the phase edit in a transaction and keeps it open
// until write, started then, waits for it. It fails when write finishes
// without waiting.
func editHeldOpen(t *testing.T, env *decisionTestEnv, ctx context.Context, moved *enrollmentTest.Phase, write func(context.Context) error) error {
	t.Helper()
	phases := boundsPhases(t, env, recurrenceGate(t, env))
	done := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if err := phaseEdit(phases)(holdCtx, moved); err != nil {
			return err
		}
		pid := backendPID(t, holdCtx, tx)
		go func() { done <- write(ctx) }()
		if waiting, err := waitUntilBlockedBy(t, holdCtx, env.db, pid, done); !waiting {
			return errors.Join(errors.New("the write did not wait for the open phase edit"), err)
		}
		return nil
	})
	require.NoError(t, holdErr)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("the write did not finish")
		return nil
	}
}

// writeHeldOpen runs write in a transaction and keeps it open until the
// phase edit, started then, waits for it.
func writeHeldOpen(t *testing.T, env *decisionTestEnv, ctx context.Context, moved *enrollmentTest.Phase, write func(context.Context) error) {
	t.Helper()
	phases := boundsPhases(t, env, recurrenceGate(t, env))
	edit := make(chan error, 1)
	holdErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(holdCtx context.Context, tx bun.Tx) error {
		if err := write(holdCtx); err != nil {
			return err
		}
		pid := backendPID(t, holdCtx, tx)
		go func() {
			edit <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
				return phaseEdit(phases)(editCtx, moved)
			})
		}()
		if waiting, err := waitUntilBlockedBy(t, holdCtx, env.db, pid, edit); !waiting {
			return errors.Join(errors.New("the phase edit did not wait for the open write"), err)
		}
		return nil
	})
	require.NoError(t, holdErr)
	select {
	case err := <-edit:
		require.NoError(t, err, "the edit works on the committed write")
	case <-ctx.Done():
		t.Fatal("the phase edit did not finish")
	}
}

// requireSingleChild returns the only child of a submission result.
func requireSingleChild(t *testing.T, result *enrollmentTest.SubmitResult) (requestID, childID int64) {
	t.Helper()
	require.NotNil(t, result)
	require.Len(t, result.Children, 1)
	return result.Request.ID, result.Children[0].ID
}

// A submission that read the phase before an edit and writes while the
// edit is open waits at the recurrence gate and copies the edited bounds;
// the list derived after the approval follows them too.
func TestSubmit_WaitsForAnOpenPhaseEditAndCopiesItsBounds(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	offering := createSourceOffering(t, env, "EingereichtNach", nil)
	var result *enrollmentTest.SubmitResult
	err := editHeldOpen(t, env, ctx, movedSourcePhase(env, -7, 14), func(writeCtx context.Context) error {
		return testpkg.WithinTenantContext(t, writeCtx, env.db, testpkg.Tenant(t), func(txCtx context.Context) error {
			var submitErr error
			result, submitErr = intake.Submit(txCtx, boundsSubmission(t, env, offering.ID, "eingereicht-nach@example.com", "Nach"))
			return submitErr
		})
	})
	require.NoError(t, err)
	requestID, childID := requireSingleChild(t, result)

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), end.AddDays(15)}}, childBookings(t, env, childID), "the booking copies the edited phase")
	c := approveIntoList(t, env, offering, requestID, childID, "EingereichtNach")
	c.assertRoster(t, start.AddDays(-7), end.AddDays(15), "after the approval")
}

// A phase edit that starts while a submission is open waits for it and then
// moves the submitted booking.
func TestSubmit_OpenSubmissionMakesThePhaseEditWait(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	offering := createSourceOffering(t, env, "EingereichtVor", nil)
	var result *enrollmentTest.SubmitResult
	writeHeldOpen(t, env, ctx, movedSourcePhase(env, -7, 14), func(holdCtx context.Context) error {
		var err error
		result, err = intake.Submit(holdCtx, boundsSubmission(t, env, offering.ID, "eingereicht-vor@example.com", "Vor"))
		return err
	})
	requestID, childID := requireSingleChild(t, result)

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), end.AddDays(15)}}, childBookings(t, env, childID), "the edit moves the submitted booking")
	c := approveIntoList(t, env, offering, requestID, childID, "EingereichtVor")
	c.assertRoster(t, start.AddDays(-7), end.AddDays(15), "after the approval")
}

// An edit of the submitted form that runs while a phase edit is open waits
// for it before it locks the request, and copies the edited bounds.
func TestReplaceEditable_WaitsForAnOpenPhaseEditAndCopiesItsBounds(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	offering := createSourceOffering(t, env, "BearbeitetNach", nil)
	submission := boundsSubmission(t, env, offering.ID, "bearbeitet-nach@example.com", "Nach")
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, intake.Submit, submission)
	require.NoError(t, err)

	var result *enrollmentTest.SubmitResult
	err = editHeldOpen(t, env, ctx, movedSourcePhase(env, 7, -14), func(writeCtx context.Context) error {
		var replaceErr error
		result, replaceErr = intake.ReplaceEditable(writeCtx, submitted.Request.StatusToken, submission)
		return replaceErr
	})
	require.NoError(t, err)
	requestID, childID := requireSingleChild(t, result)

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(7), end.AddDays(-13)}}, childBookings(t, env, childID), "the replaced booking copies the edited phase")
	c := approveIntoList(t, env, offering, requestID, childID, "BearbeitetNach")
	c.assertRoster(t, start.AddDays(7), end.AddDays(-13), "after the approval")
}

// A phase edit that starts while a form edit is open waits for it and then
// moves the replaced booking.
func TestReplaceEditable_OpenEditMakesThePhaseEditWait(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	offering := createSourceOffering(t, env, "BearbeitetVor", nil)
	submission := boundsSubmission(t, env, offering.ID, "bearbeitet-vor@example.com", "Vor")
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, intake.Submit, submission)
	require.NoError(t, err)

	pause := pausedParentChanges{inner: env.reposEnrollment.Enrollment(), reached: make(chan int, 1), release: make(chan struct{})}
	pausedIntake := gatedIntakeWith(t, env, pause)
	var result *enrollmentTest.SubmitResult
	replaced := make(chan error, 1)
	go func() {
		var err error
		result, err = pausedIntake.ReplaceEditable(ctx, submitted.Request.StatusToken, submission)
		replaced <- err
	}()
	var pid int
	select {
	case pid = <-pause.reached:
	case err := <-replaced:
		t.Fatalf("the form edit finished before its pause: %v", err)
	case <-ctx.Done():
		t.Fatal("the form edit did not reach its pause")
	}
	edit := make(chan error, 1)
	go func() {
		edit <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(editCtx context.Context) error {
			return phaseEdit(boundsPhases(t, env, recurrenceGate(t, env)))(editCtx, movedSourcePhase(env, 7, -14))
		})
	}()
	waiting, editErr := waitUntilBlockedBy(t, ctx, env.db, pid, edit)
	close(pause.release)
	require.NoError(t, <-replaced)
	if waiting {
		editErr = <-edit
	} else {
		t.Error("the phase edit did not wait for the open form edit")
	}
	require.NoError(t, editErr, "the edit works on the committed form edit")
	requestID, childID := requireSingleChild(t, result)

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(7), end.AddDays(-13)}}, childBookings(t, env, childID), "the edit moves the replaced booking")
	c := approveIntoList(t, env, offering, requestID, childID, "BearbeitetVor")
	c.assertRoster(t, start.AddDays(7), end.AddDays(-13), "after the approval")
}

// proposePhoneChange files the family's change request for the submitted
// child: a new phone number, the offering stays.
func proposePhoneChange(t *testing.T, env *decisionTestEnv, changeRequests enrollmentTest.ChangeRequests, requestID int64, token string, submission enrollmentTest.SubmitRequest, childID int64) int64 {
	t.Helper()
	// A family files change requests once the school has looked at the
	// child; the child is not approved yet, so the approval replaces its
	// bookings over the phase window.
	_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.decision.Decide, enrollmentTest.DecideInput{
		RequestID: requestID, ChildID: childID, Status: enrollmentTest.DecisionUnderReview, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	proposed := submission
	proposed.Children = []enrollmentTest.SubmitChild{submission.Children[0]}
	proposed.Children[0].ID = childID
	proposed.GuardianPhone = new("0201 123456")
	created, err := changeRequests.Propose(testpkg.Ctx(t), token, enrollmentTest.CreateChangeRequestInput{Submission: proposed, ParentNote: "Neue Telefonnummer."})
	require.NoError(t, err)
	return created.ChangeRequest.ID
}

// The approval of a change request writes the child's bookings over the
// phase window it read behind the gates: an approval that runs while a
// phase edit is open waits and copies the edited bounds.
func TestChangeRequestApproval_WaitsForAnOpenPhaseEditAndCopiesItsBounds(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	changeRequests := gatedChangeRequests(t, env)
	offering := createSourceOffering(t, env, "AntragNach", nil)
	submission := boundsSubmission(t, env, offering.ID, "antrag-nach@example.com", "Nach")
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, intake.Submit, submission)
	require.NoError(t, err)
	requestID, childID := requireSingleChild(t, submitted)
	changeRequestID := proposePhoneChange(t, env, changeRequests, requestID, submitted.Request.StatusToken, submission, childID)

	err = editHeldOpen(t, env, ctx, movedSourcePhase(env, -7, 14), func(writeCtx context.Context) error {
		return testpkg.WithinTenantContext(t, writeCtx, env.db, testpkg.Tenant(t), func(txCtx context.Context) error {
			_, approveErr := changeRequests.Approve(txCtx, changeRequestID, enrollmentTest.ReviewChangeRequestInput{ActorAccountID: env.creatorID, ActorRole: "admin"})
			return approveErr
		})
	})
	require.NoError(t, err)

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), end.AddDays(15)}}, childBookings(t, env, childID), "the approved booking copies the edited phase")
	c := approveIntoList(t, env, offering, requestID, childID, "AntragNach")
	c.assertRoster(t, start.AddDays(-7), end.AddDays(15), "after the approval")
}

// A phase edit that starts while a change request's approval is open waits
// for it and then moves the approved booking.
func TestChangeRequestApproval_OpenApprovalMakesThePhaseEditWait(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	intake := gatedIntake(t, env)
	changeRequests := gatedChangeRequests(t, env)
	offering := createSourceOffering(t, env, "AntragVor", nil)
	submission := boundsSubmission(t, env, offering.ID, "antrag-vor@example.com", "Vor")
	submitted, err := testpkg.TenantQuery1(t, ctx, env.db, intake.Submit, submission)
	require.NoError(t, err)
	requestID, childID := requireSingleChild(t, submitted)
	changeRequestID := proposePhoneChange(t, env, changeRequests, requestID, submitted.Request.StatusToken, submission, childID)

	writeHeldOpen(t, env, ctx, movedSourcePhase(env, -7, 14), func(holdCtx context.Context) error {
		_, err := changeRequests.Approve(holdCtx, changeRequestID, enrollmentTest.ReviewChangeRequestInput{ActorAccountID: env.creatorID, ActorRole: "admin"})
		return err
	})

	start, end := calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate)
	assert.Equal(t, []bookingWindow{{start.AddDays(-7), end.AddDays(15)}}, childBookings(t, env, childID), "the edit moves the approved booking")
	c := approveIntoList(t, env, offering, requestID, childID, "AntragVor")
	c.assertRoster(t, start.AddDays(-7), end.AddDays(15), "after the approval")
}

// restoreFlow is a care end of an approved child through the lifecycle and
// the routes' workflow, on the decision suites' fixed day.
type restoreFlow struct {
	*sourcedChild
	lifecycle careplan.CareLifecycle
	workflow  careExitRestoration
	actorID   int64
}

func newRestoreFlow(t *testing.T, env *decisionTestEnv, name string) restoreFlow {
	t.Helper()
	lifecycle := newCareLifecycleServiceAt(t, env.db, decisionTestToday)
	return restoreFlow{
		sourcedChild: approveSourcedChild(t, env, name),
		lifecycle:    lifecycle,
		workflow:     api.NewCarePlanRestoration(lifecycle, env.bookings, func() calendar.Date { return decisionTestToday }),
		actorID:      careActor(t, env.db),
	}
}

func (f restoreFlow) exitInput(lastCareDay calendar.Date) careplan.CareExitInput {
	return careplan.CareExitInput{StudentIDs: []int64{f.studentID}, LastCareDay: lastCareDay, Reason: careplan.CareExitReasonMovedAway}
}

// endCareOn plans the care end through the workflow, as the routes do; a
// second call changes the last day, which restores first.
func (f restoreFlow) endCareOn(t *testing.T, lastCareDay calendar.Date) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	preview, err := testpkg.TenantQuery1(t, ctx, f.env.db, f.lifecycle.Preview, f.exitInput(lastCareDay))
	require.NoError(t, err)
	require.False(t, preview.Blocked, "preview must not be blocked: %+v", preview.Students)
	_, err = testpkg.TenantQuery3(t, ctx, f.env.db, f.workflow.Confirm, preview.Token, f.exitInput(lastCareDay), f.actorID)
	require.NoError(t, err)
}

func (f restoreFlow) cancel(t *testing.T, ctx context.Context) error {
	t.Helper()
	_, err := testpkg.TenantQuery2(t, ctx, f.env.db, f.workflow.Cancel, []int64{f.studentID}, f.actorID)
	return err
}

func (f restoreFlow) editPhase(t *testing.T, startDays, endDays int) {
	t.Helper()
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), f.env.db, phaseEdit(boundsPhases(t, f.env, recurrenceGate(t, f.env))), movedSourcePhase(f.env, startDays, endDays)))
}

// A capped booking gets the phase's current end back when the care end is
// cancelled after the phase was extended; the list follows. A shorter end
// is followed the same way.
func TestCareExitCancel_CappedBookingTakesTheCurrentPhaseEnd(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name               string
		startDays, endDays int
	}{{"FrueherLaenger", -7, 14}, {"SpaeterKuerzer", 7, -14}} {
		t.Run(tc.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupDecisionTest(t)
			f := newRestoreFlow(t, env, "Gekappt"+tc.name)
			careEnd := f.start.AddDays(30)
			f.endCareOn(t, careEnd)
			require.Equal(t, []bookingWindow{{f.start, careEnd.AddDays(1)}}, childBookings(t, env, f.childID), "the care end caps the booking")

			f.editPhase(t, tc.startDays, tc.endDays)
			from := f.start.AddDays(tc.startDays)
			require.Equal(t, []bookingWindow{{from, careEnd.AddDays(1)}}, childBookings(t, env, f.childID), "the edit moves the start, the care end stays")

			require.NoError(t, f.cancel(t, testpkg.Ctx(t)))
			want := f.end.AddDays(tc.endDays + 1)
			assert.Equal(t, []bookingWindow{{from, want}}, childBookings(t, env, f.childID), "the restored end is the phase's current end")
			f.assertRoster(t, from, want, "after the cancellation")
		})
	}
}

// A booking the care end deleted, the later half of a dated switch, comes
// back with the phase's current end, later or earlier; its switch day
// stays, and the capped earlier half gets its switch day back.
func TestCareExitCancel_DeletedBookingTakesTheCurrentPhaseEnd(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		endDays int
	}{{"Laenger", 14}, {"Kuerzer", -14}} {
		t.Run(tc.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupDecisionTest(t)
			ctx := testpkg.Ctx(t)
			f := newRestoreFlow(t, env, "Geloescht"+tc.name)
			switchDay, careEnd := f.start.AddDays(60), f.start.AddDays(30)
			require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
				ctx, f.childID, enrollmentTest.Date(switchDay), []*enrollmentTest.RequestChildOffering{{CareOfferingID: f.offering.ID}}))
			f.endCareOn(t, careEnd)
			require.Equal(t, []bookingWindow{{f.start, careEnd.AddDays(1)}}, childBookings(t, env, f.childID), "the care end caps the first half and deletes the second")

			f.editPhase(t, -7, tc.endDays)
			require.NoError(t, f.cancel(t, ctx))
			assert.Equal(t, []bookingWindow{{f.start.AddDays(-7), switchDay}, {switchDay, f.end.AddDays(tc.endDays + 1)}}, childBookings(t, env, f.childID),
				"the deleted half comes back with the phase's current end")
			f.assertRoster(t, f.start.AddDays(-7), f.end.AddDays(tc.endDays+1), "after the cancellation")
		})
	}
}

// A dated switch keeps its switch day through the care end and its
// cancellation; only the outer end that was the phase's follows.
func TestCareExitCancel_KeepsTheSwitchDayOfARestoredBooking(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	f := newRestoreFlow(t, env, "Wechsel")
	switchDay, careEnd := f.start.AddDays(30), f.start.AddDays(60)
	require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).ScheduleRequestChildOfferings(
		ctx, f.childID, enrollmentTest.Date(switchDay), []*enrollmentTest.RequestChildOffering{{CareOfferingID: f.offering.ID}}))
	f.endCareOn(t, careEnd)
	f.editPhase(t, 0, 7)

	require.NoError(t, f.cancel(t, ctx))
	assert.Equal(t, []bookingWindow{{f.start, switchDay}, {switchDay, f.end.AddDays(8)}}, childBookings(t, env, f.childID),
		"the switch day stays, the phase end follows")
}

// Changing the last care day restores first and ends again. The restore
// before the new end takes the phase's current end, so a later
// cancellation restores it too.
func TestCareExitReconfirm_RestoresTheCurrentPhaseEndBeforeTheNewEnd(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	f := newRestoreFlow(t, env, "Neu")
	f.endCareOn(t, f.start.AddDays(30))
	f.editPhase(t, 0, 14)
	laterEnd := f.start.AddDays(60)
	f.endCareOn(t, laterEnd)
	require.Equal(t, []bookingWindow{{f.start, laterEnd.AddDays(1)}}, childBookings(t, env, f.childID), "the new end caps the restored booking")

	require.NoError(t, f.cancel(t, testpkg.Ctx(t)))
	assert.Equal(t, []bookingWindow{{f.start, f.end.AddDays(15)}}, childBookings(t, env, f.childID), "the cancellation restores the current phase end")
	f.assertRoster(t, f.start, f.end.AddDays(15), "after the cancellation")
}

// A cancellation that runs while a phase edit is open waits for it at the
// recurrence gate and restores the edited end.
func TestCareExitCancel_WaitsForAnOpenPhaseEdit(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	f := newRestoreFlow(t, env, "Gleichzeitig")
	f.endCareOn(t, f.start.AddDays(30))

	require.NoError(t, editHeldOpen(t, env, ctx, movedSourcePhase(env, 0, 14), f.cancelFrom(t)))
	assert.Equal(t, []bookingWindow{{f.start, f.end.AddDays(15)}}, childBookings(t, env, f.childID))
	f.assertRoster(t, f.start, f.end.AddDays(15), "after the cancellation and the edit")
}

// A phase edit that starts while a cancellation is open waits for it and
// then moves the restored booking.
func TestCareExitCancel_OpenCancellationMakesThePhaseEditWait(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 30*time.Second)
	defer cancel()
	f := newRestoreFlow(t, env, "Zuerst")
	f.endCareOn(t, f.start.AddDays(30))

	writeHeldOpen(t, env, ctx, movedSourcePhase(env, 0, 14), func(holdCtx context.Context) error {
		_, err := f.workflow.Cancel(holdCtx, []int64{f.studentID}, f.actorID)
		return err
	})
	assert.Equal(t, []bookingWindow{{f.start, f.end.AddDays(15)}}, childBookings(t, env, f.childID))
	f.assertRoster(t, f.start, f.end.AddDays(15), "after the cancellation and the edit")
}

func (f restoreFlow) cancelFrom(t *testing.T) func(context.Context) error {
	return func(ctx context.Context) error {
		return testpkg.WithinTenantContext(t, ctx, f.env.db, testpkg.Tenant(t), func(txCtx context.Context) error {
			_, err := f.workflow.Cancel(txCtx, []int64{f.studentID}, f.actorID)
			return err
		})
	}
}
