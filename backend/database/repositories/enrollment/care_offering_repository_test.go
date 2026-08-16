package enrollment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setupCareOfferingRepoTest(t *testing.T) (
	*bun.DB,
	enrollmentModels.CareOfferingRepository,
	int64,
	int64,
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	var tenantID int64 = 1
	testpkg.EnsureTestTenant(t, db, tenantID)

	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	phaseName := uniquePhaseName("offering")
	phase := makeValidPhase(phaseName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, phase)
	}))
	t.Cleanup(func() { wipePhases(db, tenantID, phaseName) })

	return db, enrollmentRepo.NewCareOfferingRepository(db), tenantID, phase.ID
}

func wipeOfferings(db *bun.DB, tenantID, phaseID int64) {
	_, _ = db.NewDelete().
		TableExpr("enrollment.care_offerings").
		Where("tenant_id = ? AND phase_id = ?", tenantID, phaseID).
		Exec(context.Background())
}

func makeOffering(phaseID int64, name string) *enrollmentModels.CareOffering {
	return &enrollmentModels.CareOffering{
		PhaseID:        phaseID,
		Name:           name,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
	}
}

func uniqueOfferingName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// --- Create + Validation ----------------------------------------------

func TestCareOfferingRepository_Create_PersistsAndReturnsID(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("create"))
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	})
	require.NoError(t, err)
	assert.NotZero(t, offering.ID)
	assert.Equal(t, tenantID, offering.TenantID)
}

func TestCareOfferingRepository_Create_PreservesExplicitCountsAsCareFalse(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("nonstat"))
	offering.CountsAsCare = false
	offering.CountsAsCareSet = true

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	}))

	var got *enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var findErr error
		got, findErr = repo.FindByID(ctx, offering.ID)
		return findErr
	}))
	require.NotNil(t, got)
	assert.False(t, got.CountsAsCare, "explicit counts_as_care=false must survive create")
}

func TestCareOfferingRepository_Create_RejectsInvalidOffering(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	offering := makeOffering(phaseID, "") // blank name → Validate fails
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

// --- FindByID ---------------------------------------------------------

func TestCareOfferingRepository_FindByID_HappyPath(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("find"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	}))

	var got *enrollmentModels.CareOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, offering.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, offering.ID, got.ID)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, got.AvailableDays)
}

func TestCareOfferingRepository_FindByID_NotFound(t *testing.T) {
	db, repo, tenantID, _ := setupCareOfferingRepoTest(t)
	var got *enrollmentModels.CareOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, 9_999_999)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
}

// --- Update ----------------------------------------------------------

func TestCareOfferingRepository_Update_PersistsEveryEditableColumn(t *testing.T) {
	// The repo explicitly sets every column on purpose — chaining one
	// .Set() onto a Model() update used to drop the rest silently. This
	// test pins that behavior by changing several fields and checking
	// they all round-trip.
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("update"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	}))

	newName := uniqueOfferingName("updated")
	desc := "Spätbetreuung mit Hausaufgabenhilfe"
	cap := 30
	price := 12000
	offering.Name = newName
	offering.Description = &desc
	offering.DaysOfWeekMode = enrollmentModels.DaysOfWeekModeParentChoice
	offering.AvailableDays = []string{"mon", "wed", "fri"}
	offering.IncludesHolidayCare = true
	offering.IncludesLunch = true
	offering.Capacity = &cap
	offering.PriceCents = &price
	offering.IsActive = false
	offering.SortOrder = 5
	offering.PickupTimes = map[string]string{"mon": "14:30", "fri": "16:00"}

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Update(ctx, offering)
	}))

	var got *enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, offering.ID)
		return fbErr
	}))
	assert.Equal(t, newName, got.Name)
	require.NotNil(t, got.Description)
	assert.Equal(t, desc, *got.Description)
	assert.Equal(t, enrollmentModels.DaysOfWeekModeParentChoice, got.DaysOfWeekMode)
	assert.Equal(t, []string{"mon", "wed", "fri"}, got.AvailableDays)
	assert.True(t, got.IncludesHolidayCare)
	assert.True(t, got.IncludesLunch)
	require.NotNil(t, got.Capacity)
	assert.Equal(t, 30, *got.Capacity)
	require.NotNil(t, got.PriceCents)
	assert.Equal(t, 12000, *got.PriceCents)
	assert.False(t, got.IsActive)
	assert.Equal(t, 5, got.SortOrder)
	assert.Equal(t, map[string]string{"mon": "14:30", "fri": "16:00"}, got.PickupTimes,
		"pickup_times must survive the explicit column-list update (#2290)")
}

func TestCareOfferingRepository_Update_RejectsInvalidOffering(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("invalidupdate"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	}))

	offering.Name = ""
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Update(ctx, offering)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestCareOfferingRepository_Update_MissingIDErrors(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	offering := makeOffering(phaseID, uniqueOfferingName("noid"))
	offering.ID = 9_999_999
	offering.TenantID = tenantID
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Update(ctx, offering)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Delete ----------------------------------------------------------

func TestCareOfferingRepository_Delete_HappyPath(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	offering := makeOffering(phaseID, uniqueOfferingName("delete"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, offering)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Delete(ctx, offering.ID)
	}))

	var got *enrollmentModels.CareOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, offering.ID)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestCareOfferingRepository_Delete_MissingIDErrors(t *testing.T) {
	db, repo, tenantID, _ := setupCareOfferingRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Delete(ctx, 9_999_999)
	})
	require.Error(t, err)
}

// --- ListByPhase / ListActiveByPhase --------------------------------

func TestCareOfferingRepository_ListByPhase_OrdersBySortOrder(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	// Out-of-order sort_order values so the test really exercises the
	// ORDER BY rather than insertion order.
	o1 := makeOffering(phaseID, uniqueOfferingName("listB"))
	o1.SortOrder = 2
	o2 := makeOffering(phaseID, uniqueOfferingName("listA"))
	o2.SortOrder = 0
	o3 := makeOffering(phaseID, uniqueOfferingName("listC"))
	o3.SortOrder = 1
	for _, o := range []*enrollmentModels.CareOffering{o1, o2, o3} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, o)
		}))
	}

	var list []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByPhase(ctx, phaseID)
		return lErr
	}))
	require.Len(t, list, 3)
	assert.Equal(t, o2.ID, list[0].ID)
	assert.Equal(t, o3.ID, list[1].ID)
	assert.Equal(t, o1.ID, list[2].ID)
}

func TestCareOfferingRepository_ListByPhase_ScopesToPhase(t *testing.T) {
	db, repo, tenantID, phaseA := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseA)

	// Second phase so we can prove the filter.
	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	other := makeValidPhase(uniquePhaseName("otherphase"))
	other.ServiceStartDate = timezone.NewDate(2027, 9, 1)
	other.ServiceEndDate = timezone.NewDate(2028, 7, 31)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, other)
	}))
	t.Cleanup(func() {
		wipeOfferings(db, tenantID, other.ID)
		wipePhases(db, tenantID, other.Name)
	})

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, makeOffering(phaseA, uniqueOfferingName("scopedA")))
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, makeOffering(other.ID, uniqueOfferingName("scopedB")))
	}))

	var list []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByPhase(ctx, phaseA)
		return lErr
	}))
	for _, o := range list {
		assert.Equal(t, phaseA, o.PhaseID,
			"ListByPhase MUST only return rows for the requested phase (saw offering %d phase %d)",
			o.ID, o.PhaseID)
	}
}

func TestCareOfferingRepository_ListByIDs_LoadsExactIDsAcrossPhases(t *testing.T) {
	db, repo, tenantID, phaseA := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseA)

	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	other := makeValidPhase(uniquePhaseName("ids-otherphase"))
	other.ServiceStartDate = timezone.NewDate(2027, 9, 1)
	other.ServiceEndDate = timezone.NewDate(2028, 7, 31)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, other)
	}))
	t.Cleanup(func() {
		wipeOfferings(db, tenantID, other.ID)
		wipePhases(db, tenantID, other.Name)
	})

	first := makeOffering(phaseA, uniqueOfferingName("idsA"))
	second := makeOffering(other.ID, uniqueOfferingName("idsB"))
	unrequested := makeOffering(phaseA, uniqueOfferingName("idsSkip"))
	for _, offering := range []*enrollmentModels.CareOffering{first, second, unrequested} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, offering)
		}))
	}

	var empty []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		empty, lErr = repo.ListByIDs(ctx, nil)
		return lErr
	}))
	assert.Empty(t, empty)

	var list []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByIDs(ctx, []int64{first.ID, second.ID})
		return lErr
	}))
	require.Len(t, list, 2)
	gotIDs := make(map[int64]bool, len(list))
	for _, offering := range list {
		gotIDs[offering.ID] = true
	}
	assert.True(t, gotIDs[first.ID])
	assert.True(t, gotIDs[second.ID])
	assert.False(t, gotIDs[unrequested.ID])
}

func TestCareOfferingRepository_ListActiveByPhase_FiltersInactive(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	active := makeOffering(phaseID, uniqueOfferingName("active"))
	inactive := makeOffering(phaseID, uniqueOfferingName("inactive"))
	inactive.IsActive = false
	for _, o := range []*enrollmentModels.CareOffering{active, inactive} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, o)
		}))
	}

	var list []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListActiveByPhase(ctx, phaseID)
		return lErr
	}))
	seenActive, seenInactive := false, false
	for _, o := range list {
		if o.ID == active.ID {
			seenActive = true
		}
		if o.ID == inactive.ID {
			seenInactive = true
		}
	}
	assert.True(t, seenActive, "active offering must appear")
	assert.False(t, seenInactive, "is_active=false offering MUST NOT appear")
}

// --- ListByTenant ----------------------------------------------------

func TestCareOfferingRepository_ListByTenant_ReturnsAllForTenant(t *testing.T) {
	db, repo, tenantID, phaseID := setupCareOfferingRepoTest(t)
	defer wipeOfferings(db, tenantID, phaseID)

	for i := 0; i < 3; i++ {
		o := makeOffering(phaseID, uniqueOfferingName(fmt.Sprintf("tenantlist-%d", i)))
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, o)
		}))
	}

	var list []*enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByTenant(ctx)
		return lErr
	}))
	count := 0
	for _, o := range list {
		if o.PhaseID == phaseID {
			count++
		}
	}
	assert.GreaterOrEqual(t, count, 3, "ListByTenant returns every offering for the tenant in context")
}
