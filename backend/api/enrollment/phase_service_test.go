package enrollment_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

func setupPhaseTest(t *testing.T) (enrollmentOwner.PhaseAdministration, *repositories.Factory, *bun.DB, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	phaseNamePrefix := "phase-" + t.Name()
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records:   repoFactory.Enrollment(),
		Offerings: phaseOfferings{rows: newCareOfferingFixtures(repoFactory.CarePlan())},
		Logger:    slog.Default(),
	})

	cleanup := func() {
		bg := context.Background()
		tenantID := testpkg.Tenant(t)
		// Deleting requests cascades request_children + request_child_offerings.
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND name LIKE ?)", tenantID, phaseNamePrefix+"%").
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND name LIKE ?)", tenantID, phaseNamePrefix+"%").
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("tenant_id = ? AND name LIKE ?", tenantID, phaseNamePrefix+"%").
			Exec(bg)
	}
	return svc, repoFactory, db, cleanup
}

func minimalPhase(t *testing.T, suffix string) *enrollmentOwner.Phase {
	p := &enrollmentOwner.Phase{
		Name:             "phase-" + suffix,
		Kind:             enrollmentOwner.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentOwner.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentOwner.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentOwner.PhaseCareOverflowWaitlist,
	}
	p.TenantID = testpkg.Tenant(t)
	return p
}

func TestPhaseService_Create_ValidatesAndPersists(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, enrollmentOwner.PhaseKindSchoolYear, created.Kind)
	assert.True(t, created.IsActive)
}

func TestPhaseService_Create_RejectsServiceDateInversion(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	bad := minimalPhase(t, t.Name())
	bad.ServiceStartDate = enrollmentOwner.Date(timezone.NewDate(2027, 9, 1))
	bad.ServiceEndDate = enrollmentOwner.Date(timezone.NewDate(2026, 7, 31))

	_, err := svc.CreatePhase(ctx, bad)
	require.Error(t, err, "service_end_date < service_start_date must be rejected")
}

func TestPhaseService_Create_RejectsUnknownKind(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	bad := minimalPhase(t, t.Name())
	bad.Kind = "invalid_kind"

	_, err := svc.CreatePhase(ctx, bad)
	require.Error(t, err)
}

func TestPhaseService_Create_RejectsDuplicateName(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	first := minimalPhase(t, t.Name()+"-dup")
	_, err := svc.CreatePhase(ctx, first)
	require.NoError(t, err)

	second := minimalPhase(t, t.Name()+"-dup")
	_, err = svc.CreatePhase(ctx, second)
	require.Error(t, err, "UNIQUE(tenant_id, name) must reject duplicate")
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseDuplicateName))
}

func TestPhaseService_Create_RejectsUnknownFormSchema(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	missing := int64(999_999_999)
	phase.FormSchemaID = &missing

	_, err := svc.CreatePhase(ctx, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrInvalidPhase))
}

func TestPhaseService_Update_AppliesChanges(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	created.Name = "phase-" + t.Name() + "-renamed"
	created.IsActive = false
	created.CareOverflowMode = enrollmentOwner.PhaseCareOverflowReject
	require.NoError(t, svc.UpdatePhase(ctx, created))

	refreshed, err := svc.PhaseByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "phase-"+t.Name()+"-renamed", refreshed.Name)
	assert.False(t, refreshed.IsActive)
	assert.Equal(t, enrollmentOwner.PhaseCareOverflowReject, refreshed.CareOverflowMode)
}

func TestPhaseService_Update_ValidatesCareOfferingsOnlyWhenServiceWindowChanges(t *testing.T) {
	t.Parallel()

	baseService, repoFactory, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	originalEnd := created.ServiceEndDate
	lockCalls := 0
	validatorCalls := 0
	guardedService := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records:   repoFactory.Enrollment(),
		Offerings: phaseOfferings{rows: newCareOfferingFixtures(repoFactory.CarePlan())},
		LockTemplateRecurrence: func(context.Context) error {
			lockCalls++
			return nil
		},
		ValidateCareOfferingPhaseChange: func(
			_ context.Context,
			phaseID int64,
			replacement *enrollmentOwner.Phase,
		) error {
			validatorCalls++
			assert.Equal(t, created.ID, phaseID)
			assert.Equal(t, enrollmentOwner.Date(timezone.Date(originalEnd).AddDays(7)), enrollmentOwner.Date(replacement.ServiceEndDate))
			return fmt.Errorf("%w: synthetic uncovered occurrence", careplan.ErrCareOfferingConfigInvalid)
		},
		Logger: slog.Default(),
	})

	created.Name += "-metadata-only"
	require.NoError(t, guardedService.UpdatePhase(ctx, created),
		"metadata-only changes must not be rejected by unrelated legacy care-offering state")
	assert.Zero(t, validatorCalls)

	created.ServiceEndDate = enrollmentOwner.Date(timezone.Date(originalEnd).AddDays(7))
	err = guardedService.UpdatePhase(ctx, created)
	require.ErrorIs(t, err, enrollmentOwner.ErrPhaseCareOfferingConflict)
	assert.Equal(t, 2, lockCalls)
	assert.Equal(t, 1, validatorCalls)

	stored, findErr := repoFactory.Enrollment().Phase(ctx, created.ID)
	require.NoError(t, findErr)
	assert.Equal(t, originalEnd, stored.ServiceEndDate,
		"a rejected service-window expansion must not be persisted")
}

// recordingSourcedTemplateResyncer captures the offering IDs the phase
// service hands to the sourced-template resync after a service-window change.
// A non-nil err is returned after recording, simulating a resync that reports
// the new window as incompatible with a sourced template.
type recordingSourcedTemplateResyncer struct {
	offeringIDs         []int64
	detachedOfferingIDs []int64
	err                 error
}

func (r *recordingSourcedTemplateResyncer) ResyncTemplatesSourcedFromOffering(
	_ context.Context, offeringID int64, _ timezone.Date,
) error {
	r.offeringIDs = append(r.offeringIDs, offeringID)
	return r.err
}

func (r *recordingSourcedTemplateResyncer) DetachTemplatesSourcedFromOffering(
	_ context.Context, offeringID int64, _ timezone.Date,
) error {
	r.detachedOfferingIDs = append(r.detachedOfferingIDs, offeringID)
	return nil
}

func TestPhaseService_Update_ResyncsSourcedTemplatesOnServiceWindowChange(t *testing.T) {
	t.Parallel()

	baseService, repoFactory, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	offering := &enrollmentModels.CareOffering{
		PhaseID:            created.ID,
		Name:               "offering-" + t.Name(),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		AutoAddGradeLevels: []int{},
		SelectionRule:      enrollmentModels.SelectionRuleOptional,
	}
	require.NoError(t, newCareOfferingFixtures(repoFactory.CarePlan()).Create(ctx, offering))

	resyncer := &recordingSourcedTemplateResyncer{}
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactory.CarePlan())},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		SourcedTemplates:       func() enrollmentOwner.SourcedTemplateResyncer { return resyncer },
		Logger:                 slog.Default(),
	})

	created.Name += "-metadata-only"
	require.NoError(t, svc.UpdatePhase(ctx, created))
	assert.Empty(t, resyncer.offeringIDs,
		"metadata-only updates must not resync sourced templates")

	created.ServiceEndDate = enrollmentOwner.Date(timezone.Date(created.ServiceEndDate).AddDays(7))
	require.NoError(t, svc.UpdatePhase(ctx, created))
	assert.Equal(t, []int64{offering.ID}, resyncer.offeringIDs,
		"a service-window change must resync every template sourcing the phase's offerings")
}

// TestPhaseService_Update_RejectsWindowChangeInvalidatingSourcedTemplate pins
// the #2147 round-7 review contract: when the sourced-template resync reports
// ErrOfferingSourceInvalid — the new service window no longer fits a sourced
// template's planning period — the update must surface as
// ErrPhaseCareOfferingConflict and request rollback of the ambient tenant
// transaction, instead of committing a phase whose sourced rosters and
// materialized occurrences could never be resynced again.
func TestPhaseService_Update_RejectsWindowChangeInvalidatingSourcedTemplate(t *testing.T) {
	t.Parallel()

	baseService, repoFactory, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := tenant.WithRollbackMarker(testpkg.Ctx(t))

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	offering := &enrollmentModels.CareOffering{
		PhaseID:            created.ID,
		Name:               "offering-" + t.Name(),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		AutoAddGradeLevels: []int{},
		SelectionRule:      enrollmentModels.SelectionRuleOptional,
	}
	require.NoError(t, newCareOfferingFixtures(repoFactory.CarePlan()).Create(ctx, offering))

	resyncer := &recordingSourcedTemplateResyncer{
		err: fmt.Errorf("offering roster resync: template 7: %w", timetable.ErrOfferingSourceInvalid),
	}
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactory.CarePlan())},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		SourcedTemplates:       func() enrollmentOwner.SourcedTemplateResyncer { return resyncer },
		Logger:                 slog.Default(),
	})

	created.ServiceEndDate = enrollmentOwner.Date(timezone.Date(created.ServiceEndDate).AddDays(7))
	err = svc.UpdatePhase(ctx, created)
	require.ErrorIs(t, err, enrollmentOwner.ErrPhaseCareOfferingConflict,
		"an incompatible sourced template must reject the window change, not be skipped")
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	assert.Equal(t, []int64{offering.ID}, resyncer.offeringIDs)
	assert.True(t, tenant.RollbackRequested(ctx),
		"the rejected update must discard the already-written phase row via the ambient-transaction rollback marker")
	persisted, err := baseService.PhaseByID(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentOwner.Date(timezone.Date(created.ServiceEndDate).AddDays(-7)), persisted.ServiceEndDate,
		"a standalone update must roll back the phase when sourced-template resync fails")
	resyncer.err = nil
	require.NoError(t, svc.UpdatePhase(testpkg.Ctx(t), created))
	persisted, err = baseService.PhaseByID(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ServiceEndDate, persisted.ServiceEndDate,
		"retry must commit the requested window after the resync failure is resolved")
}

func TestPhaseService_Update_RejectsDuplicateName(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	first, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()+"-first"))
	require.NoError(t, err)
	second, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()+"-second"))
	require.NoError(t, err)

	second.Name = first.Name
	err = svc.UpdatePhase(ctx, second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseDuplicateName))
}

func TestPhaseService_Update_RejectsUnknownFormSchema(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	missing := int64(999_999_999)
	created.FormSchemaID = &missing
	err = svc.UpdatePhase(ctx, created)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrInvalidPhase))
}

func TestPhaseService_Update_MissingPhaseReturnsNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	phase.ID = 999_999_999

	err := svc.UpdatePhase(ctx, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound))
}

func TestPhaseService_ListPublicOpen_FiltersInactiveAndClosedWindow(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	now := time.Now()
	openStart := now.AddDate(0, 0, -7)
	openEnd := now.AddDate(0, 0, 7)
	pastEnd := now.AddDate(0, 0, -1)

	// Currently open + active.
	openActive := minimalPhase(t, t.Name()+"-open-active")
	openActive.EnrollmentOpenAt = &openStart
	openActive.EnrollmentCloseAt = &openEnd
	_, err := svc.CreatePhase(ctx, openActive)
	require.NoError(t, err)

	// Open + inactive (admin hidden).
	hidden := minimalPhase(t, t.Name()+"-open-inactive")
	hidden.EnrollmentOpenAt = &openStart
	hidden.EnrollmentCloseAt = &openEnd
	hidden.IsActive = false
	_, err = svc.CreatePhase(ctx, hidden)
	require.NoError(t, err)

	// Active but window closed yesterday.
	closed := minimalPhase(t, t.Name()+"-closed-window")
	closed.EnrollmentOpenAt = &openStart
	closed.EnrollmentCloseAt = &pastEnd
	_, err = svc.CreatePhase(ctx, closed)
	require.NoError(t, err)

	open, err := svc.ListPublicOpen(ctx, now)
	require.NoError(t, err)
	testPhaseNames := map[string]struct{}{
		"phase-" + t.Name() + "-open-active":   {},
		"phase-" + t.Name() + "-open-inactive": {},
		"phase-" + t.Name() + "-closed-window": {},
	}
	matching := make([]*enrollmentOwner.Phase, 0, len(open))
	for _, phase := range open {
		if _, ok := testPhaseNames[phase.Name]; ok {
			matching = append(matching, phase)
		}
	}
	require.Len(t, matching, 1, "only this test's open+active phase should appear")
	assert.Equal(t, "phase-"+t.Name()+"-open-active", matching[0].Name)
}

// Business rule (changed): a phase with care offerings is now deletable.
// The offerings cascade away with the phase; there is no reference guard.
func TestPhaseService_Delete_RemovesPhaseWithOfferings(t *testing.T) {
	t.Parallel()

	svc, repoFactory, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phase, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Linked offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(repoFactory.CarePlan()).Create(ctx, offering))

	require.NoError(t, svc.DeletePhase(ctx, phase.ID),
		"phase with care offerings must be deletable")

	_, err = svc.PhaseByID(ctx, phase.ID)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound),
		"phase must be gone after delete")
	remaining, err := newCareOfferingFixtures(repoFactory.CarePlan()).CountByPhaseID(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "care offerings must cascade away with the phase")
}

// Business rule (changed): a phase with enrollment requests is now
// deletable. Requests + their children cascade away, but students that
// were created from those requests are PRESERVED (created_student_id is
// ON DELETE SET NULL; the student is the parent in that relationship).
func TestPhaseService_Delete_RemovesRequestsAndKeepsCreatedStudents(t *testing.T) {
	t.Parallel()

	svc, repoFactory, db, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// A real student that an approved enrollment "created".
	student := testpkg.CreateTestStudent(t, db, "Kept", "Child", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.student_profiles").Where("id = ?", student.ID).Exec(bg)
	}()

	phase, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	req := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Test",
		GuardianLastName:  "Guardian",
		GuardianEmail:     "test@example.com",
		StatusToken:       "test-token-" + t.Name(),
		SubmittedAt:       time.Now(),
	}
	req.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentAPI.InsertOwnerRequestForTest(ctx, repoFactory.Enrollment(), req))

	child := &enrollmentAPI.RequestChild{
		RequestID:        req.ID,
		FirstName:        "Kept",
		LastName:         "Child",
		DateOfBirth:      "2019-05-01",
		CreatedStudentID: &student.ID,
	}
	child.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentAPI.InsertOwnerChildForTest(ctx, repoFactory.Enrollment(), child))

	require.NoError(t, svc.DeletePhase(ctx, phase.ID),
		"phase with enrollment requests must be deletable")

	_, err = svc.PhaseByID(ctx, phase.ID)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound))
	reqCount, err := repoFactory.Enrollment().CountPhaseRequests(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, reqCount, "requests must cascade away with the phase")

	// The student must survive — deleting the request child never deletes
	// the student it points to.
	studentCount, err := db.NewSelect().
		TableExpr("users.student_profiles").
		Where("id = ?", student.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, studentCount,
		"student created from the phase must survive phase deletion")
}

func TestPhaseService_DeleteImpact_ReportsCounts(t *testing.T) {
	t.Parallel()

	svc, repoFactory, db, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Impact", "Child", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.student_profiles").Where("id = ?", student.ID).Exec(bg)
	}()

	phase, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Impact offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(repoFactory.CarePlan()).Create(ctx, offering))

	req := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Impact",
		GuardianLastName:  "Guardian",
		GuardianEmail:     "impact@example.com",
		StatusToken:       "impact-token-" + t.Name(),
		SubmittedAt:       time.Now(),
	}
	req.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentAPI.InsertOwnerRequestForTest(ctx, repoFactory.Enrollment(), req))

	child := &enrollmentAPI.RequestChild{
		RequestID:        req.ID,
		FirstName:        "Impact",
		LastName:         "Child",
		DateOfBirth:      "2019-05-01",
		CreatedStudentID: &student.ID,
	}
	child.TenantID = testpkg.Tenant(t)
	require.NoError(t, enrollmentAPI.InsertOwnerChildForTest(ctx, repoFactory.Enrollment(), child))

	impact, err := svc.DeleteImpact(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, impact.Requests, "one request references the phase")
	assert.Equal(t, 1, impact.CareOfferings, "one care offering references the phase")
	assert.Equal(t, 1, impact.StudentsKept, "one created student would be kept")
}

func TestPhaseService_Delete_RemovesEmptyPhase(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phase, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.NoError(t, svc.DeletePhase(ctx, phase.ID))

	_, err = svc.PhaseByID(ctx, phase.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound))
}

func TestPhaseService_GetByID_NotFoundSentinel(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	_, err := svc.PhaseByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound))
}

func TestPhaseService_GetByID_PreservesRepositoryFailure(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("database unavailable")
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records: findByIDErrorPhaseRepo{err: repoErr},
	})

	_, err := svc.PhaseByID(context.Background(), 123)

	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
	assert.False(t, errors.Is(err, enrollmentOwner.ErrPhaseNotFound))
}

type findByIDErrorPhaseRepo struct {
	enrollmentAPI.TestPhaseRecords
	err error
}

func (r findByIDErrorPhaseRepo) Phase(context.Context, int64) (*enrollmentOwner.Phase, error) {
	return nil, r.err
}

// phaseServiceWithCalendarPeriods wires the optional CalendarPeriods dep
// on top of the standard setup so the link validation actually runs.
func phaseServiceWithCalendarPeriods(t *testing.T) (enrollmentOwner.PhaseAdministration, *repositories.Factory, *bun.DB, func()) {
	t.Helper()
	_, repoFactory, db, cleanup := setupPhaseTest(t)
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Records:   repoFactory.Enrollment(),
		Offerings: phaseOfferings{rows: newCareOfferingFixtures(repoFactory.CarePlan())},
		Calendar:  repoFactory.SchoolCalendar(),
		Logger:    slog.Default(),
	})
	return svc, repoFactory, db, cleanup
}

func TestPhaseService_Create_WithCalendarPeriodLink(t *testing.T) {
	t.Parallel()

	svc, _, db, cleanup := phaseServiceWithCalendarPeriods(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := &calendarPeriod{
		Name:            "period-" + t.Name(),
		PeriodType:      schoolcalendar.PeriodTypeSemester,
		StartDate:       timezone.NewDate(2026, 8, 1),
		EndDate:         timezone.NewDate(2027, 1, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCalendarPeriodRow(ctx, db, period))
	defer func() {
		_, _ = db.NewDelete().
			TableExpr("schedule.calendar_periods").
			Where("id = ?", period.ID).
			Exec(context.Background())
	}()

	phase := minimalPhase(t, t.Name())
	phase.CalendarPeriodID = &period.ID

	created, err := svc.CreatePhase(ctx, phase)
	require.NoError(t, err)
	require.NotNil(t, created.CalendarPeriodID)
	assert.Equal(t, period.ID, *created.CalendarPeriodID)

	fetched, err := svc.PhaseByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.CalendarPeriodID)
	assert.Equal(t, period.ID, *fetched.CalendarPeriodID)
}

// Regression: the repo's explicit Set() list in Update() was missing
// calendar_period_id, so linking an EXISTING phase to a period silently
// persisted nothing (create worked, update didn't).
func TestPhaseService_Update_PersistsCalendarPeriodLink(t *testing.T) {
	t.Parallel()

	svc, _, db, cleanup := phaseServiceWithCalendarPeriods(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := &calendarPeriod{
		Name:            "period-" + t.Name(),
		PeriodType:      schoolcalendar.PeriodTypeSemester,
		StartDate:       timezone.NewDate(2026, 8, 1),
		EndDate:         timezone.NewDate(2027, 1, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCalendarPeriodRow(ctx, db, period))
	defer func() {
		_, _ = db.NewDelete().
			TableExpr("schedule.calendar_periods").
			Where("id = ?", period.ID).
			Exec(context.Background())
	}()

	created, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.Nil(t, created.CalendarPeriodID)

	created.CalendarPeriodID = &period.ID
	require.NoError(t, svc.UpdatePhase(ctx, created))

	fetched, err := svc.PhaseByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.CalendarPeriodID, "update must persist the calendar period link")
	assert.Equal(t, period.ID, *fetched.CalendarPeriodID)

	// Unlinking must persist too (NULL round-trip).
	fetched.CalendarPeriodID = nil
	require.NoError(t, svc.UpdatePhase(ctx, fetched))
	unlinked, err := svc.PhaseByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, unlinked.CalendarPeriodID)
}

func TestPhaseService_Create_RejectsUnknownCalendarPeriod(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := phaseServiceWithCalendarPeriods(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	missing := int64(999_999_999)
	phase.CalendarPeriodID = &missing

	_, err := svc.CreatePhase(ctx, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrInvalidPhase))
}

func TestPhaseService_Update_RejectsUnknownCalendarPeriod(t *testing.T) {
	t.Parallel()

	svc, _, _, cleanup := phaseServiceWithCalendarPeriods(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	created, err := svc.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	missing := int64(999_999_999)
	created.CalendarPeriodID = &missing
	err = svc.UpdatePhase(ctx, created)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentOwner.ErrInvalidPhase))
}

func TestPhaseService_Create_PropagatesCalendarPeriodLookupFailure(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("synthetic database failure")
	svc := enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{
		Calendar: failingCalendarPeriodService{err: lookupErr},
		Logger:   slog.Default(),
	})
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	periodID := int64(42)
	phase.CalendarPeriodID = &periodID

	_, err := svc.CreatePhase(ctx, phase)
	require.Error(t, err)
	assert.ErrorIs(t, err, lookupErr)
	assert.False(t, errors.Is(err, enrollmentOwner.ErrInvalidPhase))
}

type failingCalendarPeriodService struct {
	err error
}

func (s failingCalendarPeriodService) FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error) {
	return schoolcalendar.CalendarPeriod{}, s.err
}
