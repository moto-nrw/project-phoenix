package enrollment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// uniquePhaseName produces a per-test phase name so the (tenant_id,
// name) unique constraint doesn't collide when tests run together.
func uniquePhaseName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, testpkg.UniqueSuffix())
}

// wipePhases removes every phase row for the tenant. Each test calls
// this from a defer so we don't bleed rows between tests.
func wipePhases(db *bun.DB, tenantID int64, names ...string) {
	bg := context.Background()
	if len(names) == 0 {
		// No-op when no names — protects against accidental tenant-wide
		// wipes in shared-tenant tests.
		return
	}
	_, _ = db.NewDelete().
		TableExpr("enrollment.phases").
		Where("tenant_id = ? AND name IN (?)", tenantID, bun.List(names)).
		Exec(bg)
}

func setupOwnerPhaseTest(t *testing.T) (*bun.DB, *capability.Module, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return db, enrollmentCompose.New(), tenantID
}

// --- Create + Validation -----------------------------------------------

func TestPhaseRepository_Create_PersistsAndReturnsID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("create")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	})
	require.NoError(t, err)
	assert.NotZero(t, phase.ID)
	assert.Equal(t, tenantID, phase.TenantID, "EnsureTenantID must stamp tenant_id from ctx")
}

func TestPhaseRepository_Create_RejectsInvalidPhase(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)

	phase := makeOwnerEligibilityPhase("")
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phase validation")
}

// --- FindByID ----------------------------------------------------------

func TestPhaseRepository_FindByID_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("findbyid")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	var got *capability.Phase
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Phase(ctx, phase.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, phase.ID, got.ID)
	assert.Equal(t, name, got.Name)
}

func TestPhaseRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)

	var got *capability.Phase
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Phase(ctx, 9_999_999)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "not found")
}

// --- Update ------------------------------------------------------------

func TestPhaseRepository_Update_PersistsChanges(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("update")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	// Change name + service end + active.
	newName := uniquePhaseName("updated")
	defer wipePhases(db, tenantID, newName)
	phase.Name = newName
	phase.ServiceEndDate = capability.Date(timezone.NewDate(2027, 8, 15))
	phase.IsActive = false

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.UpdatePhase(ctx, phase)
	}))

	var got *capability.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Phase(ctx, phase.ID)
		return fbErr
	}))
	assert.Equal(t, newName, got.Name)
	assert.False(t, got.IsActive)
	assert.Equal(t, capability.Date(timezone.NewDate(2027, 8, 15)), got.ServiceEndDate)
}

func TestPhaseRepository_Update_RejectsZeroID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)

	phase := makeOwnerEligibilityPhase(uniquePhaseName("noid"))
	// Don't set ID.
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.UpdatePhase(ctx, phase)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestPhaseRepository_Update_MissingRowErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)

	phase := makeOwnerEligibilityPhase(uniquePhaseName("nope"))
	phase.ID = 9_999_999
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.UpdatePhase(ctx, phase)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPhaseRepository_Update_RejectsInvalidPhase(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("invalidupdate")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	// End-before-start is rejected by Validate.
	phase.ServiceStartDate = capability.Date(timezone.NewDate(2030, 1, 1))
	phase.ServiceEndDate = capability.Date(timezone.NewDate(2029, 12, 31))
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.UpdatePhase(ctx, phase)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phase validation")
}

// --- Delete ------------------------------------------------------------

func TestPhaseRepository_Delete_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("delete")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().DeletePhase(ctx, phase.ID)
	}))

	var got *capability.Phase
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Phase(ctx, phase.ID)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestPhaseRepository_Delete_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupOwnerPhaseTest(t)

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().DeletePhase(ctx, 9_999_999)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- ListByTenant ------------------------------------------------------

func TestPhaseRepository_ListByTenant_OrdersByServiceStartDesc(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	a := uniquePhaseName("listA")
	b := uniquePhaseName("listB")
	c := uniquePhaseName("listC")
	defer wipePhases(db, tenantID, a, b, c)

	phaseA := makeOwnerEligibilityPhase(a)
	phaseA.ServiceStartDate = capability.Date(timezone.NewDate(2025, 9, 1))
	phaseA.ServiceEndDate = capability.Date(timezone.NewDate(2026, 7, 31))

	phaseB := makeOwnerEligibilityPhase(b)
	phaseB.ServiceStartDate = capability.Date(timezone.NewDate(2026, 9, 1))
	phaseB.ServiceEndDate = capability.Date(timezone.NewDate(2027, 7, 31))

	phaseC := makeOwnerEligibilityPhase(c)
	phaseC.ServiceStartDate = capability.Date(timezone.NewDate(2027, 9, 1))
	phaseC.ServiceEndDate = capability.Date(timezone.NewDate(2028, 7, 31))

	for _, p := range []*capability.Phase{phaseA, phaseB, phaseC} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertPhase(ctx, p)
		}))
	}

	var list []*capability.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().Phases(ctx)
		return lErr
	}))

	// Filter to ours then verify DESC order.
	var ours []*capability.Phase
	for _, p := range list {
		if p.Name == a || p.Name == b || p.Name == c {
			ours = append(ours, p)
		}
	}
	require.Len(t, ours, 3)
	assert.Equal(t, c, ours[0].Name, "newest service_start_date first")
	assert.Equal(t, b, ours[1].Name)
	assert.Equal(t, a, ours[2].Name)
}

// --- ListPublicOpen ----------------------------------------------------

func TestPhaseRepository_ListPublicOpen_OnlyReturnsActiveInWindow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	openInactive := uniquePhaseName("openInactive")
	closedActive := uniquePhaseName("closedActive")
	openActive := uniquePhaseName("openActive")
	defer wipePhases(db, tenantID, openInactive, closedActive, openActive)

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	openAt := now.Add(-24 * time.Hour)
	closeAt := now.Add(24 * time.Hour)
	pastCloseAt := now.Add(-1 * time.Hour)
	futureOpenAt := now.Add(48 * time.Hour)

	// (1) Open window, but is_active=false → excluded.
	pInactive := makeOwnerEligibilityPhase(openInactive)
	pInactive.IsActive = false
	pInactive.EnrollmentOpenAt = &openAt
	pInactive.EnrollmentCloseAt = &closeAt

	// (2) Active but window already closed → excluded.
	pClosed := makeOwnerEligibilityPhase(closedActive)
	pClosed.EnrollmentOpenAt = &openAt
	pClosed.EnrollmentCloseAt = &pastCloseAt

	// (3) Active, window not yet open → excluded.
	farFutureCloseAt := now.Add(72 * time.Hour)
	pFuture := makeOwnerEligibilityPhase(uniquePhaseName("futureActive"))
	defer wipePhases(db, tenantID, pFuture.Name)
	pFuture.EnrollmentOpenAt = &futureOpenAt
	pFuture.EnrollmentCloseAt = &farFutureCloseAt

	// (4) Happy path — open + active + within window → included.
	pOpen := makeOwnerEligibilityPhase(openActive)
	pOpen.EnrollmentOpenAt = &openAt
	pOpen.EnrollmentCloseAt = &closeAt

	for _, p := range []*capability.Phase{pInactive, pClosed, pFuture, pOpen} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertPhase(ctx, p)
		}))
	}

	var list []*capability.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().PublicOpenPhases(ctx, now)
		return lErr
	}))

	ours := make(map[string]bool)
	for _, p := range list {
		ours[p.Name] = true
	}
	assert.True(t, ours[openActive], "open + active + in-window phase MUST appear")
	assert.False(t, ours[openInactive], "is_active=false phase MUST NOT appear")
	assert.False(t, ours[closedActive], "past-close phase MUST NOT appear")
	assert.False(t, ours[pFuture.Name], "future-open phase MUST NOT appear")
}

func TestPhaseRepository_ListPublicOpen_NullBoundsTreatedAsUnbounded(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	name := uniquePhaseName("nullBounds")
	defer wipePhases(db, tenantID, name)

	phase := makeOwnerEligibilityPhase(name)
	// EnrollmentOpenAt and CloseAt left nil → permanently open.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	now := time.Now()
	var list []*capability.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().PublicOpenPhases(ctx, now)
		return lErr
	}))

	found := false
	for _, p := range list {
		if p.Name == name {
			found = true
		}
	}
	assert.True(t, found, "NULL open/close bounds must be treated as unbounded (always open)")
}

// --- ListWithExpiredRolloverDeadline -----------------------------------

func TestPhaseRepository_ListWithExpiredRolloverDeadline_OnlyReturnsExpired(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	noDeadline := uniquePhaseName("noDeadline")
	pending := uniquePhaseName("pending")
	expired := uniquePhaseName("expired")
	defer wipePhases(db, tenantID, noDeadline, pending, expired)

	asOf := time.Date(2027, 7, 15, 0, 0, 0, 0, time.UTC)
	pastDeadline := asOf.Add(-1 * time.Hour)
	futureDeadline := asOf.Add(24 * time.Hour)

	// (1) No rollover deadline → excluded.
	pNo := makeOwnerEligibilityPhase(noDeadline)
	// (2) Deadline in the future → excluded.
	pPending := makeOwnerEligibilityPhase(pending)
	pPending.RolloverDeadline = &futureDeadline
	// (3) Deadline in the past → included.
	pExp := makeOwnerEligibilityPhase(expired)
	pExp.RolloverDeadline = &pastDeadline

	for _, p := range []*capability.Phase{pNo, pPending, pExp} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertPhase(ctx, p)
		}))
	}

	var list []*capability.Phase
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().PhasesWithExpiredRolloverDeadline(ctx, asOf)
		return lErr
	}))

	seen := map[string]bool{}
	for _, p := range list {
		seen[p.Name] = true
	}
	assert.True(t, seen[expired], "expired-deadline phase must appear")
	assert.False(t, seen[pending], "future-deadline phase MUST NOT appear")
	assert.False(t, seen[noDeadline], "no-deadline phase MUST NOT appear")
}

// --- ExistsByFormSchemaID ----------------------------------------------

func TestPhaseRepository_ExistsByFormSchemaID_TrueWhenReferenced(t *testing.T) {
	t.Parallel()

	// We need a real form_schema row first so the FK constraint on
	// enrollment.phases.form_schema_id passes.
	db, repo, tenantID := setupOwnerPhaseTest(t)
	account := testpkg.CreateTestAccount(t, db, "phase-exists")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	schemaRepo := enrollmentCompose.New()
	schema := &capability.FormSchema{
		Name:      uniqueSchemaName("phase-exists"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: account.ID,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return schemaRepo.InsertSchemaVersion(ctx, schema)
	}))
	t.Cleanup(func() { wipeSchemas(db, tenantID, account.ID) })

	name := uniquePhaseName("schemaRef")
	defer wipePhases(db, tenantID, name)
	phase := makeOwnerEligibilityPhase(name)
	phase.FormSchemaID = &schema.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, phase)
	}))

	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		count, eErr = enrollmentCompose.New().CountPhaseSchemaReferences(ctx, []int64{schema.ID})
		return eErr
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "ExistsByFormSchemaID must be true when a phase still references the schema")
}

func TestPhaseRepository_ExistsByFormSchemaID_FalseWhenUnreferenced(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupOwnerPhaseTest(t)

	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		count, eErr = enrollmentCompose.New().CountPhaseSchemaReferences(ctx, []int64{9_999_999})
		return eErr
	})
	require.NoError(t, err)
	assert.Zero(t, count, "ExistsByFormSchemaID must be false for an unreferenced schema id")
}

// --- ExistsByRolloverSourcePhaseID -------------------------------------

func TestPhaseRepository_ExistsByRolloverSourcePhaseID_TrueAfterRollover(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupOwnerPhaseTest(t)
	sourceName := uniquePhaseName("rolloverSource")
	rolloverName := uniquePhaseName("rolloverTarget")
	defer wipePhases(db, tenantID, sourceName, rolloverName)

	source := makeOwnerEligibilityPhase(sourceName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, source)
	}))

	mode := enrollmentModels.PhaseRolloverModeOptOut
	rollover := makeOwnerEligibilityPhase(rolloverName)
	rollover.ServiceStartDate = capability.Date(timezone.NewDate(2027, 9, 1))
	rollover.ServiceEndDate = capability.Date(timezone.NewDate(2028, 7, 31))
	rollover.RolloverSourcePhaseID = &source.ID
	rollover.RolloverMode = &mode
	rolloverDeadline := time.Date(2027, 7, 1, 0, 0, 0, 0, time.UTC)
	rollover.RolloverDeadline = &rolloverDeadline
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertPhase(ctx, rollover)
	}))

	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = enrollmentCompose.New().HasRolloverSuccessor(ctx, source.ID)
		return eErr
	})
	require.NoError(t, err)
	assert.True(t, exists, "rollover destination existing means the source has been rolled")
}

func TestPhaseRepository_ExistsByRolloverSourcePhaseID_FalseWhenUnreferenced(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupOwnerPhaseTest(t)

	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = enrollmentCompose.New().HasRolloverSuccessor(ctx, 9_999_999)
		return eErr
	})
	require.NoError(t, err)
	assert.False(t, exists, "unreferenced source phase ID returns false")
}
