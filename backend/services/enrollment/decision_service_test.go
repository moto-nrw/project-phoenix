package enrollment_test

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Decision service integration tests. These hit the real test DB
// (testpkg.SetupTestDB) so they run only in CI environments where
// the test DB is wired. Locally they skip via SetupTestDB's contract.
//
// Coverage focus: the public surface of DecisionService (List, Get,
// Decide, ListChildOfferings) plus the downstream-record-creation
// behaviour of the approval branch (the most load-bearing path in
// the enrollment workflow).

// decisionTestEnv augments rolloverTestEnv with a wired DecisionService
// so we don't have to repeat the dependency injection in every test.
// Reuses the rollover env's tenant + schema + source-phase fixture so
// teardown stays unified.
type decisionTestEnv struct {
	*rolloverTestEnv
	decision enrollmentService.DecisionService
}

func setupDecisionTest(t *testing.T) (*decisionTestEnv, func()) {
	t.Helper()
	env, cleanup := setupRolloverTest(t)

	repoFactory := repositories.NewFactory(env.db)
	decision := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		PhaseRepo:                repoFactory.Phase,
		FormSchemaRepo:           repoFactory.FormSchema,
		PersonRepo:               repoFactory.Person,
		StudentRepo:              repoFactory.Student,
		StudentGuardianRepo:      repoFactory.StudentGuardian,
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		GuardianPhoneRepo:        repoFactory.GuardianPhoneNumber,
		PickupScheduleRepo:       repoFactory.StudentPickupSchedule,
		ArrivalScheduleRepo:      repoFactory.StudentArrivalSchedule,
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
	return &decisionTestEnv{rolloverTestEnv: env, decision: decision}, cleanup
}

// submitOneChild submits a single-child enrollment in the source
// phase and returns the resulting request + child ids so tests can
// drive Decide against a known row.
func submitOneChild(t *testing.T, env *decisionTestEnv, guardianEmail, childFirst, childLast string) (requestID, childID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     guardianEmail,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
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
	return res.Request.ID, res.Children[0].ID
}

// ---- Get ----------------------------------------------------------------

func TestDecisionService_Get_NotFound(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.decision.Get(ctx, 99_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound),
		"missing id must surface ErrDecisionRequestNotFound")
}

func TestDecisionService_Get_ReturnsRequestPhaseAndChildren(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, _ := submitOneChild(t, env, "get-happy@example.com", "Lina", "Get")

	summary, err := env.decision.Get(ctx, reqID)
	require.NoError(t, err)
	require.NotNil(t, summary.Request)
	assert.Equal(t, reqID, summary.Request.ID)
	require.NotNil(t, summary.Phase)
	assert.Equal(t, env.sourcePhase.ID, summary.Phase.ID)
	require.Len(t, summary.Children, 1)
	assert.Equal(t, "Lina", summary.Children[0].FirstName)
}

// ---- List ---------------------------------------------------------------

func TestDecisionService_List_ReturnsAllRequestsInTenant(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, _ = submitOneChild(t, env, "list-a@example.com", "A", "Eins")
	_, _ = submitOneChild(t, env, "list-b@example.com", "B", "Zwei")

	list, err := env.decision.List(ctx, enrollmentService.RequestFilters{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestDecisionService_List_FilterByPhaseNarrowsResults(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, _ = submitOneChild(t, env, "list-phase@example.com", "Phase", "Child")

	list, err := env.decision.List(ctx, enrollmentService.RequestFilters{
		PhaseID: env.sourcePhase.ID,
	})
	require.NoError(t, err)
	for _, row := range list {
		assert.Equal(t, env.sourcePhase.ID, row.Request.PhaseID,
			"phase filter must scope results")
	}
}

func TestDecisionService_List_FilterByChildStatusNarrowsResults(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, _ = submitOneChild(t, env, "list-status@example.com", "Child", "Status")

	// All freshly submitted children carry status=submitted by default.
	list, err := env.decision.List(ctx, enrollmentService.RequestFilters{
		ChildStatus: enrollmentModels.ChildStatusSubmitted,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, list, "filter by 'submitted' must return at least the fresh row")
}

// ---- Decide: validation -------------------------------------------------

func TestDecisionService_Decide_RejectsZeroRequestID(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: 0,
		ChildID:   1,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionInvalidStatus))
}

func TestDecisionService_Decide_RejectsZeroChildID(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, _ := submitOneChild(t, env, "decide-zero-child@example.com", "X", "Y")
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: reqID,
		ChildID:   0,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionInvalidStatus))
}

func TestDecisionService_Decide_RejectsUnknownStatus(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-bad-status@example.com", "X", "Y")
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: reqID,
		ChildID:   childID,
		Status:    enrollmentService.DecisionStatus("garbage"),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionInvalidStatus))
}

func TestDecisionService_Decide_RequestNotFound(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: 99_999_999,
		ChildID:   1,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound))
}

func TestDecisionService_Decide_ChildNotFound(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, _ := submitOneChild(t, env, "decide-child-nf@example.com", "X", "Y")
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: reqID,
		ChildID:   99_999_999,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionChildNotFound))
}

// ---- Decide: non-terminal status transitions ----------------------------

func TestDecisionService_Decide_WaitlistedSetsStatus(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-wait@example.com", "Will", "Wait")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionWaitlisted,
		Reason:     "voll ausgebucht",
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child)
	assert.Equal(t, enrollmentModels.ChildStatusWaitlisted, outcome.Child.Status)
	require.NotNil(t, outcome.Child.StatusReason)
	assert.Equal(t, "voll ausgebucht", *outcome.Child.StatusReason)
}

func TestDecisionService_Decide_RejectedSetsStatus(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-rej@example.com", "Will", "Reject")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionRejected,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusRejected, outcome.Child.Status)
}

func TestDecisionService_Decide_UnderReviewSetsStatus(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-review@example.com", "Will", "Review")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionUnderReview,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusUnderReview, outcome.Child.Status)
}

func TestDecisionService_Decide_TerminalTransitionRejected(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-term@example.com", "Will", "Term")
	// First decide → rejected (terminal).
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionRejected,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	// Second decide → trying to move to under_review must fail.
	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionUnderReview,
		ReviewedBy: env.creatorID,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionAlreadyTerminal))
}

// ---- Decide: approval (the heavy path) ----------------------------------

func TestDecisionService_Decide_ApprovedCreatesDownstreamRecords(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-approve@example.com", "Anna", "Approved")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child)
	assert.Equal(t, enrollmentModels.ChildStatusApproved, outcome.Child.Status)

	// applyApproval must link the new student.id onto request_children.created_student_id
	// (or equivalent) so the admin UI can navigate to the new student.
	require.NotNil(t, outcome.Child.CreatedStudentID,
		"approval must stamp the created student id on the request_child row")
	studentID := *outcome.Child.CreatedStudentID
	assert.Positive(t, studentID)

	// The student row must exist and carry the school class derived
	// from the grade level the parent supplied.
	student, err := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student)
	assert.NotEmpty(t, student.SchoolClass, "school class must be derived from target_grade_level")
}

func TestDecisionService_Decide_ApprovedStampsConsentTimestamps(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-consents@example.com", "Anna", "Consents")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	// All four consent flags were submitted true; the decision service
	// stamps all four _accepted_at columns at approval time.
	assert.NotNil(t, student.AGBAcceptedAt, "AGB consent must be stamped")
	assert.NotNil(t, student.DataProcessingAcceptedAt, "DSGVO consent must be stamped")
	assert.NotNil(t, student.EmailContactAcceptedAt, "Email-contact consent must be stamped")
	assert.NotNil(t, student.PhotoConsentGivenAt, "Photo consent must be stamped")
	require.NotNil(t, student.PhotoConsentGivenBy)
	assert.Equal(t, env.creatorID, *student.PhotoConsentGivenBy,
		"photo_consent_given_by must reference the reviewer account")
}

func TestDecisionService_Decide_ApprovedIsIdempotent(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "decide-idem@example.com", "Anna", "Idem")
	first, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, first.Child.CreatedStudentID)
	firstStudentID := *first.Child.CreatedStudentID

	// Second call with the same status must be a no-op — no new
	// student should be created.
	second, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, second.Child.CreatedStudentID)
	assert.Equal(t, firstStudentID, *second.Child.CreatedStudentID,
		"second approve must not duplicate the student row")
}

func TestDecisionService_Decide_ApprovedUsesFixedOfferingDaysForActivityEnrollment(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Fixed-Days")
	group := &activitiesModels.Group{
		Name:            "Decision Fixed Days",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      true,
	}
	group.SetTenantID(1)
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))
	period := createCareOfferingTestPeriod(t, env.db, "decision-fixed-days",
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 8, 31, 0, 0, 0, 0, time.UTC))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayThursday, &period.ID)
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.schedules").
			Where("activity_group_id = ?", group.ID).
			Exec(ctx)
		testpkg.CleanupTableRecords(t, env.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, env.db, "activities.categories", category.ID)
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Fixed Tue Thu",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"tue", "thu"},
		IsActive:        true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	req := enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Fixed",
		GuardianEmail:     "fixed-days@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Fina",
				LastName:         "Fixed",
				DateOfBirth:      time.Date(2018, 4, 15, 0, 0, 0, 0, time.UTC),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
			},
		},
	}
	submitted, err := env.requestSvc.Submit(ctx, req)
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, 1).
		Where(`"student_enrollment".student_id = ?`, *outcome.Child.CreatedStudentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Scan(ctx))
	require.Len(t, rows, 1)
	assert.Equal(t, []int{2, 4}, rows[0].SelectedWeekdays,
		"fixed offering approval must constrain enrollment to available_days")
	require.NotNil(t, rows[0].CalendarPeriodID)
	assert.Equal(t, period.ID, *rows[0].CalendarPeriodID)
}

// ---- ListChildOfferings -------------------------------------------------

func TestDecisionService_ListChildOfferings_InvalidRequestID(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.decision.ListChildOfferings(ctx, 0)
	require.Error(t, err)
}

func TestDecisionService_ListChildOfferings_EmptyWhenNoOfferingsPicked(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "lco-empty@example.com", "Lina", "NoCare")

	rows, err := env.decision.ListChildOfferings(ctx, reqID)
	require.NoError(t, err)
	// Child exists in the map with an empty slice; no offerings selected.
	assert.Empty(t, rows[childID])
}
