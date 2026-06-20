package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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

// stubActivationSettings is a fake DecisionSettingsResolver returning a
// fixed enrollment.default_activation_mode, so the immediate/scheduled
// approval paths can be exercised without writing config.setting_values
// rows. Any other key resolves to "" (registry-default behaviour).
type stubActivationSettings struct {
	mode string
}

func (s stubActivationSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == configModel.KeyEnrollmentDefaultActivationMode {
		return s.mode, nil
	}
	return "", nil
}

func setupDecisionTest(t *testing.T) (*decisionTestEnv, func()) {
	// nil Settings exercises the safe default (scheduled).
	return setupDecisionTestWithSettings(t, nil)
}

func setupDecisionTestWithSettings(
	t *testing.T,
	settings enrollmentService.DecisionSettingsResolver,
) (*decisionTestEnv, func()) {
	t.Helper()
	env, cleanup := setupRolloverTest(t)

	repoFactory := repositories.NewFactory(env.db)
	decision := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestGuardianRepo:      repoFactory.RequestGuardian,
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
		Settings:                 settings,
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
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: &grade,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	return res.Request.ID, res.Children[0].ID
}

func setSourcePhaseServiceStartDate(t *testing.T, env *decisionTestEnv, serviceStartDate timezone.Date) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	env.sourcePhase.ServiceStartDate = serviceStartDate
	if !env.sourcePhase.ServiceEndDate.After(serviceStartDate) {
		env.sourcePhase.ServiceEndDate = timezone.NewDate(serviceStartDate.Year, serviceStartDate.Month+10, serviceStartDate.Day)
	}
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
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

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, authorize.GuardianRolePrimaryGuardian, links[0].GuardianRole)
	assert.True(t, authorize.StudentGuardianHasPermission(links[0], authorize.GuardianPermissionPortalAccess))
	assert.True(t, authorize.StudentGuardianHasPermission(links[0], authorize.GuardianPermissionEnrollmentSubmit))
}

// TestDecisionService_Decide_ApprovedLinksAdditionalGuardians verifies the
// "GENAUSO" mapping: on approval every co-guardian stored on the request is
// linked to the created student as an additional students_guardians row
// (IsPrimary=false, but emergency-contact + can-pickup like the primary),
// and the resolved guardian_profiles id is stamped back on the request row.
// The email-less co-guardian must resolve to a contact profile with NULL email.
func TestDecisionService_Decide_ApprovedLinksAdditionalGuardians(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "primary-guardian@example.com", "Kjell", "Approved")

	// Two co-guardians: one contact-only (no email), one with an email.
	opaPhone := "012345678"
	omaEmail := "oma@example.com"
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID, FirstName: "Opa", LastName: "Schmidt", Phone: &opaPhone, SortOrder: 0,
	}))
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID, FirstName: "Oma", LastName: "Schmidt", Email: &omaEmail, SortOrder: 1,
	}))

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	// THREE guardian links: primary + two co-guardians.
	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, links, 3, "primary + two co-guardians must all be linked")
	primaryCount := 0
	for _, l := range links {
		if l.IsPrimary {
			primaryCount++
			assert.Equal(t, authorize.GuardianRolePrimaryGuardian, l.GuardianRole)
			assert.True(t, authorize.StudentGuardianHasPermission(l, authorize.GuardianPermissionPortalAccess))
			assert.True(t, authorize.StudentGuardianHasPermission(l, authorize.GuardianPermissionEnrollmentSubmit))
			continue
		}
		assert.True(t, l.IsEmergencyContact, "co-guardians mapped like primary: emergency contact")
		assert.True(t, l.CanPickup, "co-guardians mapped like primary: can pick up")
		assert.Equal(t, authorize.GuardianRoleEmergency, l.GuardianRole)
		assert.False(t, authorize.StudentGuardianHasPermission(l, authorize.GuardianPermissionPortalAccess))
		assert.False(t, authorize.StudentGuardianHasPermission(l, authorize.GuardianPermissionEnrollmentSubmit))
	}
	assert.Equal(t, 1, primaryCount, "exactly one primary guardian")

	// Both co-guardian rows must be stamped with their resolved profile id,
	// and the email-less one must resolve to a profile with NULL email.
	extras, err := env.repos.RequestGuardian.ListByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Len(t, extras, 2)
	for _, ex := range extras {
		require.NotNil(t, ex.GuardianProfileID, "approval must stamp the resolved profile id")
		profile, perr := env.repos.GuardianProfile.FindByID(ctx, *ex.GuardianProfileID)
		require.NoError(t, perr)
		require.NotNil(t, profile)
		assert.Equal(t, ex.LastName, profile.LastName)
		if ex.Email == nil {
			assert.Nil(t, profile.Email, "email-less co-guardian creates a contact profile with NULL email")
		} else {
			require.NotNil(t, profile.Email)
			assert.Equal(t, *ex.Email, *profile.Email)
		}
	}
}

// TestDecisionService_Decide_PersistsCoGuardianPhone verifies that on
// approval a co-guardian's phone number is written to
// users.guardian_phone_numbers, mirroring the primary guardian. The
// phone-only contact (no email) is the critical case: that number is the
// ONLY way to reach the approved emergency contact, so dropping it would
// leave them unreachable.
func TestDecisionService_Decide_PersistsCoGuardianPhone(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "primary-phone@example.com", "Kjell", "Approved")

	// One phone-only co-guardian (no email) and one with both email + phone.
	opaPhone := "0151-9990001"
	omaEmail := "oma-phone@example.com"
	omaPhone := "0151-9990002"
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID, FirstName: "Opa", LastName: "Phone", Phone: &opaPhone, SortOrder: 0,
	}))
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID, FirstName: "Oma", LastName: "Phone", Email: &omaEmail, Phone: &omaPhone, SortOrder: 1,
	}))

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	// Each co-guardian's resolved profile must carry the submitted phone.
	extras, err := env.repos.RequestGuardian.ListByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Len(t, extras, 2)

	wantPhone := map[string]string{"Opa": "0151-9990001", "Oma": "0151-9990002"}
	for _, ex := range extras {
		require.NotNil(t, ex.GuardianProfileID, "approval stamps the resolved profile id")
		phones, perr := env.repos.GuardianPhoneNumber.FindByGuardianID(ctx, *ex.GuardianProfileID)
		require.NoError(t, perr)
		require.Len(t, phones, 1, "co-guardian %s must have exactly one phone persisted", ex.FirstName)
		assert.Equal(t, wantPhone[ex.FirstName], phones[0].PhoneNumber)
		assert.True(t, phones[0].IsPrimary, "the co-guardian's only number is their primary")
	}
}

func TestDecisionService_Decide_ContactListSelfGuardianDoesNotAbortApproval(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{{
		Key:         "contacts",
		Label:       "Weitere Kontakte",
		Type:        enrollmentModels.FormFieldContactList,
		Target:      enrollmentModels.TargetStudentContacts,
		AppliesToCh: true,
		SortOrder:   0,
	}}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	guardianEmail := "self-contact@example.com"
	guardianPhone := "015126829060"
	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Franziska",
		GuardianLastName:  "Bahnemann",
		GuardianEmail:     guardianEmail,
		GuardianPhone:     &guardianPhone,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Self",
			LastName:         "Contact",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			CustomData: map[string]any{
				"contacts": []any{map[string]any{
					"first_name":           "Franziska",
					"last_name":            "Bahnemann",
					"email":                guardianEmail,
					"relationship_type":    "parent",
					"is_emergency_contact": true,
					"can_pickup":           true,
				}},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  res.Request.ID,
		ChildID:    res.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.Len(t, links, 1, "self-contact must reuse the primary guardian link instead of aborting the tx")
	assert.True(t, links[0].IsPrimary)

	phones, err := env.repos.GuardianPhoneNumber.FindByGuardianID(ctx, links[0].GuardianProfileID)
	require.NoError(t, err, "auto guardian_phone runs after contact dispatch; this proves the tx stayed usable")
	require.Len(t, phones, 1)
	assert.Equal(t, guardianPhone, phones[0].PhoneNumber)
}

// TestDecisionService_Decide_AppliesDepartureField verifies the unified
// weekday_mode departure field (#1610) flows from the enrollment submission
// onto the approved student's departure_days, deriving the legacy mirrors.
func TestDecisionService_Decide_AppliesDepartureField(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Pin a schema that carries the unified departure field onto the phase.
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{{
		Key:         "departure",
		Label:       "Geh- und Abholregelung",
		Type:        enrollmentModels.FormFieldWeekdayMode,
		Target:      enrollmentModels.TargetStudentDeparture,
		AppliesToCh: true,
		SortOrder:   0,
	}}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	// Submit a child whose custom_data carries the per-day departure plan.
	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     "dep-approve@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Dep",
			LastName:         "Child",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			CustomData: map[string]any{
				"departure": map[string]any{"mon": "bus", "wed": "pickup"},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  res.Request.ID,
		ChildID:    res.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DepartureBus, student.DepartureDays.ModeFor(usersModels.PickupDayMonday))
	assert.Equal(t, usersModels.DeparturePickup, student.DepartureDays.ModeFor(usersModels.PickupDayWednesday))
	assert.True(t, student.BusDays[usersModels.BusDayMonday], "derived bus_days mirror")
	assert.True(t, student.PickupDays[usersModels.PickupDayWednesday], "derived pickup_days mirror")
}

// TestDecisionService_Decide_AppliesCoupledCompanionNote verifies the "mit wem"
// note (#1694) that the enrollment form couples onto the allowed-departure-modes
// field via a reserved custom-data key (not a separate schema field) lands on
// the approved student's departure_companion_note.
func TestDecisionService_Decide_AppliesCoupledCompanionNote(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{{
		Key:         "allowed_modes",
		Label:       "Erlaubte Heimwege",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		Target:      enrollmentModels.TargetStudentAllowedDepartureModes,
		AppliesToCh: true,
		SortOrder:   0,
	}}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     "companion-note@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Comp",
			LastName:         "Child",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			CustomData: map[string]any{
				"allowed_modes": map[string]any{"mon": []any{"accompanied"}},
				// Reserved coupled key carrying the free-text companion note.
				enrollmentModels.TargetStudentDepartureCompanionNote: "Geschwisterkind Mia (1b)",
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  res.Request.ID,
		ChildID:    res.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DepartureAccompanied,
		student.AllowedDepartureModes.DepartureDays().ModeFor(usersModels.PickupDayMonday),
		"accompanied mode must reach the student")
	require.NotNil(t, student.DepartureCompanionNote, "coupled companion note must be applied")
	assert.Equal(t, "Geschwisterkind Mia (1b)", *student.DepartureCompanionNote)
}

// TestDecisionService_Decide_SkipsCompanionNoteWithoutAccompanied verifies the
// coupled "mit wem" note is NOT applied when the child's submitted modes contain
// no accompanied day, so a stale/crafted note never lands on a child with no
// "Mit anderem Kind" plan (#1694). The server guard is the authority here — the
// client's hasAccompanied gate cannot be trusted.
func TestDecisionService_Decide_SkipsCompanionNoteWithoutAccompanied(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{{
		Key:         "allowed_modes",
		Label:       "Erlaubte Heimwege",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		Target:      enrollmentModels.TargetStudentAllowedDepartureModes,
		AppliesToCh: true,
		SortOrder:   0,
	}}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     "no-accompanied@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Solo",
			LastName:         "Child",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			CustomData: map[string]any{
				// No accompanied day, yet a companion note rides along.
				"allowed_modes": map[string]any{"mon": []any{"bus"}},
				enrollmentModels.TargetStudentDepartureCompanionNote: "Geschwisterkind Mia (1b)",
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  res.Request.ID,
		ChildID:    res.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Nil(t, student.DepartureCompanionNote,
		"companion note must not be applied without an accompanied day")
}

// ---- Decide: activation mode (enrollment.default_activation_mode) -------

// Default / "scheduled": an approved child becomes a PENDING student
// pinned to the phase's service-start date, so the activate-students
// scheduler flips it to active when that date arrives. Uses the nil-
// Settings setup, which must behave identically to an explicit
// "scheduled" override (the safe default).
func TestDecisionService_Decide_ApprovedScheduledKeepsStudentPending(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "activation-scheduled@example.com", "Sched", "Uled")
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
	assert.Equal(t, usersModels.StudentStatusPending, student.Status,
		"scheduled mode must create the student as pending")
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t,
		env.sourcePhase.ServiceStartDate.Format("2006-01-02"),
		student.EnrolledFrom.Format("2006-01-02"),
		"enrolled_from must be pinned to the phase service-start date")

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, child.ActivationMode)
	require.NotNil(t, child.ActivateOn)
	assert.Equal(t,
		env.sourcePhase.ServiceStartDate.Format("2006-01-02"),
		child.ActivateOn.Format("2006-01-02"),
		"scheduled approval must stamp the planned activation date")
}

// "immediate": an approved child becomes an ACTIVE student right away so
// it appears in lists/attendance/check-in immediately. enrolled_from is
// still pinned to the phase's service-start date (the mode changes only
// the status, never the dates).
func TestDecisionService_Decide_ApprovedImmediateActivatesStudent(t *testing.T) {
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeImmediate,
	})
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "activation-immediate@example.com", "Immo", "Diate")
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
	assert.Equal(t, usersModels.StudentStatusActive, student.Status,
		"immediate mode must activate the student on approval")
	require.NotNil(t, student.EnrolledFrom)
	assert.Equal(t,
		env.sourcePhase.ServiceStartDate.Format("2006-01-02"),
		student.EnrolledFrom.Format("2006-01-02"),
		"enrolled_from must stay the phase service-start date even in immediate mode")

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildActivationImmediate, child.ActivationMode)
	assert.Nil(t, child.ActivateOn)
}

func TestDecisionService_Decide_ApprovedScheduledPastStartActivatesStudent(t *testing.T) {
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeScheduled,
	})
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "activation-scheduled-past@example.com", "Past", "Start")
	startDate := timezone.TodayDate().AddDays(-1)
	setSourcePhaseServiceStartDate(t, env, startDate)

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
	assert.Equal(t, usersModels.StudentStatusActive, student.Status,
		"scheduled mode must activate immediately when service_start_date is today or already past")

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, child.ActivationMode)
	require.NotNil(t, child.ActivateOn)
	assert.Equal(t, startDate.Format("2006-01-02"), child.ActivateOn.Format("2006-01-02"))
}

func TestDecisionService_Decide_ApprovedUnknownActivationModeFallsBackToScheduled(t *testing.T) {
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{mode: "bogus"})
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, childID := submitOneChild(t, env, "activation-unknown@example.com", "Safe", "Default")
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
	assert.Equal(t, usersModels.StudentStatusPending, student.Status,
		"unknown setting values must not activate students")

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, child.ActivationMode)
	require.NotNil(t, child.ActivateOn)
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
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
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
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
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

func TestDecisionService_Decide_ApprovedPreservesLegacyNonTemplateLinkedOffering(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Legacy-Linked")
	group := &activitiesModels.Group{
		Name:            "Decision Legacy Linked",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      false,
	}
	group.SetTenantID(1)
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))
	defer func() {
		testpkg.CleanupTableRecords(t, env.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, env.db, "activities.categories", category.ID)
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Legacy Linked",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon", "wed"},
		IsActive:        true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Legacy",
		GuardianEmail:     "legacy-linked@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Legacy",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
			},
		},
	})
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
	assert.Nil(t, rows[0].CalendarPeriodID)
	assert.Empty(t, rows[0].SelectedWeekdays)
}

func TestDecisionService_Decide_RolloverApprovalMaterializesSourcePhaseOffering(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Rollover-Linked")
	group := &activitiesModels.Group{
		Name:            "Decision Rollover Linked",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      false,
	}
	group.SetTenantID(1)
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))
	defer func() {
		testpkg.CleanupTableRecords(t, env.db, "activities.groups", group.ID)
		testpkg.CleanupTableRecords(t, env.db, "activities.categories", category.ID)
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Rollover Source Linked",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon", "wed"},
		IsActive:        true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Rollover",
		GuardianEmail:     "rollover-linked@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Rina",
				LastName:         "Rollover",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	sourceOutcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, sourceOutcome.Child.CreatedStudentID)
	studentID := *sourceOutcome.Child.CreatedStudentID

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env.rolloverTestEnv, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.NotNil(t, result.Phase)

	rolledChildren, err := env.repos.RequestChild.ListByPhaseAndStatuses(ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolledChildren, 1)
	rolled := rolledChildren[0]
	require.NotNil(t, rolled.RolloverSourceChildID)
	assert.Equal(t, submitted.Children[0].ID, *rolled.RolloverSourceChildID)

	rolledLinks, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled.ID)
	require.NoError(t, err)
	require.Len(t, rolledLinks, 1)
	assert.Equal(t, offering.ID, rolledLinks[0].CareOfferingID,
		"rollover intentionally copies the source-phase offering id")

	rolloverOutcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  rolled.RequestID,
		ChildID:    rolled.ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, rolloverOutcome.Child.CreatedStudentID)
	assert.Equal(t, studentID, *rolloverOutcome.Child.CreatedStudentID)

	count, err := env.db.NewSelect().
		Model((*activitiesModels.StudentEnrollment)(nil)).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, 1).
		Where(`"student_enrollment".student_id = ?`, studentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Where(`"student_enrollment".valid_from = ?`, result.Phase.ServiceStartDate).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count,
		"rollover approval must materialize the copied source-phase offering for the target phase window")
}

func TestDecisionService_Decide_ApprovedRejectsEmptyDaysForTemplateOffering(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Empty-Days")
	group := &activitiesModels.Group{
		Name:            "Decision Empty Days",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      true,
	}
	group.SetTenantID(1)
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))
	period := createCareOfferingTestPeriod(t, env.db, "decision-empty-days",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)
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
		Name:            "Empty Template Days",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{},
		IsActive:        true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "EmptyDays",
		GuardianEmail:     "empty-days@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Emil",
				LastName:         "EmptyDays",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)

	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no selected or available days")
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
