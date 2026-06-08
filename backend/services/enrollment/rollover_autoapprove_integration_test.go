package enrollment_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
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
		StudentRepo:              repoFactory.Student,
		StudentGuardianRepo:      repoFactory.StudentGuardian,
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		StudentEnrollmentRepo:    repoFactory.StudentEnrollment,
		ActivityGroupRepo:        repoFactory.ActivityGroup,
		ActivityScheduleRepo:     repoFactory.ActivitySchedule,
		CalendarPeriodRepo:       repoFactory.CalendarPeriod,
		AccountRepo:              repoFactory.Account,
		AccountTenantRepo:        repoFactory.AccountTenant,
		AccountRoleRepo:          repoFactory.AccountRole,
		RoleRepo:                 repoFactory.Role,
		OutboxEnqueuer:           env.outbox,
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		Logger:                   slog.Default(),
	})

	// Rebuild the rollover service so DecisionService is injected.
	env.rolloverSvc = enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                env.repos.Phase,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
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
	ctx := testpkg.TenantContext(1)

	// 1. Submit + manually approve.
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
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
				DateOfBirth:      time.Date(2018, 4, 15, 0, 0, 0, 0, time.UTC),
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
	dob := time.Date(2018, 4, 15, 0, 0, 0, 0, time.UTC)
	person.Birthday = &dob
	person.SetTenantID(1)
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
	student.SetTenantID(1)
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
	ctx := testpkg.TenantContext(1)
	rows, err := env.repos.Student.List(ctx, map[string]interface{}{
		"person_id": personID,
	})
	require.NoError(t, err)
	return len(rows)
}

func TestRolloverService_AutoApprove_EndToEndUpdatesExistingStudent(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

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
	assert.Equal(t, classForGrade(2), refreshed.SchoolClass,
		"grade was bumped 1 → 2; school_class must follow")
	require.NotNil(t, refreshed.EnrolledFrom)
	assert.Equal(t, result.Phase.ServiceStartDate, *refreshed.EnrolledFrom,
		"enrolled_from must follow the new phase's service window")
	require.NotNil(t, refreshed.EnrolledUntil)
	assert.Equal(t, result.Phase.ServiceEndDate, *refreshed.EnrolledUntil)
}

func TestRolloverService_AutoApprove_DoesNotDuplicateStudents(t *testing.T) {
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

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
