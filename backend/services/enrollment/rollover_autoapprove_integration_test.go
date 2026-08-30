package enrollment_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// This file exercises the rollover_auto_approve = true path end-to-end:
// the deadline worker routes each auto_renewed row through the real
// DecisionService, whose applyApprovalRollover branch updates the
// existing student record instead of stamping out a duplicate.
//
// The seed path skips the full first-year approval flow (which would
// also exercise person/student creation through DecisionService) and
// pre-builds the source Person + Student + RequestChild directly. We
// only need a source row whose CreatedStudentID points at a real
// student; the test then asserts the rollover updates THAT student
// rather than inserting a second one.

func setupAutoApproveIntegrationEnv(t *testing.T) (*rolloverTestEnv, func()) {
	return setupAutoApproveIntegrationEnvWithSettings(t, nil)
}

func setupAutoApproveIntegrationEnvWithSettings(
	t *testing.T,
	settings enrollmentService.DecisionSettingsResolver,
) (*rolloverTestEnv, func()) {
	t.Helper()
	env, cleanup := setupRolloverTest(t)

	repoFactory := repositories.NewFactory(env.db)

	decision := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		PhaseRepo:                repoFactory.Phase,
		PersonRepo:               repoFactory.Person,
		StaffRepo:                repoFactory.Staff,
		StudentRepo:              repoFactory.Student,
		StudentGuardianRepo:      repoFactory.StudentGuardian,
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		StudentEnrollmentRepo:    repoFactory.StudentEnrollment,
		ActivityGroupRepo:        repoFactory.ActivityGroup,
		ActivityScheduleRepo:     repoFactory.ActivitySchedule,
		CalendarPeriodRepo:       repoFactory.CalendarPeriod,
		TimeframeRepo:            repoFactory.Timeframe,
		ActivityExceptionRepo:    repoFactory.ActivityException,
		AccountRepo:              repoFactory.Account,
		AccountTenantRepo:        repoFactory.AccountTenant,
		AccountRoleRepo:          repoFactory.AccountRole,
		RoleRepo:                 repoFactory.Role,
		OutboxEnqueuer:           env.outbox,
		StudentAudit:             usersService.NewStudentAuditService(repoFactory.StudentFieldEdit, slog.Default()),
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		Settings:                 settings,
		Logger:                   slog.Default(),
	})

	// Rebuild the rollover service so DecisionService is injected.
	env.rolloverSvc = enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                env.repos.Phase,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		OfferingCatalogCloner:    env.offeringCloner,
		OutboxEnqueuer:           env.outbox,
		Settings:                 env.settings,
		DecisionService:          decision,
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
		Logger:                   slog.Default(),
	})

	return env, cleanup
}

// seedApprovedChildWithStudent mirrors the post-approval state the
// rollover auto-approve path needs to see: an approved request_child
// pointing at a real users.students row. We skip the full decision-
// service Apply because the rollover test environment doesn't seed
// the guardian-profile chain the way the production flow does.
func seedApprovedChildWithStudent(
	t *testing.T,
	env *rolloverTestEnv,
	guardianFirst, guardianLast, guardianEmail string,
	childFirst, childLast string,
	grade int16,
) (sourceChild *enrollmentModels.RequestChild, existingStudent *usersModels.Student) {
	t.Helper()
	ctx := testpkg.Ctx(t)

	// 1. Submit + manually approve.
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: guardianFirst,
		GuardianLastName:  guardianLast,
		GuardianEmail:     guardianEmail,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           false,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        childFirst,
				LastName:         childLast,
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: &grade,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	sourceChildID := res.Children[0].ID

	// 2. Create a real Person + Student that mirror the child.
	person := &usersModels.Person{
		FirstName: childFirst,
		LastName:  childLast,
	}
	dob := timezone.NewDate(2018, 4, 15)
	person.Birthday = &dob
	person.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.Person.Create(ctx, person))

	startDate := env.sourcePhase.ServiceStartDate
	endDate := env.sourcePhase.ServiceEndDate
	guardianEmailCopy := guardianEmail
	classFromGrade := classForGrade(grade)
	student := &usersModels.Student{
		PersonID:      person.ID,
		SchoolClass:   classFromGrade,
		Status:        usersModels.StudentStatusActive,
		EnrolledFrom:  &startDate,
		EnrolledUntil: &endDate,
		GuardianEmail: &guardianEmailCopy,
	}
	student.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.Student.Create(ctx, student))

	// 3. Link the source child to the student and stamp approved.
	require.NoError(t, env.repos.RequestChild.LinkCreatedStudent(ctx, sourceChildID, student.ID))
	require.NoError(t, env.repos.RequestChild.UpdateStatus(
		ctx, sourceChildID, enrollmentModels.ChildStatusApproved, nil, env.creatorID,
	))

	loaded, err := env.repos.RequestChild.FindByID(ctx, sourceChildID)
	require.NoError(t, err)
	return loaded, student
}

// classForGrade matches decisionService.gradeToClass — the student
// schema's school_class is free-form text and the decision flow
// today renders the grade number with no suffix (e.g. "2"). Admins
// rename via the student profile UI.
func classForGrade(grade int16) string {
	if grade <= 0 {
		return ""
	}
	return string(rune('0' + grade))
}

// countStudentsForPerson returns how many users.students rows point at
// the given PersonID. StudentRepository has no Count method, so we
// pull the rows via the existing List filter and count the slice.
func countStudentsForPerson(t *testing.T, env *rolloverTestEnv, personID int64) int {
	t.Helper()
	ctx := testpkg.Ctx(t)
	rows, err := env.repos.Student.List(ctx, map[string]interface{}{
		"person_id": personID,
	})
	require.NoError(t, err)
	return len(rows)
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestRolloverService_AutoApprove_EndToEndUpdatesExistingStudent(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	source, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Beispiel", "anna@example.com",
		"Lina", "Beispiel",
		int16(1),
	)
	require.NotNil(t, source.CreatedStudentID)
	require.Equal(t, existing.ID, *source.CreatedStudentID)
	require.Equal(t, classForGrade(1), existing.SchoolClass)

	// Run the rollover with auto_approve = true and a past deadline.
	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "auto-approve-integration-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)
	require.Equal(t, 1, result.RequestCount)

	// Sanity: before the worker runs, the new row is auto_renewed.
	preWorker, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	require.NoError(t, err)
	require.Len(t, preWorker, 1)
	require.NotNil(t, preWorker[0].RolloverSourceChildID)
	assert.Equal(t, source.ID, *preWorker[0].RolloverSourceChildID)

	// Tick the deadline worker — auto_approve flag should route the
	// row through DecisionService.Decide(approved) which in turn
	// runs applyApprovalRollover (no new Person/Student rows).
	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved, "auto_approve must promote to approved")
	assert.Equal(t, 0, summary.AutoRenewedToSubmitted, "no bulk-submitted fallback")
	assert.Equal(t, 0, summary.AutoApproveErrors)

	// The new row is now approved AND linked to the SAME student id.
	approved, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusApproved},
	)
	require.NoError(t, err)
	require.Len(t, approved, 1)
	require.NotNil(t, approved[0].CreatedStudentID, "rollover approval must link to a student")
	assert.Equal(t, existing.ID, *approved[0].CreatedStudentID,
		"auto-approved rollover must reuse the existing student, not create a duplicate")

	// The existing student row was UPDATED in place: school_class
	// bumped to the new grade, enrolled window pinned to the new
	// phase. We re-read the student rather than trusting the in-
	// memory copy.
	refreshed, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusActive, refreshed.Status,
		"already-active rollover students must stay active even for a future phase")
	assert.Equal(t, classForGrade(2), refreshed.SchoolClass,
		"grade was bumped 1 → 2; school_class must follow")
	require.NotNil(t, refreshed.EnrolledFrom)
	assert.Equal(t, result.Phase.ServiceStartDate, *refreshed.EnrolledFrom,
		"enrolled_from must follow the new phase's service window")
	require.NotNil(t, refreshed.EnrolledUntil)
	assert.Equal(t, result.Phase.ServiceEndDate, *refreshed.EnrolledUntil)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, approved[0].ActivationMode)
	require.NotNil(t, approved[0].ActivateOn)
	assert.Equal(t, result.Phase.ServiceStartDate.Format("2006-01-02"), approved[0].ActivateOn.Format("2006-01-02"))
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestRolloverService_AutoApprove_InactiveExistingStudentImmediateBecomesActive(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnvWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeImmediate,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Inactive", "inactive-immediate@example.com",
		"Lina", "Inactive",
		int16(1),
	)
	existing.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "inactive-immediate-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved)
	assert.Equal(t, 0, summary.AutoApproveErrors)

	approved, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusApproved},
	)
	require.NoError(t, err)
	require.Len(t, approved, 1)
	assert.Equal(t, enrollmentModels.ChildActivationImmediate, approved[0].ActivationMode)
	assert.Nil(t, approved[0].ActivateOn)

	refreshed, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusActive, refreshed.Status)
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestRolloverService_AutoApprove_StatusChangeWritesSystemAudit(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnvWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeImmediate,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Audit", "inactive-audit@example.com",
		"Lina", "Audit",
		int16(1),
	)
	existing.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-time.Hour)
	req.Name = "inactive-audit-target"
	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved)
	assert.Equal(t, 0, summary.AutoApproveErrors)

	history, err := env.repos.StudentFieldEdit.GetByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, auditModels.StudentFieldStatus, history[0].FieldName)
	assert.Equal(t, auditModels.StudentFieldEditSystemActorID, history[0].EditedBy)
	assert.Equal(t, auditModels.StudentFieldEditSystemActorName, history[0].EditedByName)
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestRolloverService_AutoApprove_InactiveExistingStudentFutureScheduledBecomesPending(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Inactive", "inactive-scheduled@example.com",
		"Lina", "Inactive",
		int16(1),
	)
	existing.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "inactive-scheduled-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved)
	assert.Equal(t, 0, summary.AutoApproveErrors)

	refreshed, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusPending, refreshed.Status)

	approved, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusApproved},
	)
	require.NoError(t, err)
	require.Len(t, approved, 1)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, approved[0].ActivationMode)
	require.NotNil(t, approved[0].ActivateOn)
	assert.Equal(t, result.Phase.ServiceStartDate.Format("2006-01-02"), approved[0].ActivateOn.Format("2006-01-02"))
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestRolloverService_AutoApprove_InactiveExistingStudentPastScheduledBecomesActive(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Inactive", "inactive-scheduled-past@example.com",
		"Lina", "Inactive",
		int16(1),
	)
	existing.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, existing))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.ServiceStartDate = timezone.TodayDate().AddDays(-1)
	req.ServiceEndDate = timezone.NewDate(req.ServiceStartDate.Year, req.ServiceStartDate.Month+10, req.ServiceStartDate.Day)
	req.Name = "inactive-scheduled-past-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved)
	assert.Equal(t, 0, summary.AutoApproveErrors)

	refreshed, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusActive, refreshed.Status)

	approved, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusApproved},
	)
	require.NoError(t, err)
	require.Len(t, approved, 1)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, approved[0].ActivationMode)
	require.NotNil(t, approved[0].ActivateOn)
	assert.Equal(t, result.Phase.ServiceStartDate.Format("2006-01-02"), approved[0].ActivateOn.Format("2006-01-02"))
}

func TestRolloverService_AutoApprove_DoesNotDuplicateStudents(t *testing.T) {
	t.Parallel()

	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Beispiel", "anna@example.com",
		"Lina", "Beispiel",
		int16(1),
	)

	// Count students for this PersonID before the rollover.
	beforeCount := countStudentsForPerson(t, env, existing.PersonID)

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "no-dup-target"
	_, createErr := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, createErr)

	_, workerErr := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, workerErr)

	afterCount := countStudentsForPerson(t, env, existing.PersonID)
	assert.Equal(t, beforeCount, afterCount,
		"auto-approve must not insert a second student row for the same person")
}

func TestRolloverService_AutoApprove_ValidationFailureRollsBackStudentUpdate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	source, existing := seedApprovedChildWithStudent(
		t, env,
		"Anna", "Rollback", "auto-approve-rollback@example.com",
		"Lina", "Rollback",
		int16(1),
	)
	require.NotNil(t, existing.EnrolledFrom)
	require.NotNil(t, existing.EnrolledUntil)
	originalClass := existing.SchoolClass
	originalStatus := existing.Status
	originalFrom := *existing.EnrolledFrom
	originalUntil := *existing.EnrolledUntil

	// A valid template-linked source offering; the rollover clones it into
	// the target phase (#2249). After the rollover we corrupt the CLONE the
	// way legacy rows looked before #1885 made available_days mandatory:
	// Decide then fails in care recurrence validation only after
	// applyApprovalRollover updates the student — the rollback under test.
	group := createCareOfferingTemplateGroup(t, env.db, "rollover-savepoint")
	period := createCareOfferingTestPeriod(
		t,
		env.db,
		"rollover-savepoint",
		timezone.NewDate(2027, 8, 1),
		timezone.NewDate(2028, 8, 31),
	)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)
	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Rollover savepoint offering",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"tue"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID: source.ID,
		CareOfferingID: offering.ID,
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, link))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-time.Hour)
	req.Name = "auto-approve-savepoint-rollback-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, result.ClonedOfferingCount)

	// Blank the target-phase clone's days directly, bypassing Validate —
	// simulating a legacy-invalid row in the phase Decide will materialize.
	_, updErr := env.db.NewUpdate().
		TableExpr("enrollment.care_offerings").
		Set("available_days = '[]'::jsonb").
		Where("phase_id = ?", result.Phase.ID).
		Exec(ctx)
	require.NoError(t, updErr)

	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx,
		result.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	require.NoError(t, err)
	require.Len(t, rolled, 1)
	rolledChildID := rolled[0].ID

	var summary *enrollmentService.DeadlineWorkerSummary
	err = testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var workerErr error
		summary, workerErr = env.rolloverSvc.RunDeadlineWorker(txCtx, time.Now())
		return workerErr
	})
	require.NoError(t, err, "ordinary Decide failures must be isolated to their row savepoint")
	require.NotNil(t, summary)
	assert.Equal(t, 0, summary.AutoRenewedToApproved)
	assert.Equal(t, 1, summary.AutoApproveErrors)

	refreshed, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Equal(t, originalClass, refreshed.SchoolClass,
		"failed approval must not commit the target grade/class")
	assert.Equal(t, originalStatus, refreshed.Status)
	require.NotNil(t, refreshed.EnrolledFrom)
	require.NotNil(t, refreshed.EnrolledUntil)
	assert.Equal(t, originalFrom, *refreshed.EnrolledFrom,
		"failed approval must not commit the target enrollment start")
	assert.Equal(t, originalUntil, *refreshed.EnrolledUntil,
		"failed approval must not commit the target enrollment end")

	rolledAfter, err := env.repos.RequestChild.FindByID(ctx, rolledChildID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusAutoRenewed, rolledAfter.Status)
	assert.Nil(t, rolledAfter.CreatedStudentID, "failed approval must not link the existing student")

	enrollments, err := env.repos.StudentEnrollment.FindByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	for _, enrollment := range enrollments {
		if enrollment.EnrollmentRequestChildID != nil {
			assert.NotEqual(t, rolledChildID, *enrollment.EnrollmentRequestChildID,
				"failed approval must not leave a materialized roster row")
		}
	}
}
