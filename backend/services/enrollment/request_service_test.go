package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// stubRequestSettings is a minimal RequestSettingsResolver. Tests set
// values directly via the maps and rely on HasTenantOverride to be true
// for any key present in stringValues / boolValues / intValues. This
// mirrors the production "DB override → registry default" precedence
// without requiring the real settings registry.
type stubRequestSettings struct {
	mu           sync.Mutex
	stringValues map[string]string
	boolValues   map[string]bool
	intValues    map[string]int
}

func newStubRequestSettings() *stubRequestSettings {
	return &stubRequestSettings{
		stringValues: make(map[string]string),
		boolValues:   make(map[string]bool),
		intValues:    make(map[string]int),
	}
}

func (s *stubRequestSettings) HasTenantOverride(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stringValues[key]; ok {
		return true, nil
	}
	if _, ok := s.boolValues[key]; ok {
		return true, nil
	}
	if _, ok := s.intValues[key]; ok {
		return true, nil
	}
	return false, nil
}

func (s *stubRequestSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.boolValues[key]
	if !ok {
		return false, nil
	}
	return v, nil
}

func (s *stubRequestSettings) ResolveString(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stringValues[key], nil
}

func (s *stubRequestSettings) ResolveInt(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intValues[key], nil
}

// recordingOutbox captures every Enqueue call the service makes so tests
// can assert both confirmation emails were enqueued with the right kind.
type recordingOutbox struct {
	mu      sync.Mutex
	entries []enrollmentService.OutboxEnqueueRequest
}

func (r *recordingOutbox) Enqueue(_ context.Context, req enrollmentService.OutboxEnqueueRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, req)
	return nil
}

func (r *recordingOutbox) ByKind(kind string) []enrollmentService.OutboxEnqueueRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]enrollmentService.OutboxEnqueueRequest, 0, len(r.entries))
	for _, e := range r.entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// requestTestEnv bundles everything a request-service test needs.
type requestTestEnv struct {
	db        *bun.DB
	svc       enrollmentService.RequestService
	phase     *enrollmentModels.Phase
	schemaID  int64
	creatorID int64
	settings  *stubRequestSettings
	outbox    *recordingOutbox
	phaseID   int64
}

func setupRequestTest(t *testing.T) (*requestTestEnv, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)

	repoFactory := repositories.NewFactory(db)
	settings := newStubRequestSettings()
	// Default to "enabled, no window restrictions" - individual tests
	// override these maps to drive specific branches.
	settings.boolValues[configModel.KeyEnrollmentEnabled] = true
	settings.boolValues[configModel.KeyEnrollmentAllowSubmissionEdit] = true
	settings.intValues[configModel.KeyEnrollmentGradeLevelMax] = 4
	settings.intValues[configModel.KeyEnrollmentStatusTokenTTLDays] = 365

	outbox := &recordingOutbox{}
	svc := enrollmentService.NewRequestService(enrollmentService.RequestServiceConfig{
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

	ctx := testpkg.TenantContext(1)

	// FormSchema is required for Submit (FK on enrollment.requests.schema_id).
	// Use the form_schema service to publish a minimal active version.
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Submission", "Tester")
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, account.ID)
	require.NoError(t, err)

	// Phase is the new parent entity for offerings + submissions
	// (replaced calendar_period in the phase model). The phase owns the
	// open window (defaults: unbounded), the form schema (defaults:
	// nil = fall back to tenant active), and the overflow mode.
	phase := &enrollmentModels.Phase{
		Name:             "request-test-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		ServiceEndDate:   time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC),
		IsActive:         true,
		FormSchemaID:     &schema.ID,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.SetTenantID(1)
	require.NoError(t, repoFactory.Phase.Create(ctx, phase))

	env := &requestTestEnv{
		db:        db,
		svc:       svc,
		phase:     phase,
		schemaID:  schema.ID,
		creatorID: account.ID,
		settings:  settings,
		outbox:    outbox,
		phaseID:   phase.ID,
	}

	cleanup := func() {
		bg := context.Background()
		// Wipe rate-limit rows so a follow-on test in the same run gets
		// a fresh bucket. Keyed (tenant_id, key_type, key_value); shared
		// tenant_id=1 across tests.
		_, _ = db.NewDelete().
			TableExpr("enrollment.submission_rate_limits").
			Where("tenant_id = ?", 1).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("phase_id = ?", phase.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", phase.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", phase.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.form_schemas").
			Where("created_by = ?", account.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(bg)
		_ = db.Close()
	}
	return env, cleanup
}

func validSubmission(phaseID int64) enrollmentService.SubmitRequest {
	return enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     "anna@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           false,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Lina",
				LastName:         "Beispiel",
				DateOfBirth:      time.Date(2018, 4, 15, 0, 0, 0, 0, time.UTC),
				TargetGradeLevel: testpkg.Int16Ptr(1),
			},
		},
	}
}

// --- Submit ---

func TestRequestService_Submit_PersistsRequestChildAndEnqueuesEmails(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Configure two notification recipients to verify both admin emails
	// + one parent email get enqueued.
	env.settings.stringValues[configModel.KeyEnrollmentNotificationEmails] =
		"admin1@example.com, admin2@example.com"

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	require.NotNil(t, result.Request)
	require.Len(t, result.Children, 1)
	assert.NotEmpty(t, result.Request.StatusToken, "status token must be generated")
	assert.Contains(t, result.StatusURL, "/enroll/status/", "status URL must point at the parent page")
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, result.Children[0].Status)

	parents := env.outbox.ByKind("enrollment_submitted")
	require.Len(t, parents, 1, "exactly one parent confirmation email must be enqueued")
	admins := env.outbox.ByKind("enrollment_admin_notification")
	require.Len(t, admins, 2, "one admin notification per configured recipient")
}

func TestRequestService_Submit_RejectsWhenDisabled(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.boolValues[configModel.KeyEnrollmentEnabled] = false
	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		"disabled enrollment must return ErrEnrollmentDisabled, got %v", err)
}

func TestRequestService_Submit_RejectsOutsideWindow(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Set the phase's window to "ended yesterday" via repo update,
	// then re-submit. Phase-window enforcement runs in Submit's
	// IsEnrollmentWindowOpen check.
	pastStart := time.Now().AddDate(0, 0, -7)
	pastEnd := time.Now().AddDate(0, 0, -1)
	env.phase.EnrollmentOpenAt = &pastStart
	env.phase.EnrollmentCloseAt = &pastEnd
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed))
}

func TestRequestService_Submit_RejectsInvalidEmail(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	req := validSubmission(env.phaseID)
	req.GuardianEmail = "not-an-email"
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission))
}

func TestRequestService_Submit_NoLegalBlocksConfiguredRequiresNoConsentFlags(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	req := validSubmission(env.phaseID)
	req.ConsentFlags = map[string]any{}
	_, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
}

func TestRequestService_Submit_AGBToggleWithoutTextDoesNotRequireConsent(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.boolValues[configModel.KeyEnrollmentLegalTermsEnabled] = true

	req := validSubmission(env.phaseID)
	req.ConsentFlags = map[string]any{}
	_, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
}

func TestRequestService_Submit_AGBRequiredWhenTermsBlockConfigured(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.boolValues[configModel.KeyEnrollmentLegalTermsEnabled] = true
	env.settings.stringValues[configModel.KeyEnrollmentLegalAGBText] = "Ganztag Info-Brief"

	req := validSubmission(env.phaseID)
	req.ConsentFlags = map[string]any{"agb": false}
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission))
}

func TestRequestService_Submit_DSGVORequiredWhenPrivacyBlockConfigured(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.stringValues[configModel.KeyEnrollmentLegalDSGVOText] = "Datenschutzinformation"

	req := validSubmission(env.phaseID)
	req.ConsentFlags = map[string]any{}
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission))

	req.ConsentFlags = map[string]any{"data_processing": true}
	_, err = env.svc.Submit(ctx, req)
	require.NoError(t, err)
}

func TestRequestService_Submit_PhotoLegalBlockIsOptional(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	env.settings.stringValues[configModel.KeyEnrollmentLegalPhotoText] = "Fotoeinwilligung"

	req := validSubmission(env.phaseID)
	req.ConsentFlags = map[string]any{"photo": false}
	_, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
}

// Regression for issue #1465: a guardian phone that fails the canonical
// student phone format (e.g. "12345") must be rejected at submit with
// ErrInvalidGuardianPhone, instead of being stored and later blocking
// approval with a 500.
func TestRequestService_Submit_RejectsInvalidGuardianPhone(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	badPhone := "12345"
	req := validSubmission(env.phaseID)
	req.GuardianPhone = &badPhone
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidGuardianPhone),
		"invalid guardian phone must return ErrInvalidGuardianPhone; got %v", err)
}

func TestRequestService_Submit_RejectsMissingRequiredGuardianPhone(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.db.ExecContext(ctx, `
		UPDATE enrollment.form_schemas
		SET core_requirements = '{"guardian_phone": true}'::jsonb
		WHERE id = ?
	`, env.schemaID)
	require.NoError(t, err)

	_, err = env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission),
		"missing required guardian phone must return ErrInvalidSubmission; got %v", err)
}

// Regression: submit used to accept guardian emails that net/mail.ParseAddress
// allows but the canonical student-email rule rejects at approval (e.g.
// "test@localhost" or "a@b" — no dot in the domain). Such a request passed
// submit, got stored, then could never be approved (student.Validate failed),
// leaving it permanently stuck. Submit must now reject them up front with
// ErrInvalidGuardianEmail, using the same rule as approval.
func TestRequestService_Submit_RejectsEmailRejectedAtApproval(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	for _, badEmail := range []string{"test@localhost", "a@b"} {
		req := validSubmission(env.phaseID)
		req.GuardianEmail = badEmail
		_, err := env.svc.Submit(ctx, req)
		require.Error(t, err, "submit must reject %q (approval would reject it)", badEmail)
		assert.True(t, errors.Is(err, enrollmentService.ErrInvalidGuardianEmail),
			"invalid guardian email must return ErrInvalidGuardianEmail; got %v", err)
	}
}

func TestRequestService_Submit_RejectsNoChildren(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	req := validSubmission(env.phaseID)
	req.Children = nil
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission))
}

// TestRequestService_Submit_RejectsMissingTargetGradeLevel verifies the
// public form contract: every child submission must carry a grade
// level. Allowing nil here used to leave admins unable to approve the
// resulting student because users.Student.Validate() rejects empty
// school_class downstream.
func TestRequestService_Submit_RejectsMissingTargetGradeLevel(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	req := validSubmission(env.phaseID)
	req.Children[0].TargetGradeLevel = nil
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrInvalidSubmission),
		"missing target_grade_level must return ErrInvalidSubmission; got %v", err)
	assert.Contains(t, err.Error(), "target_grade_level")
}

func TestRequestService_Submit_RejectsInactiveOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Phase model: per-offering windows are gone. The only way a
	// parent's selection can be rejected at offering level is via
	// is_active=false (admin soft-disabled the row).
	inactiveOffering := &enrollmentModels.CareOffering{
		PhaseID:        env.phaseID,
		Name:           "Inactive slot",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       false,
	}
	inactiveOffering.SetTenantID(1)
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, inactiveOffering))

	req := validSubmission(env.phaseID)
	req.Children[0].OfferingIDs = []int64{inactiveOffering.ID}
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingClosed),
		"inactive offering selection must return ErrCareOfferingClosed, got %v", err)
}

func TestRequestService_Submit_EnforcesPhaseCareOfferingSelectionMode(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)

	env.phase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionAtLeastOne
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingMissing),
		"at_least_one mode must reject children without an offering; got %v", err)
}

func TestRequestService_Submit_EnforcesExactlyOneCareOffering(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)

	env.phase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionExactlyOne
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	first := setupCareOfferingForCapacity(t, env, 0)
	second := setupCareOfferingForCapacity(t, env, 0)
	req := validSubmission(env.phaseID)
	req.Children[0].OfferingIDs = []int64{first.ID, second.ID}

	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingExactlyOneRequired),
		"exactly_one mode must reject multiple selected offerings; got %v", err)
}

// TestRequestService_Submit_ExactlyOneCountsOnlyChoosableOfferings verifies
// that a required (always-on) offering is orthogonal to the selection mode:
// it must NOT count toward "exactly one". A mandatory base offering plus
// exactly one choosable offering submits successfully, while the lone
// required offering (zero choosable) and required-plus-two-choosable are
// both rejected. This guards the contradiction where a required base
// offering would otherwise make exactly_one unsatisfiable.
func TestRequestService_Submit_ExactlyOneCountsOnlyChoosableOfferings(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)

	env.phase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionExactlyOne
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	// Mandatory base offering: always-on, no capacity limit.
	required := &enrollmentModels.CareOffering{
		PhaseID:        env.phaseID,
		Name:           "Mandatory base care",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		IsRequired:     true,
	}
	required.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, required))

	// Two choosable time-slot offerings the parent picks exactly one of.
	slotA := setupCareOfferingForCapacity(t, env, 5)
	slotB := setupCareOfferingForCapacity(t, env, 5)

	// required + exactly one choosable → valid.
	ok := validSubmission(env.phaseID)
	ok.GuardianEmail = "valid@example.com"
	ok.Children[0].OfferingIDs = []int64{required.ID, slotA.ID}
	_, err := env.svc.Submit(ctx, ok)
	require.NoError(t, err,
		"required base offering plus exactly one choosable must submit; got %v", err)

	// required + two choosable → rejected (choosable count is 2).
	tooMany := validSubmission(env.phaseID)
	tooMany.GuardianEmail = "toomany@example.com"
	tooMany.Children[0].OfferingIDs = []int64{required.ID, slotA.ID, slotB.ID}
	_, err = env.svc.Submit(ctx, tooMany)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingExactlyOneRequired),
		"two choosable offerings must be rejected under exactly_one; got %v", err)

	// required only, zero choosable → rejected (choosable count is 0).
	none := validSubmission(env.phaseID)
	none.GuardianEmail = "none@example.com"
	none.Children[0].OfferingIDs = []int64{required.ID}
	_, err = env.svc.Submit(ctx, none)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingExactlyOneRequired),
		"a lone required offering does not satisfy exactly_one; got %v", err)
}

// TestRequestService_Submit_AtLeastOneCountsOnlyChoosableOfferings is the
// at_least_one counterpart: a required base offering does not satisfy "at
// least one" on its own; the parent must additionally pick a choosable
// offering.
func TestRequestService_Submit_AtLeastOneCountsOnlyChoosableOfferings(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)

	env.phase.CareOfferingSelectionMode = enrollmentModels.PhaseCareOfferingSelectionAtLeastOne
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	required := &enrollmentModels.CareOffering{
		PhaseID:        env.phaseID,
		Name:           "Mandatory base care",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		IsRequired:     true,
	}
	required.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, required))

	choosable := setupCareOfferingForCapacity(t, env, 5)

	// required only → rejected (zero choosable selected).
	none := validSubmission(env.phaseID)
	none.GuardianEmail = "none@example.com"
	none.Children[0].OfferingIDs = []int64{required.ID}
	_, err := env.svc.Submit(ctx, none)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingMissing),
		"a lone required offering does not satisfy at_least_one; got %v", err)

	// required + one choosable → valid.
	ok := validSubmission(env.phaseID)
	ok.GuardianEmail = "valid@example.com"
	ok.Children[0].OfferingIDs = []int64{required.ID, choosable.ID}
	_, err = env.svc.Submit(ctx, ok)
	require.NoError(t, err,
		"required base offering plus one choosable must submit under at_least_one; got %v", err)
}

// --- GetByStatusToken ---

func TestRequestService_GetByStatusToken_RoundTrip(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	token := result.Request.StatusToken

	// Use a non-tenant context to verify the service-level token lookup
	// works without prior tenant-tx middleware (mirrors the public route).
	req, children, err := env.svc.GetByStatusToken(context.Background(), token)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Len(t, children, 1)
	assert.Equal(t, "Anna", req.GuardianFirstName)
	assert.Equal(t, "Lina", children[0].FirstName)
}

func TestRequestService_GetByStatusToken_UnknownReturnsNotFound(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	_, _, err := env.svc.GetByStatusToken(context.Background(), "nope-no-such-token")
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRequestNotFound))
}

// Regression: status_reason is admin-internal free text. When the phase
// has show_status_reason_to_parent=false (the default), the public status
// page must NOT surface a stored rejection/waitlist note.
func TestRequestService_GetByStatusToken_RedactsReasonWhenPhaseDisablesIt(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	// Admin records a rejection with an internal reason.
	reason := "Intern: Kapazität voll, Geschwisterkind bevorzugt"
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(
		ctx, result.Children[0].ID, enrollmentModels.ChildStatusRejected, &reason, env.creatorID,
	))

	// Phase defaults to show_status_reason_to_parent=false.
	_, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Nil(t, children[0].StatusReason,
		"status reason must be hidden when the phase disables parent-facing reasons")
}

// Counterpart: when the phase opts in via show_status_reason_to_parent,
// the stored reason is surfaced to the parent unchanged.
func TestRequestService_GetByStatusToken_SurfacesReasonWhenPhaseEnablesIt(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	reason := "Leider keine freien Plätze in diesem Durchgang"
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.RequestChild.UpdateStatus(
		ctx, result.Children[0].ID, enrollmentModels.ChildStatusRejected, &reason, env.creatorID,
	))

	env.phase.ShowStatusReasonToParent = true
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	_, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.NotNil(t, children[0].StatusReason)
	assert.Equal(t, reason, *children[0].StatusReason)
}

// --- Edit ---

func TestRequestService_Edit_AppliesPatchWhileSubmitted(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	token := result.Request.StatusToken

	newPhone := "+49 30 12345"
	err = env.svc.Edit(ctx, token, enrollmentService.EditPatch{
		GuardianPhone: &newPhone,
	})
	require.NoError(t, err)

	req, _, err := env.svc.GetByStatusToken(ctx, token)
	require.NoError(t, err)
	require.NotNil(t, req.GuardianPhone)
	assert.Equal(t, newPhone, *req.GuardianPhone)
}

func TestRequestService_Edit_LocksAfterReviewStarted(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	token := result.Request.StatusToken

	// Move the only child to under_review so Edit must reject.
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t,
		repoFactory.RequestChild.UpdateStatus(
			ctx, result.Children[0].ID, enrollmentModels.ChildStatusUnderReview, nil, 0,
		),
	)

	newName := "Geändert"
	err = env.svc.Edit(ctx, token, enrollmentService.EditPatch{
		GuardianFirstName: &newName,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrEditNotAllowed))
}

func TestRequestService_Edit_RespectsAllowSubmissionEditFalse(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	token := result.Request.StatusToken

	env.settings.boolValues[configModel.KeyEnrollmentAllowSubmissionEdit] = false
	newName := "Verboten"
	err = env.svc.Edit(ctx, token, enrollmentService.EditPatch{
		GuardianFirstName: &newName,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrEditNotAllowed))
}

// --- Withdraw ---

func TestRequestService_Withdraw_PerChildSetsWithdrawnStatus(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Two children so we can withdraw one and confirm the other survives.
	req := validSubmission(env.phaseID)
	req.Children = append(req.Children, enrollmentService.SubmitChild{
		FirstName:        "Tom",
		LastName:         "Beispiel",
		DateOfBirth:      time.Date(2019, 8, 1, 0, 0, 0, 0, time.UTC),
		TargetGradeLevel: testpkg.Int16Ptr(2),
	})
	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)

	require.NoError(t, env.svc.Withdraw(ctx, result.Request.StatusToken, result.Children[0].ID))

	_, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.Len(t, children, 2)

	byID := make(map[int64]string, len(children))
	for _, c := range children {
		byID[c.ID] = c.Status
	}
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, byID[result.Children[0].ID])
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, byID[result.Children[1].ID],
		"sibling withdraw must not affect other children")
}

func TestRequestService_Withdraw_BulkSetsWithdrawnAt(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	// childID == 0 means "withdraw the whole request".
	require.NoError(t, env.svc.Withdraw(ctx, result.Request.StatusToken, 0))

	req, children, err := env.svc.GetByStatusToken(ctx, result.Request.StatusToken)
	require.NoError(t, err)
	require.NotNil(t, req.WithdrawnAt, "request.withdrawn_at must be set after bulk withdraw")
	require.Len(t, children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, children[0].Status)
}

func TestRequestService_Withdraw_PerChildRejectsApproved(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	// Promote child to approved → per-child withdraw must fail.
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t,
		repoFactory.RequestChild.UpdateStatus(
			ctx, result.Children[0].ID, enrollmentModels.ChildStatusApproved, nil, 0,
		),
	)

	err = env.svc.Withdraw(ctx, result.Request.StatusToken, result.Children[0].ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrWithdrawNotAllowed))
}

// --- Rate limiting ---

// TestRequestService_Submit_RateLimitsByEmail walks the configured
// per-email threshold (5 submissions / 24h). The 6th attempt for the
// same email must return ErrRateLimited even when the IP varies.
//
// We vary the child first/last name per iteration so the per-(parent,
// child, phase) duplicate guard added in the parent-enrollment fixup
// doesn't short-circuit the rate-limit path; the rate-limit bucket is
// keyed on (tenant_id, email) so child identity is independent.
func TestRequestService_Submit_RateLimitsByEmail(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Use distinct IPs so we hit the email bucket, not the IP one.
	for i := 0; i < 5; i++ {
		req := validSubmission(env.phaseID)
		req.RemoteIP = "10.0.0." + strconv.Itoa(i+1)
		req.Children[0].FirstName = "Lina" + strconv.Itoa(i)
		req.Children[0].LastName = "Beispiel" + strconv.Itoa(i)
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err, "attempt %d should succeed", i+1)
	}

	// 6th attempt → over the email threshold.
	req := validSubmission(env.phaseID)
	req.RemoteIP = "10.0.0.99"
	req.Children[0].FirstName = "LinaOver"
	req.Children[0].LastName = "BeispielOver"
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRateLimited),
		"6th submission with same email must hit ErrRateLimited; got %v", err)
}

// TestRequestService_Submit_RateLimitsByIP walks the per-IP threshold
// (10 submissions / 1h). Email is varied so we don't hit that bucket
// first.
func TestRequestService_Submit_RateLimitsByIP(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	const sharedIP = "203.0.113.7"
	for i := 0; i < 10; i++ {
		req := validSubmission(env.phaseID)
		req.RemoteIP = sharedIP
		req.GuardianEmail = "anna+" + strconv.Itoa(i) + "@example.com"
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err, "IP attempt %d should succeed", i+1)
	}

	// 11th request → over the IP threshold.
	req := validSubmission(env.phaseID)
	req.RemoteIP = sharedIP
	req.GuardianEmail = "anna+over@example.com"
	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRateLimited),
		"11th submission from same IP must hit ErrRateLimited; got %v", err)
}

// TestRequestService_Submit_RateLimitTenantIsolation verifies that one
// tenant burning through its bucket can't rate-limit another tenant.
// The bucket key is (tenant_id, key_type, key_value).
//
// Same caveat as RateLimitsByEmail: child first/last names vary per
// iteration so the duplicate guard doesn't reject before the
// rate-limit bucket fills.
func TestRequestService_Submit_RateLimitTenantIsolation(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Tenant 1 burns through its email bucket.
	for i := 0; i < 5; i++ {
		req := validSubmission(env.phaseID)
		req.RemoteIP = "10.0.0." + strconv.Itoa(i+1)
		req.Children[0].FirstName = "Lina" + strconv.Itoa(i)
		req.Children[0].LastName = "Beispiel" + strconv.Itoa(i)
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err)
	}

	// Tenant 1 hits the limit.
	overReq := validSubmission(env.phaseID)
	overReq.RemoteIP = "10.0.0.99"
	overReq.Children[0].FirstName = "LinaOver"
	overReq.Children[0].LastName = "BeispielOver"
	_, err := env.svc.Submit(ctx, overReq)
	require.Error(t, err)
	require.True(t, errors.Is(err, enrollmentService.ErrRateLimited))

	// A submission with a different tenant_id must NOT be limited by
	// tenant 1's counter. Direct repo call asserts the upsert keys on
	// (tenant_id, key_type, key_value). Tenant 2 needs to exist in
	// platform.schools to satisfy the FK; EnsureTestTenant is idempotent.
	testpkg.EnsureTestTenant(t, env.db, 2)
	repoFactory := repositories.NewFactory(env.db)
	tenantTwoEmail := "anna+" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.submission_rate_limits").
			Where("tenant_id = ?", 2).
			Where("key_value = ?", tenantTwoEmail).
			Exec(context.Background())
	})
	state, err := repoFactory.SubmissionRateLimit.IncrementAttempts(
		ctx, 2, enrollmentModels.SubmissionRateLimitKeyTypeEmail, tenantTwoEmail, 24*time.Hour,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Attempts,
		"a different tenant's bucket must start fresh, got %d", state.Attempts)
}

// --- Capacity overflow ---

func setupCareOfferingForCapacity(t *testing.T, env *requestTestEnv, capacity int) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	repoFactory := repositories.NewFactory(env.db)
	cap := capacity
	offering := &enrollmentModels.CareOffering{
		PhaseID:             env.phaseID,
		Name:                "Capacity test slot",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		Capacity:            &cap,
		IsActive:            true,
	}
	offering.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, offering))
	return offering
}

// setPhaseOverflowMode is the phase-model replacement for the old
// tenant-wide setting toggle. Updates the phase row so the next
// Submit's applyCapacityOverflow reads the new mode.
func setPhaseOverflowMode(t *testing.T, env *requestTestEnv, mode string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	env.phase.CareOverflowMode = mode
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))
}

// TestRequestService_Submit_CapacityOverflowWaitlist verifies that when
// the offering is at capacity AND the tenant mode is 'waitlist', the
// new child lands as 'waitlisted' instead of 'submitted'.
func TestRequestService_Submit_CapacityOverflowWaitlist(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowWaitlist)

	offering := setupCareOfferingForCapacity(t, env, 1)

	// Fill the slot via a normal submission.
	req1 := validSubmission(env.phaseID)
	req1.GuardianEmail = "first@example.com"
	req1.Children[0].OfferingIDs = []int64{offering.ID}
	r1, err := env.svc.Submit(ctx, req1)
	require.NoError(t, err)
	require.Equal(t, enrollmentModels.ChildStatusSubmitted, r1.Children[0].Status)

	// Second parent picks the same now-full offering → should be waitlisted.
	req2 := validSubmission(env.phaseID)
	req2.GuardianEmail = "second@example.com"
	req2.Children[0].OfferingIDs = []int64{offering.ID}
	r2, err := env.svc.Submit(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusWaitlisted, r2.Children[0].Status,
		"over-capacity child must be persisted as waitlisted under mode=waitlist")
}

// TestRequestService_Submit_CapacityOverflowReject verifies that
// mode=reject aborts the submission with ErrCareOfferingFull instead
// of writing rows.
func TestRequestService_Submit_CapacityOverflowReject(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowReject)

	offering := setupCareOfferingForCapacity(t, env, 1)

	req1 := validSubmission(env.phaseID)
	req1.GuardianEmail = "first@example.com"
	req1.Children[0].OfferingIDs = []int64{offering.ID}
	_, err := env.svc.Submit(ctx, req1)
	require.NoError(t, err)

	req2 := validSubmission(env.phaseID)
	req2.GuardianEmail = "second@example.com"
	req2.Children[0].OfferingIDs = []int64{offering.ID}
	_, err = env.svc.Submit(ctx, req2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingFull),
		"over-capacity submission must return ErrCareOfferingFull under mode=reject; got %v", err)
}

// TestRequestService_Submit_CapacityOverflowAllow verifies that mode=allow
// preserves the default 'submitted' status even when the offering is at
// or over capacity. The admin sorts it out via PR 8 review.
func TestRequestService_Submit_CapacityOverflowAllow(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowAllow)

	offering := setupCareOfferingForCapacity(t, env, 1)

	req1 := validSubmission(env.phaseID)
	req1.GuardianEmail = "first@example.com"
	req1.Children[0].OfferingIDs = []int64{offering.ID}
	_, err := env.svc.Submit(ctx, req1)
	require.NoError(t, err)

	req2 := validSubmission(env.phaseID)
	req2.GuardianEmail = "second@example.com"
	req2.Children[0].OfferingIDs = []int64{offering.ID}
	r2, err := env.svc.Submit(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, r2.Children[0].Status,
		"over-capacity child must remain 'submitted' under mode=allow")
}

// TestRequestService_Submit_CapacityIntraSubmissionCounting verifies the
// "earlier child in the same submission counts" rule. Two siblings in
// one form selecting the same offering with capacity=1 must trigger
// the overflow path for the second sibling, not silently pass through.
func TestRequestService_Submit_CapacityIntraSubmissionCounting(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowReject)

	offering := setupCareOfferingForCapacity(t, env, 1)

	req := validSubmission(env.phaseID)
	req.Children[0].OfferingIDs = []int64{offering.ID}
	req.Children = append(req.Children, enrollmentService.SubmitChild{
		FirstName:        "Tom",
		LastName:         "Beispiel",
		DateOfBirth:      time.Date(2019, 8, 1, 0, 0, 0, 0, time.UTC),
		TargetGradeLevel: testpkg.Int16Ptr(2),
		OfferingIDs:      []int64{offering.ID},
	})

	_, err := env.svc.Submit(ctx, req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingFull),
		"two siblings claiming the same single slot must trigger overflow; got %v", err)
}

// TestRequestService_Submit_CapacityNullMeansUnlimited verifies that an
// offering with NULL capacity never triggers overflow regardless of how
// many children claim it.
func TestRequestService_Submit_CapacityNullMeansUnlimited(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	setPhaseOverflowMode(t, env, enrollmentModels.PhaseCareOverflowReject)

	repoFactory := repositories.NewFactory(env.db)
	offering := &enrollmentModels.CareOffering{
		PhaseID:             env.phaseID,
		Name:                "Unlimited slot",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon"},
		IncludesHolidayCare: false,
		IncludesLunch:       false,
		Capacity:            nil, // unlimited
		IsActive:            true,
	}
	offering.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, offering))

	for i := 0; i < 3; i++ {
		req := validSubmission(env.phaseID)
		req.GuardianEmail = "u" + strconv.Itoa(i) + "@example.com"
		req.RemoteIP = "10.1.0." + strconv.Itoa(i+1)
		req.Children[0].OfferingIDs = []int64{offering.ID}
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err, "unlimited capacity must accept all submissions")
	}
}

// --- Dedup / duplicate enrollment guard ---

// TestRequestService_Submit_RejectsDuplicateChild covers the dedup
// guard: a parent who already has an in-flight submission for a child
// can't submit the same child again in the same phase. The dedup check
// is case-insensitive on email and child names, runs inside the write
// tx under an advisory lock so concurrent submits serialize.
func TestRequestService_Submit_RejectsDuplicateChild(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	first, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	require.NotNil(t, first.Request)

	// Same parent + same child + same phase = duplicate.
	dup := validSubmission(env.phaseID)
	dup.RemoteIP = "10.0.0.99"
	_, err = env.svc.Submit(ctx, dup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDuplicateEnrollment),
		"second identical submission must hit ErrDuplicateEnrollment; got %v", err)
}

// TestRequestService_Submit_DedupCaseInsensitive verifies the dedup
// check normalizes email + child names. A parent who originally
// submitted with "anna@example.com" and "Lina Beispiel" can't slip
// the same child past the guard by retyping the form with
// "Anna@Example.com" and "  lina   beispiel  ".
func TestRequestService_Submit_DedupCaseInsensitive(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	variant := validSubmission(env.phaseID)
	variant.GuardianEmail = "  Anna@EXAMPLE.com  "
	variant.Children[0].FirstName = "  LINA"
	variant.Children[0].LastName = "beispiel  "
	variant.RemoteIP = "10.0.0.50"
	_, err = env.svc.Submit(ctx, variant)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDuplicateEnrollment),
		"case-different rewrite must still hit dedup; got %v", err)
}

// TestRequestService_Submit_DedupAllowsDifferentChild verifies the
// guard is per-(parent, child, phase). A parent submitting one child
// after another in the same phase must succeed — twins or siblings
// are a legitimate use case.
func TestRequestService_Submit_DedupAllowsDifferentChild(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	sibling := validSubmission(env.phaseID)
	sibling.Children[0].FirstName = "Max"
	sibling.Children[0].LastName = "Beispiel"
	sibling.RemoteIP = "10.0.0.51"
	_, err = env.svc.Submit(ctx, sibling)
	require.NoError(t, err, "different child name must pass dedup")
}

// TestRequestService_Submit_DedupAllowsDifferentParent verifies two
// distinct parents can each submit a child named "Lina Beispiel" in
// the same phase. Dedup is keyed on the parent's email.
func TestRequestService_Submit_DedupAllowsDifferentParent(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)

	other := validSubmission(env.phaseID)
	other.GuardianEmail = "different@example.com"
	other.RemoteIP = "10.0.0.52"
	_, err = env.svc.Submit(ctx, other)
	require.NoError(t, err, "different parent email must pass dedup")
}

// TestRequestService_Submit_DedupIgnoresWithdrawn covers the
// "retry-after-withdrawal" branch. A parent who withdrew an
// enrollment must be able to resubmit the same child later — the
// dedup query filters out rows where the child status is withdrawn or
// rejected.
func TestRequestService_Submit_DedupIgnoresWithdrawn(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	first, err := env.svc.Submit(ctx, validSubmission(env.phaseID))
	require.NoError(t, err)
	require.Len(t, first.Children, 1)

	// Withdraw the child via the service so the status flip lands
	// through the same code path the parent would hit.
	require.NoError(t, env.svc.Withdraw(ctx, first.Request.StatusToken, first.Children[0].ID))

	retry := validSubmission(env.phaseID)
	retry.RemoteIP = "10.0.0.53"
	_, err = env.svc.Submit(ctx, retry)
	require.NoError(t, err, "withdrawn submission must not block a resubmit")
}

// --- Basis phase (form_schema_id IS NULL) ---

// TestRequestService_Submit_BasisPhaseNoFallback guards the Basis
// invariant: a phase with form_schema_id=NULL submits with schema_id
// NULL on the request row. Previously the resolver fell back to the
// tenant's currently-active schema, leaking custom fields into every
// Basis phase as soon as one was published. The fixup makes Basis
// strict — admin said "nur die Standardfelder", so we honor it.
func TestRequestService_Submit_BasisPhaseNoFallback(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Build a second phase with FormSchemaID nil ("Basis"). The shared
	// env phase doesn't pin a schema either, but it's safer to spell
	// it out so the test breaks loudly if the default changes.
	basis := &enrollmentModels.Phase{
		Name:             "basis-" + t.Name(),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: env.phase.ServiceStartDate,
		ServiceEndDate:   env.phase.ServiceEndDate,
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
		FormSchemaID:     nil,
	}
	basis.SetTenantID(1)
	repoFactory := repositories.NewFactory(env.db)
	require.NoError(t, repoFactory.Phase.Create(ctx, basis))
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", basis.ID).
			Exec(context.Background())
	}()

	req := validSubmission(basis.ID)
	req.GuardianEmail = "basis-tester@example.com"
	req.Children[0].FirstName = "BasisChild"
	req.Children[0].LastName = "BasisName"
	res, err := env.svc.Submit(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Request)
	assert.Nil(t, res.Request.SchemaID,
		"Basis phase must persist with schema_id=NULL, not silently fall back to the tenant's active schema")
}

// TestRequestService_Submit_HiddenRequiredFieldDoesNotBlockAndIsNotPersisted
// is the key regression for conditional required fields on the real submit
// path: a required field hidden by its show-if condition must NOT block an
// otherwise valid submit, and a value smuggled for that hidden field must NOT
// be persisted.
func TestRequestService_Submit_HiddenRequiredFieldDoesNotBlockAndIsNotPersisted(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Schema: per-child boolean controller "has_allergy" + per-child REQUIRED
	// text "which_allergy" that is only visible when has_allergy == true.
	repoFactory := repositories.NewFactory(env.db)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "has_allergy", Label: "Allergie?", Type: enrollmentModels.FormFieldBoolean, AppliesToCh: true, SortOrder: 0},
		{
			Key: "which_allergy", Label: "Welche?", Type: enrollmentModels.FormFieldText,
			AppliesToCh: true, Required: true, SortOrder: 1,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
				Operator: enrollmentModels.ConditionOpEquals, Value: true,
			},
		},
	}, env.creatorID)
	require.NoError(t, err)

	// Point the phase at this schema.
	env.phase.FormSchemaID = &schema.ID
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	// has_allergy=false → which_allergy is hidden + unanswered. The client
	// also smuggles a stale value for the hidden field.
	req := validSubmission(env.phaseID)
	req.Children[0].CustomData = map[string]any{
		"has_allergy":   false,
		"which_allergy": "smuggled value",
	}

	result, err := env.svc.Submit(ctx, req)
	require.NoError(t, err, "a hidden required field must not block an otherwise valid submit")
	require.Len(t, result.Children, 1)

	child := result.Children[0]
	_, hasHidden := child.CustomData["which_allergy"]
	assert.False(t, hasHidden, "value for the hidden field must be sanitized out before persisting")
	assert.Equal(t, false, child.CustomData["has_allergy"], "visible controller answer is kept")
}

// TestRequestService_Submit_VisibleRequiredFieldStillEnforced is the positive
// counterpart: when the controller makes the required field visible, leaving it
// blank must still block the submit.
func TestRequestService_Submit_VisibleRequiredFieldStillEnforced(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	repoFactory := repositories.NewFactory(env.db)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "has_allergy", Label: "Allergie?", Type: enrollmentModels.FormFieldBoolean, AppliesToCh: true, SortOrder: 0},
		{
			Key: "which_allergy", Label: "Welche?", Type: enrollmentModels.FormFieldText,
			AppliesToCh: true, Required: true, SortOrder: 1,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
				Operator: enrollmentModels.ConditionOpEquals, Value: true,
			},
		},
	}, env.creatorID)
	require.NoError(t, err)
	env.phase.FormSchemaID = &schema.ID
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	// has_allergy=true → which_allergy is visible + required, but left blank.
	req := validSubmission(env.phaseID)
	req.Children[0].CustomData = map[string]any{"has_allergy": true}

	_, err = env.svc.Submit(ctx, req)
	require.Error(t, err, "a visible required field left blank must block the submit")
}

// TestRequestService_Submit_RequiredStructuredFieldValidatesEntries guards the
// real submit path for structured required fields: a non-empty array is NOT
// enough — malformed entries (a contact with no name / no contact channel)
// must be rejected at submit, not pass through to a later approval failure.
func TestRequestService_Submit_RequiredStructuredFieldValidatesEntries(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	repoFactory := repositories.NewFactory(env.db)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})
	// Required per-child contact_list (canonical target student.contacts).
	schema, err := schemaSvc.PublishVersion(ctx, []enrollmentModels.FormField{
		{
			Key: "contacts", Label: "Notfallkontakte",
			Type: enrollmentModels.FormFieldContactList, AppliesToCh: true,
			Required: true, Target: enrollmentModels.TargetStudentContacts, SortOrder: 0,
		},
	}, env.creatorID)
	require.NoError(t, err)
	env.phase.FormSchemaID = &schema.ID
	require.NoError(t, repoFactory.Phase.Update(ctx, env.phase))

	// Malformed: a single contact with no name and no email/phone.
	bad := validSubmission(env.phaseID)
	bad.Children[0].CustomData = map[string]any{"contacts": []any{map[string]any{}}}
	_, err = env.svc.Submit(ctx, bad)
	require.Error(t, err, "a required contact_list with a malformed entry must be rejected at submit")

	// Valid: a complete contact (name + email).
	ok := validSubmission(env.phaseID)
	ok.Children[0].CustomData = map[string]any{"contacts": []any{
		map[string]any{"first_name": "Eva", "last_name": "Muster", "email": "eva@example.test"},
	}}
	res, err := env.svc.Submit(ctx, ok)
	require.NoError(t, err, "a valid required contact_list must be accepted")
	require.Len(t, res.Children, 1)
}
