package enrollmenthttp_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

func setupPhaseTest(t *testing.T) (enrollmentNative.PhaseAdministration, *owners.EnrollmentFlowTestRepositories, owners.TimetableCarePlanRows, api.TimetableSchoolCalendar, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	repoFactory, repoFactoryCare, _, _, repoFactoryCalendar, _ := flowRepositories(t, db)
	svc := NewTestPhases(TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactoryCare.CarePlan)},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Logger:                 slog.Default(),
	})
	return svc, repoFactory, repoFactoryCare, repoFactoryCalendar, db
}

func minimalPhase(t *testing.T, suffix string) *enrollmentNative.Phase {
	p := &enrollmentNative.Phase{
		Name:             "phase-" + suffix,
		Kind:             enrollmentNative.PhaseKindSchoolYear,
		ServiceStartDate: enrollmentNative.Date(calendar.NewDate(2026, 9, 1)),
		ServiceEndDate:   enrollmentNative.Date(calendar.NewDate(2027, 7, 31)),
		IsActive:         true,
		CareOverflowMode: enrollmentNative.PhaseCareOverflowWaitlist,
	}
	p.TenantID = testpkg.Tenant(t)
	return p
}

func TestPhaseService_Create_ValidatesAndPersists(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	created, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, enrollmentNative.PhaseKindSchoolYear, created.Kind)
	assert.True(t, created.IsActive)
}

func TestPhaseService_Create_RejectsServiceDateInversion(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	bad := minimalPhase(t, t.Name())
	bad.ServiceStartDate = enrollmentNative.Date(calendar.NewDate(2027, 9, 1))
	bad.ServiceEndDate = enrollmentNative.Date(calendar.NewDate(2026, 7, 31))

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, bad)
	require.Error(t, err, "service_end_date < service_start_date must be rejected")
}

func TestPhaseService_Create_RejectsUnknownKind(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	bad := minimalPhase(t, t.Name())
	bad.Kind = "invalid_kind"

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, bad)
	require.Error(t, err)
}

func TestPhaseService_Create_RejectsDuplicateName(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	first := minimalPhase(t, t.Name()+"-dup")
	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, first)
	require.NoError(t, err)

	second := minimalPhase(t, t.Name()+"-dup")
	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, second)
	require.Error(t, err, "UNIQUE(tenant_id, name) must reject duplicate")
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseDuplicateName))
}

func TestPhaseService_Create_RejectsUnknownFormSchema(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	missing := int64(999_999_999)
	phase.FormSchemaID = &missing

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrInvalidPhase))
}

func TestPhaseService_Update_AppliesChanges(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	created, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	created.Name = "phase-" + t.Name() + "-renamed"
	created.IsActive = false
	created.CareOverflowMode = enrollmentNative.PhaseCareOverflowReject
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created))

	refreshed, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.PhaseByID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "phase-"+t.Name()+"-renamed", refreshed.Name)
	assert.False(t, refreshed.IsActive)
	assert.Equal(t, enrollmentNative.PhaseCareOverflowReject, refreshed.CareOverflowMode)
}

func TestPhaseService_Update_ValidatesCareOfferingsOnlyWhenServiceWindowChanges(t *testing.T) {
	t.Parallel()

	baseService, repoFactory, repoFactoryCare, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	originalEnd := created.ServiceEndDate
	lockCalls := 0
	validatorCalls := 0
	guardedService := NewTestPhases(TestPhaseDependencies{
		Records:   repoFactory.Enrollment(),
		Offerings: phaseOfferings{rows: newCareOfferingFixtures(repoFactoryCare.CarePlan)},
		LockTemplateRecurrence: func(context.Context) error {
			lockCalls++
			return nil
		},
		ValidateCareOfferingPhaseChange: func(
			_ context.Context,
			phaseID int64,
			replacement *enrollmentNative.Phase,
		) error {
			validatorCalls++
			assert.Equal(t, created.ID, phaseID)
			assert.Equal(t, enrollmentNative.Date(calendar.Date(originalEnd).AddDays(7)), enrollmentNative.Date(replacement.ServiceEndDate))
			return fmt.Errorf("%w: synthetic uncovered occurrence", careplan.ErrCareOfferingConfigInvalid)
		},
		Resyncer: &recordingSourcedTemplateResyncer{},
		Logger:   slog.Default(),
	})

	created.Name += "-metadata-only"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), guardedService.UpdatePhase, created),
		"metadata-only changes must not be rejected by unrelated legacy care-offering state")
	assert.Zero(t, validatorCalls)

	created.ServiceEndDate = enrollmentNative.Date(calendar.Date(originalEnd).AddDays(7))
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), guardedService.UpdatePhase, created)
	require.ErrorIs(t, err, enrollmentNative.ErrPhaseCareOfferingConflict)
	assert.Equal(t, 2, lockCalls)
	assert.Equal(t, 1, validatorCalls)

	stored, findErr := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), repoFactory.Enrollment().Phase, created.ID)
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
	_ context.Context, offeringID int64, _ calendar.Date,
) error {
	r.offeringIDs = append(r.offeringIDs, offeringID)
	return r.err
}

func (r *recordingSourcedTemplateResyncer) DetachTemplatesSourcedFromOffering(
	_ context.Context, offeringID int64, _ calendar.Date,
) error {
	r.detachedOfferingIDs = append(r.detachedOfferingIDs, offeringID)
	return nil
}

func TestPhaseService_Update_ResyncsSourcedTemplatesOnServiceWindowChange(t *testing.T) {
	t.Parallel()

	baseService, repoFactory, repoFactoryCare, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	offering := &offeringContract.CareOfferingRecord{
		PhaseID:            created.ID,
		Name:               "offering-" + t.Name(),
		DaysOfWeekMode:     offeringContract.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		AutoAddGradeLevels: []int{},
		SelectionRule:      offeringContract.SelectionRuleOptional,
	}
	require.NoError(t, newCareOfferingFixtures(repoFactoryCare.CarePlan).Create(ctx, offering))

	resyncer := &recordingSourcedTemplateResyncer{}
	svc := NewTestPhases(TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactoryCare.CarePlan)},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		Resyncer:               resyncer,
		Logger:                 slog.Default(),
	})

	created.Name += "-metadata-only"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created))
	assert.Empty(t, resyncer.offeringIDs,
		"metadata-only updates must not resync sourced templates")

	created.ServiceEndDate = enrollmentNative.Date(calendar.Date(created.ServiceEndDate).AddDays(7))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created))
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

	baseService, repoFactory, repoFactoryCare, _, _ := setupPhaseTest(t)
	ctx := tenant.WithRollbackMarker(testpkg.Ctx(t))

	created, err := baseService.CreatePhase(ctx, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	offering := &offeringContract.CareOfferingRecord{
		PhaseID:            created.ID,
		Name:               "offering-" + t.Name(),
		DaysOfWeekMode:     offeringContract.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		AutoAddGradeLevels: []int{},
		SelectionRule:      offeringContract.SelectionRuleOptional,
	}
	require.NoError(t, newCareOfferingFixtures(repoFactoryCare.CarePlan).Create(ctx, offering))

	resyncer := &recordingSourcedTemplateResyncer{
		err: fmt.Errorf("offering roster resync: template 7: %w", timetable.ErrOfferingSourceInvalid),
	}
	svc := NewTestPhases(TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactoryCare.CarePlan)},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		Resyncer:               resyncer,
		Logger:                 slog.Default(),
	})

	created.ServiceEndDate = enrollmentNative.Date(calendar.Date(created.ServiceEndDate).AddDays(7))
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created)
	require.ErrorIs(t, err, enrollmentNative.ErrPhaseCareOfferingConflict,
		"an incompatible sourced template must reject the window change, not be skipped")
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	assert.Equal(t, []int64{offering.ID}, resyncer.offeringIDs)
	assert.True(t, tenant.RollbackRequested(ctx),
		"the rejected update must discard the already-written phase row via the ambient-transaction rollback marker")
	persisted, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), baseService.PhaseByID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.Date(calendar.Date(created.ServiceEndDate).AddDays(-7)), persisted.ServiceEndDate,
		"a standalone update must roll back the phase when sourced-template resync fails")
	resyncer.err = nil
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), svc.UpdatePhase, created))
	persisted, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), baseService.PhaseByID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ServiceEndDate, persisted.ServiceEndDate,
		"retry must commit the requested window after the resync failure is resolved")
}

func TestPhaseService_Update_RejectsDuplicateName(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	first, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()+"-first"))
	require.NoError(t, err)
	second, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()+"-second"))
	require.NoError(t, err)

	second.Name = first.Name
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, second)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseDuplicateName))
}

func TestPhaseService_Update_RejectsUnknownFormSchema(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	created, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	missing := int64(999_999_999)
	created.FormSchemaID = &missing
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrInvalidPhase))
}

func TestPhaseService_Update_MissingPhaseReturnsNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	phase.ID = 999_999_999

	err := testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound))
}

func TestPhaseService_ListPublicOpen_FiltersInactiveAndClosedWindow(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	now := time.Now()
	openStart := now.AddDate(0, 0, -7)
	openEnd := now.AddDate(0, 0, 7)
	pastEnd := now.AddDate(0, 0, -1)

	// Currently open + active.
	openActive := minimalPhase(t, t.Name()+"-open-active")
	openActive.EnrollmentOpenAt = &openStart
	openActive.EnrollmentCloseAt = &openEnd
	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, openActive)
	require.NoError(t, err)

	// Open + inactive (admin hidden).
	hidden := minimalPhase(t, t.Name()+"-open-inactive")
	hidden.EnrollmentOpenAt = &openStart
	hidden.EnrollmentCloseAt = &openEnd
	hidden.IsActive = false
	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, hidden)
	require.NoError(t, err)

	// Active but window closed yesterday.
	closed := minimalPhase(t, t.Name()+"-closed-window")
	closed.EnrollmentOpenAt = &openStart
	closed.EnrollmentCloseAt = &pastEnd
	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, closed)
	require.NoError(t, err)

	open, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.ListPublicOpen, now)
	require.NoError(t, err)
	testPhaseNames := map[string]struct{}{
		"phase-" + t.Name() + "-open-active":   {},
		"phase-" + t.Name() + "-open-inactive": {},
		"phase-" + t.Name() + "-closed-window": {},
	}
	matching := make([]*enrollmentNative.Phase, 0, len(open))
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

	svc, _, repoFactoryCare, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	phase, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	offering := &offeringContract.CareOfferingRecord{
		PhaseID:        phase.ID,
		Name:           "Linked offering",
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(repoFactoryCare.CarePlan).Create(ctx, offering))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.DeletePhase, phase.ID),
		"phase with care offerings must be deletable")

	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.PhaseByID, phase.ID)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound),
		"phase must be gone after delete")
	remaining, err := newCareOfferingFixtures(repoFactoryCare.CarePlan).CountByPhaseID(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "care offerings must cascade away with the phase")
}

// Business rule (changed): a phase with enrollment requests is now
// deletable. Requests + their children cascade away, but students that
// were created from those requests are PRESERVED (created_student_id is
// ON DELETE SET NULL; the student is the parent in that relationship).
func TestPhaseService_Delete_RemovesRequestsAndKeepsCreatedStudents(t *testing.T) {
	t.Parallel()

	svc, repoFactory, _, _, db := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	// A real student that an approved enrollment "created".
	student := testpkg.CreateTestStudent(t, db, "Kept", "Child", "1a")

	phase, err := testpkg.TenantQuery1(t, ctx, db, svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	req := &enrollmentAPI.Request{
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

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.DeletePhase, phase.ID),
		"phase with enrollment requests must be deletable")

	_, err = testpkg.TenantQuery1(t, ctx, db, svc.PhaseByID, phase.ID)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound))
	reqCount, err := testpkg.TenantQuery1(t, ctx, db, repoFactory.Enrollment().CountPhaseRequests, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, reqCount, "requests must cascade away with the phase")

	// The student must survive — deleting the request child never deletes
	// the student it points to.
	studentCount, err := db.NewSelect().
		TableExpr("users.student_profiles").
		Where("id = ?", student.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, int(studentCount),
		"student created from the phase must survive phase deletion")
}

func TestPhaseService_DeleteImpact_ReportsCounts(t *testing.T) {
	t.Parallel()

	svc, repoFactory, repoFactoryCare, _, db := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Impact", "Child", "1a")

	phase, err := testpkg.TenantQuery1(t, ctx, db, svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	offering := &offeringContract.CareOfferingRecord{
		PhaseID:        phase.ID,
		Name:           "Impact offering",
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.TenantID = testpkg.Tenant(t)
	require.NoError(t, newCareOfferingFixtures(repoFactoryCare.CarePlan).Create(ctx, offering))

	req := &enrollmentAPI.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Impact",
		GuardianLastName:  "Guardian",
		GuardianEmail:     "impact@example.com",
		StatusToken:       fmt.Sprintf("impact-token-%s-%d", t.Name(), testpkg.Tenant(t)),
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

	impact, err := testpkg.TenantQuery1(t, ctx, db, svc.DeleteImpact, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, impact.Requests, "one request references the phase")
	assert.Equal(t, 1, impact.CareOfferings, "one care offering references the phase")
	assert.Equal(t, 1, impact.StudentsKept, "one created student would be kept")
}

func TestPhaseService_Delete_RemovesEmptyPhase(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	phase, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.DeletePhase, phase.ID))

	_, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.PhaseByID, phase.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound))
}

func TestPhaseService_GetByID_NotFoundSentinel(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := setupPhaseTest(t)
	ctx := testpkg.Ctx(t)

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.PhaseByID, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound))
}

func TestPhaseService_GetByID_PreservesRepositoryFailure(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("database unavailable")
	svc := NewTestPhases(TestPhaseDependencies{
		Records: findByIDErrorPhaseRepo{err: repoErr}, Offerings: phaseOfferings{},
		LockTemplateRecurrence: func(context.Context) error { return nil }, Resyncer: &recordingSourcedTemplateResyncer{},
	})

	_, err := svc.PhaseByID(context.Background(), 123)

	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
	assert.False(t, errors.Is(err, enrollmentNative.ErrPhaseNotFound))
}

type findByIDErrorPhaseRepo struct {
	TestPhaseRecords
	err error
}

func (r findByIDErrorPhaseRepo) Phase(context.Context, int64) (*enrollmentNative.Phase, error) {
	return nil, r.err
}

// phaseServiceWithCalendarPeriods wires the optional CalendarPeriods dep
// on top of the standard setup so the link validation actually runs.
func phaseServiceWithCalendarPeriods(t *testing.T) (enrollmentNative.PhaseAdministration, *owners.EnrollmentFlowTestRepositories, *bun.DB) {
	t.Helper()
	_, repoFactory, repoFactoryCare, repoFactoryCalendar, db := setupPhaseTest(t)
	svc := NewTestPhases(TestPhaseDependencies{
		Records:                repoFactory.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(repoFactoryCare.CarePlan)},
		Calendar:               repoFactoryCalendar.SchoolCalendar(),
		LockTemplateRecurrence: func(context.Context) error { return nil },
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Logger:                 slog.Default(),
	})
	return svc, repoFactory, db
}

func TestPhaseService_Create_WithCalendarPeriodLink(t *testing.T) {
	t.Parallel()

	svc, _, db := phaseServiceWithCalendarPeriods(t)
	ctx := testpkg.Ctx(t)

	period := &calendarPeriod{
		Name:            "period-" + t.Name(),
		PeriodType:      schoolcalendar.PeriodTypeSemester,
		StartDate:       calendar.NewDate(2026, 8, 1),
		EndDate:         calendar.NewDate(2027, 1, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCalendarPeriodRow(ctx, db, period))

	phase := minimalPhase(t, t.Name())
	phase.CalendarPeriodID = &period.ID

	created, err := testpkg.TenantQuery1(t, ctx, db, svc.CreatePhase, phase)
	require.NoError(t, err)
	require.NotNil(t, created.CalendarPeriodID)
	assert.Equal(t, period.ID, *created.CalendarPeriodID)

	fetched, err := testpkg.TenantQuery1(t, ctx, db, svc.PhaseByID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.CalendarPeriodID)
	assert.Equal(t, period.ID, *fetched.CalendarPeriodID)
}

// Regression: the repo's explicit Set() list in Update() was missing
// calendar_period_id, so linking an EXISTING phase to a period silently
// persisted nothing (create worked, update didn't).
func TestPhaseService_Update_PersistsCalendarPeriodLink(t *testing.T) {
	t.Parallel()

	svc, _, db := phaseServiceWithCalendarPeriods(t)
	ctx := testpkg.Ctx(t)

	period := &calendarPeriod{
		Name:            "period-" + t.Name(),
		PeriodType:      schoolcalendar.PeriodTypeSemester,
		StartDate:       calendar.NewDate(2026, 8, 1),
		EndDate:         calendar.NewDate(2027, 1, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, createCalendarPeriodRow(ctx, db, period))

	created, err := testpkg.TenantQuery1(t, ctx, db, svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)
	require.Nil(t, created.CalendarPeriodID)

	created.CalendarPeriodID = &period.ID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.UpdatePhase, created))

	fetched, err := testpkg.TenantQuery1(t, ctx, db, svc.PhaseByID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.CalendarPeriodID, "update must persist the calendar period link")
	assert.Equal(t, period.ID, *fetched.CalendarPeriodID)

	// Unlinking must persist too (NULL round-trip).
	fetched.CalendarPeriodID = nil
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, svc.UpdatePhase, fetched))
	unlinked, err := testpkg.TenantQuery1(t, ctx, db, svc.PhaseByID, created.ID)
	require.NoError(t, err)
	assert.Nil(t, unlinked.CalendarPeriodID)
}

func TestPhaseService_Create_RejectsUnknownCalendarPeriod(t *testing.T) {
	t.Parallel()

	svc, _, _ := phaseServiceWithCalendarPeriods(t)
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	missing := int64(999_999_999)
	phase.CalendarPeriodID = &missing

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, phase)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrInvalidPhase))
}

func TestPhaseService_Update_RejectsUnknownCalendarPeriod(t *testing.T) {
	t.Parallel()

	svc, _, _ := phaseServiceWithCalendarPeriods(t)
	ctx := testpkg.Ctx(t)

	created, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, minimalPhase(t, t.Name()))
	require.NoError(t, err)

	missing := int64(999_999_999)
	created.CalendarPeriodID = &missing
	err = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), svc.UpdatePhase, created)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentNative.ErrInvalidPhase))
}

func TestPhaseService_Create_PropagatesCalendarPeriodLookupFailure(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("synthetic database failure")
	svc := NewTestPhases(TestPhaseDependencies{
		Records: unusedPhaseRecords{}, Offerings: phaseOfferings{},
		LockTemplateRecurrence: func(context.Context) error { return nil }, Resyncer: &recordingSourcedTemplateResyncer{},
		Calendar: failingCalendarPeriodService{err: lookupErr},
		Logger:   slog.Default(),
	})
	ctx := testpkg.Ctx(t)

	phase := minimalPhase(t, t.Name())
	periodID := int64(42)
	phase.CalendarPeriodID = &periodID

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.CreatePhase, phase)
	require.Error(t, err)
	assert.ErrorIs(t, err, lookupErr)
	assert.False(t, errors.Is(err, enrollmentNative.ErrInvalidPhase))
}

type failingCalendarPeriodService struct {
	err error
}

func (s failingCalendarPeriodService) FindCalendarPeriod(context.Context, int64) (schoolcalendar.CalendarPeriod, error) {
	return schoolcalendar.CalendarPeriod{}, s.err
}
