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
	// Default to "enabled, no window restrictions" — individual tests
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
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:   "Lina",
				LastName:    "Beispiel",
				DateOfBirth: time.Date(2018, 4, 15, 0, 0, 0, 0, time.UTC),
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
		FirstName:   "Tom",
		LastName:    "Beispiel",
		DateOfBirth: time.Date(2019, 8, 1, 0, 0, 0, 0, time.UTC),
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
func TestRequestService_Submit_RateLimitsByEmail(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Use distinct IPs so we hit the email bucket, not the IP one.
	for i := 0; i < 5; i++ {
		req := validSubmission(env.phaseID)
		req.RemoteIP = "10.0.0." + strconv.Itoa(i+1)
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err, "attempt %d should succeed", i+1)
	}

	// 6th attempt → over the email threshold.
	req := validSubmission(env.phaseID)
	req.RemoteIP = "10.0.0.99"
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
func TestRequestService_Submit_RateLimitTenantIsolation(t *testing.T) {
	env, cleanup := setupRequestTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// Tenant 1 burns through its email bucket.
	for i := 0; i < 5; i++ {
		req := validSubmission(env.phaseID)
		req.RemoteIP = "10.0.0." + strconv.Itoa(i+1)
		_, err := env.svc.Submit(ctx, req)
		require.NoError(t, err)
	}

	// Tenant 1 hits the limit.
	overReq := validSubmission(env.phaseID)
	overReq.RemoteIP = "10.0.0.99"
	_, err := env.svc.Submit(ctx, overReq)
	require.Error(t, err)
	require.True(t, errors.Is(err, enrollmentService.ErrRateLimited))

	// A submission with a different tenant_id must NOT be limited by
	// tenant 1's counter. Direct repo call asserts the upsert keys on
	// (tenant_id, key_type, key_value). Tenant 2 needs to exist in
	// platform.schools to satisfy the FK; EnsureTestTenant is idempotent.
	testpkg.EnsureTestTenant(t, env.db, 2)
	repoFactory := repositories.NewFactory(env.db)
	state, err := repoFactory.SubmissionRateLimit.IncrementAttempts(
		ctx, 2, enrollmentModels.SubmissionRateLimitKeyTypeEmail, "anna@example.com", 24*time.Hour,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Attempts,
		"a different tenant's bucket must start fresh, got %d", state.Attempts)
	// Cleanup the bucket row we just inserted for tenant 2.
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.submission_rate_limits").
			Where("tenant_id = ?", 2).
			Exec(context.Background())
	})
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
		FirstName:   "Tom",
		LastName:    "Beispiel",
		DateOfBirth: time.Date(2019, 8, 1, 0, 0, 0, 0, time.UTC),
		OfferingIDs: []int64{offering.ID},
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
