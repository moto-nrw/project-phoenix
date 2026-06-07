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

// setupCareTest wires a real DB-backed CareOfferingService and creates a
// phase the offerings can FK against. Phase model replaced calendar
// periods as the parent entity for care offerings (migration 1.15.68).
func setupCareTest(t *testing.T) (*bun.DB, enrollmentService.CareOfferingService, *enrollmentModels.Phase, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:   repoFactory.CareOffering,
		Logger: slog.Default(),
	})

	phase := &enrollmentModels.Phase{
		Name:             uniqueSchemaName("phase-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		ServiceEndDate:   time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	phase.SetTenantID(1)
	ctx := testpkg.TenantContext(1)
	require.NoError(t, repoFactory.Phase.Create(ctx, phase))

	cleanup := func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", phase.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", phase.ID).
			Exec(bg)
		_ = db.Close()
	}

	return db, svc, phase, cleanup
}

func TestCareOfferingService_Create_AndListByPhase(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	_ = db
	ctx := testpkg.TenantContext(1)

	offering := &enrollmentModels.CareOffering{
		PhaseID:             phase.ID,
		Name:                "Regelbetreuung",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		IsActive:            true,
		SortOrder:           0,
	}
	offering.SetTenantID(1)

	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, phase.ID, created.PhaseID)

	list, err := svc.ListByPhase(ctx, phase.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Regelbetreuung", list[0].Name)
}

func TestCareOfferingService_ListActiveByPhase_FiltersInactive(t *testing.T) {
	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	active := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Aktiv",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	active.SetTenantID(1)
	_, err := svc.Create(ctx, active)
	require.NoError(t, err)

	inactive := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Inaktiv",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       false,
	}
	inactive.SetTenantID(1)
	_, err = svc.Create(ctx, inactive)
	require.NoError(t, err)

	publicList, err := svc.ListActiveByPhase(ctx, phase.ID)
	require.NoError(t, err)
	require.Len(t, publicList, 1, "ListActiveByPhase must drop is_active=false rows")
	assert.Equal(t, "Aktiv", publicList[0].Name)
}

func TestCareOfferingService_GetByID_NotFoundSentinel(t *testing.T) {
	_, svc, _, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingNotFound))
}

func TestCareOfferingService_Update_AppliesChanges(t *testing.T) {
	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Original",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.SetTenantID(1)
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	created.Name = "Updated"
	created.IsActive = false
	require.NoError(t, svc.Update(ctx, created))

	refreshed, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", refreshed.Name)
	assert.False(t, refreshed.IsActive)
}

func TestCareOfferingService_Clone_RepointsToTargetPhase(t *testing.T) {
	db, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	source := &enrollmentModels.CareOffering{
		PhaseID:             phase.ID,
		Name:                "Original",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		IsActive:            true,
	}
	source.SetTenantID(1)
	created, err := svc.Create(ctx, source)
	require.NoError(t, err)

	// Build a second phase as the clone target.
	repoFactory := repositories.NewFactory(db)
	target := &enrollmentModels.Phase{
		Name:             uniqueSchemaName("phase-clone-target-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: time.Date(2027, 9, 1, 0, 0, 0, 0, time.UTC),
		ServiceEndDate:   time.Date(2028, 7, 31, 0, 0, 0, 0, time.UTC),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	target.SetTenantID(1)
	require.NoError(t, repoFactory.Phase.Create(ctx, target))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("phase_id = ?", target.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", target.ID).
			Exec(bg)
	})

	clone, err := svc.Clone(ctx, created.ID, target.ID)
	require.NoError(t, err)
	require.NotZero(t, clone.ID)
	assert.NotEqual(t, created.ID, clone.ID, "clone must get a fresh BIGSERIAL id")
	assert.Equal(t, target.ID, clone.PhaseID, "clone must point at the target phase")
	assert.Equal(t, "Original", clone.Name)
	assert.Equal(t, []string{"mon", "tue"}, clone.AvailableDays)
}

func TestCareOfferingService_Delete_RemovesRow(t *testing.T) {
	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	offering := &enrollmentModels.CareOffering{
		PhaseID:        phase.ID,
		Name:           "Soon-deleted",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	}
	offering.SetTenantID(1)
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, created.ID))

	_, err = svc.GetByID(ctx, created.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingNotFound))
}

func TestCareOfferingService_RejectsMixedRuleInSameGroup(t *testing.T) {
	_, svc, phase, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	groupOffering := func(name, rule string) *enrollmentModels.CareOffering {
		o := &enrollmentModels.CareOffering{
			PhaseID:        phase.ID,
			Name:           name,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"mon"},
			IsActive:       true,
			SelectionGroup: "tag",
			SelectionRule:  rule,
		}
		o.SetTenantID(1)
		return o
	}

	first, err := svc.Create(ctx, groupOffering("A", enrollmentModels.SelectionRuleExactlyOne))
	require.NoError(t, err)

	// A non-optional rule differing from the group's existing rule is rejected.
	_, err = svc.Create(ctx, groupOffering("B", enrollmentModels.SelectionRuleAtLeastOne))
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))

	// An optional (or empty-rule) sibling in a non-optional group is ALSO
	// rejected — the submit path counts all members, so the group must be
	// homogeneous.
	_, err = svc.Create(ctx, groupOffering("C", enrollmentModels.SelectionRuleOptional))
	require.Error(t, err, "optional sibling in an exactly_one group must be rejected")
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))

	// A matching rule is accepted.
	_, err = svc.Create(ctx, groupOffering("D", enrollmentModels.SelectionRuleExactlyOne))
	require.NoError(t, err)

	// Updating the first offering to a different rule is likewise rejected
	// while siblings still hold the old rule.
	first.SelectionRule = enrollmentModels.SelectionRuleAtMostOne
	err = svc.Update(ctx, first)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingGroupRuleConflict))
}
