package enrollment_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// rolloverTestEnv bundles the shared dependencies. Mirrors the pattern
// in request_service_test.go so we get tenant + form schema + base
// phase out of the box.
type rolloverTestEnv struct {
	db             *bun.DB
	repos          *repositories.Factory
	rolloverSvc    enrollmentService.RolloverService
	requestSvc     enrollmentService.RequestService
	offeringCloner enrollmentService.RolloverOfferingCatalogCloner
	settings       *stubRequestSettings
	outbox         *recordingOutbox
	sourcePhase    *enrollmentModels.Phase
	creatorID      int64
}

func setupRolloverTest(t *testing.T) (*rolloverTestEnv, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	// Close the pool after all fixture cleanups registered by the test. The
	// decision-service tests reuse this setup and may register additional
	// t.Cleanup hooks (for example calendar periods); closing inside the
	// returned cleanup made those hooks run against a closed pool and leak rows
	// into subsequent tests in the package-isolated database.
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))

	repoFactory := repositories.NewFactory(db)
	settings := newStubRequestSettings()
	settings.boolValues[configModel.KeyEnrollmentEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentAllowSubmissionEdit] = true
	settings.boolValues[configModel.KeyEnrollmentCollectGradeLevel] = true
	settings.boolValues[configModel.KeyEnrollmentCareOfferingsEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentWaitlistEnabled] = true
	settings.stringValues[configModel.KeyEnrollmentDuplicateHandling] = configModel.EnrollmentDuplicateHandlingWarn
	settings.intValues[configModel.KeyEnrollmentGradeLevelMax] = 4
	settings.intValues[configModel.KeyEnrollmentStatusTokenTTLDays] = 365
	// The shared env submits all four standard consent flags. Configure the
	// matching legal texts so the resolved block contract declares them.
	// Submit persists only consent keys the contract knows, so an
	// unconfigured tenant would silently drop these flags.
	settings.boolValues[configModel.KeyEnrollmentLegalTermsEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentLegalDSGVOEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentLegalEmailContactEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentLegalPhotoEnabled] = true
	settings.stringValues[configModel.KeyEnrollmentLegalAGBText] = "AGB Text"
	settings.stringValues[configModel.KeyEnrollmentLegalDSGVOText] = "DSGVO Text"
	settings.stringValues[configModel.KeyEnrollmentLegalEmailContactText] = "E-Mail Text"
	settings.stringValues[configModel.KeyEnrollmentLegalPhotoText] = "Foto Text"

	outbox := &recordingOutbox{}
	requestSvc := enrollmentService.NewRequestService(enrollmentService.RequestServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		FormSchemaRepo:           repoFactory.FormSchema,
		PhaseRepo:                repoFactory.Phase,
		SchoolRepo:               repoFactory.School,
		RateLimitRepo:            repoFactory.SubmissionRateLimit,
		OutboxEnqueuer:           outbox,
		Settings:                 settings,
		FrontendURL:              "http://localhost:3000",
		DB:                       db,
		Logger:                   slog.Default(),
	})

	careOfferingSvc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                     repoFactory.CareOffering,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		ActivityGroupRepo:        repoFactory.ActivityGroup,
		ActivityScheduleRepo:     repoFactory.ActivitySchedule,
		CalendarPeriodRepo:       repoFactory.CalendarPeriod,
		TimeframeRepo:            repoFactory.Timeframe,
		ActivityExceptionRepo:    repoFactory.ActivityException,
		PhaseRepo:                repoFactory.Phase,
		Settings:                 settings,
		Logger:                   slog.Default(),
	})
	offeringCloner, ok := careOfferingSvc.(enrollmentService.RolloverOfferingCatalogCloner)
	require.True(t, ok, "care offering service must implement RolloverOfferingCatalogCloner")

	rolloverSvc := enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                repoFactory.Phase,
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		OfferingCatalogCloner:    offeringCloner,
		OutboxEnqueuer:           outbox,
		Settings:                 settings,
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       db,
		Logger:                   slog.Default(),
	})

	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Rollover", "Tester")
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, "Testformular Rollover", []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	sourcePhase := &enrollmentModels.Phase{
		Name:             "rollover-source-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2026, 9, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
		FormSchemaID:     &schema.ID,
	}
	sourcePhase.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repoFactory.Phase.Create(ctx, sourcePhase))

	env := &rolloverTestEnv{
		db:             db,
		repos:          repoFactory,
		rolloverSvc:    rolloverSvc,
		requestSvc:     requestSvc,
		offeringCloner: offeringCloner,
		settings:       settings,
		outbox:         outbox,
		sourcePhase:    sourcePhase,
		creatorID:      account.ID,
	}

	cleanup := func() {
		bg := context.Background()
		tenantID := testpkg.Tenant(t)
		_, _ = db.NewRaw(`
			WITH phase_scope AS (
				SELECT id
				FROM enrollment.phases
				WHERE tenant_id = ?
				  AND (id = ? OR rollover_source_phase_id = ?)
			)
			DELETE FROM enrollment.request_child_offerings rco
			USING enrollment.request_children rc, enrollment.requests r, phase_scope ps
			WHERE rco.request_child_id = rc.id
			  AND rc.request_id = r.id
			  AND r.phase_id = ps.id
		`, tenantID, sourcePhase.ID, sourcePhase.ID).Exec(bg)
		_, _ = db.NewRaw(`
			WITH phase_scope AS (
				SELECT id
				FROM enrollment.phases
				WHERE tenant_id = ?
				  AND (id = ? OR rollover_source_phase_id = ?)
			)
			DELETE FROM enrollment.request_children rc
			USING enrollment.requests r, phase_scope ps
			WHERE rc.request_id = r.id
			  AND r.phase_id = ps.id
		`, tenantID, sourcePhase.ID, sourcePhase.ID).Exec(bg)
		_, _ = db.NewRaw(`
			DELETE FROM enrollment.requests r
			USING enrollment.phases p
			WHERE r.phase_id = p.id
			  AND p.tenant_id = ?
			  AND (p.id = ? OR p.rollover_source_phase_id = ?)
		`, tenantID, sourcePhase.ID, sourcePhase.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.care_offerings").
			Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND (id = ? OR rollover_source_phase_id = ?))",
				tenantID, sourcePhase.ID, sourcePhase.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.submission_rate_limits").
			Where("tenant_id = ? AND key_value LIKE ?", tenantID, "%@example.com").Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.phases").
			Where("tenant_id = ? AND (id = ? OR rollover_source_phase_id = ?)", tenantID, sourcePhase.ID, sourcePhase.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.form_schemas").Where("created_by = ?", account.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(bg)
	}
	return env, cleanup
}

// seedApprovedChild walks one parent submission through Submit + admin
// approval so the source phase has a real `approved` request_children
// row to roll forward. Returns the child for assertions.
func seedApprovedChild(t *testing.T, env *rolloverTestEnv, phaseID int64, guardianFirst, guardianLast, guardianEmail, childFirst, childLast string, grade int16) *enrollmentModels.RequestChild {
	t.Helper()
	ctx := testpkg.Ctx(t)

	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           phaseID,
		GuardianFirstName: guardianFirst,
		GuardianLastName:  guardianLast,
		GuardianEmail:     guardianEmail,
		ConsentFlags:      rolloverConsentFlags(),
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

	// Approve directly via the repo — the full decision service path
	// creates Person/Student/etc., which is more than the rollover
	// tests need. Status alone is what triggers the rollover scan.
	child := res.Children[0]
	require.NoError(t, env.repos.RequestChild.UpdateStatus(
		ctx, child.ID, enrollmentModels.ChildStatusApproved, nil, env.creatorID,
	))
	updated, err := env.repos.RequestChild.FindByID(ctx, child.ID)
	require.NoError(t, err)
	return updated
}

func rolloverConsentFlags() map[string]any {
	return map[string]any{
		"agb":             true,
		"data_processing": true,
		"email_contact":   true,
		"photo":           false,
	}
}

// validRolloverRequest is a request-body builder. Caller passes mode +
// bumpsGrade; everything else defaults to a valid yearly rollover.
func validRolloverRequest(env *rolloverTestEnv, mode string, bumpsGrade bool) enrollmentService.CreatePhaseFromSourceRequest {
	return enrollmentService.CreatePhaseFromSourceRequest{
		SourcePhaseID:      env.sourcePhase.ID,
		Name:               "rollover-target-" + mode,
		Kind:               enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:   timezone.NewDate(2027, 9, 1),
		ServiceEndDate:     timezone.NewDate(2028, 7, 31),
		RolloverMode:       mode,
		RolloverDeadline:   time.Date(2027, 8, 15, 0, 0, 0, 0, time.UTC),
		RolloverBumpsGrade: bumpsGrade,
	}
}

func rolloverServiceWithSettings(
	env *rolloverTestEnv,
	settings enrollmentService.RequestSettingsResolver,
) enrollmentService.RolloverService {
	return enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                env.repos.Phase,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		OfferingCatalogCloner:    env.offeringCloner,
		OutboxEnqueuer:           env.outbox,
		Settings:                 settings,
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
		Logger:                   slog.Default(),
	})
}

// --- CreatePhaseFromSource ---

func TestRolloverService_CreatePhaseFromSource_RejectsMissingGradeSettingsService(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	service := rolloverServiceWithSettings(env, nil)
	result, err := service.CreatePhaseFromSource(
		ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true),
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "enrollment.grade_level_max")
	exists, lookupErr := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
	require.NoError(t, lookupErr)
	assert.False(t, exists, "configuration failure must not create a follow-up phase")
}

func TestRolloverService_CreatePhaseFromSource_RejectsGradeSettingReadFailure(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	env.settings.intErrors[configModel.KeyEnrollmentGradeLevelMax] = errors.New("settings unavailable")

	result, err := env.rolloverSvc.CreatePhaseFromSource(
		ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true),
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "settings unavailable")
	exists, lookupErr := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
	require.NoError(t, lookupErr)
	assert.False(t, exists, "settings read failure must not create a follow-up phase")
}

func TestRolloverService_CreatePhaseFromSource_RejectsOutOfRangeGradeSetting(t *testing.T) {
	t.Parallel()

	for _, value := range []int{0, 14} {
		t.Run(fmt.Sprintf("value_%d", value), func(t *testing.T) {
			env, cleanup := setupRolloverTest(t)
			defer cleanup()
			ctx := testpkg.Ctx(t)
			env.settings.intValues[configModel.KeyEnrollmentGradeLevelMax] = value

			result, err := env.rolloverSvc.CreatePhaseFromSource(
				ctx,
				validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true),
			)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorContains(t, err, "outside 1..13")
			exists, lookupErr := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
			require.NoError(t, lookupErr)
			assert.False(t, exists, "invalid cap must not create a follow-up phase")
		})
	}
}

func TestRolloverService_CreatePhaseFromSource_OptOutHappyPath(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", one)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Phase)
	assert.Equal(t, 1, result.SourceChildCount)
	assert.Equal(t, 1, result.RolledCount)
	assert.Equal(t, 0, result.ReviewCount)
	assert.Equal(t, 1, result.RequestCount)
	assert.Equal(t, 1, result.EnqueuedEmails)

	// The new phase points back at the source.
	require.NotNil(t, result.Phase.RolloverSourcePhaseID)
	assert.Equal(t, env.sourcePhase.ID, *result.Phase.RolloverSourcePhaseID)
	require.NotNil(t, result.Phase.RolloverMode)
	assert.Equal(t, enrollmentModels.PhaseRolloverModeOptOut, *result.Phase.RolloverMode)

	// The carried child sits in auto_renewed (opt_out semantics).
	children, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, "Lina", children[0].FirstName)
	// Grade bumped from 1 → 2.
	require.NotNil(t, children[0].TargetGradeLevel)
	assert.Equal(t, int16(2), *children[0].TargetGradeLevel)
	// Backlink set.
	require.NotNil(t, children[0].RolloverSourceChildID)
}

// TestRolloverService_CreatePhaseFromSource_CarriesEligibilityForward proves
// the successor phase inherits the source's audience + eligible-class gate.
// Without the explicit copy, Phase.Validate defaults the successor to
// audience=open with an empty class list, silently making a rolled
// linked/class-restricted phase public — and the successor is the active
// phase parents actually submit to (#1663).
func TestRolloverService_CreatePhaseFromSource_CarriesEligibilityForward(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	env.sourcePhase.Audience = enrollmentModels.PhaseAudienceLinkedParents
	// Eligible classes must also be offered by the phase (#1663); the
	// successor inherits both lists, so seed available to match before the
	// rollover re-validates the copied config.
	env.sourcePhase.AvailableSchoolClasses = []string{"2a", "2b"}
	env.sourcePhase.EligibleSchoolClasses = []string{"2a", "2b"}
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	// A class-restricted successor now requires concrete-class collection to be
	// active — the rollover enforces the same collectability invariant as the
	// admin create/update paths (#1663). collect_grade_level is already on in
	// setupRolloverTest; enable collect_school_class so this restricted rollover
	// is a valid configuration.
	env.settings.boolValues[configModel.KeyEnrollmentCollectSchoolClass] = true

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Phase)

	assert.Equal(t, enrollmentModels.PhaseAudienceLinkedParents, result.Phase.Audience,
		"successor must inherit the source audience, not default to open")
	assert.Equal(t, []string{"2a", "2b"}, result.Phase.EligibleSchoolClasses,
		"successor must inherit the eligible-class gate")
}

func TestRolloverService_CreatePhaseFromSource_OptInLandsInPendingRenewal(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	two := int16(2)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", two)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true))
	require.NoError(t, err)
	assert.Equal(t, 1, result.RolledCount)

	children, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingRenewal},
	)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, int16(3), *children[0].TargetGradeLevel)
}

func TestRolloverService_CreatePhaseFromSource_GradeAboveMaxGoesToReview(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	four := int16(4) // max is 4, bumped → 5 → above cap
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", four)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	assert.Equal(t, 0, result.RolledCount, "above-cap children must not auto-roll")
	assert.Equal(t, 1, result.ReviewCount)
	assert.Equal(t, 1, result.ReviewByReason[enrollmentModels.ReviewReasonGradeAboveMax])

	review, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingAdminReview},
	)
	require.NoError(t, err)
	require.Len(t, review, 1)
	require.NotNil(t, review[0].ReviewReason)
	assert.Equal(t, enrollmentModels.ReviewReasonGradeAboveMax, *review[0].ReviewReason)
}

func TestRolloverService_CreatePhaseFromSource_NoGradeLevelGoesToReview(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// The public Submit path now requires a grade level (form-level
	// invariant). To simulate legacy / migrated data that landed in
	// the table without a grade we submit with a grade, then null it
	// out via direct UPDATE before triggering the rollover scan.
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.com",
		ConsentFlags:      rolloverConsentFlags(),
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Beispiel",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
		},
	})
	require.NoError(t, err)
	_, err = env.db.NewUpdate().
		Table("enrollment.request_children").
		Set("target_grade_level = NULL").
		Where("id = ?", res.Children[0].ID).
		Exec(ctx)
	require.NoError(t, err, "manual NULL of target_grade_level for legacy-data simulation")
	require.NoError(t, env.repos.RequestChild.UpdateStatus(
		ctx, res.Children[0].ID, enrollmentModels.ChildStatusApproved, nil, env.creatorID,
	))

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	assert.Equal(t, 1, result.ReviewCount)
	assert.Equal(t, 1, result.ReviewByReason[enrollmentModels.ReviewReasonNoGradeLevel])
}

func TestRolloverService_CreatePhaseFromSource_BumpsGradeFalseKeepsGrade(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	two := int16(2)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", two)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, false))
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)

	children, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusAutoRenewed},
	)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, int16(2), *children[0].TargetGradeLevel, "half-year rollover must not bump grade")
}

func TestRolloverService_CreatePhaseFromSource_SkipsNonApproved(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Approved child — rolls forward.
	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Approved", "approved@example.com", "Lina", "Approved", one)

	// Non-approved child (withdrawn) — must NOT roll forward.
	res, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Beata",
		GuardianLastName:  "Withdrawn",
		GuardianEmail:     "withdrawn@example.com",
		ConsentFlags:      rolloverConsentFlags(),
		Children: []enrollmentService.SubmitChild{
			{FirstName: "Max", LastName: "Withdrawn", DateOfBirth: timezone.NewDate(2018, 4, 15), TargetGradeLevel: &one},
		},
	})
	require.NoError(t, err)
	require.NoError(t, env.repos.RequestChild.UpdateStatus(
		ctx, res.Children[0].ID, enrollmentModels.ChildStatusWithdrawn, nil, env.creatorID,
	))

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	assert.Equal(t, 1, result.SourceChildCount, "only the approved child counts as source")
	assert.Equal(t, 1, result.RolledCount)
	assert.Equal(t, 1, result.RequestCount, "withdrawn parent must not get a renewal request")
}

func TestRolloverService_CreatePhaseFromSource_SourceWithNoApprovedCreatesEmptyPhase(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// No seed at all — source phase has zero approved children. The
	// admin should still get a new phase so the "Anschlussphase
	// erstellen" flow stays consistent regardless of enrollment count.
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Phase)
	assert.Equal(t, 0, result.SourceChildCount)
	assert.Equal(t, 0, result.RolledCount)
	assert.Equal(t, 0, result.ReviewCount)
	assert.Equal(t, 0, result.RequestCount)
	assert.Equal(t, 0, result.EnqueuedEmails)
	assert.Equal(t, env.sourcePhase.ID, *result.Phase.RolloverSourcePhaseID, "lineage to source phase preserved")
}

func TestRolloverService_CreatePhaseFromSource_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	bad := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true)
	bad.Name = ""
	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, bad)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRolloverInvalidRequest))

	bad = validRolloverRequest(env, "", true)
	_, err = env.rolloverSvc.CreatePhaseFromSource(ctx, bad)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRolloverInvalidRequest))
}

func TestRolloverService_CreatePhaseFromSource_EnqueuesCorrectEmailKind(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", one)

	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true))
	require.NoError(t, err)
	optInEntries := env.outbox.ByKind("enrollment_rollover_opt_in")
	assert.Len(t, optInEntries, 1, "opt_in mode must enqueue opt_in email")
	assert.Empty(t, env.outbox.ByKind("enrollment_rollover_opt_out"))
}

// --- ListReviewQueue ---

func TestRolloverService_ListReviewQueue_ReturnsPendingReviewRows(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	four := int16(4)
	source := seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Beispiel", "anna@example.com", "Lina", "Beispiel", four)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.ReviewCount)

	items, err := env.rolloverSvc.ListReviewQueue(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Lina", items[0].Child.FirstName)
	require.NotNil(t, items[0].SourceChild)
	assert.Equal(t, source.ID, items[0].SourceChild.ID)
}

// --- DecideReview ---

func TestRolloverService_DecideReview_KeepWithClassOverride(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	four := int16(4)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Repeater", "rep@example.com", "Lina", "Repeater", four)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.ReviewCount)

	queue, err := env.rolloverSvc.ListReviewQueue(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	reviewID := queue[0].Child.ID

	newGrade := int16(4) // keep at grade 4 (repeater)
	err = env.rolloverSvc.DecideReview(ctx, enrollmentService.DecideReviewRequest{
		RequestChildID: reviewID,
		Decision:       enrollmentService.ReviewDecisionKeep,
		NewGradeLevel:  &newGrade,
		AdminAccountID: env.creatorID,
	})
	require.NoError(t, err)

	updated, err := env.repos.RequestChild.FindByID(ctx, reviewID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusAutoRenewed, updated.Status, "keep must promote to auto_renewed")
	require.NotNil(t, updated.TargetGradeLevel)
	assert.Equal(t, int16(4), *updated.TargetGradeLevel)
	assert.Nil(t, updated.ReviewReason, "review_reason must be cleared on resolution")
}

func TestRolloverService_DecideReview_DropWithdraws(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	four := int16(4)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Leaver", "leaver@example.com", "Max", "Leaver", four)
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	queue, err := env.rolloverSvc.ListReviewQueue(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, queue, 1)

	err = env.rolloverSvc.DecideReview(ctx, enrollmentService.DecideReviewRequest{
		RequestChildID: queue[0].Child.ID,
		Decision:       enrollmentService.ReviewDecisionDrop,
		AdminAccountID: env.creatorID,
	})
	require.NoError(t, err)

	updated, err := env.repos.RequestChild.FindByID(ctx, queue[0].Child.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, updated.Status)
}

// TestRolloverService_CreatePhaseFromSource_DropsClassForReviewRow proves a
// row heading into admin review never carries the source's concrete class.
// The carried letter (e.g. "5a") would otherwise win on approval even after
// the admin overrides the target grade in the review queue. Issue #1833.
func TestRolloverService_CreatePhaseFromSource_DropsClassForReviewRow(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Seed at a valid grade, then push the source above the grade cap
	// (max = 4) and pin a concrete class directly. A non-bumping rollover of
	// an above-cap child is exactly the review case that used to carry the
	// stale class forward.
	source := seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Cap", "cap-review@example.com", "Lina", "Cap", int16(4))
	_, err := env.db.NewUpdate().
		TableExpr("enrollment.request_children").
		Set("target_grade_level = ?", int16(5)).
		Set("target_school_class = ?", "5a").
		Where("id = ?", source.ID).
		Exec(context.Background())
	require.NoError(t, err)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, false))
	require.NoError(t, err)
	require.Equal(t, 1, result.ReviewCount, "above-cap child must land in admin review")

	queue, err := env.rolloverSvc.ListReviewQueue(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Nil(t, queue[0].Child.TargetSchoolClass, "review row must not carry the stale concrete class")
	require.NotNil(t, queue[0].Child.TargetGradeLevel)
	assert.Equal(t, int16(5), *queue[0].Child.TargetGradeLevel, "target grade is still carried for admin context")
}

func TestRolloverService_DecideReview_RejectsUnknownDecision(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	err := env.rolloverSvc.DecideReview(ctx, enrollmentService.DecideReviewRequest{
		RequestChildID: 12345,
		Decision:       "bogus",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRolloverReviewInvalid))
}

// --- Deadline worker ---

func TestRolloverService_RunDeadlineWorker_TransitionsStatuses(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "OptIn", "optin@example.com", "Lina", "OptIn", one)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Beata", "OptOut", "optout@example.com", "Max", "OptOut", one)

	// Opt-in target: one row in pending_renewal.
	optInReq := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true)
	optInReq.Name = "opt-in-target"
	// Deadline already in the past so the worker resolves it.
	optInReq.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	// Only Anna submits to opt-in target (we'll roll Beata via opt-out separately).
	// To keep this single-phase rollover deterministic, we just roll
	// the same source twice with different modes by creating two
	// separate sources is overkill. Instead: roll once with opt_in
	// and assert pending_renewal → withdrawn.
	optInResult, err := env.rolloverSvc.CreatePhaseFromSource(ctx, optInReq)
	require.NoError(t, err)
	require.Greater(t, optInResult.RolledCount, 0)

	// Worker pass with now == future → resolves the past-deadline row.
	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.PhasesProcessed)
	assert.Equal(t, optInResult.RolledCount, summary.PendingRenewalToWithdrawn,
		"every pending_renewal in the past-deadline phase must transition")

	// All pending_renewal rows in that phase must now be withdrawn.
	remainingPending, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, optInResult.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingRenewal},
	)
	require.NoError(t, err)
	assert.Empty(t, remainingPending)
}

func TestRolloverService_RunDeadlineWorker_IsIdempotent(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Test", "anna@example.com", "Lina", "Test", one)

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	first, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, first.AutoRenewedToSubmitted)

	// Second run finds nothing to transition.
	second, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, second.AutoRenewedToSubmitted)
	assert.Equal(t, 0, second.PendingRenewalToWithdrawn)
}

// fakeApproveDecisionService stands in for the real DecisionService
// when we only want to assert the auto-approve path *would* be invoked
// — the real service requires Person/Student/etc. repos that the
// rollover test env doesn't spin up. The stub flips status to approved
// the same way Decide would, so the deadline worker's counting logic
// is exercised end-to-end.
type fakeApproveDecisionService struct {
	repo  enrollmentModels.RequestChildRepository
	calls int
}

func (f *fakeApproveDecisionService) List(_ context.Context, _ enrollmentService.RequestFilters) ([]*enrollmentService.RequestSummary, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) ListByStudent(_ context.Context, _ int64) ([]*enrollmentService.RequestSummary, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) Get(_ context.Context, _ int64) (*enrollmentService.RequestSummary, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) ListChildOfferings(_ context.Context, _ int64) (map[int64]enrollmentService.ChildOfferingSet, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) UpdateChildOfferings(_ context.Context, _ enrollmentService.UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) ListOfferingAdjustments(_ context.Context, _, _ int64) ([]*auditModels.EnrollmentOfferingAdjustment, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) ExportPhase(_ context.Context, _, _ int64, _, _, _ string) (*enrollmentService.PhaseExport, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) ExportStudent(_ context.Context, _, _ int64, _, _ string) (*enrollmentService.StudentEnrollmentExport, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) RecordPhaseExportAudit(_ context.Context, _ int64, _ string, _ *enrollmentModels.Phase, _, _ string, _, _ int) error {
	return nil
}

func (f *fakeApproveDecisionService) RestoreWithdrawn(_ context.Context, _, _ int64) (*enrollmentService.RestoreOutcome, error) {
	return nil, nil
}

func (f *fakeApproveDecisionService) Decide(ctx context.Context, input enrollmentService.DecideInput) (*enrollmentService.DecideOutcome, error) {
	f.calls++
	if err := f.repo.UpdateStatus(ctx, input.ChildID, string(input.Status), nil, 0); err != nil {
		return nil, err
	}
	return &enrollmentService.DecideOutcome{}, nil
}

func TestRolloverService_RunDeadlineWorker_AutoApprovePromotesToApproved(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Auto", "auto@example.com", "Lina", "Auto", one)

	// Build a rollover service WITH a stub decision service wired so
	// the auto-approve path runs in the deadline worker. The default
	// env doesn't wire a decision service (decision needs Person/
	// Student repos and the full chain).
	stubDecision := &fakeApproveDecisionService{repo: env.repos.RequestChild}
	autoApproveSvc := enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                env.repos.Phase,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		OutboxEnqueuer:           env.outbox,
		Settings:                 env.settings,
		DecisionService:          stubDecision,
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
		Logger:                   slog.Default(),
	})

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	req.Name = "auto-approve-target"
	result, err := autoApproveSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)

	summary, err := autoApproveSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.AutoRenewedToApproved, "auto_approve=true must route through Decide → approved")
	assert.Equal(t, 0, summary.AutoRenewedToSubmitted)
	assert.Equal(t, 1, stubDecision.calls)
}

func TestRolloverService_RunDeadlineWorker_AutoApproveFallbackWithoutDecisionService(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Fallback", "fb@example.com", "Lina", "Fallback", one)

	// auto_approve=true on the phase but no DecisionService wired —
	// must fall back to bulk-promotion-to-submitted rather than
	// crashing.
	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	_, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)

	summary, err := env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, summary.AutoRenewedToApproved)
	assert.Equal(t, 1, summary.AutoRenewedToSubmitted, "fallback must still resolve the cohort to submitted")
}

// --- ConfirmRenewal (opt-in parent path) ---

func TestRequestService_ConfirmRenewal_TransitionsPendingRenewalToSubmitted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "OptIn", "anna@example.com", "Lina", "OptIn", one)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)

	// Find the new request's status token so the parent endpoint can
	// authenticate.
	children, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingRenewal},
	)
	require.NoError(t, err)
	require.Len(t, children, 1)
	parentReq, err := env.repos.Request.FindByID(ctx, children[0].RequestID)
	require.NoError(t, err)

	confirmed, err := env.requestSvc.ConfirmRenewal(ctx, parentReq.StatusToken)
	require.NoError(t, err)
	assert.Equal(t, 1, confirmed)

	updated, err := env.repos.RequestChild.FindByID(ctx, children[0].ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, updated.Status)
}

func TestRequestService_ConfirmRenewal_IsIdempotent(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	one := int16(1)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Idem", "anna@example.com", "Lina", "Idem", one)

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptIn, true))
	require.NoError(t, err)
	children, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingRenewal},
	)
	require.NoError(t, err)
	parentReq, err := env.repos.Request.FindByID(ctx, children[0].RequestID)
	require.NoError(t, err)

	first, err := env.requestSvc.ConfirmRenewal(ctx, parentReq.StatusToken)
	require.NoError(t, err)
	assert.Equal(t, 1, first)

	second, err := env.requestSvc.ConfirmRenewal(ctx, parentReq.StatusToken)
	require.NoError(t, err)
	assert.Equal(t, 0, second, "second confirm is a no-op")
}

func TestRolloverService_RunDeadlineWorker_LeavesAdminReviewAlone(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	four := int16(4) // bumps to 5, above cap → admin review
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Cap", "cap@example.com", "Lina", "Cap", four)

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverDeadline = time.Now().Add(-1 * time.Hour)
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, result.ReviewCount)

	_, err = env.rolloverSvc.RunDeadlineWorker(ctx, time.Now())
	require.NoError(t, err)

	reviewRows, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID,
		[]string{enrollmentModels.ChildStatusPendingAdminReview},
	)
	require.NoError(t, err)
	assert.Len(t, reviewRows, 1, "admin-review rows must survive the deadline worker")
}
