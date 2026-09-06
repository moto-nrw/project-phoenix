package enrollment_test

import (
	"context"
	"testing"

	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupChildOfferingTest gives us a tenant + phase + request + child +
// care offering — the full chain needed to insert a request_child × care_offering
// row.
func setupChildOfferingTest(t *testing.T) (
	*bun.DB,
	*owner.Module,
	int64, // tenantID
	int64, // requestChildID
	int64, // careOfferingID
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phaseRepo := enrollmentCompose.New()
	phaseName := uniquePhaseName("childoffering")
	phase := makeOwnerEligibilityPhase(phaseName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.InsertPhase(ctx, phase)
	}))

	reqRepo := enrollmentCompose.New()
	token := uniqueToken("childoffering")
	req := makeOwnerRequest(phase.ID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return reqRepo.InsertRequest(ctx, req)
	}))

	childRepo := enrollmentCompose.New()
	child := makeChild(req.ID, "Lara", "Beispiel")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.InsertChild(ctx, child)
	}))

	offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
	offering := makeOffering(phase.ID, uniqueOfferingName("childoffering"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, offering)
	}))

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.request_child_offerings").
			Where("tenant_id = ? AND request_child_id = ?", tenantID, child.ID).
			Exec(bg)
		wipeOfferings(db, tenantID, phase.ID)
		wipeRequests(db, tenantID, token)
		wipePhases(db, tenantID, phaseName)
	})

	return db, enrollmentCompose.New(), tenantID, child.ID, offering.ID
}

// addGradedChild creates a sibling child under the same request with an
// explicit grade level and status, and registers its cleanup. Building a new
// row beats mutating the shared fixture: RequestChildRepository has no plain
// Update, and the graded/terminal variants each need their own row anyway.
func addGradedChild(
	t *testing.T,
	db *bun.DB,
	tenantID, requestID int64,
	firstName string,
	grade *int16,
	status string,
) int64 {
	t.Helper()
	childRepo := enrollmentCompose.New()
	child := makeChild(requestID, firstName, "Beispiel")
	child.TargetGradeLevel = grade
	child.Status = status
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.InsertChild(ctx, child)
	}))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().
			TableExpr("enrollment.request_child_offerings").
			Where("tenant_id = ? AND request_child_id = ?", tenantID, child.ID).
			Exec(bg)
		_, _ = db.NewDelete().
			TableExpr("enrollment.request_children").
			Where("tenant_id = ? AND id = ?", tenantID, child.ID).
			Exec(bg)
	})
	return child.ID
}

// requestIDOf resolves the request the shared fixture's child belongs to, so
// siblings land under the same request (and the same cleanup).
func requestIDOf(t *testing.T, db *bun.DB, tenantID, childID int64) int64 {
	t.Helper()
	childRepo := enrollmentCompose.New()
	var requestID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		child, err := childRepo.ChildByID(ctx, childID)
		if err != nil {
			return err
		}
		requestID = child.RequestID
		return nil
	}))
	return requestID
}

// addSiblingOffering creates another care offering in the same phase, so a
// batched query has more than one id to separate.
func addSiblingOffering(t *testing.T, db *bun.DB, tenantID, phaseID int64, prefix string) int64 {
	t.Helper()
	offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
	offering := makeOffering(phaseID, uniqueOfferingName(prefix))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, offering)
	}))
	return offering.ID
}

// phaseIDOfOffering resolves the phase an offering belongs to. Both aggregates
// are phase-scoped, and the shared fixture does not hand the phase back.
func phaseIDOfOffering(t *testing.T, db *bun.DB, tenantID, offeringID int64) int64 {
	t.Helper()
	offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
	var phaseID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		offering, err := offeringRepo.FindByID(ctx, offeringID)
		if err != nil {
			return err
		}
		phaseID = offering.PhaseID
		return nil
	}))
	return phaseID
}

// addRolloverSuccessorHolding reproduces a historical rollover reference to
// a source-phase offering. Capacity must count it even though current rollover
// clones the catalog. Returns the successor phase and child IDs.
func addRolloverSuccessorHolding(
	t *testing.T,
	db *bun.DB,
	tenantID, careOfferingID int64,
	grade *int16,
) (int64, int64) {
	t.Helper()
	phaseRepo := enrollmentCompose.New()
	requestRepo := enrollmentCompose.New()
	childRepo := enrollmentCompose.New()
	offeringRepo := enrollmentCompose.New()
	phaseName := uniquePhaseName("rolloverleak")
	token := uniqueToken("rolloverleak")

	var phaseID, childID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		successor := makeOwnerEligibilityPhase(phaseName)
		if err := phaseRepo.InsertPhase(ctx, successor); err != nil {
			return err
		}
		phaseID = successor.ID
		req := makeOwnerRequest(successor.ID, token, "rolled@example.test")
		if err := requestRepo.InsertRequest(ctx, req); err != nil {
			return err
		}
		rolled := makeChild(req.ID, "Rolled", "Kind")
		rolled.TargetGradeLevel = grade
		if err := childRepo.InsertChild(ctx, rolled); err != nil {
			return err
		}
		childID = rolled.ID
		return offeringRepo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
			RequestChildID: rolled.ID, CareOfferingID: careOfferingID,
		})
	}))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("enrollment.request_child_offerings").
			Where("tenant_id = ? AND request_child_id = ?", tenantID, childID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("enrollment.request_children").
			Where("tenant_id = ? AND id = ?", tenantID, childID).Exec(bg)
		wipeRequests(db, tenantID, token)
		wipePhases(db, tenantID, phaseName)
	})
	return phaseID, childID
}

// --- Create + ListByRequestChildID ----------------------------------------

func TestOwnerOffering_Create_PersistsAndReturnsID(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()

	notes := "Bitte Mo+Mi"
	row := &owner.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		SelectedDays:   []string{"mon", "wed"},
		Notes:          &notes,
	}
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertRequestChildOffering(ctx, row)
	})
	require.NoError(t, err)
	assert.NotZero(t, row.ID)
	assert.Equal(t, tenantID, row.TenantID)

	var window struct {
		Start timezone.Date `bun:"service_start_date"`
		End   timezone.Date `bun:"service_end_date"`
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return db.NewSelect().
			TableExpr(`enrollment.request_children AS "request_child"`).
			Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
			Join(`INNER JOIN enrollment.phases AS "phase" ON "phase".id = "request".phase_id`).
			ColumnExpr(`"phase".service_start_date`).
			ColumnExpr(`"phase".service_end_date`).
			Where(`"request_child".id = ?`, childID).
			Scan(ctx, &window)
	}))
	require.NotNil(t, row.ValidFrom)
	require.NotNil(t, row.ValidUntil)
	assert.Equal(t, owner.Date(window.Start), *row.ValidFrom)
	assert.Equal(t, owner.Date(window.End.AddDays(1)), *row.ValidUntil)
}

func TestOwnerOffering_Create_BoundsPartialValidityWindow(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()
	validFrom := timezone.NewDate(2026, 10, 1)
	row := &owner.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		ValidFrom:      offeringDatePointer(validFrom),
	}

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertRequestChildOffering(ctx, row)
	}))

	require.NotNil(t, row.ValidUntil)
	assert.Equal(t, owner.Date("2027-08-01"), *row.ValidUntil)
}

func TestOwnerOffering_ListByRequestChildID_ReturnsAllForChild(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()

	// Second offering so the child has two picks.
	offering2Repo := carePlanTest.NewCareOfferingRepository(t, db)
	o2 := makeOffering(0, uniqueOfferingName("second"))
	// We need o2's PhaseID; fetch the first offering's phase via repo.
	var first *enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		first, fbErr = offering2Repo.FindByID(ctx, offeringID)
		return fbErr
	}))
	o2.PhaseID = first.PhaseID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offering2Repo.Create(ctx, o2)
	}))

	for _, oid := range []int64{offeringID, o2.ID} {
		row := &owner.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: oid,
			SelectedDays:   nil,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertRequestChildOffering(ctx, row)
		}))
	}

	var list []*owner.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.RequestChildOfferingHistory(ctx, childID)
		return lErr
	}))
	require.Len(t, list, 2)
}

func TestOwnerOffering_ListByRequestChildIDs_BatchLoad(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()

	// Two offering links for the one child.
	offering2Repo := carePlanTest.NewCareOfferingRepository(t, db)
	var first *enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		first, fbErr = offering2Repo.FindByID(ctx, offeringID)
		return fbErr
	}))
	o2 := makeOffering(first.PhaseID, uniqueOfferingName("batch"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offering2Repo.Create(ctx, o2)
	}))
	for _, oid := range []int64{offeringID, o2.ID} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
				RequestChildID: childID,
				CareOfferingID: oid,
			})
		}))
	}

	var list []*owner.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.RequestChildOfferingHistoryForChildren(ctx, []int64{childID})
		return lErr
	}))
	require.Len(t, list, 2)
	assert.Equal(t, childID, list[0].RequestChildID)
}

func TestOwnerOffering_ListByRequestChildIDsAtDate_ExcludesHistoricalIntervals(t *testing.T) {
	t.Parallel()

	db, _, tenantID, childID, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()
	offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
	var first *enrollmentModels.CareOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		first, err = offeringRepo.FindByID(ctx, offeringID)
		return err
	}))
	currentOffering := makeOffering(first.PhaseID, uniqueOfferingName("batch-current"))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, currentOffering)
	}))
	onDate := timezone.NewDate(2026, 10, 1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidUntil:     offeringDatePointer(onDate),
		}); err != nil {
			return err
		}
		return repo.InsertRequestChildOffering(ctx, &owner.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: currentOffering.ID,
			ValidFrom:      offeringDatePointer(onDate),
		})
	}))

	var history, active []*owner.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		history, err = repo.RequestChildOfferingHistoryForChildren(ctx, []int64{childID})
		if err != nil {
			return err
		}
		active, err = repo.RequestChildOfferingsForChildrenAtDate(ctx, []int64{childID}, owner.Date(onDate))
		return err
	}))
	require.Len(t, history, 2, "batch history must retain expired offering intervals")
	require.Len(t, active, 1)
	assert.Equal(t, currentOffering.ID, active[0].CareOfferingID)
}

func TestOwnerOffering_ListByRequestChildIDs_EmptyInputShortCircuits(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()
	var list []*owner.RequestChildOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.RequestChildOfferingHistoryForChildren(ctx, nil)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestOwnerOffering_ListByRequestChildID_EmptyResultNoError(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()
	var list []*owner.RequestChildOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.RequestChildOfferingHistory(ctx, 9_999_999)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- CountActiveByCareOffering -------------------------------------------

func TestOwnerOffering_CapacityPeak_ExcludesTerminalStatuses(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, offeringID := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()

	// Need three additional children with different statuses so we can
	// verify the COUNT only includes non-terminal rows. Fetch the
	// request_id from the first child via raw SQL — we just need any
	// request id that already exists for the tenant.
	var requestID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return db.NewRaw(`SELECT request_id FROM enrollment.request_children WHERE tenant_id = ? LIMIT 1`, tenantID).
			Scan(ctx, &requestID)
	}))

	childRepo := enrollmentCompose.New()
	statuses := []string{
		enrollmentModels.ChildStatusSubmitted,
		enrollmentModels.ChildStatusApproved,
		enrollmentModels.ChildStatusWaitlisted,
		enrollmentModels.ChildStatusRejected,  // EXCLUDED
		enrollmentModels.ChildStatusWithdrawn, // EXCLUDED
	}
	for i, status := range statuses {
		c := makeChild(requestID, "Counter", "Child")
		c.Status = status
		c.SortOrder = i + 100
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return childRepo.InsertChild(ctx, c)
		}))
		row := &owner.RequestChildOffering{
			RequestChildID: c.ID,
			CareOfferingID: offeringID,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertRequestChildOffering(ctx, row)
		}))
	}

	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		count, cErr = repo.OfferingCapacityPeak(ctx, offeringID, nil, "2026-09-01", "2027-08-01")
		return cErr
	})
	require.NoError(t, err)
	// 3 active (submitted/approved/waitlisted) of the 5 we added; the
	// rejected + withdrawn rows MUST be excluded.
	assert.Equal(t, 3, count,
		"OfferingCapacityPeak must EXCLUDE rejected + withdrawn (capacity logic depends on it)")
}

func TestOwnerOffering_CapacityPeak_ZeroWhenUnused(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupChildOfferingTest(t)
	repo := enrollmentCompose.New()
	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		count, cErr = repo.OfferingCapacityPeak(ctx, 9_999_999, nil, "2026-09-01", "2027-08-01")
		return cErr
	})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
