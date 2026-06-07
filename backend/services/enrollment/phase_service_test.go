package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupPhaseTest(t *testing.T) (enrollmentService.PhaseService, *repositories.Factory, *bun.DB, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	phaseNamePrefix := "phase-" + t.Name()
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewPhaseService(enrollmentService.PhaseServiceConfig{
		Repo:             repoFactory.Phase,
		RequestRepo:      repoFactory.Request,
		RequestChildRepo: repoFactory.RequestChild,
		CareOfferingRepo: repoFactory.CareOffering,
		DB:               db,
		Logger:           slog.Default(),
	})

	cleanup := func() {
		bg := context.Background()
		// Deleting requests cascades request_children + request_child_offerings.
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND name LIKE ?)", 1, phaseNamePrefix+"%").
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id IN (SELECT id FROM enrollment.phases WHERE tenant_id = ? AND name LIKE ?)", 1, phaseNamePrefix+"%").
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("tenant_id = ? AND name LIKE ?", 1, phaseNamePrefix+"%").
			Exec(bg)
		_ = db.Close()
	}
	return svc, repoFactory, db, cleanup
}

func minimalPhase(suffix string) *enrollmentModels.Phase {
	p := &enrollmentModels.Phase{
		Name:             "phase-" + suffix,
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		ServiceEndDate:   time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	p.SetTenantID(1)
	return p
}

func TestPhaseService_Create_ValidatesAndPersists(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	created, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, enrollmentModels.PhaseKindSchoolYear, created.Kind)
	assert.True(t, created.IsActive)
}

func TestPhaseService_Create_RejectsServiceDateInversion(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	bad := minimalPhase(t.Name())
	bad.ServiceStartDate = time.Date(2027, 9, 1, 0, 0, 0, 0, time.UTC)
	bad.ServiceEndDate = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	_, err := svc.Create(ctx, bad)
	require.Error(t, err, "service_end_date < service_start_date must be rejected")
}

func TestPhaseService_Create_RejectsUnknownKind(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	bad := minimalPhase(t.Name())
	bad.Kind = "invalid_kind"

	_, err := svc.Create(ctx, bad)
	require.Error(t, err)
}

func TestPhaseService_Create_RejectsDuplicateName(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	first := minimalPhase(t.Name() + "-dup")
	_, err := svc.Create(ctx, first)
	require.NoError(t, err)

	second := minimalPhase(t.Name() + "-dup")
	_, err = svc.Create(ctx, second)
	require.Error(t, err, "UNIQUE(tenant_id, name) must reject duplicate")
}

func TestPhaseService_Update_AppliesChanges(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	created, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	created.Name = "phase-" + t.Name() + "-renamed"
	created.IsActive = false
	created.CareOverflowMode = enrollmentModels.PhaseCareOverflowReject
	require.NoError(t, svc.Update(ctx, created))

	refreshed, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "phase-"+t.Name()+"-renamed", refreshed.Name)
	assert.False(t, refreshed.IsActive)
	assert.Equal(t, enrollmentModels.PhaseCareOverflowReject, refreshed.CareOverflowMode)
}

func TestPhaseService_ListPublicOpen_FiltersInactiveAndClosedWindow(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	now := time.Now()
	openStart := now.AddDate(0, 0, -7)
	openEnd := now.AddDate(0, 0, 7)
	pastEnd := now.AddDate(0, 0, -1)

	// Currently open + active.
	openActive := minimalPhase(t.Name() + "-open-active")
	openActive.EnrollmentOpenAt = &openStart
	openActive.EnrollmentCloseAt = &openEnd
	_, err := svc.Create(ctx, openActive)
	require.NoError(t, err)

	// Open + inactive (admin hidden).
	hidden := minimalPhase(t.Name() + "-open-inactive")
	hidden.EnrollmentOpenAt = &openStart
	hidden.EnrollmentCloseAt = &openEnd
	hidden.IsActive = false
	_, err = svc.Create(ctx, hidden)
	require.NoError(t, err)

	// Active but window closed yesterday.
	closed := minimalPhase(t.Name() + "-closed-window")
	closed.EnrollmentOpenAt = &openStart
	closed.EnrollmentCloseAt = &pastEnd
	_, err = svc.Create(ctx, closed)
	require.NoError(t, err)

	open, err := svc.ListPublicOpen(ctx, now)
	require.NoError(t, err)
	testPhaseNames := map[string]struct{}{
		"phase-" + t.Name() + "-open-active":   {},
		"phase-" + t.Name() + "-open-inactive": {},
		"phase-" + t.Name() + "-closed-window": {},
	}
	matching := make([]*enrollmentModels.Phase, 0, len(open))
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
	svc, repoFactory, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	phase, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Linked offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, offering))

	require.NoError(t, svc.Delete(ctx, phase.ID),
		"phase with care offerings must be deletable")

	_, err = svc.GetByID(ctx, phase.ID)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseNotFound),
		"phase must be gone after delete")
	remaining, err := repoFactory.CareOffering.CountByPhaseID(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "care offerings must cascade away with the phase")
}

// Business rule (changed): a phase with enrollment requests is now
// deletable. Requests + their children cascade away, but students that
// were created from those requests are PRESERVED (created_student_id is
// ON DELETE SET NULL; the student is the parent in that relationship).
func TestPhaseService_Delete_RemovesRequestsAndKeepsCreatedStudents(t *testing.T) {
	svc, repoFactory, db, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// A real student that an approved enrollment "created".
	student := testpkg.CreateTestStudent(t, db, "Kept", "Child", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(bg)
		testpkg.CleanupPerson(t, db, student.PersonID)
	}()

	phase, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	req := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Test",
		GuardianLastName:  "Guardian",
		GuardianEmail:     "test@example.com",
		StatusToken:       "test-token-" + t.Name(),
		SubmittedAt:       time.Now(),
	}
	req.SetTenantID(1)
	require.NoError(t, repoFactory.Request.Create(ctx, req))

	child := &enrollmentModels.RequestChild{
		RequestID:        req.ID,
		FirstName:        "Kept",
		LastName:         "Child",
		DateOfBirth:      time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC),
		CreatedStudentID: &student.ID,
	}
	child.SetTenantID(1)
	require.NoError(t, repoFactory.RequestChild.Create(ctx, child))

	require.NoError(t, svc.Delete(ctx, phase.ID),
		"phase with enrollment requests must be deletable")

	_, err = svc.GetByID(ctx, phase.ID)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseNotFound))
	reqCount, err := repoFactory.Request.CountByPhaseID(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, reqCount, "requests must cascade away with the phase")

	// The student must survive — deleting the request child never deletes
	// the student it points to.
	studentCount, err := db.NewSelect().
		TableExpr("users.students").
		Where("id = ?", student.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, studentCount,
		"student created from the phase must survive phase deletion")
}

func TestPhaseService_DeleteImpact_ReportsCounts(t *testing.T) {
	svc, repoFactory, db, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "Impact", "Child", "1a")
	defer func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(bg)
		testpkg.CleanupPerson(t, db, student.PersonID)
	}()

	phase, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Impact offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, offering))

	req := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Impact",
		GuardianLastName:  "Guardian",
		GuardianEmail:     "impact@example.com",
		StatusToken:       "impact-token-" + t.Name(),
		SubmittedAt:       time.Now(),
	}
	req.SetTenantID(1)
	require.NoError(t, repoFactory.Request.Create(ctx, req))

	child := &enrollmentModels.RequestChild{
		RequestID:        req.ID,
		FirstName:        "Impact",
		LastName:         "Child",
		DateOfBirth:      time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC),
		CreatedStudentID: &student.ID,
	}
	child.SetTenantID(1)
	require.NoError(t, repoFactory.RequestChild.Create(ctx, child))

	impact, err := svc.DeleteImpact(ctx, phase.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, impact.Requests, "one request references the phase")
	assert.Equal(t, 1, impact.CareOfferings, "one care offering references the phase")
	assert.Equal(t, 1, impact.StudentsKept, "one created student would be kept")
}

func TestPhaseService_Delete_RemovesEmptyPhase(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	phase, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)
	require.NoError(t, svc.Delete(ctx, phase.ID))

	_, err = svc.GetByID(ctx, phase.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseNotFound))
}

func TestPhaseService_GetByID_NotFoundSentinel(t *testing.T) {
	svc, _, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseNotFound))
}
