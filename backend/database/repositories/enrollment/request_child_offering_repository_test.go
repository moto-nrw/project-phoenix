package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupChildOfferingTest gives us a tenant + phase + request + child +
// care offering — the full chain needed to insert a request_child × care_offering
// row.
func setupChildOfferingTest(t *testing.T) (
	*bun.DB,
	enrollmentModels.RequestChildOfferingRepository,
	int64, // tenantID
	int64, // requestChildID
	int64, // careOfferingID
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	var tenantID int64 = 1
	testpkg.EnsureTestTenant(t, db, tenantID)

	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	phaseName := uniquePhaseName("childoffering")
	phase := makeValidPhase(phaseName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, phase)
	}))

	reqRepo := enrollmentRepo.NewRequestRepository(db)
	token := uniqueToken("childoffering")
	req := makeRequest(phase.ID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return reqRepo.Create(ctx, req)
	}))

	childRepo := enrollmentRepo.NewRequestChildRepository(db)
	child := makeChild(req.ID, "Lara", "Beispiel")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.Create(ctx, child)
	}))

	offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
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

	return db, enrollmentRepo.NewRequestChildOfferingRepository(db), tenantID, child.ID, offering.ID
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
	childRepo := enrollmentRepo.NewRequestChildRepository(db)
	child := makeChild(requestID, firstName, "Beispiel")
	child.TargetGradeLevel = grade
	child.Status = status
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.Create(ctx, child)
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
	childRepo := enrollmentRepo.NewRequestChildRepository(db)
	var requestID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		child, err := childRepo.FindByID(ctx, childID)
		if err != nil {
			return err
		}
		requestID = child.RequestID
		return nil
	}))
	return requestID
}

// --- Create + ListByRequestChildID ----------------------------------------

func TestRequestChildOfferingRepository_Create_PersistsAndReturnsID(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)

	notes := "Bitte Mo+Mi"
	row := &enrollmentModels.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		SelectedDays:   []string{"mon", "wed"},
		Notes:          &notes,
	}
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, row)
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
	assert.Equal(t, window.Start, *row.ValidFrom)
	assert.Equal(t, window.End.AddDays(1), *row.ValidUntil)
}

func TestRequestChildOfferingRepository_Create_BoundsPartialValidityWindow(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	validFrom := timezone.NewDate(2026, 10, 1)
	row := &enrollmentModels.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offeringID,
		ValidFrom:      &validFrom,
	}

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, row)
	}))

	require.NotNil(t, row.ValidUntil)
	assert.Equal(t, timezone.NewDate(2027, 8, 1), *row.ValidUntil)
}

func TestRequestChildOfferingRepository_ListByRequestChildID_ReturnsAllForChild(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)

	// Second offering so the child has two picks.
	offering2Repo := enrollmentRepo.NewCareOfferingRepository(db)
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
		row := &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: oid,
			SelectedDays:   nil,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, row)
		}))
	}

	var list []*enrollmentModels.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByRequestChildID(ctx, childID)
		return lErr
	}))
	require.Len(t, list, 2)
}

func TestRequestChildOfferingRepository_ListByRequestChildIDs_BatchLoad(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)

	// Two offering links for the one child.
	offering2Repo := enrollmentRepo.NewCareOfferingRepository(db)
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
			return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
				RequestChildID: childID,
				CareOfferingID: oid,
			})
		}))
	}

	var list []*enrollmentModels.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByRequestChildIDs(ctx, []int64{childID})
		return lErr
	}))
	require.Len(t, list, 2)
	assert.Equal(t, childID, list[0].RequestChildID)
}

func TestRequestChildOfferingRepository_ListByRequestChildIDsAtDate_ExcludesHistoricalIntervals(t *testing.T) {
	db, repo, tenantID, childID, offeringID := setupChildOfferingTest(t)
	offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
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
		if err := repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offeringID,
			ValidUntil:     &onDate,
		}); err != nil {
			return err
		}
		return repo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: currentOffering.ID,
			ValidFrom:      &onDate,
		})
	}))

	var history, active []*enrollmentModels.RequestChildOffering
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		history, err = repo.ListByRequestChildIDs(ctx, []int64{childID})
		if err != nil {
			return err
		}
		active, err = repo.ListByRequestChildIDsAtDate(ctx, []int64{childID}, onDate)
		return err
	}))
	require.Len(t, history, 2, "batch history must retain expired offering intervals")
	require.Len(t, active, 1)
	assert.Equal(t, currentOffering.ID, active[0].CareOfferingID)
}

func TestRequestChildOfferingRepository_ListByRequestChildIDs_EmptyInputShortCircuits(t *testing.T) {
	db, repo, tenantID, _, _ := setupChildOfferingTest(t)
	var list []*enrollmentModels.RequestChildOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByRequestChildIDs(ctx, nil)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestRequestChildOfferingRepository_ListByRequestChildID_EmptyResultNoError(t *testing.T) {
	db, repo, tenantID, _, _ := setupChildOfferingTest(t)
	var list []*enrollmentModels.RequestChildOffering
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByRequestChildID(ctx, 9_999_999)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- CountActiveByCareOffering -------------------------------------------

func TestRequestChildOfferingRepository_CountActiveByCareOffering_ExcludesTerminalStatuses(t *testing.T) {
	db, repo, tenantID, _, offeringID := setupChildOfferingTest(t)

	// Need three additional children with different statuses so we can
	// verify the COUNT only includes non-terminal rows. Fetch the
	// request_id from the first child via raw SQL — we just need any
	// request id that already exists for the tenant.
	var requestID int64
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return db.NewRaw(`SELECT request_id FROM enrollment.request_children WHERE tenant_id = ? LIMIT 1`, tenantID).
			Scan(ctx, &requestID)
	}))

	childRepo := enrollmentRepo.NewRequestChildRepository(db)
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
			return childRepo.Create(ctx, c)
		}))
		row := &enrollmentModels.RequestChildOffering{
			RequestChildID: c.ID,
			CareOfferingID: offeringID,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, row)
		}))
	}

	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		count, cErr = repo.CountActiveByCareOffering(ctx, offeringID)
		return cErr
	})
	require.NoError(t, err)
	// 3 active (submitted/approved/waitlisted) of the 5 we added; the
	// rejected + withdrawn rows MUST be excluded.
	assert.Equal(t, 3, count,
		"CountActiveByCareOffering must EXCLUDE rejected + withdrawn (capacity logic depends on it)")
}

func TestRequestChildOfferingRepository_CountActiveByCareOffering_ZeroWhenUnused(t *testing.T) {
	db, repo, tenantID, _, _ := setupChildOfferingTest(t)
	var count int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var cErr error
		count, cErr = repo.CountActiveByCareOffering(ctx, 9_999_999)
		return cErr
	})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
