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
)

func setupPhaseTest(t *testing.T) (enrollmentService.PhaseService, *repositories.Factory, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewPhaseService(enrollmentService.PhaseServiceConfig{
		Repo:             repoFactory.Phase,
		RequestRepo:      repoFactory.Request,
		CareOfferingRepo: repoFactory.CareOffering,
		Logger:           slog.Default(),
	})

	cleanup := func() {
		bg := context.Background()
		// Wipe in dependency order. Tests use tenant_id=1 throughout.
		_, _ = db.NewDelete().
			TableExpr("enrollment.requests").
			Where("tenant_id = ?", 1).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("tenant_id = ?", 1).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("tenant_id = ?", 1).
			Exec(bg)
	}
	return svc, repoFactory, cleanup
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
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	created, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, enrollmentModels.PhaseKindSchoolYear, created.Kind)
	assert.True(t, created.IsActive)
}

func TestPhaseService_Create_RejectsServiceDateInversion(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	bad := minimalPhase(t.Name())
	bad.ServiceStartDate = time.Date(2027, 9, 1, 0, 0, 0, 0, time.UTC)
	bad.ServiceEndDate = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	_, err := svc.Create(ctx, bad)
	require.Error(t, err, "service_end_date < service_start_date must be rejected")
}

func TestPhaseService_Create_RejectsUnknownKind(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	bad := minimalPhase(t.Name())
	bad.Kind = "invalid_kind"

	_, err := svc.Create(ctx, bad)
	require.Error(t, err)
}

func TestPhaseService_Create_RejectsDuplicateName(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	first := minimalPhase("dup-test")
	_, err := svc.Create(ctx, first)
	require.NoError(t, err)

	second := minimalPhase("dup-test")
	_, err = svc.Create(ctx, second)
	require.Error(t, err, "UNIQUE(tenant_id, name) must reject duplicate")
}

func TestPhaseService_Update_AppliesChanges(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	created, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	created.Name = "renamed phase"
	created.IsActive = false
	created.CareOverflowMode = enrollmentModels.PhaseCareOverflowReject
	require.NoError(t, svc.Update(ctx, created))

	refreshed, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed phase", refreshed.Name)
	assert.False(t, refreshed.IsActive)
	assert.Equal(t, enrollmentModels.PhaseCareOverflowReject, refreshed.CareOverflowMode)
}

func TestPhaseService_ListPublicOpen_FiltersInactiveAndClosedWindow(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	now := time.Now()
	openStart := now.AddDate(0, 0, -7)
	openEnd := now.AddDate(0, 0, 7)
	pastEnd := now.AddDate(0, 0, -1)

	// Currently open + active.
	openActive := minimalPhase("open-active")
	openActive.EnrollmentOpenAt = &openStart
	openActive.EnrollmentCloseAt = &openEnd
	_, err := svc.Create(ctx, openActive)
	require.NoError(t, err)

	// Open + inactive (admin hidden).
	hidden := minimalPhase("open-inactive")
	hidden.EnrollmentOpenAt = &openStart
	hidden.EnrollmentCloseAt = &openEnd
	hidden.IsActive = false
	_, err = svc.Create(ctx, hidden)
	require.NoError(t, err)

	// Active but window closed yesterday.
	closed := minimalPhase("closed-window")
	closed.EnrollmentOpenAt = &openStart
	closed.EnrollmentCloseAt = &pastEnd
	_, err = svc.Create(ctx, closed)
	require.NoError(t, err)

	open, err := svc.ListPublicOpen(ctx, now)
	require.NoError(t, err)
	require.Len(t, open, 1, "only the open+active phase should appear")
	assert.Equal(t, "phase-open-active", open[0].Name)
}

func TestPhaseService_Delete_RefusesWhenOfferingsReference(t *testing.T) {
	svc, repoFactory, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	phase, err := svc.Create(ctx, minimalPhase(t.Name()))
	require.NoError(t, err)

	// Anchor an offering on the phase so delete must refuse.
	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Linked offering",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.SetTenantID(1)
	require.NoError(t, repoFactory.CareOffering.Create(ctx, offering))

	err = svc.Delete(ctx, phase.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseHasReferences),
		"phase with care offerings must not be deletable; got %v", err)
}

func TestPhaseService_Delete_RemovesEmptyPhase(t *testing.T) {
	svc, _, cleanup := setupPhaseTest(t)
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
	svc, _, cleanup := setupPhaseTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrPhaseNotFound))
}
