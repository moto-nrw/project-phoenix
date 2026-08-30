package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
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
	mode                  string
	notificationMode      string
	careOfferingsEnabled  *bool
	bookingsAuthoritative *bool
}

func (s stubActivationSettings) ResolveString(_ context.Context, key string) (string, error) {
	if key == configModel.KeyEnrollmentDefaultActivationMode {
		return s.mode, nil
	}
	if key == configModel.KeyEnrollmentNotifyPerDecision {
		if s.notificationMode != "" {
			return s.notificationMode, nil
		}
		return configModel.EnrollmentNotifyPerDecisionImmediate, nil
	}
	return "", nil
}

func (s stubActivationSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if key == configModel.KeyEnrollmentCareOfferingsEnabled && s.careOfferingsEnabled != nil {
		return *s.careOfferingsEnabled, nil
	}
	if key == configModel.KeyEnrollmentBookingsAuthoritative {
		return s.bookingsAuthoritative != nil && *s.bookingsAuthoritative, nil
	}
	return true, nil
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
	decision := newDecisionServiceForTest(env, settings, nil)
	return &decisionTestEnv{rolloverTestEnv: env, decision: decision}, cleanup
}

func newDecisionServiceForTest(
	env *rolloverTestEnv,
	settings enrollmentService.DecisionSettingsResolver,
	lockTemplateRecurrence func(context.Context) error,
) enrollmentService.DecisionService {
	return newDecisionServiceForTestWithCareWithdrawal(env, settings, lockTemplateRecurrence, nil)
}

func newDecisionServiceForTestWithCareWithdrawal(
	env *rolloverTestEnv,
	settings enrollmentService.DecisionSettingsResolver,
	lockTemplateRecurrence func(context.Context) error,
	careWithdrawal enrollmentService.CareWithdrawalReconciler,
) enrollmentService.DecisionService {
	repoFactory := repositories.NewFactory(env.db)
	if careWithdrawal == nil {
		careWithdrawal = usersService.NewCareLifecycleService(usersService.CareLifecycleDependencies{
			StudentRepo: repoFactory.Student, PersonRepo: repoFactory.Person,
			CareExitRepo: repoFactory.CareExit, CleanupRepo: repoFactory.CareExitCleanup,
			WithdrawalRepo: repoFactory.CareWithdrawal, TagReleaser: repoFactory.GradeTransition,
			AuditService:          usersService.NewStudentAuditService(repoFactory.StudentFieldEdit, slog.Default()),
			BookingsAuthoritative: testBookingsAuthority(settings),
			DB:                    env.db, Logger: slog.Default(),
		})
	}
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestGuardianRepo:      repoFactory.RequestGuardian,
		LateInviteRepo:           repoFactory.LateInvite,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		PhaseRepo:                repoFactory.Phase,
		FormSchemaRepo:           repoFactory.FormSchema,
		OfferingAdjustmentRepo:   repoFactory.EnrollmentOfferingAdjustment,
		RestorationAuditRepo:     repoFactory.EnrollmentRestorationAudit,
		PersonRepo:               repoFactory.Person,
		StaffRepo:                repoFactory.Staff,
		StudentRepo:              repoFactory.Student,
		StudentGuardianRepo:      repoFactory.StudentGuardian,
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		GuardianPhoneRepo:        repoFactory.GuardianPhoneNumber,
		PickupScheduleRepo:       repoFactory.StudentPickupSchedule,
		PickupBaselines: scheduletest.NewPickupBaselineService(
			repoFactory.StudentPickupSchedule,
			repoFactory.RequestChildOffering,
			repoFactory.CareOffering,
		),
		ArrivalScheduleRepo:    repoFactory.StudentArrivalSchedule,
		StudentEnrollmentRepo:  repoFactory.StudentEnrollment,
		ActivityGroupRepo:      repoFactory.ActivityGroup,
		ActivityScheduleRepo:   repoFactory.ActivitySchedule,
		CalendarPeriodRepo:     repoFactory.CalendarPeriod,
		TimeframeRepo:          repoFactory.Timeframe,
		ActivityExceptionRepo:  repoFactory.ActivityException,
		AccountRepo:            repoFactory.Account,
		AccountTenantRepo:      repoFactory.AccountTenant,
		AccountRoleRepo:        repoFactory.AccountRole,
		RoleRepo:               repoFactory.Role,
		OutboxEnqueuer:         env.outbox,
		StudentAudit:           usersService.NewStudentAuditService(repoFactory.StudentFieldEdit, slog.Default()),
		CareWithdrawal:         careWithdrawal,
		FrontendURL:            "http://localhost:3000",
		ParentsURL:             "http://parents.localhost:3000",
		Settings:               settings,
		LockTemplateRecurrence: lockTemplateRecurrence,
		InstanceRosters: scheduleService.NewRosterReconciler(
			repoFactory.ActivityInstance,
			repoFactory.InstanceStudent,
			repoFactory.StudentEnrollment,
			slog.Default(),
		),
		Logger: slog.Default(),
		Today:  func() timezone.Date { return timezone.NewDate(2026, 8, 24) },
	})
}

func testBookingsAuthority(settings enrollmentService.DecisionSettingsResolver) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		if settings == nil {
			return false, nil
		}
		return settings.ResolveBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative)
	}
}

func changeRequestApplierForTest(t *testing.T, env *decisionTestEnv) enrollmentService.ChangeRequestDecisionApplier {
	t.Helper()
	applier, ok := env.decision.(enrollmentService.ChangeRequestDecisionApplier)
	require.True(t, ok, "decision service must implement the change-request applier contract")
	return applier
}

func assertOfferingAdjustmentWaitsForRecurrenceGate(
	t *testing.T,
	env *decisionTestEnv,
	decision enrollmentService.DecisionService,
	input enrollmentService.UpdateChildOfferingsInput,
) {
	t.Helper()

	holderAcquired := make(chan struct{})
	releaseHolder := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			if err := scheduleService.LockTenantRecurrenceWrites(txCtx, env.db); err != nil {
				return err
			}
			close(holderAcquired)
			<-releaseHolder
			return nil
		})
	}()

	select {
	case <-holderAcquired:
	case err := <-holderDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out acquiring recurrence gate in holder transaction")
	}

	adjustmentDone := make(chan error, 1)
	go func() {
		adjustmentDone <- testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, err := decision.UpdateChildOfferings(txCtx, input)
			return err
		})
	}()

	select {
	case err := <-adjustmentDone:
		close(releaseHolder)
		require.NoError(t, <-holderDone)
		require.FailNow(t, "offering adjustment bypassed the recurrence gate", "returned early with %v", err)
	case <-time.After(200 * time.Millisecond):
		// Expected: the adjustment is waiting on the advisory transaction lock.
	}

	close(releaseHolder)
	require.NoError(t, <-holderDone)
	select {
	case err := <-adjustmentDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "offering adjustment did not resume after recurrence gate release")
	}
}

// submitOneChild submits a single-child enrollment in the source
// phase and returns the resulting request + child ids so tests can
// drive Decide against a known row.
func submitOneChild(t *testing.T, env *decisionTestEnv, guardianEmail, childFirst, childLast string) (requestID, childID int64) {
	t.Helper()
	return submitOneChildWithCustomData(t, env, guardianEmail, childFirst, childLast, nil)
}

func submitOneChildWithCustomData(
	t *testing.T,
	env *decisionTestEnv,
	guardianEmail string,
	childFirst string,
	childLast string,
	customData map[string]any,
) (requestID, childID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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
				CustomData:       customData,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	return res.Request.ID, res.Children[0].ID
}

func submitDecisionSiblings(t *testing.T, env *decisionTestEnv, guardianEmail string) *enrollmentService.SubmitResult {
	t.Helper()
	grade := int16(2)
	result, err := env.requestSvc.Submit(testpkg.Ctx(t), enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Digest",
		GuardianEmail:     guardianEmail,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Digest",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: &grade,
			},
			{
				FirstName:        "Noah",
				LastName:         "Digest",
				DateOfBirth:      timezone.NewDate(2019, 7, 2),
				TargetGradeLevel: &grade,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Children, 2)
	return result
}

func publishDecisionScheduleSchema(t *testing.T, env *decisionTestEnv, key, target string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	field := enrollmentModels.FormField{
		Key:         key,
		Label:       "Schedule",
		Type:        enrollmentModels.FormFieldWeekdaySchedule,
		AppliesToCh: true,
		Target:      target,
		SortOrder:   0,
	}
	if target == enrollmentModels.TargetSchedulePickup {
		field.AllowedTimes = []string{
			"07:45",
			"14:45",
			"16:00",
		}
	}
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung", []enrollmentModels.FormField{
		field,
	}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
}

func publishLegacyDuplicateTargetSchema(t *testing.T, env *decisionTestEnv, name string, fields []enrollmentModels.FormField) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	schema := &enrollmentModels.FormSchema{
		Name:      name,
		Version:   1,
		CreatedBy: env.creatorID,
		Fields:    fields,
	}
	schema.SetTenantID(testpkg.Tenant(t))
	_, err := env.db.NewInsert().Model(schema).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Returning("*").Exec(ctx)
	require.NoError(t, err)
	require.Greater(t, schema.ID, int64(0))

	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
}

func publishDecisionContactListSchema(t *testing.T, env *decisionTestEnv) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 2", []enrollmentModels.FormField{{
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
}

func phoneOnlyContactEntry(firstName, lastName, phone string, isEmergencyContact, canPickup bool) map[string]any {
	return map[string]any{
		"first_name": firstName,
		"last_name":  lastName,
		"phone_numbers": []any{map[string]any{
			"phone_number": phone,
			"phone_type":   "mobile",
			"is_primary":   true,
		}},
		"is_emergency_contact": isEmergencyContact,
		"can_pickup":           canPickup,
	}
}

func contactSnapshot(childID int64, contacts []any) map[string]any {
	return map[string]any{
		"children": []any{map[string]any{
			"id": childID,
			"custom_data": map[string]any{
				"contacts": contacts,
			},
		}},
	}
}

func createReviewerStaffWithDistinctAccount(t *testing.T, env *decisionTestEnv) (*usersModels.Staff, int64) {
	t.Helper()
	for i := 0; i < 5; i++ {
		_ = testpkg.CreateTestStaff(t, env.db, "ReviewerOffset", "Staff")
		staff, account := testpkg.CreateTestStaffWithAccount(t, env.db, "Reviewer", "Schedule")
		if staff.ID != account.ID {
			return staff, account.ID
		}
	}
	staff, account := testpkg.CreateTestStaffWithAccount(t, env.db, "Reviewer", "ScheduleFinal")
	require.NotEqual(t, staff.ID, account.ID, "test setup must prove account id and staff id are not interchangeable")
	return staff, account.ID
}

func setSourcePhaseServiceStartDate(t *testing.T, env *decisionTestEnv, serviceStartDate timezone.Date) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	env.sourcePhase.ServiceStartDate = serviceStartDate
	if !env.sourcePhase.ServiceEndDate.After(serviceStartDate) {
		env.sourcePhase.ServiceEndDate = timezone.NewDate(serviceStartDate.Year, serviceStartDate.Month+10, serviceStartDate.Day)
	}
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
}

// ---- Get ----------------------------------------------------------------

func TestDecisionService_Get_NotFound(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := env.decision.Get(ctx, 99_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound),
		"missing id must surface ErrDecisionRequestNotFound")
}

func TestDecisionService_Get_ReturnsRequestPhaseAndChildren(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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

func TestDecisionService_Get_LoadsLateInviteUsedForRequest(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, _ := submitOneChild(t, env, "submitted@example.com", "Lina", "GetInvite")
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.Request)(nil)).
		ModelTableExpr(`enrollment.requests AS "request"`).
		Set("submission_source = ?", enrollmentModels.RequestSourceLateInvite).
		Where(`"request".id = ?`, reqID).
		Exec(ctx)
	require.NoError(t, err)
	invite := &enrollmentModels.LateInvite{
		PhaseID:       env.sourcePhase.ID,
		TokenHash:     "decision-get-late-invite-" + t.Name(),
		GuardianEmail: "invited@example.com",
		ExpiresAt:     time.Now().Add(time.Hour),
		CreatedBy:     env.creatorID,
	}
	require.NoError(t, env.repos.LateInvite.Create(ctx, invite))
	require.NoError(t, env.repos.LateInvite.MarkUsed(ctx, invite.ID, reqID, time.Now()))
	defer func() {
		_, _ = env.db.NewDelete().
			Model((*enrollmentModels.LateInvite)(nil)).
			Where("id = ?", invite.ID).
			Exec(context.Background())
	}()

	summary, err := env.decision.Get(ctx, reqID)

	require.NoError(t, err)
	require.NotNil(t, summary.LateInvite)
	assert.Equal(t, invite.ID, summary.LateInvite.ID)
	assert.Equal(t, "invited@example.com", summary.LateInvite.GuardianEmail)
}

// ---- List ---------------------------------------------------------------

func TestDecisionService_List_ReturnsAllRequestsInTenant(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, _ = submitOneChild(t, env, "list-a@example.com", "A", "Eins")
	_, _ = submitOneChild(t, env, "list-b@example.com", "B", "Zwei")

	list, err := env.decision.List(ctx, enrollmentService.RequestFilters{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestDecisionService_List_FilterByPhaseNarrowsResults(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: 0,
		ChildID:   1,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionInvalidStatus))
}

func TestDecisionService_Decide_RejectsZeroChildID(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: 99_999_999,
		ChildID:   1,
		Status:    enrollmentService.DecisionApproved,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound))
}

func TestDecisionService_Decide_ChildNotFound(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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

func TestDecisionService_Decide_ConcurrentSiblingResolutionsEnqueueOneCompleteDigest(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		notificationMode: configModel.EnrollmentNotifyPerDecisionDigest,
	})
	defer cleanup()
	submitted := submitDecisionSiblings(t, env, "concurrent-digest@example.com")

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, child := range submitted.Children {
		childID := child.ID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
				_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
					RequestID:  submitted.Request.ID,
					ChildID:    childID,
					Status:     enrollmentService.DecisionRejected,
					ReviewedBy: env.creatorID,
				})
				return err
			})
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	digests := env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest)
	require.Len(t, digests, 1)
	rejectedNames, ok := digests[0].Payload["rejected_names"].([]string)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"Lina Digest", "Noah Digest"}, rejectedNames)
}

func TestDecisionService_Decide_PinsDigestModeAcrossSettingChange(t *testing.T) {
	t.Parallel()

	settings := newStubRequestSettings()
	settings.stringValues[configModel.KeyEnrollmentNotifyPerDecision] = configModel.EnrollmentNotifyPerDecisionDigest
	env, cleanup := setupDecisionTestWithSettings(t, settings)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	submitted := submitDecisionSiblings(t, env, "pinned-digest@example.com")

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
		Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	assert.Empty(t, env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest))

	settings.mu.Lock()
	settings.stringValues[configModel.KeyEnrollmentNotifyPerDecision] = configModel.EnrollmentNotifyPerDecisionImmediate
	settings.mu.Unlock()
	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[1].ID,
		Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	assert.Len(t, env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest), 1)
	assert.Empty(t, env.outbox.ByKind(platformModels.EmailKindEnrollmentRejected))
	stored, err := env.repos.Request.FindByID(ctx, submitted.Request.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.DecisionNotificationMode)
	assert.Equal(t, configModel.EnrollmentNotifyPerDecisionDigest, *stored.DecisionNotificationMode)
}

func TestDecisionService_Decide_PinsImmediateModeAcrossSettingChange(t *testing.T) {
	t.Parallel()

	settings := newStubRequestSettings()
	settings.stringValues[configModel.KeyEnrollmentNotifyPerDecision] = configModel.EnrollmentNotifyPerDecisionImmediate
	env, cleanup := setupDecisionTestWithSettings(t, settings)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	submitted := submitDecisionSiblings(t, env, "pinned-immediate@example.com")

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
		Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	settings.mu.Lock()
	settings.stringValues[configModel.KeyEnrollmentNotifyPerDecision] = configModel.EnrollmentNotifyPerDecisionDigest
	settings.mu.Unlock()
	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[1].ID,
		Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	assert.Len(t, env.outbox.ByKind(platformModels.EmailKindEnrollmentRejected), 2)
	assert.Empty(t, env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest))
	stored, err := env.repos.Request.FindByID(ctx, submitted.Request.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.DecisionNotificationMode)
	assert.Equal(t, configModel.EnrollmentNotifyPerDecisionImmediate, *stored.DecisionNotificationMode)
}

func TestDecisionService_Decide_WaitlistedPromotionVersionsDigestState(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		notificationMode: configModel.EnrollmentNotifyPerDecisionDigest,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)
	submitted := submitDecisionSiblings(t, env, "digest-transition@example.com")

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionRejected,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[1].ID,
		Status:     enrollmentService.DecisionWaitlisted,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[1].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	// Same-state retry remains a no-op and must not enqueue another digest.
	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[1].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	digests := env.outbox.ByKind(platformModels.EmailKindEnrollmentDecisionDigest)
	require.Len(t, digests, 2)
	assert.NotEqual(t, digests[0].IdempotencyKey, digests[1].IdempotencyKey)
	assert.NotEmpty(t, digests[0].IdempotencyKey)
	assert.NotEmpty(t, digests[1].IdempotencyKey)
}

func TestDecisionService_Decide_ReopenedSameStatusVersionsParentNotification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
		kind string
	}{
		{name: "digest", mode: configModel.EnrollmentNotifyPerDecisionDigest, kind: platformModels.EmailKindEnrollmentDecisionDigest},
		{name: "immediate", mode: configModel.EnrollmentNotifyPerDecisionImmediate, kind: platformModels.EmailKindEnrollmentRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{notificationMode: tt.mode})
			defer cleanup()
			ctx := testpkg.Ctx(t)
			requestID, childID := submitOneChild(t, env, "repeated-decision-"+tt.name+"@example.com", "Lina", "Wiederholt")

			_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
				RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
			})
			require.NoError(t, err)
			// Approved change-request application uses this same repository
			// transition when a rejected child's stored data materially changes.
			require.NoError(t, env.repos.RequestChild.UpdateStatus(
				ctx, childID, enrollmentModels.ChildStatusUnderReview, nil, env.creatorID,
			))
			_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
				RequestID: requestID, ChildID: childID, Status: enrollmentService.DecisionRejected, ReviewedBy: env.creatorID,
			})
			require.NoError(t, err)

			notifications := env.outbox.ByKind(tt.kind)
			require.Len(t, notifications, 2)
			assert.NotEqual(t, notifications[0].IdempotencyKey, notifications[1].IdempotencyKey)
		})
	}
}

// ---- Decide: approval (the heavy path) ----------------------------------

func TestDecisionService_Decide_ApprovedCreatesDownstreamRecords(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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

func TestDecisionService_SyncApprovedChildData_RelinksPrimaryGuardian(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "old-primary@example.com", "Lina", "PrimaryRelink")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	beforeLinks, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, beforeLinks, 1)
	oldProfileID := beforeLinks[0].GuardianProfileID

	req, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	req.GuardianFirstName = "Neue"
	req.GuardianLastName = "Hauptperson"
	req.GuardianEmail = "new-primary@example.com"
	require.NoError(t, env.repos.Request.UpdateGuardianDataWithEmail(ctx, req))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	afterLinks, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, afterLinks, 1)
	assert.True(t, afterLinks[0].IsPrimary)
	assert.NotEqual(t, oldProfileID, afterLinks[0].GuardianProfileID)
	assert.Equal(t, authorize.GuardianRolePrimaryGuardian, afterLinks[0].GuardianRole)
	assert.True(t, authorize.StudentGuardianHasPermission(afterLinks[0], authorize.GuardianPermissionPortalAccess))

	newProfile, err := env.repos.GuardianProfile.FindByEmail(ctx, "new-primary@example.com")
	require.NoError(t, err)
	assert.Equal(t, newProfile.ID, afterLinks[0].GuardianProfileID)
}

func TestDecisionService_SyncApprovedChildData_UpdatesStandalonePrimaryGuardianName(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "primary-name-correction@example.com", "Lina", "NameCorrection")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	before, err := env.repos.GuardianProfile.FindByEmail(ctx, "primary-name-correction@example.com")
	require.NoError(t, err)
	assert.Equal(t, "Eltern", before.FirstName)
	assert.Equal(t, "Test", before.LastName)

	req, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	req.GuardianFirstName = "Mara"
	req.GuardianLastName = "Korrigiert"
	require.NoError(t, env.repos.Request.UpdateGuardianDataWithEmail(ctx, req))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	after, err := env.repos.GuardianProfile.FindByEmail(ctx, "primary-name-correction@example.com")
	require.NoError(t, err)
	assert.Equal(t, "Mara", after.FirstName)
	assert.Equal(t, "Korrigiert", after.LastName)
}

func TestDecisionService_SyncApprovedChildData_DoesNotOverwriteAccountLinkedPrimaryGuardianName(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "primary-account-name@example.com", "Lina", "AccountName")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	profile, err := env.repos.GuardianProfile.FindByEmail(ctx, "primary-account-name@example.com")
	require.NoError(t, err)
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Portal", "Parent")
	require.NoError(t, env.repos.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))

	req, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	req.GuardianFirstName = "Nicht"
	req.GuardianLastName = "Ueberschreiben"
	require.NoError(t, env.repos.Request.UpdateGuardianDataWithEmail(ctx, req))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	after, err := env.repos.GuardianProfile.FindByID(ctx, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, "Eltern", after.FirstName)
	assert.Equal(t, "Test", after.LastName)
	require.NotNil(t, after.AccountID)
	assert.Equal(t, account.ID, *after.AccountID)
}

func TestDecisionService_SyncApprovedChildData_ReconcilesRemovedAdditionalGuardians(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "primary-reconcile@example.com", "Lina", "CoGuardian")
	oldEmail := "old-co-guardian@example.com"
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID,
		FirstName: "Old",
		LastName:  "Guardian",
		Email:     &oldEmail,
		SortOrder: 0,
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

	previousGuardians, err := env.repos.RequestGuardian.ListByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Len(t, previousGuardians, 1)
	require.NotNil(t, previousGuardians[0].GuardianProfileID)
	oldProfileID := *previousGuardians[0].GuardianProfileID

	require.NoError(t, env.repos.RequestGuardian.DeleteByRequestID(ctx, reqID))
	newEmail := "new-co-guardian@example.com"
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID,
		FirstName: "New",
		LastName:  "Guardian",
		Email:     &newEmail,
		SortOrder: 0,
	}))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:                reqID,
		ChildID:                  childID,
		ActorAccountID:           env.creatorID,
		ReplaceTargetedData:      true,
		PreviousRequestGuardians: previousGuardians,
	})
	require.NoError(t, err)

	newProfile, err := env.repos.GuardianProfile.FindByEmail(ctx, newEmail)
	require.NoError(t, err)
	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	var oldLinked bool
	var newLinked bool
	for _, link := range links {
		if link.GuardianProfileID == oldProfileID {
			oldLinked = true
		}
		if link.GuardianProfileID == newProfile.ID {
			newLinked = true
			assert.False(t, link.IsPrimary)
			assert.True(t, link.IsEmergencyContact)
			assert.True(t, link.CanPickup)
		}
	}
	assert.False(t, oldLinked, "removed co-guardian must lose the approved child link")
	assert.True(t, newLinked, "new co-guardian must be linked to the approved child")
}

func TestDecisionService_SyncApprovedChildData_UpgradesExistingContactListLinkForCoGuardian(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionContactListSchema(t, env)

	contactEmail := "weak-contact-to-coguardian@example.com"
	weakContact := map[string]any{
		"first_name":           "Weak",
		"last_name":            "Contact",
		"email":                contactEmail,
		"is_emergency_contact": false,
		"can_pickup":           false,
	}
	reqID, childID := submitOneChildWithCustomData(t, env, "primary-upgrade-coguardian@example.com", "Lina", "Upgrade", map[string]any{
		"contacts": []any{weakContact},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	profile, err := env.repos.GuardianProfile.FindByEmail(ctx, contactEmail)
	require.NoError(t, err)
	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	var weakLink *usersModels.StudentGuardian
	for _, link := range links {
		if link != nil && link.GuardianProfileID == profile.ID {
			weakLink = link
			break
		}
	}
	require.NotNil(t, weakLink)
	require.False(t, weakLink.IsEmergencyContact)
	require.False(t, weakLink.CanPickup)

	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID,
		FirstName: "Weak",
		LastName:  "Contact",
		Email:     &contactEmail,
		SortOrder: 0,
	}))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	links, err = env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	var upgraded *usersModels.StudentGuardian
	for _, link := range links {
		if link != nil && link.GuardianProfileID == profile.ID {
			upgraded = link
			break
		}
	}
	require.NotNil(t, upgraded)
	assert.False(t, upgraded.IsPrimary)
	assert.True(t, upgraded.IsEmergencyContact)
	assert.True(t, upgraded.CanPickup)
	assert.Equal(t, authorize.GuardianRoleEmergency, upgraded.GuardianRole)
	assert.False(t, authorize.StudentGuardianHasPermission(upgraded, authorize.GuardianPermissionPortalAccess))
}

func TestDecisionService_Decide_UpdatesStandaloneCoGuardianNameWhenReusingEmail(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "primary-coguardian-name@example.com", "Kjell", "CoGuardianName")
	email := "coguardian-name-correction@example.com"
	profile := &usersModels.GuardianProfile{
		FirstName:              "Old",
		LastName:               "Name",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.GuardianProfile.Create(ctx, profile))
	require.NoError(t, env.repos.RequestGuardian.Create(ctx, &enrollmentModels.RequestGuardian{
		RequestID: reqID,
		FirstName: "New",
		LastName:  "Name",
		Email:     &email,
		SortOrder: 0,
	}))

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	updated, err := env.repos.GuardianProfile.FindByEmail(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, profile.ID, updated.ID)
	assert.Equal(t, "New", updated.FirstName)
	assert.Equal(t, "Name", updated.LastName)
}

// TestDecisionService_Decide_PersistsCoGuardianPhone verifies that on
// approval a co-guardian's phone number is written to
// users.guardian_phone_numbers, mirroring the primary guardian. The
// phone-only contact (no email) is the critical case: that number is the
// ONLY way to reach the approved emergency contact, so dropping it would
// leave them unreachable.
func TestDecisionService_Decide_PersistsCoGuardianPhone(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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

func TestDecisionService_SyncApprovedChildData_ReplacesRemovedContactList(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 3", []enrollmentModels.FormField{{
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

	oldEmail := "old-contact-list@example.com"
	reqID, childID := submitOneChildWithCustomData(t, env, "primary-contact-list@example.com", "Lina", "Contacts", map[string]any{
		"contacts": []any{map[string]any{
			"first_name":           "Old",
			"last_name":            "Contact",
			"email":                oldEmail,
			"is_emergency_contact": true,
			"can_pickup":           true,
		}},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	oldProfile, err := env.repos.GuardianProfile.FindByEmail(ctx, oldEmail)
	require.NoError(t, err)
	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(links), 2)

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
		PreviousSnapshot: map[string]any{
			"children": []any{map[string]any{
				"id": childID,
				"custom_data": map[string]any{
					"contacts": []any{map[string]any{
						"first_name":           "Old",
						"last_name":            "Contact",
						"email":                oldEmail,
						"is_emergency_contact": true,
						"can_pickup":           true,
					}},
				},
			}},
		},
	})
	require.NoError(t, err)

	links, err = env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	for _, link := range links {
		assert.NotEqual(t, oldProfile.ID, link.GuardianProfileID, "removed contact-list guardian must lose the approved child link")
	}
}

func TestDecisionService_SyncApprovedChildData_ReplacesRemovedPhoneOnlyContactList(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionContactListSchema(t, env)

	contact := phoneOnlyContactEntry("Phone", "Only", "0176 111222", true, true)
	reqID, childID := submitOneChildWithCustomData(t, env, "primary-phone-remove@example.com", "Lina", "PhoneRemove", map[string]any{
		"contacts": []any{contact},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	var oldProfileID int64
	for _, link := range links {
		if link != nil && !link.IsPrimary {
			oldProfileID = link.GuardianProfileID
			break
		}
	}
	require.NotZero(t, oldProfileID, "initial approval must create a phone-only contact link")

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
		PreviousSnapshot:    contactSnapshot(childID, []any{contact}),
	})
	require.NoError(t, err)

	links, err = env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	for _, link := range links {
		assert.NotEqual(t, oldProfileID, link.GuardianProfileID, "removed phone-only contact must lose the approved child link")
	}
}

func TestDecisionService_SyncApprovedChildData_ReusesUnchangedPhoneOnlyContactList(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionContactListSchema(t, env)

	contact := phoneOnlyContactEntry("Phone", "Stable", "0176 333444", true, true)
	reqID, childID := submitOneChildWithCustomData(t, env, "primary-phone-reuse@example.com", "Lina", "PhoneReuse", map[string]any{
		"contacts": []any{contact},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{"contacts": []any{contact}}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
		PreviousSnapshot:    contactSnapshot(childID, []any{contact}),
	})
	require.NoError(t, err)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	nonPrimary := 0
	for _, link := range links {
		if link != nil && !link.IsPrimary {
			nonPrimary++
		}
	}
	assert.Equal(t, 1, nonPrimary, "unchanged phone-only contact must reuse the existing profile/link instead of duplicating it")
}

func TestDecisionService_SyncApprovedChildData_UpdatesExistingContactListPermissions(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionContactListSchema(t, env)

	contactEmail := "toggle-contact-list@example.com"
	oldContact := map[string]any{
		"first_name":           "Toggle",
		"last_name":            "Contact",
		"email":                contactEmail,
		"is_emergency_contact": true,
		"can_pickup":           true,
	}
	reqID, childID := submitOneChildWithCustomData(t, env, "primary-contact-toggle@example.com", "Lina", "ContactToggle", map[string]any{
		"contacts": []any{oldContact},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	profile, err := env.repos.GuardianProfile.FindByEmail(ctx, contactEmail)
	require.NoError(t, err)

	newContact := map[string]any{
		"first_name":           "Toggle",
		"last_name":            "Contact",
		"email":                contactEmail,
		"is_emergency_contact": false,
		"can_pickup":           false,
	}
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{"contacts": []any{newContact}}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
		PreviousSnapshot:    contactSnapshot(childID, []any{oldContact}),
	})
	require.NoError(t, err)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	var contactLink *usersModels.StudentGuardian
	for _, link := range links {
		if link != nil && link.GuardianProfileID == profile.ID {
			contactLink = link
			break
		}
	}
	require.NotNil(t, contactLink)
	assert.False(t, contactLink.CanPickup)
	assert.False(t, contactLink.IsEmergencyContact)
	assert.Equal(t, authorize.GuardianRoleCustom, contactLink.GuardianRole)
	assert.False(t, authorize.StudentGuardianHasPermission(contactLink, authorize.GuardianPermissionPortalAccess))
}

func TestDecisionService_Decide_ContactListSelfGuardianDoesNotAbortApproval(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 4", []enrollmentModels.FormField{{
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
		TenantID:          testpkg.Tenant(t),
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Pin a schema that carries the unified departure field onto the phase.
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 5", []enrollmentModels.FormField{{
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
		TenantID:          testpkg.Tenant(t),
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 6", []enrollmentModels.FormField{{
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
		TenantID:          testpkg.Tenant(t),
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Entscheidung 7", []enrollmentModels.FormField{{
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
		TenantID:          testpkg.Tenant(t),
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

// TestDecisionService_Decide_MergesDuplicateAllowedDepartureFields covers the
// last-field-wins data loss (#1694): a legacy schema (created before
// FormSchema.Validate rejected duplicate targets) carries TWO fields targeting
// student.allowed_departure_modes — the first allows accompanied, the later one
// only bus. Validation/sanitization accept the submission on any-field semantics,
// so approval must MERGE the fields (union) rather than let the later bus-only
// field silently drop the accompanied mode and its companion note. The schema is
// inserted raw to reproduce the pre-validation shape that the model now forbids.
func TestDecisionService_Decide_MergesDuplicateAllowedDepartureFields(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Raw insert bypasses FormSchema.Validate (which now rejects duplicate
	// targets), reproducing a legacy schema already persisted in the DB.
	schema := &enrollmentModels.FormSchema{
		Name:      "legacy-dup-departure",
		Version:   1,
		CreatedBy: env.creatorID,
		Fields: []enrollmentModels.FormField{
			{Key: "modes_a", Label: "Erlaubte Heimwege", Type: enrollmentModels.FormFieldWeekdayMultiMode, Target: enrollmentModels.TargetStudentAllowedDepartureModes, AppliesToCh: true, SortOrder: 0},
			{Key: "modes_b", Label: "Heimwege (Kopie)", Type: enrollmentModels.FormFieldWeekdayMultiMode, Target: enrollmentModels.TargetStudentAllowedDepartureModes, AppliesToCh: true, SortOrder: 1},
		},
	}
	schema.SetTenantID(testpkg.Tenant(t))
	_, err := env.db.NewInsert().Model(schema).
		ModelTableExpr(`enrollment.form_schemas AS "form_schema"`).
		Returning("*").Exec(ctx)
	require.NoError(t, err)
	require.Greater(t, schema.ID, int64(0))
	// The schema row is left to the env teardown: requests created below hold an
	// FK to it, mirroring the other schema-creating tests in this file.

	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	grade := int16(2)
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Test",
		GuardianEmail:     "dup-departure@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Merge",
			LastName:         "Child",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			CustomData: map[string]any{
				// Earlier field allows accompanied; later field only bus. The later
				// field must NOT erase the accompanied mode from the earlier one.
				"modes_a": map[string]any{"mon": []any{"accompanied"}},
				"modes_b": map[string]any{"mon": []any{"bus"}},
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
	assert.True(t, student.AllowedDepartureModes.HasMode(usersModels.DepartureAccompanied),
		"accompanied mode from the earlier field must survive the merge")
	require.NotNil(t, student.DepartureCompanionNote, "companion note must survive the merge")
	assert.Equal(t, "Geschwisterkind Mia (1b)", *student.DepartureCompanionNote)
	require.NotNil(t, student.PickupStatus)
	assert.Equal(t, usersModels.PickupStatusAccompanied, *student.PickupStatus,
		"legacy pickup_status must reflect accompanied, not collapse to self-goer")
}

// ---- Decide: activation mode (enrollment.default_activation_mode) -------

// Default / "scheduled": an approved child becomes a PENDING student
// pinned to the phase's service-start date, so the activate-students
// scheduler flips it to active when that date arrives. Uses the nil-
// Settings setup, which must behave identically to an explicit
// "scheduled" override (the safe default).
func TestDecisionService_Decide_ApprovedScheduledKeepsStudentPending(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeImmediate,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{
		mode: configModel.EnrollmentActivationModeScheduled,
	})
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "activation-scheduled-past@example.com", "Past", "Start")
	startDate := timezone.NewDate(2026, 8, 24).AddDays(-1)
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
	t.Parallel()

	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{mode: "bogus"})
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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

func TestDecisionService_Decide_SchedulePickupUsesReviewerStaffID(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionScheduleSchema(t, env, "pickup_times", enrollmentModels.TargetSchedulePickup)
	reviewerStaff, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "pickup-schedule-author@example.com", "Anna", "Pickup", map[string]any{
		"pickup_times": map[string]any{"mon": "14:45", "wed": "16:00"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, reviewerStaff.ID, row.CreatedBy,
			"pickup schedule created_by must reference users.staff, not the reviewer account")
		assert.NotEqual(t, reviewerAccountID, row.CreatedBy,
			"the regression requires account id and staff id to stay distinct")
	}
}

func TestDecisionService_SyncApprovedChildData_ReplacesRemovedPickupSchedule(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionScheduleSchema(t, env, "pickup_times", enrollmentModels.TargetSchedulePickup)
	_, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "pickup-sync@example.com", "Anna", "PickupSync", map[string]any{
		"pickup_times": map[string]any{"mon": "14:45", "wed": "16:00"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      reviewerAccountID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	rows, err = env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "replacement sync must delete pickup schedules removed from the approved snapshot")
}

func TestDecisionService_SyncApprovedChildData_DuplicatePickupTargetDeletesOnce(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env, "legacy-dup-pickup", []enrollmentModels.FormField{
		{
			Key: "pickup_visible", Label: "Abholzeiten",
			Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentModels.TargetSchedulePickup, AllowedTimes: []string{"14:45", "16:00"}, SortOrder: 0,
		},
		{
			Key: "pickup_hidden", Label: "Abholzeiten alt",
			Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentModels.TargetSchedulePickup, AllowedTimes: []string{"14:45", "16:00"}, SortOrder: 1,
		},
	})
	_, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "pickup-duplicate-target@example.com", "Anna", "PickupDup", map[string]any{
		"pickup_visible": map[string]any{"mon": "14:45", "wed": "16:00"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      reviewerAccountID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	rows, err = env.repos.StudentPickupSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Len(t, rows, 2, "omitted duplicate pickup target must not delete rows recreated by the visible target")
}

func TestDecisionService_SyncApprovedChildData_DuplicateArrivalTargetDeletesOnce(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env, "legacy-dup-arrival", []enrollmentModels.FormField{
		{
			Key: "arrival_visible", Label: "Ankunftszeiten",
			Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentModels.TargetScheduleArrival, SortOrder: 0,
		},
		{
			Key: "arrival_hidden", Label: "Ankunftszeiten alt",
			Type: enrollmentModels.FormFieldWeekdaySchedule, AppliesToCh: true,
			Target: enrollmentModels.TargetScheduleArrival, SortOrder: 1,
		},
	})
	_, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "arrival-duplicate-target@example.com", "Anna", "ArrivalDup", map[string]any{
		"arrival_visible": map[string]any{"mon": "07:45"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	rows, err := env.repos.StudentArrivalSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      reviewerAccountID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	rows, err = env.repos.StudentArrivalSchedule.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "omitted duplicate arrival target must not delete rows recreated by the visible target")
}

func TestDecisionService_SyncApprovedChildData_DuplicateStudentTargetKeepsSubmittedSiblingValue(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env, "legacy-dup-health", []enrollmentModels.FormField{
		{
			Key: "health_visible", Label: "Gesundheit",
			Type: enrollmentModels.FormFieldTextarea, AppliesToCh: true,
			Target: enrollmentModels.TargetStudentHealthInfo, SortOrder: 0,
		},
		{
			Key: "health_hidden", Label: "Gesundheit alt",
			Type: enrollmentModels.FormFieldTextarea, AppliesToCh: true,
			Target: enrollmentModels.TargetStudentHealthInfo, SortOrder: 1,
		},
	})

	reqID, childID := submitOneChildWithCustomData(t, env, "health-duplicate-target@example.com", "Anna", "HealthDup", map[string]any{
		"health_visible": "Asthma",
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	student, err := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student.HealthInfo)
	assert.Equal(t, "Asthma", *student.HealthInfo)
}

func TestDecisionService_SyncApprovedChildData_AuditsTrackedStudentChanges(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env, "student-health-audit", []enrollmentModels.FormField{{
		Key:         "health_info",
		Label:       "Gesundheit",
		Type:        enrollmentModels.FormFieldTextarea,
		AppliesToCh: true,
		Target:      enrollmentModels.TargetStudentHealthInfo,
		SortOrder:   0,
	}})

	reqID, childID := submitOneChildWithCustomData(t, env, "health-audit@example.com", "Anna", "Audit", map[string]any{
		"health_info": "Asthma",
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{"health_info": "Diabetes"}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:           reqID,
		ChildID:             childID,
		ActorAccountID:      env.creatorID,
		ReplaceTargetedData: true,
	})
	require.NoError(t, err)

	history, err := env.repos.StudentFieldEdit.GetByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, auditModels.StudentFieldHealthInfo, history[0].FieldName)
	assert.Equal(t, env.creatorID, history[0].EditedBy)
	require.NotNil(t, history[0].OldValue)
	require.NotNil(t, history[0].NewValue)
	assert.Equal(t, "Asthma", *history[0].OldValue)
	assert.Equal(t, "Diabetes", *history[0].NewValue)
}

func TestDecisionService_SyncApprovedChildData_AuditsPersistedChangeDespiteTargetError(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env, "partial-student-health-audit", []enrollmentModels.FormField{
		{
			Key:         "health_info",
			Label:       "Gesundheit",
			Type:        enrollmentModels.FormFieldTextarea,
			AppliesToCh: true,
			Target:      enrollmentModels.TargetStudentHealthInfo,
			SortOrder:   0,
		},
		{
			Key:          "pickup_times",
			Label:        "Abholzeiten",
			Type:         enrollmentModels.FormFieldWeekdaySchedule,
			AppliesToCh:  true,
			Target:       enrollmentModels.TargetSchedulePickup,
			AllowedTimes: []string{"14:45"},
			SortOrder:    1,
		},
	})
	_, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "partial-health-audit@example.com", "Anna", "Audit", map[string]any{
		"health_info":  "Asthma",
		"pickup_times": map[string]any{"mon": "14:45"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{
		"health_info":  "Diabetes",
		"pickup_times": map[string]any{"mon": "14:45"},
	}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	actor := testpkg.CreateTestAccount(t, env.db, "partial-health-audit-reviewer")
	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: actor.ID,
	})
	require.NoError(t, err, "a best-effort schedule error must not discard a persisted student update")

	student, err := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student.HealthInfo)
	assert.Equal(t, "Diabetes", *student.HealthInfo)

	history, err := env.repos.StudentFieldEdit.GetByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, auditModels.StudentFieldHealthInfo, history[0].FieldName)
	assert.Equal(t, actor.ID, history[0].EditedBy)
	require.NotNil(t, history[0].OldValue)
	require.NotNil(t, history[0].NewValue)
	assert.Equal(t, "Asthma", *history[0].OldValue)
	assert.Equal(t, "Diabetes", *history[0].NewValue)
}

func TestDecisionService_Decide_ScheduleArrivalUsesReviewerStaffID(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionScheduleSchema(t, env, "arrival_times", enrollmentModels.TargetScheduleArrival)
	reviewerStaff, reviewerAccountID := createReviewerStaffWithDistinctAccount(t, env)

	reqID, childID := submitOneChildWithCustomData(t, env, "arrival-schedule-author@example.com", "Anna", "Arrival", map[string]any{
		"arrival_times": map[string]any{"mon": "07:45"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccountID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	rows, err := env.repos.StudentArrivalSchedule.FindByStudentID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, reviewerStaff.ID, rows[0].CreatedBy,
		"arrival schedule created_by must reference users.staff, not the reviewer account")
	assert.NotEqual(t, reviewerAccountID, rows[0].CreatedBy,
		"the regression requires account id and staff id to stay distinct")
	assert.True(t, rows[0].ExpectedArrival.IsZero(),
		"enrollment rows mark the care day and inherit later class-time changes")
}

func TestDecisionService_Decide_ScheduleTargetWithoutReviewerStaffDoesNotAbortApproval(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishDecisionScheduleSchema(t, env, "pickup_times", enrollmentModels.TargetSchedulePickup)
	reviewerAccount := testpkg.CreateTestAccount(t, env.db, "reviewer-without-staff")

	reqID, childID := submitOneChildWithCustomData(t, env, "pickup-no-staff@example.com", "Anna", "NoStaff", map[string]any{
		"pickup_times": map[string]any{"mon": "14:45"},
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: reviewerAccount.ID,
	})
	require.NoError(t, err, "targeted-field schedule failures must not poison the approval transaction")
	require.NotNil(t, outcome.Child.CreatedStudentID)
	assert.Equal(t, enrollmentModels.ChildStatusApproved, outcome.Child.Status)

	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "without a staff author, schedule dispatch should skip before inserting")
}

// TestDecisionService_Decide_ExistingStudentScheduleReplacementFailureRollsBack
// locks in the #1663 fix. The existing-student re-enrollment path runs
// applyTargetedFields with ReplaceSchedules, which DELETES the matched
// student's live arrival/pickup rows before re-inserting the resubmitted
// weekdays. A dispatch failure AFTER that delete must be fatal so the
// surrounding tenant tx rolls the whole approval back: the live schedule must
// never be committed deleted-but-not-rebuilt. Contrast with
// TestDecisionService_Decide_ScheduleTargetWithoutReviewerStaffDoesNotAbortApproval
// above, which is the additive fresh-create path where a skip is harmless.
func TestDecisionService_Decide_ExistingStudentScheduleReplacementFailureRollsBack(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// A pickup-schedule form field so approval takes the ReplaceSchedules
	// delete-then-reinsert path.
	publishDecisionScheduleSchema(t, env, "pickup_times", enrollmentModels.TargetSchedulePickup)

	// The already-enrolled student, carrying a live pickup schedule row.
	existing := testpkg.CreateTestStudent(t, env.db, "Mara", "Bestand", "2a")
	scheduleAuthor := testpkg.CreateTestStaff(t, env.db, "Betreuer", "Bestand")
	seededPickup := testpkg.CreateTestPickupSchedule(t, env.db, existing.ID, scheduleModels.WeekdayMonday, scheduleAuthor.ID, "14:45")

	// A fresh submission carrying a resubmitted pickup schedule, matched to the
	// existing student so approval renews it instead of creating a duplicate.
	reqID, childID := submitOneChildWithCustomData(t, env, "existing-reenroll@example.com", "Mara", "Bestand", map[string]any{
		"pickup_times": map[string]any{"mon": "16:00"},
	})
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.RequestChild)(nil)).
		ModelTableExpr(`enrollment.request_children AS "request_child"`).
		Set("matched_student_id = ?", existing.ID).
		Where(`"request_child".id = ?`, childID).
		Exec(ctx)
	require.NoError(t, err)

	// Reviewer account with no linked staff → resolveReviewerStaffID fails, so
	// dispatchWeekdaySchedule errors AFTER DeleteByStudentID has already run.
	reviewerNoStaff := testpkg.CreateTestAccount(t, env.db, "reviewer-no-staff-existing")

	// Drive Decide inside a tenant tx (production wraps it via middleware) so a
	// returned error actually rolls the schedule delete back.
	decideErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := env.decision.Decide(txCtx, enrollmentService.DecideInput{
			RequestID:  reqID,
			ChildID:    childID,
			Status:     enrollmentService.DecisionApproved,
			ReviewedBy: reviewerNoStaff.ID,
		})
		return err
	})
	require.Error(t, decideErr, "a schedule-replacement dispatch failure must abort the existing-student approval")

	// The rollback must preserve the student's original pickup schedule.
	rows, err := env.repos.StudentPickupSchedule.FindByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the live pickup schedule must survive the rolled-back approval")
	assert.Equal(t, seededPickup.Weekday, rows[0].Weekday)

	// The child must not be left approved — the whole decision rolled back.
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status,
		"a rolled-back approval must leave the child in its pre-decision status")
}

// matchChildToExistingStudent pins matched_student_id on a submitted child, the
// way an existing_students phase does at submission time, so approval takes the
// re-enrollment branch instead of creating a second Person + Student.
func matchChildToExistingStudent(t *testing.T, env *decisionTestEnv, childID, studentID int64) {
	t.Helper()
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.RequestChild)(nil)).
		ModelTableExpr(`enrollment.request_children AS "request_child"`).
		Set("matched_student_id = ?", studentID).
		Where(`"request_child".id = ?`, childID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func TestDecisionService_Decide_ExistingStudentCareRenewalObsoletesWithdrawalWithoutGap(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestStudent(t, env.db, "Lina", "Verlaengert", "2a")
	requestID, childID := submitReEnrollment(
		t, env,
		"Eltern", "Verlaengert", "renewal-with-care@example.com", nil,
		"Lina", "Verlaengert", map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
	)
	matchChildToExistingStudent(t, env, childID, existing.ID)
	care := createAdjustmentCareOfferingWith(t, env, "Betreuung Verlaengerung", func(offering *enrollmentModels.CareOffering) {
		offering.CountsAsCare = true
		offering.CountsAsCareSet = true
	})
	createChildOfferingLink(t, env, childID, care.ID, nil, nil)

	studentID := existing.ID
	actorID := env.creatorID
	repo := env.repos.CareWithdrawal
	require.NoError(t, repo.UpsertPending(ctx, &usersModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: env.sourcePhase.ServiceStartDate,
		Trigger:               usersModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actorID, WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt: time.Now(),
	}))

	err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, decideErr := env.decision.Decide(txCtx, enrollmentService.DecideInput{
			RequestID: requestID, ChildID: childID,
			Status: enrollmentService.DecisionApproved, ReviewedBy: actorID,
		})
		return decideErr
	})
	require.NoError(t, err)
	pending, err := pendingWithdrawalForStudent(ctx, repo, existing.ID)
	require.NoError(t, err)
	assert.Nil(t, pending, "a renewal starting on the first bookingless day leaves no care gap")
}

// submitReEnrollment submits a full renewal form for an already-enrolled child.
// Consent flags are caller-supplied because withdrawal is the point of one of
// the tests below.
func submitReEnrollment(
	t *testing.T,
	env *decisionTestEnv,
	guardianFirst, guardianLast, guardianEmail string,
	guardianPhone *string,
	childFirst, childLast string,
	consentFlags map[string]any,
) (requestID, childID int64) {
	t.Helper()
	grade := int16(2)
	res, err := env.requestSvc.Submit(testpkg.Ctx(t), enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: guardianFirst,
		GuardianLastName:  guardianLast,
		GuardianEmail:     guardianEmail,
		GuardianPhone:     guardianPhone,
		ConsentFlags:      consentFlags,
		Children: []enrollmentService.SubmitChild{{
			FirstName:        childFirst,
			LastName:         childLast,
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
		}},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	return res.Request.ID, res.Children[0].ID
}

func markRequestAsUsedLateInvite(
	t *testing.T,
	env *decisionTestEnv,
	requestID int64,
	guardianEmail string,
) func() {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := env.db.NewUpdate().
		Model((*enrollmentModels.Request)(nil)).
		ModelTableExpr(`enrollment.requests AS "request"`).
		Set("submission_source = ?", enrollmentModels.RequestSourceLateInvite).
		Where(`"request".id = ?`, requestID).
		Exec(ctx)
	require.NoError(t, err)

	invite := &enrollmentModels.LateInvite{
		PhaseID:       env.sourcePhase.ID,
		TokenHash:     "decision-access-identity-" + t.Name(),
		GuardianEmail: guardianEmail,
		ExpiresAt:     time.Now().Add(time.Hour),
		CreatedBy:     env.creatorID,
	}
	require.NoError(t, env.repos.LateInvite.Create(ctx, invite))
	require.NoError(t, env.repos.LateInvite.MarkUsed(ctx, invite.ID, requestID, time.Now()))

	return func() {
		_, _ = env.db.NewDelete().
			Model((*enrollmentModels.LateInvite)(nil)).
			Where("id = ?", invite.ID).
			Exec(context.Background())
	}
}

// TestDecisionService_Decide_ExistingStudentAppliesWithdrawnConsent locks in the
// #1663 fix for consent withdrawal. The re-enrollment form re-asks every
// configured consent, so a box the guardian leaves unchecked must CLEAR the
// timestamp the matched student still carries from the original enrollment —
// otherwise the school keeps acting on a consent that was explicitly withdrawn.
func TestDecisionService_Decide_ExistingStudentAppliesWithdrawnConsent(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestStudent(t, env.db, "Nele", "Widerruf", "2a")

	// Consent recorded by the ORIGINAL enrollment.
	stamped := time.Now().Add(-24 * time.Hour)
	_, err := env.db.NewUpdate().
		Model((*usersModels.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("photo_consent_given_at = ?", stamped).
		Set("photo_consent_given_by = ?", env.creatorID).
		Set("email_contact_accepted_at = ?", stamped).
		Where(`"student".id = ?`, existing.ID).
		Exec(ctx)
	require.NoError(t, err)

	// The renewal withdraws photo + email contact and keeps the required ones.
	reqID, childID := submitReEnrollment(t, env, "Nina", "Widerruf", "withdraw-consent@example.com", nil,
		"Nele", "Widerruf", map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   false,
			"photo":           false,
		})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	student, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	assert.Nil(t, student.PhotoConsentGivenAt, "withdrawn photo consent must be cleared on approval")
	assert.Nil(t, student.PhotoConsentGivenBy, "the photo-consent author must be cleared with the consent")
	assert.Nil(t, student.EmailContactAcceptedAt, "withdrawn email-contact consent must be cleared on approval")
	assert.NotNil(t, student.AGBAcceptedAt, "accepted consent must still be stamped")
	assert.NotNil(t, student.DataProcessingAcceptedAt, "accepted consent must still be stamped")
}

// TestDecisionService_Decide_ExistingStudentLinksSubmittedGuardian locks in the
// #1663 fix for guardian reconciliation. A matched student can be imported,
// manually created, or enrolled before guardian links existed — assuming it
// already carries the submitted guardian silently discards that guardian, their
// phone number and their portal access.
func TestDecisionService_Decide_ExistingStudentLinksSubmittedGuardian(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// An imported child: enrolled, but with no guardian relationship at all.
	existing := testpkg.CreateTestStudent(t, env.db, "Milo", "Import", "3b")

	guardianEmail := "reenroll-guardian@example.com"
	guardianPhone := "015126829060"
	reqID, childID := submitReEnrollment(t, env, "Mara", "Import", guardianEmail, &guardianPhone,
		"Milo", "Import", map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	require.Len(t, links, 1, "the submitted guardian must be linked to the matched student")
	assert.True(t, links[0].IsPrimary, "the submitted guardian becomes the primary relationship")
	assert.True(t, links[0].CanPickup)

	profile, err := env.repos.GuardianProfile.FindByID(ctx, links[0].GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile.Email)
	assert.Equal(t, guardianEmail, *profile.Email)

	phones, err := env.repos.GuardianPhoneNumber.FindByGuardianID(ctx, profile.ID)
	require.NoError(t, err)
	require.Len(t, phones, 1, "the submitted phone number must reach the guardian profile")
	assert.Equal(t, guardianPhone, phones[0].PhoneNumber)

	require.NotNil(t, outcome.PendingInvite,
		"a guardian without a portal account must be invited, or the renewal grants no portal access")
	assert.Equal(t, profile.ID, outcome.PendingInvite.GuardianProfileID)

	student, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	require.NotNil(t, student.GuardianEmail)
	assert.Equal(t, guardianEmail, *student.GuardianEmail, "the renewal restates the guardian contact data")
	require.NotNil(t, student.GuardianPhone)
	assert.Equal(t, guardianPhone, *student.GuardianPhone)
}

func TestDecisionService_Decide_LateInviteRenewalLinksInviteRecipient(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	unrelatedParent := testpkg.CreateTestParentGuardianChain(t, env.db)
	existing := testpkg.CreateTestStudent(t, env.db, "Milo", "Nachzuegler", "3b")

	invitedEmail := "invited-renewal@example.test"
	reqID, childID := submitReEnrollment(t, env, "Mara", "Nachzuegler", unrelatedParent.Email, nil,
		"Milo", "Nachzuegler", map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		})
	cleanupInvite := markRequestAsUsedLateInvite(t, env, reqID, invitedEmail)
	defer cleanupInvite()
	matchChildToExistingStudent(t, env, childID, existing.ID)

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.NotEqual(t, unrelatedParent.GuardianProfileID, links[0].GuardianProfileID,
		"the editable contact address must not grant an unrelated account access")
	profile, err := env.repos.GuardianProfile.FindByID(ctx, links[0].GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile.Email)
	assert.Equal(t, invitedEmail, *profile.Email)

	student, err := env.repos.Student.FindByID(ctx, existing.ID)
	require.NoError(t, err)
	require.NotNil(t, student.GuardianEmail)
	assert.Equal(t, unrelatedParent.Email, *student.GuardianEmail,
		"the corrected address must remain available as contact data")
	require.NotNil(t, outcome.PendingInvite)
	assert.Equal(t, profile.ID, outcome.PendingInvite.GuardianProfileID)
}

// TestDecisionService_Decide_ExistingStudentKeepsForeignPrimaryGuardianLink is
// the counterweight to the test above: the submitted guardian becomes primary,
// but a guardian who held that link from a DIFFERENT source (last year's
// approval, an import, the other parent) keeps their relationship. Deleting it
// would strip a real guardian of pickup authority and parent-portal access just
// because the other parent filed this year's renewal (#1663).
func TestDecisionService_Decide_ExistingStudentKeepsForeignPrimaryGuardianLink(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestStudent(t, env.db, "Jonte", "Zweitkind", "1a")

	// The other parent, primary since the original enrollment.
	otherParent := testpkg.CreateTestGuardianProfile(t, env.db, "other-parent")
	otherLink := &usersModels.StudentGuardian{
		StudentID:          existing.ID,
		GuardianProfileID:  otherParent.ID,
		RelationshipType:   "guardian",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
	}
	authorize.ApplyStudentGuardianRole(otherLink, authorize.GuardianRolePrimaryGuardian)
	otherLink.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.StudentGuardian.Create(ctx, otherLink))

	reqID, childID := submitReEnrollment(t, env, "Sven", "Zweitkind", "submitting-parent@example.com", nil,
		"Jonte", "Zweitkind", map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		})
	matchChildToExistingStudent(t, env, childID, existing.ID)

	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	links, err := env.repos.StudentGuardian.FindByStudentID(ctx, existing.ID)
	require.NoError(t, err)
	require.Len(t, links, 2, "the pre-existing guardian must survive the renewal")

	var kept, submitted *usersModels.StudentGuardian
	for _, link := range links {
		if link.GuardianProfileID == otherParent.ID {
			kept = link
			continue
		}
		submitted = link
	}
	require.NotNil(t, kept, "the pre-existing guardian link must not be deleted")
	require.NotNil(t, submitted, "the submitted guardian must be linked")
	assert.False(t, kept.IsPrimary, "only one primary link may remain")
	assert.True(t, kept.CanPickup, "demotion must not strip the kept guardian's pickup authority")
	assert.True(t, submitted.IsPrimary, "the submitted guardian becomes the primary relationship")
}

func TestDecisionService_Decide_ApprovedIsIdempotent(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Fixed-Days")
	room := testpkg.CreateTestRoom(t, env.db, "Decision-Fixed-Days")
	group := &activitiesModels.Group{
		Name:            "Decision Fixed Days",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      true,
		PlannedRoomID:   &room.ID,
	}
	group.SetTenantID(testpkg.Tenant(t))
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
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Fixed Tue Thu",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"tue", "thu"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	req := enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, *outcome.Child.CreatedStudentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Scan(ctx))
	require.Len(t, rows, 1)
	assert.Equal(t, []int{2, 4}, rows[0].SelectedWeekdays,
		"fixed offering approval must constrain enrollment to available_days")
	require.NotNil(t, rows[0].CalendarPeriodID)
	assert.Equal(t, period.ID, *rows[0].CalendarPeriodID)
}

func TestDecisionService_UpdateChildOfferings_RebuildsEverySplitSeriesSegment(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := &scheduleModels.CalendarPeriod{
		Name: "decision-split-series-" + t.Name(), PeriodType: scheduleModels.PeriodTypeCustom,
		StartDate: timezone.NewDate(2026, 8, 1), EndDate: timezone.NewDate(2027, 8, 31),
		WeekCycleLength: 1, IsActive: true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CalendarPeriod.Create(ctx, period))
	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Split-Series")
	rootRoom := testpkg.CreateTestRoom(t, env.db, "Decision-Split-Root")
	successorRoom := testpkg.CreateTestRoom(t, env.db, "Decision-Split-Successor")
	timeframe := testpkg.CreateTestTimeframeForTenant(t, env.db, testpkg.Tenant(t), "Decision Split")
	root := &activitiesModels.Group{
		Name: "Decision Split Root", Type: activitiesModels.GroupTypeCare,
		CategoryID: category.ID, MaxParticipants: 20, IsOpen: true,
		IsTemplate: true, CalendarPeriodID: &period.ID, PlannedRoomID: &rootRoom.ID,
	}
	root.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, root))
	successor := &activitiesModels.Group{
		Name: "Decision Split Successor", Type: activitiesModels.GroupTypeCare,
		CategoryID: category.ID, MaxParticipants: 20, IsOpen: true,
		IsTemplate: true, CalendarPeriodID: &period.ID, SeriesRootID: &root.ID, PlannedRoomID: &successorRoom.ID,
	}
	successor.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, successor))
	boundary := timezone.NewDate(2027, 1, 1)
	rootSchedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: root.ID,
		ValidUntil: &boundary, TimeframeID: &timeframe.ID,
	}
	rootSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivitySchedule.Create(ctx, rootSchedule))
	successorSchedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: successor.ID,
		ValidFrom: &boundary, TimeframeID: &timeframe.ID,
	}
	successorSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivitySchedule.Create(ctx, successorSchedule))
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("activities.schedules").
			Where("activity_group_id IN (?)", bun.List([]int64{root.ID, successor.ID})).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("activities.groups").
			Where("id IN (?)", bun.List([]int64{root.ID, successor.ID})).Exec(context.Background())
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID: env.sourcePhase.ID, ActivityGroupID: &root.ID,
		Name: "Split Series Care", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon"}, IsActive: true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "SplitSeries",
		GuardianEmail: "split-series-roster@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName: "Sina", LastName: "SplitSeries",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2), OfferingIDs: []int64{offering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	childID := submitted.Children[0].ID
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	assertSeriesRoster := func() {
		rows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
		byGroup := make(map[int64]activitiesModels.StudentEnrollment, len(rows))
		for _, row := range rows {
			if row.EnrollmentRequestChildID != nil && *row.EnrollmentRequestChildID == childID {
				byGroup[row.ActivityGroupID] = row
			}
		}
		require.Contains(t, byGroup, root.ID)
		require.Contains(t, byGroup, successor.ID)
		assert.Equal(t, []int{activitiesModels.WeekdayMonday}, byGroup[root.ID].SelectedWeekdays)
		assert.Equal(t, []int{activitiesModels.WeekdayMonday}, byGroup[successor.ID].SelectedWeekdays)
	}
	assertSeriesRoster()

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Serienzuordnung erneut speichern",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{{
			OfferingID: offering.ID,
		}},
	})
	require.NoError(t, err)
	assertSeriesRoster()

	lockedDecision := newDecisionServiceForTest(
		env.rolloverTestEnv,
		nil,
		func(lockCtx context.Context) error {
			return scheduleService.LockTenantRecurrenceWrites(lockCtx, env.db)
		},
	)
	assertOfferingAdjustmentWaitsForRecurrenceGate(t, env, lockedDecision, enrollmentService.UpdateChildOfferingsInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Serienzuordnung unter Sperre erneut speichern",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{{
			OfferingID: offering.ID,
		}},
	})
	assertSeriesRoster()
}

func TestDecisionService_Decide_ApprovedPreservesLegacyNonTemplateLinkedOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Legacy-Linked")
	group := &activitiesModels.Group{
		Name:            "Decision Legacy Linked",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      false,
	}
	group.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Legacy Linked",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon", "wed"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, *outcome.Child.CreatedStudentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Scan(ctx))
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].CalendarPeriodID)
	assert.Empty(t, rows[0].SelectedWeekdays)
}

// Since #2249 the rollover clones the offering catalog into the target
// phase and remaps the carried booking onto the clone — including a
// historical non-template activity-group link, which the clone carries
// verbatim so Decide keeps materializing it for the target window.
func TestDecisionService_Decide_RolloverApprovalMaterializesClonedOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Rollover-Linked")
	group := &activitiesModels.Group{
		Name:            "Decision Rollover Linked",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      false,
	}
	group.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivityGroup.Create(ctx, group))

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Rollover Source Linked",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon", "wed"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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

	clones, err := env.repos.CareOffering.ListByPhase(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, clones, 1)
	clone := clones[0]
	assert.NotEqual(t, offering.ID, clone.ID)
	require.NotNil(t, clone.ActivityGroupID)
	assert.Equal(t, group.ID, *clone.ActivityGroupID,
		"a historical non-template link is carried verbatim onto the clone")

	rolledLinks, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled.ID)
	require.NoError(t, err)
	require.Len(t, rolledLinks, 1)
	assert.Equal(t, clone.ID, rolledLinks[0].CareOfferingID,
		"the carried booking must reference the target-phase clone (#2249)")

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
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, studentID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Where(`"student_enrollment".valid_from = ?`, result.Phase.ServiceStartDate).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count,
		"rollover approval must materialize the copied source-phase offering for the target phase window")

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      rolled.RequestID,
		ChildID:        rolled.ID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Rollover-Angebot unverändert gespeichert",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: clone.ID},
		},
	})
	require.NoError(t, err)
	rolledLinks, err = env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled.ID)
	require.NoError(t, err)
	require.Len(t, rolledLinks, 1)
	assert.Equal(t, clone.ID, rolledLinks[0].CareOfferingID,
		"adjustment save must keep the target-phase clone reference")
}

func TestDecisionService_Decide_ApprovedRejectsEmptyDaysForTemplateOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	category := testpkg.CreateTestActivityCategory(t, env.db, "Decision-Empty-Days")
	room := testpkg.CreateTestRoom(t, env.db, "Decision-Empty-Days")
	group := &activitiesModels.Group{
		Name:            "Decision Empty Days",
		Type:            activitiesModels.GroupTypeCare,
		CategoryID:      category.ID,
		MaxParticipants: 20,
		IsOpen:          true,
		IsTemplate:      true,
		PlannedRoomID:   &room.ID,
	}
	group.SetTenantID(testpkg.Tenant(t))
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
	}()

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Empty Template Days",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"tue"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))
	// Simulate a legacy row saved before #1885 made available_days
	// mandatory: clear the days directly, bypassing Validate. Such rows
	// still exist in production and Decide must keep rejecting them.
	_, updErr := env.db.NewUpdate().
		TableExpr("enrollment.care_offerings").
		Set("available_days = '[]'::jsonb").
		Where("id = ?", offering.ID).
		Exec(ctx)
	require.NoError(t, updErr)

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := env.decision.ListChildOfferings(ctx, 0)
	require.Error(t, err)
}

func TestDecisionService_ListChildOfferings_EmptyWhenNoOfferingsPicked(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "lco-empty@example.com", "Lina", "NoCare")

	rows, err := env.decision.ListChildOfferings(ctx, reqID)
	require.NoError(t, err)
	// Child exists in the map with both groups empty; no offerings selected.
	assert.Empty(t, rows[childID].Current)
	assert.Empty(t, rows[childID].Upcoming)
}

// #2185: the parent portal shows lunch/holiday/price and the validity of every
// booking, including one that only takes effect later. Staff answering a
// guardian's question must see the same facts, so ListChildOfferings carries
// the offering attributes and keeps future rows (flagged), dropping only rows
// whose interval has already ended.
func TestDecisionService_ListChildOfferings_CarriesAttributesAndFutureBookings(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	today := timezone.NewDate(2026, 8, 24)
	setSourcePhaseServiceStartDate(t, env, today.AddDays(-30))
	reqID, childID := submitOneChild(t, env, "lco-attrs@example.com", "Lina", "Attrs")

	price := 8550
	current := createAdjustmentCareOfferingWith(t, env, "Ganztag aktuell", func(o *enrollmentModels.CareOffering) {
		o.IncludesLunch = true
		o.IncludesHolidayCare = true
		o.PriceCents = &price
	})
	upcoming := createAdjustmentCareOfferingWith(t, env, "Ganztag ab Herbst", nil)
	ended := createAdjustmentCareOfferingWith(t, env, "Randstunde beendet", nil)

	futureStart := today.AddDays(30)
	endedUntil := today
	createChildOfferingLink(t, env, childID, current.ID, nil, nil)
	createChildOfferingLink(t, env, childID, upcoming.ID, &futureStart, nil)
	createChildOfferingLink(t, env, childID, ended.ID, nil, &endedUntil)

	rows, err := env.decision.ListChildOfferings(ctx, reqID)
	require.NoError(t, err)

	byOffering := map[int64]enrollmentService.ChildOfferingRow{}
	for _, row := range rows[childID].Current {
		byOffering[row.OfferingID] = row
	}
	require.Len(t, byOffering, 1, "only the effective booking is the selection on file")
	require.Len(t, rows[childID].Upcoming, 1, "the future booking is kept, separately")
	assert.Equal(t, upcoming.ID, rows[childID].Upcoming[0].OfferingID)

	currentRow := byOffering[current.ID]
	assert.True(t, currentRow.IncludesLunch)
	assert.True(t, currentRow.IncludesHolidayCare)
	require.NotNil(t, currentRow.PriceCents)
	assert.Equal(t, price, *currentRow.PriceCents)
	assert.False(t, currentRow.StartsLater)

	upcomingRow := rows[childID].Upcoming[0]
	assert.True(t, upcomingRow.StartsLater, "a booking starting in the future must be flagged")
	require.NotNil(t, upcomingRow.ValidFrom)
	assert.Equal(t, futureStart, *upcomingRow.ValidFrom)
}

// #2185 parity guard: the editor seeds from ListChildOfferings' Current
// group, the save replaces whatever UpdateChildOfferings reads as current.
// If those two ever answer differently, an untouched save deletes the
// difference.
//
// Scope, honestly: this pins agreement for the states reachable today. It
// does NOT by itself prove the read path may re-derive the rule — a
// hand-written predicate passes this test, because the repository's
// pre-phase-start fallback is unreachable while currentOfferingSelectionDate
// clamps up to the service start (see
// TestRequestChildOfferingRepo_AtDateBeforeServiceStart for the branch a
// copy would miss). That is exactly why the read path asks the repository
// instead of copying the predicate: the two are then the same code, and no
// future change to the clamp can separate them.
func TestDecisionService_ListChildOfferings_MatchesWritePathSelection(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	today := timezone.TodayDate()
	for _, tc := range []struct {
		name         string
		slug         string
		serviceStart timezone.Date
	}{
		{"phase still ahead", "ahead", today.AddDays(45)},
		{"phase running", "running", today.AddDays(-30)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setSourcePhaseServiceStartDate(t, env, tc.serviceStart)
			reqID, childID := submitOneChild(t, env,
				"lco-parity-"+tc.slug+"@example.com", "Lina", "Parity")

			current := createAdjustmentCareOfferingWith(t, env, "Aktuell "+tc.name, nil)
			later := createAdjustmentCareOfferingWith(t, env, "Später "+tc.name, nil)
			laterStart := tc.serviceStart.AddDays(60)
			createChildOfferingLink(t, env, childID, current.ID, nil, &laterStart)
			createChildOfferingLink(t, env, childID, later.ID, &laterStart, nil)

			rows, err := env.decision.ListChildOfferings(ctx, reqID)
			require.NoError(t, err)

			// What the save would replace, read exactly as the write path reads it.
			phase, err := env.repos.Phase.FindByID(ctx, env.sourcePhase.ID)
			require.NoError(t, err)
			writeLinks, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(
				ctx, childID, enrollmentService.WriteSelectionDateForTest(phase))
			require.NoError(t, err)

			readIDs := make([]int64, 0, len(rows[childID].Current))
			for _, row := range rows[childID].Current {
				readIDs = append(readIDs, row.OfferingID)
			}
			writeIDs := make([]int64, 0, len(writeLinks))
			for _, link := range writeLinks {
				writeIDs = append(writeIDs, link.CareOfferingID)
			}
			assert.ElementsMatch(t, writeIDs, readIDs,
				"the editor must seed exactly the selection the save replaces")
		})
	}
}

// catalogFailureRepo makes only the catalog batch lookup fail; every other
// method keeps the real behaviour through the embedded repository.
type catalogFailureRepo struct {
	enrollmentModels.CareOfferingRepository
}

func (catalogFailureRepo) ListByIDs(_ context.Context, _ []int64) ([]*enrollmentModels.CareOffering, error) {
	return nil, errors.New("synthetic catalog outage")
}

// #2185: a catalog outage must NOT swallow the child's bookings. The admin
// detail handler treats ListChildOfferings as best-effort, so an error there
// renders the child with no Betreuungsangebote at all — the admin then
// corrects on top of an empty selection and the save deletes the family's
// real bookings. Degrading to rows without catalog data keeps that from
// happening.
func TestDecisionService_ListChildOfferings_DegradesOnCatalogFailure(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "lco-catalog@example.com", "Lina", "Catalog")
	offering := createAdjustmentCareOffering(t, env, "Randstunde Katalogausfall")
	createChildOfferingLink(t, env, childID, offering.ID, nil, nil)

	repoFactory := repositories.NewFactory(env.db)
	degraded := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         catalogFailureRepo{repoFactory.CareOffering},
		PhaseRepo:                repoFactory.Phase,
	})

	rows, err := degraded.ListChildOfferings(ctx, reqID)
	require.NoError(t, err, "a catalog outage must not fail the whole lookup")
	require.Len(t, rows[childID].Current, 1,
		"the booking must survive without its catalog entry")
	assert.Equal(t, offering.ID, rows[childID].Current[0].OfferingID)
}

func createChildOfferingLink(
	t *testing.T,
	env *decisionTestEnv,
	requestChildID, careOfferingID int64,
	validFrom, validUntil *timezone.Date,
) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID: requestChildID,
		CareOfferingID: careOfferingID,
		SelectedDays:   []string{"mon"},
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, link))
}

// #2185: the staff view and the parent portal must judge "starts later" from
// the SAME day. Clamping the staff reference date up to the service start
// (what reportOfferingDate does, for reports) hid exactly the badge the
// parents app shows during the whole pre-start review period — a guardian on
// the phone saw "ab 06.10." on every booking, staff saw none.
//
// Only the badge half of that divergence is reachable: a row cannot end
// before the service start (the repository defaults valid_from to the phase
// start and request_child_offerings_nonempty_validity requires
// valid_from < valid_until), so no pre-start row can be dropped either way.
func TestDecisionService_ListChildOfferings_UsesTodayBeforeServiceStart(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	today := timezone.TodayDate()
	serviceStart := today.AddDays(45)
	setSourcePhaseServiceStartDate(t, env, serviceStart)
	reqID, childID := submitOneChild(t, env, "lco-prestart@example.com", "Lina", "PreStart")

	fromStart := createAdjustmentCareOfferingWith(t, env, "Ab Periodenstart", nil)
	createChildOfferingLink(t, env, childID, fromStart.ID, &serviceStart, nil)

	rows, err := env.decision.ListChildOfferings(ctx, reqID)
	require.NoError(t, err)

	// The write path clamps its selection date up to the service start, so
	// this very row IS what a correction replaces. Grouping it as "upcoming"
	// would let the editor seed empty and the save delete the family's
	// bookings.
	require.Len(t, rows[childID].Current, 1,
		"a pre-start booking is not yet in effect but is still the selection on file")
	assert.Equal(t, fromStart.ID, rows[childID].Current[0].OfferingID)
	assert.True(t, rows[childID].Current[0].StartsLater,
		"a booking effective from the service start has not started while the phase is still ahead")
	assert.Empty(t, rows[childID].Upcoming)
}

func TestBookingViewDate(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 8)

	assert.Equal(t, today, enrollmentService.BookingViewDate(today, timezone.NewDate(2027, 7, 31)),
		"inside or ahead of the period the reference date is simply today")
	assert.Equal(t, today, enrollmentService.BookingViewDate(today, timezone.Date{}),
		"a missing period end must not move the reference date")
	assert.Equal(t, timezone.NewDate(2026, 7, 31),
		enrollmentService.BookingViewDate(today, timezone.NewDate(2026, 7, 31)),
		"after the period ended the final state is what a reader needs")
}

// ---- Offering adjustments ----------------------------------------------

func TestDecisionService_UpdateChildOfferings_RejectsNonApprovedChild(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "adjust-pending@example.com", "Lina", "Pending")
	offering := createAdjustmentCareOffering(t, env, "Randstunde Pending")

	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Testkorrektur",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID, SelectedDays: []string{"fri"}},
		},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingAdjustmentInvalid)
}

func TestDecisionService_UpdateChildOfferings_ReplacesLinksAndWritesAudit(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "adjust-approved@example.com", "Lina", "Approved")
	offering := createAdjustmentCareOffering(t, env, "Randstunde Approved")
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	updated, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Randstunde nachgetragen",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID, SelectedDays: []string{"fri"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, enrollmentModels.ChildStatusApproved, updated.Status)

	links, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, childID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, offering.ID, links[0].CareOfferingID)
	assert.Equal(t, []string{"fri"}, links[0].SelectedDays)
	assert.Equal(t, []string{"fri"}, links[0].ManualSelectedDays)

	adjustments, err := env.decision.ListOfferingAdjustments(ctx, reqID, childID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Equal(t, "Randstunde nachgetragen", adjustments[0].Reason)
	assert.Equal(t, reqID, adjustments[0].RequestID)
	assert.Equal(t, childID, adjustments[0].RequestChildID)
	assert.Equal(t, *outcome.Child.CreatedStudentID, adjustments[0].StudentID)
	assert.JSONEq(t, `[]`, string(adjustments[0].Before))
	assert.Contains(t, string(adjustments[0].After), "Randstunde Approved")
}

func TestDecisionService_UpdateChildOfferings_RejectsGroupRuleViolation(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "adjust-group-rule@example.com", "Lina", "GroupRule")
	first := createAdjustmentCareOfferingWith(t, env, "Frühbetreuung", func(o *enrollmentModels.CareOffering) {
		o.SelectionGroup = "randzeiten"
		o.SelectionRule = enrollmentModels.SelectionRuleAtMostOne
		o.SortOrder = 101
	})
	second := createAdjustmentCareOfferingWith(t, env, "Spätbetreuung", func(o *enrollmentModels.CareOffering) {
		o.SelectionGroup = "randzeiten"
		o.SelectionRule = enrollmentModels.SelectionRuleAtMostOne
		o.SortOrder = 102
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Ungültige Randzeiten",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: first.ID, SelectedDays: []string{"mon"}},
			{OfferingID: second.ID, SelectedDays: []string{"mon"}},
		},
	})

	require.ErrorIs(t, err, enrollmentService.ErrOfferingAdjustmentInvalid)
}

func TestDecisionService_UpdateChildOfferings_RematerializesRequiredAutomaticLunch(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "adjust-lunch@example.com", "Lina", "Lunch")
	care := createAdjustmentCareOfferingWith(t, env, "Ganztag", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare = true
		o.CountsAsCareSet = true
		o.SortOrder = 101
	})
	lunch := createAdjustmentCareOfferingWith(t, env, "Mittagessen", func(o *enrollmentModels.CareOffering) {
		o.IsRequired = true
		o.IncludesLunch = true
		o.CountsAsCare = false
		o.CountsAsCareSet = true
		o.SortOrder = 102
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Ganztag nachgetragen",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: care.ID, SelectedDays: []string{"mon", "wed"}},
		},
	})
	require.NoError(t, err)

	links, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, childID)
	require.NoError(t, err)
	require.Len(t, links, 2)
	linksByOfferingID := map[int64]*enrollmentModels.RequestChildOffering{}
	for _, link := range links {
		linksByOfferingID[link.CareOfferingID] = link
	}
	assert.Equal(t, []string{"mon", "wed"}, linksByOfferingID[care.ID].ManualSelectedDays)
	require.Contains(t, linksByOfferingID, lunch.ID)
	assert.Equal(t, []string{"mon", "wed"}, linksByOfferingID[lunch.ID].AutomaticSelectedDays)
	assert.Empty(t, linksByOfferingID[lunch.ID].ManualSelectedDays)
}

func TestDecisionService_UpdateChildOfferings_IgnoresInactiveOfferingsDuringValidation(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "adjust-inactive@example.com", "Lina", "Inactive")
	active := createAdjustmentCareOfferingWith(t, env, "Aktive Betreuung", func(o *enrollmentModels.CareOffering) {
		o.SortOrder = 101
	})
	createAdjustmentCareOfferingWith(t, env, "Inaktive Pflichtbetreuung", func(o *enrollmentModels.CareOffering) {
		o.IsActive = false
		o.IsRequired = true
		o.SortOrder = 102
	})
	createAdjustmentCareOfferingWith(t, env, "Inaktive Automatik", func(o *enrollmentModels.CareOffering) {
		o.IsActive = false
		o.AutoAddTriggerOfferingIDs = []int64{active.ID}
		o.SortOrder = 103
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      reqID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Aktive Betreuung korrigiert",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: active.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	links, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, childID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, active.ID, links[0].CareOfferingID)
}

func TestDecisionService_UpdateChildOfferings_RemovesSourcedEnrollmentAfterOfferingGroupChange(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "AdjustOldGroup")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "AdjustNewGroup")
	offering := createAdjustmentCareOfferingWith(t, env, "Ganztag Gruppe", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 101
	})

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "GroupChange",
		GuardianEmail:     "adjust-group-change@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "GroupChange",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
				OfferingDays:     []enrollmentService.SubmitOfferingDays{{OfferingID: offering.ID, SelectedDays: []string{"mon"}}},
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

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, rows, 1)
	assert.Equal(t, oldGroup.ID, rows[0].ActivityGroupID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, submitted.Children[0].ID, *rows[0].EnrollmentRequestChildID)

	manualEnrollment := &activitiesModels.StudentEnrollment{
		StudentID:       *outcome.Child.CreatedStudentID,
		ActivityGroupID: oldGroup.ID,
		ValidFrom:       env.sourcePhase.ServiceStartDate,
		ValidUntil:      &env.sourcePhase.ServiceEndDate,
	}
	require.NoError(t, env.repos.StudentEnrollment.Create(ctx, manualEnrollment))
	_, err = env.db.NewRaw(`
		UPDATE activities.student_enrollments
		SET created_at = NOW() + INTERVAL '10 minutes',
			updated_at = NOW() + INTERVAL '10 minutes'
		WHERE id = ?
	`, manualEnrollment.ID).Exec(ctx)
	require.NoError(t, err)
	offering.ActivityGroupID = &newGroup.ID
	require.NoError(t, env.repos.CareOffering.Update(ctx, offering))

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      submitted.Request.ID,
		ChildID:        submitted.Children[0].ID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Gruppe korrigiert",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows = listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, rows, 2)
	byGroupID := map[int64]activitiesModels.StudentEnrollment{}
	for _, row := range rows {
		byGroupID[row.ActivityGroupID] = row
	}
	oldRow, ok := byGroupID[oldGroup.ID]
	require.True(t, ok, "manual old-group enrollment must survive adjustment")
	assert.Nil(t, oldRow.EnrollmentRequestChildID)
	newRow, ok := byGroupID[newGroup.ID]
	require.True(t, ok, "adjustment must materialize the corrected group")
	require.NotNil(t, newRow.EnrollmentRequestChildID)
	assert.Equal(t, submitted.Children[0].ID, *newRow.EnrollmentRequestChildID)
}

func TestDecisionService_UpdateChildOfferings_RemovesLegacyUnsourcedEnrollmentAfterOfferingGroupChange(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "AdjustLegacyOldGroup")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "AdjustLegacyNewGroup")
	offering := createAdjustmentCareOfferingWith(t, env, "Ganztag Legacy Gruppe", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 101
	})

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "LegacyGroupChange",
		GuardianEmail:     "adjust-legacy-group-change@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "LegacyGroupChange",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
				OfferingDays:     []enrollmentService.SubmitOfferingDays{{OfferingID: offering.ID, SelectedDays: []string{"mon"}}},
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

	initialRows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, initialRows, 1)
	require.NotNil(t, initialRows[0].EnrollmentRequestChildID)
	_, err = env.db.NewUpdate().
		TableExpr("activities.student_enrollments").
		Set("enrollment_request_child_id = NULL").
		Where("id = ?", initialRows[0].ID).
		Exec(ctx)
	require.NoError(t, err)

	manualEnrollment := &activitiesModels.StudentEnrollment{
		StudentID:       *outcome.Child.CreatedStudentID,
		ActivityGroupID: oldGroup.ID,
		ValidFrom:       env.sourcePhase.ServiceStartDate,
		ValidUntil:      &env.sourcePhase.ServiceEndDate,
	}
	require.NoError(t, env.repos.StudentEnrollment.Create(ctx, manualEnrollment))
	_, err = env.db.NewRaw(`
		UPDATE activities.student_enrollments
		SET created_at = NOW() + INTERVAL '10 minutes',
			updated_at = NOW() + INTERVAL '10 minutes'
		WHERE id = ?
	`, manualEnrollment.ID).Exec(ctx)
	require.NoError(t, err)

	offering.ActivityGroupID = &newGroup.ID
	require.NoError(t, env.repos.CareOffering.Update(ctx, offering))

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      submitted.Request.ID,
		ChildID:        submitted.Children[0].ID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Legacy-Gruppe korrigiert",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.NotEqual(t, initialRows[0].ID, row.ID, "legacy materialized row must be removed")
	}
	manual, err := env.repos.StudentEnrollment.FindByID(ctx, manualEnrollment.ID)
	require.NoError(t, err)
	assert.Nil(t, manual.EnrollmentRequestChildID)
	newRows := 0
	for _, row := range rows {
		if row.ActivityGroupID == newGroup.ID {
			newRows++
			require.NotNil(t, row.EnrollmentRequestChildID)
			assert.Equal(t, submitted.Children[0].ID, *row.EnrollmentRequestChildID)
		}
	}
	assert.Equal(t, 1, newRows)
}

func TestDecisionService_UpdateChildOfferings_RemovesSourcedEnrollmentAfterPhaseWindowChange(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, env.db, "AdjustWindowGroup")
	offering := createAdjustmentCareOfferingWith(t, env, "Ganztag Zeitraum", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &group.ID
		o.SortOrder = 101
	})

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "WindowChange",
		GuardianEmail:     "adjust-window-change@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "WindowChange",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{offering.ID},
				OfferingDays:     []enrollmentService.SubmitOfferingDays{{OfferingID: offering.ID, SelectedDays: []string{"mon"}}},
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

	initialRows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, initialRows, 1)
	initialFrom := initialRows[0].ValidFrom
	require.NotNil(t, initialRows[0].ValidUntil)
	initialUntil := *initialRows[0].ValidUntil

	env.sourcePhase.ServiceStartDate = timezone.NewDate(2026, 10, 1)
	env.sourcePhase.ServiceEndDate = timezone.NewDate(2027, 8, 15)
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	// This test isolates a phase-window correction, not a planned individual
	// care end. Keep the student's master interval aligned with the corrected
	// phase; otherwise enrolled_until must win and cap every rematerialization.
	_, err = env.db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", env.sourcePhase.ServiceEndDate).
		Where("id = ?", *outcome.Child.CreatedStudentID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      submitted.Request.ID,
		ChildID:        submitted.Children[0].ID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Zeitraum korrigiert",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, rows, 1)
	assert.NotEqual(t, initialFrom, rows[0].ValidFrom)
	assert.NotEqual(t, initialUntil, *rows[0].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *rows[0].ValidUntil)
}

func TestDecisionService_ApprovalUsesExclusiveEndForOneDayPhase(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	serviceDay := timezone.NewDate(2027, time.April, 5)
	env.sourcePhase.ServiceStartDate = serviceDay
	env.sourcePhase.ServiceEndDate = serviceDay
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	group := testpkg.CreateTestActivityGroup(t, env.db, "OneDayExclusiveEndGroup")
	offering := createAdjustmentCareOfferingWith(t, env, "Eintagesbetreuung", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &group.ID
	})

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "OneDay",
		GuardianEmail:     "decision-one-day-exclusive-end@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Lina",
			LastName:         "OneDay",
			DateOfBirth:      timezone.NewDate(2018, time.April, 15),
			TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:      []int64{offering.ID},
			OfferingDays: []enrollmentService.SubmitOfferingDays{{
				OfferingID:   offering.ID,
				SelectedDays: []string{"mon"},
			}},
		}},
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

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, *outcome.Child.CreatedStudentID)
	require.Len(t, rows, 1)
	assert.Equal(t, serviceDay, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, serviceDay.AddDays(1), *rows[0].ValidUntil)
}

func listStudentEnrollmentRowsForDecisionTest(t *testing.T, env *decisionTestEnv, studentID int64) []activitiesModels.StudentEnrollment {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, studentID).
		OrderExpr(`"student_enrollment".id`).
		Scan(ctx))
	return rows
}

func createAdjustmentCareOffering(t *testing.T, env *decisionTestEnv, name string) *enrollmentModels.CareOffering {
	t.Helper()
	return createAdjustmentCareOfferingWith(t, env, name, nil)
}

func createAdjustmentCareOfferingWith(t *testing.T, env *decisionTestEnv, name string, mutate func(*enrollmentModels.CareOffering)) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.Ctx(t)
	offering := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           name,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		CountsAsCare:   false,
		SortOrder:      100,
	}
	if mutate != nil {
		mutate(offering)
	}
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))
	return offering
}
