package migrations

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sqlStateError mimics a driver error with a SQLSTATE so tests can inject
// deadlocks and serialization failures at the commit seam.
type sqlStateError string

func (e sqlStateError) Error() string     { return "injected SQLSTATE " + string(e) }
func (e sqlStateError) Field(byte) string { return string(e) }

type legacyOffering struct {
	child, offering             int64
	selected, manual, automatic string // JSON array literals or "" for NULL
	notes                       *string
	validFrom, validUntil       *string
	createdAt, updatedAt        time.Time
}

func insertLegacyOffering(t *testing.T, db *testpkg.DB, tenantID int64, row legacyOffering) int64 {
	t.Helper()
	nullable := func(json string) *string {
		if json == "" {
			return nil
		}
		return &json
	}
	if row.createdAt.IsZero() {
		row.createdAt = time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	}
	if row.updatedAt.IsZero() {
		row.updatedAt = row.createdAt
	}
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		(tenant_id, request_child_id, care_offering_id, selected_days, manual_selected_days, automatic_selected_days,
		 notes, valid_from, valid_until, created_at, updated_at)
		VALUES (?, ?, ?, ?::jsonb, ?::jsonb, ?::jsonb, ?, ?::date, ?::date, ?, ?) RETURNING id`,
		tenantID, row.child, row.offering, nullable(row.selected), nullable(row.manual), nullable(row.automatic),
		row.notes, row.validFrom, row.validUntil, row.createdAt, row.updatedAt).Scan(t.Context(), &id))
	return id
}

type requestChildStorageFixture struct {
	tenant    int64
	children  []int64
	offerings []int64
	legacyIDs []int64
}

// createRequestChildStorageFixture builds two children and two offerings with five
// legacy rows: one pair carries three consecutive intervals (origin plus two
// approved changes), the others are single rows with NULL days and notes.
// Every row keeps the write-path invariant selected = manual ∪ automatic.
func createRequestChildStorageFixture(t *testing.T, db *testpkg.DB) requestChildStorageFixture {
	t.Helper()
	tenant := testpkg.Tenant(t)
	phaseID, requestID, firstChild := testpkg.CreateAuditAdjustmentChain(t, db)
	secondChild := testpkg.CreateAuditAdjustmentChild(t, db, requestID)
	first := testpkg.CreateTestCareOffering(t, db, phaseID, "Backfill Montag")
	second := testpkg.CreateTestCareOffering(t, db, phaseID, "Backfill Freitag")
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	rows := []legacyOffering{
		{child: firstChild, offering: first.ID, selected: `["mon","tue"]`, manual: `["mon"]`, automatic: `["tue"]`,
			notes: new("submitted"), validFrom: new("2026-08-01"), validUntil: new("2026-10-01"), createdAt: base},
		{child: firstChild, offering: first.ID, selected: `["mon","tue","wed"]`, manual: `["mon","wed"]`, automatic: `["tue"]`,
			notes: new("approved change"), validFrom: new("2026-10-01"), validUntil: new("2027-01-01"), createdAt: base.Add(time.Hour)},
		{child: firstChild, offering: first.ID, selected: `["wed"]`, automatic: `["wed"]`,
			validFrom: new("2027-01-01"), createdAt: base.Add(2 * time.Hour), updatedAt: base.Add(3 * time.Hour)},
		{child: firstChild, offering: second.ID, validFrom: new("2026-08-01"), validUntil: new("2027-08-01"), createdAt: base},
		{child: secondChild, offering: second.ID, selected: `[]`, notes: new(""), createdAt: base.Add(time.Minute)},
	}
	fixture := requestChildStorageFixture{tenant: tenant, children: []int64{firstChild, secondChild}, offerings: []int64{first.ID, second.ID}}
	for _, row := range rows {
		fixture.legacyIDs = append(fixture.legacyIDs, insertLegacyOffering(t, db, tenant, row))
	}
	return fixture
}

func runBackfill(t *testing.T, db *testpkg.DB, options RequestChildStorageBackfillOptions) RequestChildStorageTenantReport {
	t.Helper()
	options.TenantIDs = []int64{testpkg.Tenant(t)}
	report, err := RunRequestChildStorageBackfill(t.Context(), db, options)
	require.NoError(t, err)
	require.Len(t, report.Tenants, 1)
	return report.Tenants[0]
}

func countRows(t *testing.T, db *testpkg.DB, table string, tenantID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s WHERE tenant_id = ?`, table), tenantID).Scan(t.Context(), &count))
	return count
}

func checkpointRow(t *testing.T, db *testpkg.DB, tenantID int64) map[string]any {
	t.Helper()
	var row map[string]any
	require.NoError(t, db.NewRaw(`SELECT * FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id = ?`, tenantID).Scan(t.Context(), &row))
	return row
}

func requireVerified(t *testing.T, db *testpkg.DB, tenantID int64) RequestChildStorageVerification {
	t.Helper()
	verification, err := verifyRequestChildStorageTenant(t.Context(), db, tenantID, nil)
	require.NoError(t, err)
	require.Equal(t, verification.SourceBookings, verification.TargetBookings)
	require.Equal(t, verification.SourceBookingsChecksum, verification.TargetBookingsChecksum)
	require.Zero(t, verification.Mismatches(), "bookings, planned days and Anmeldung selections must match")
	require.Equal(t, verification.SourceSelections, verification.TargetSelections)
	require.Equal(t, verification.SourceSelectionsChecksum, verification.TargetSelectionsChecksum)
	require.True(t, verification.Equal())
	require.Equal(t, countRows(t, db, "enrollment.request_child_offerings", tenantID), int(verification.TargetBookings))
	return verification
}

type selectionRow struct {
	RequestChildID, CareOfferingID int64
	SelectedDays, Notes            *string
	CreatedAt                      time.Time
}

func selectionRows(t *testing.T, db *testpkg.DB, tenantID int64) []selectionRow {
	t.Helper()
	var rows []selectionRow
	require.NoError(t, db.NewRaw(`SELECT request_child_id, care_offering_id, selected_days::text AS selected_days, notes, created_at
		FROM enrollment.request_child_offering_selections WHERE tenant_id = ? ORDER BY request_child_id, care_offering_id`, tenantID).Scan(t.Context(), &rows))
	return rows
}

func TestRequestChildStorageBackfillCompletesWithLegacyAnmeldung(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, report.Complete)
	require.True(t, report.Verification.Equal())
	require.EqualValues(t, 5, report.Verification.TargetBookings)
	require.EqualValues(t, 3, report.Verification.TargetSelections, "one Anmeldung selection per pair")
	require.Zero(t, report.Verification.DayDifferences)
	require.NoError(t, requestChildStorageBackfillUp(t.Context(), db))
	requireVerified(t, db, fixture.tenant)
}

// Pre-breakdown legacy rows carry their days only in selected_days. The
// booking must plan those days, or the children lose them at Cutover.
func TestRequestChildStorageBackfillCarriesLegacyDaysIntoBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenant := testpkg.Tenant(t)
	phaseID, _, child := testpkg.CreateAuditAdjustmentChain(t, db)
	legacy := testpkg.CreateTestCareOffering(t, db, phaseID, "Legacy Tage")
	split := testpkg.CreateTestCareOffering(t, db, phaseID, "Aufgeteilte Tage")
	empty := testpkg.CreateTestCareOffering(t, db, phaseID, "Leere Aufteilung")
	legacyID := insertLegacyOffering(t, db, tenant, legacyOffering{child: child, offering: legacy.ID, selected: `["mon","wed"]`,
		validFrom: new("2026-08-01"), validUntil: new("2027-08-01")})
	splitID := insertLegacyOffering(t, db, tenant, legacyOffering{child: child, offering: split.ID, selected: `["tue","fri"]`,
		manual: `["tue"]`, automatic: `["fri"]`, validFrom: new("2026-08-01"), validUntil: new("2027-08-01")})
	emptyID := insertLegacyOffering(t, db, tenant, legacyOffering{child: child, offering: empty.ID, selected: `["thu"]`,
		manual: `[]`, automatic: `[]`, validFrom: new("2026-08-01"), validUntil: new("2027-08-01")})

	report := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, report.Complete)
	var bookings []struct {
		ID                int64
		Manual, Automatic *string
	}
	require.NoError(t, db.NewRaw(`SELECT id, manual_selected_days::text AS manual, automatic_selected_days::text AS automatic
		FROM enrollment.care_offering_bookings WHERE tenant_id = ? ORDER BY id`, tenant).Scan(t.Context(), &bookings))
	require.Len(t, bookings, 3)
	assert.Equal(t, legacyID, bookings[0].ID)
	assert.Equal(t, `["mon", "wed"]`, *bookings[0].Manual, "legacy days are manual by construction")
	assert.Nil(t, bookings[0].Automatic)
	assert.Equal(t, splitID, bookings[1].ID)
	assert.Equal(t, `["tue"]`, *bookings[1].Manual, "an existing breakdown copies unchanged")
	assert.Equal(t, `["fri"]`, *bookings[1].Automatic)
	assert.Equal(t, emptyID, bookings[2].ID)
	assert.Equal(t, `["thu"]`, *bookings[2].Manual, "empty parts count as absent, like the application")

	selections := selectionRows(t, db, tenant)
	require.Len(t, selections, 3)
	assert.Equal(t, `["mon", "wed"]`, *selections[0].SelectedDays)
	assert.Equal(t, `["tue"]`, *selections[1].SelectedDays, "the Anmeldung excludes automatic additions")
	assert.Equal(t, `["thu"]`, *selections[2].SelectedDays)
	requireVerified(t, db, tenant)
}

// A legacy row whose breakdown disagrees with the days planned today would
// change a child's plan at Cutover. That is a real difference and blocks.
func TestRequestChildStorageBackfillBlocksPlannedDayDifferences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenant := testpkg.Tenant(t)
	phaseID, _, child := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Abweichende Tage")
	insertLegacyOffering(t, db, tenant, legacyOffering{child: child, offering: offering.ID, selected: `["mon","tue"]`,
		manual: `["mon"]`, validFrom: new("2026-08-01"), validUntil: new("2027-08-01")})

	report := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.False(t, report.Complete)
	require.True(t, report.Stable, "the copy itself is exact; recopying cannot repair the source")
	assert.EqualValues(t, 1, report.Verification.DayDifferences)
	assert.EqualValues(t, 1, report.Verification.Mismatches())
	assert.EqualValues(t, 1, checkpointRow(t, db, tenant)["mismatch_count"])
	assert.EqualValues(t, 1, checkpointRow(t, db, tenant)["day_differences"], "a source defect stays distinguishable from a copy defect")
	require.ErrorContains(t, requestChildStorageBackfillUp(t.Context(), db), "did not reach")
}

func TestRequestChildStorageSelectionFollowsEarliestSurvivingInterval(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	initial := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, initial.Complete)

	// A change effective from the original start deleted the original
	// interval; the surviving approved change is now the earliest one.
	_, err := db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE tenant_id = ? AND id <> ?`, fixture.tenant, fixture.legacyIDs[1]).Exec(t.Context())
	require.NoError(t, err)
	first := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, first.Complete)
	selections := selectionRows(t, db, fixture.tenant)
	require.Len(t, selections, 1, "selections of pairs without legacy rows are removed")
	assert.Equal(t, `["mon", "wed"]`, *selections[0].SelectedDays, "an earlier copied selection is re-derived, not kept")
	assert.Equal(t, "approved change", *selections[0].Notes)

	// A target-only selection is an orphan; verify-only reports it without repair.
	_, err = db.NewRaw(`INSERT INTO enrollment.request_child_offering_selections (tenant_id, request_child_id, care_offering_id, selected_days)
		VALUES (?, ?, ?, '["fri"]'::jsonb)`, fixture.tenant, fixture.children[1], fixture.offerings[1]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE enrollment.request_child_storage_backfill_checkpoints SET complete = TRUE, provenance_policy = '' WHERE tenant_id = ?`, fixture.tenant).Exec(t.Context())
	require.NoError(t, err)
	verified := runBackfill(t, db, RequestChildStorageBackfillOptions{VerifyOnly: true})
	require.False(t, verified.Complete, "a guessed target and old success are not evidence")
	require.EqualValues(t, 1, verified.Verification.Orphans)
	require.Equal(t, false, checkpointRow(t, db, fixture.tenant)["complete"])
	require.Equal(t, requestChildOriginPolicy, checkpointRow(t, db, fixture.tenant)["provenance_policy"])
	require.Equal(t, 2, countRows(t, db, "enrollment.request_child_offering_selections", fixture.tenant), "verify-only changes evidence, not target data")

	replayed := runBackfill(t, db, RequestChildStorageBackfillOptions{Restart: true})
	require.Equal(t, first.Verification.SourceBookingsChecksum, replayed.Verification.SourceBookingsChecksum)
	require.Equal(t, first.Verification.SourceSelectionsChecksum, replayed.Verification.SourceSelectionsChecksum)
	require.True(t, replayed.Complete)
	require.Equal(t, 1, countRows(t, db, "enrollment.request_child_offering_selections", fixture.tenant))

	report, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{})
	require.NoError(t, err)
	require.Empty(t, report.IncompleteTenants())
	require.NoError(t, requestChildStorageBackfillUp(t.Context(), db))
}

func TestRequestChildStorageDoesNotTrustPartialOrForeignChangeHistory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	var requestID int64
	require.NoError(t, db.NewRaw(`SELECT request_id FROM enrollment.request_children WHERE id = ?`, fixture.children[0]).Scan(t.Context(), &requestID))
	insertChange := func(tenantID, requestID, childID int64) {
		// Real change-request snapshot shape has offering IDs/days but does
		// not establish original per-offering notes or submission version.
		_, err := db.NewRaw(`INSERT INTO enrollment.change_requests
			(tenant_id, request_id, request_child_id, status, base_snapshot, proposed_snapshot)
			VALUES (?, ?, ?, 'approved', '{}'::jsonb,
			jsonb_build_object('children', jsonb_build_array(jsonb_build_object('offering_days',
			jsonb_build_array(jsonb_build_object('offering_id', ?::text, 'selected_days', '["fri"]'::jsonb))))))`,
			tenantID, requestID, childID, strconv.FormatInt(fixture.offerings[0], 10)).Exec(t.Context())
		require.NoError(t, err)
	}
	insertChange(fixture.tenant, requestID, fixture.children[0])
	t.Run("foreign snapshot", func(t *testing.T) {
		_ = testpkg.OwnCtx(t)
		testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
		_, foreignRequest, foreignChild := testpkg.CreateAuditAdjustmentChain(t, db)
		insertChange(testpkg.Tenant(t), foreignRequest, foreignChild)
	})
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, report.Complete)
	require.EqualValues(t, 3, report.Verification.TargetSelections)
	selections := selectionRows(t, db, fixture.tenant)
	require.Equal(t, fixture.offerings[0], selections[0].CareOfferingID)
	require.Equal(t, `["mon"]`, *selections[0].SelectedDays, "the selection comes from the legacy Anmeldung, not the change snapshot")
	requireVerified(t, db, fixture.tenant)
	var changes int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.change_requests WHERE proposed_snapshot->'children' IS NOT NULL`).Scan(t.Context(), &changes))
	require.Equal(t, 2, changes, "backfill must not rewrite the approved amendment evidence")
}

func TestRequestChildStorageVerificationUsesOneSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	initial := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{VerifyOnly: true, afterVerificationCounts: func() error {
		_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET updated_at = updated_at + interval '1 hour' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
		return err
	}})
	require.Equal(t, initial.Verification.SourceBookingsChecksum, report.Verification.SourceBookingsChecksum)
	require.Equal(t, report.Verification.SourceBookingsChecksum, report.Verification.TargetBookingsChecksum)
	require.Zero(t, report.Verification.Mismatches(), "row comparisons must share the pre-edit checksum snapshot")
	require.NotEmpty(t, report.Verification.Snapshot)
	next := runBackfill(t, db, RequestChildStorageBackfillOptions{VerifyOnly: true})
	require.EqualValues(t, 1, next.Verification.Mismatches(), "next observation sees the committed source edit")
	require.NotEqual(t, next.Verification.SourceBookingsChecksum, next.Verification.TargetBookingsChecksum)
	require.Equal(t, false, checkpointRow(t, db, fixture.tenant)["complete"])
}

func TestRequestChildStorageExcludesConcurrentWriters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	checked := false
	runBackfill(t, db, RequestChildStorageBackfillOptions{AfterBatch: func(int64, RequestChildStorageBatch) error {
		if checked {
			return nil
		}
		checked = true
		for _, options := range []RequestChildStorageBackfillOptions{{}, {Restart: true}, {VerifyOnly: true}} {
			options.TenantIDs = []int64{fixture.tenant}
			_, err := RunRequestChildStorageBackfill(t.Context(), db, options)
			require.ErrorContains(t, err, "another run, reset or rollback is active")
		}
		require.ErrorContains(t, requestChildStorageBackfillDown(t.Context(), db), "another run, reset or rollback is active")
		return nil
	}})
	require.True(t, checked)
	runBackfill(t, db, RequestChildStorageBackfillOptions{VerifyOnly: true})
}

func TestRequestChildStorageMeasuresSuccessfulLockWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET updated_at = updated_at + interval '1 hour' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.NewRaw(`SELECT id FROM enrollment.care_offering_bookings WHERE id = ? FOR UPDATE`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{TenantIDs: []int64{fixture.tenant}})
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%DELETE FROM enrollment.care_offering_bookings%')`).Scan(t.Context(), &waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	require.NoError(t, tx.Commit())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("backfill did not finish after release")
	}
	checkpoint := checkpointRow(t, db, fixture.tenant)
	require.Greater(t, checkpoint["lock_wait_ms"].(float64), float64(50))
	require.EqualValues(t, 0, checkpoint["lock_timeouts"])
}

func TestRequestChildStorageBackfillCopiesOwnedFields(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 2})

	require.True(t, report.Complete)
	require.True(t, report.Stable)
	assert.Equal(t, fixture.legacyIDs[len(fixture.legacyIDs)-1], report.HighWaterMark)
	assert.EqualValues(t, 5, report.RowsScanned)
	assert.EqualValues(t, 3, report.BatchesCompleted)
	assert.EqualValues(t, 0, report.BatchesRetried)
	assert.EqualValues(t, 8, report.RowsCopied, "five bookings and three Anmeldung selections")
	assert.EqualValues(t, 0, report.RowsSkipped)
	assert.EqualValues(t, 5, report.Verification.SourceBookings)
	assert.EqualValues(t, 3, report.Verification.SourceSelections)
	assert.Zero(t, report.Verification.DayDifferences)
	assert.Zero(t, report.Verification.Mismatches())
	assert.Zero(t, report.Verification.OldestUnmigrated, "nothing is left to migrate")
	assert.Greater(t, report.BatchMax, time.Duration(0))

	// Bookings keep the legacy id and copy exactly the effective-state fields.
	var bookings []struct {
		ID, RequestChildID, CareOfferingID int64
		Manual, Automatic                  *string
		ValidFrom, ValidUntil              *string
		CreatedAt, UpdatedAt               time.Time
	}
	require.NoError(t, db.NewRaw(`SELECT id, request_child_id, care_offering_id, manual_selected_days::text AS manual,
		automatic_selected_days::text AS automatic, valid_from::text AS valid_from, valid_until::text AS valid_until, created_at, updated_at
		FROM enrollment.care_offering_bookings WHERE tenant_id = ? ORDER BY id`, fixture.tenant).Scan(t.Context(), &bookings))
	require.Len(t, bookings, 5)
	for i, booking := range bookings {
		assert.Equal(t, fixture.legacyIDs[i], booking.ID)
	}
	assert.Equal(t, `["mon"]`, *bookings[0].Manual)
	assert.Equal(t, `["tue"]`, *bookings[0].Automatic)
	assert.Equal(t, "2026-08-01", *bookings[0].ValidFrom)
	assert.Equal(t, "2026-10-01", *bookings[0].ValidUntil)
	assert.Nil(t, bookings[2].Manual)
	assert.Nil(t, bookings[2].ValidUntil)
	assert.True(t, bookings[2].UpdatedAt.After(bookings[2].CreatedAt), "legacy timestamps are preserved")
	assert.Nil(t, bookings[3].Manual, "no days stay no days")
	assert.Equal(t, `[]`, *bookings[4].Manual, "legacy days copy into the manual part")
	assert.Nil(t, bookings[4].ValidFrom)

	// Each pair's earliest interval without automatic additions is its Anmeldung.
	selections := selectionRows(t, db, fixture.tenant)
	require.Len(t, selections, 3)
	assert.Equal(t, fixture.offerings[0], selections[0].CareOfferingID)
	assert.Equal(t, `["mon"]`, *selections[0].SelectedDays)
	assert.Equal(t, "submitted", *selections[0].Notes)
	assert.True(t, selections[0].CreatedAt.Equal(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)))
	assert.Nil(t, selections[1].SelectedDays)
	assert.Nil(t, selections[1].Notes)
	assert.Equal(t, `[]`, *selections[2].SelectedDays)
	assert.Equal(t, "", *selections[2].Notes)
	assert.Equal(t, requestChildOriginPolicy, report.Verification.ProvenancePolicy)
	assert.NotEmpty(t, report.Verification.Snapshot)

	// The sequence moved past the copied ids so application inserts cannot collide.
	var nextID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.care_offering_bookings (tenant_id, request_child_id, care_offering_id, valid_from)
		VALUES (?, ?, ?, '2028-01-01') RETURNING id`, fixture.tenant, fixture.children[1], fixture.offerings[0]).Scan(t.Context(), &nextID))
	assert.Greater(t, nextID, fixture.legacyIDs[len(fixture.legacyIDs)-1])

	checkpoint := checkpointRow(t, db, fixture.tenant)
	assert.Equal(t, true, checkpoint["complete"])
	assert.Equal(t, true, checkpoint["stable"])
	assert.EqualValues(t, 0, checkpoint["mismatch_count"])
	assert.Equal(t, checkpoint["source_bookings_checksum"], checkpoint["target_bookings_checksum"])
	assert.Equal(t, checkpoint["source_selections_checksum"], checkpoint["target_selections_checksum"])
	assert.NotNil(t, checkpoint["verified_at"])
}

func TestRequestChildStorageBackfillResumesAtEveryBatchBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	interrupted := errors.New("operator stopped the backfill")
	var marks []int64
	options := RequestChildStorageBackfillOptions{BatchSize: 2, AfterBatch: func(_ int64, batch RequestChildStorageBatch) error {
		marks = append(marks, batch.HighWaterMark)
		return interrupted
	}}
	var runs int
	for {
		runs++
		require.Less(t, runs, 10, "the run must converge")
		report, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{
			BatchSize: options.BatchSize, AfterBatch: options.AfterBatch, TenantIDs: []int64{fixture.tenant}})
		if err == nil {
			require.Empty(t, report.IncompleteTenants(), "the resumed run completes")
			break
		}
		require.ErrorIs(t, err, interrupted)
		checkpoint := checkpointRow(t, db, fixture.tenant)
		assert.EqualValues(t, marks[len(marks)-1], checkpoint["high_water_mark"], "the checkpoint holds the interrupted batch")
		assert.Equal(t, false, checkpoint["complete"])
		assert.EqualValues(t, len(marks), checkpoint["batches_completed"])
	}
	assert.Equal(t, 4, runs, "three sweep batches interrupt once each, the fourth run reconciles and verifies")
	assert.Equal(t, []int64{fixture.legacyIDs[1], fixture.legacyIDs[3], fixture.legacyIDs[4]}, marks)
	requireVerified(t, db, fixture.tenant)
}

func TestRequestChildStorageBackfillStopsAtBatchBoundaryOnCancel(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var committed []int64
	_, err := RunRequestChildStorageBackfill(ctx, db, RequestChildStorageBackfillOptions{
		TenantIDs: []int64{fixture.tenant}, BatchSize: 2,
		BeforeCommit: func(int64, int) error {
			cancel() // the signal arrives while a batch is in flight
			return nil
		},
		AfterBatch: func(_ int64, batch RequestChildStorageBatch) error {
			committed = append(committed, batch.HighWaterMark)
			return nil
		},
	})
	require.ErrorIs(t, err, ErrRequestChildStorageBackfillStopped)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []int64{fixture.legacyIDs[1]}, committed, "the in-flight batch commits, the next one never starts")
	checkpoint := checkpointRow(t, db, fixture.tenant)
	assert.EqualValues(t, fixture.legacyIDs[1], checkpoint["high_water_mark"])
	assert.Equal(t, 2, countRows(t, db, "enrollment.care_offering_bookings", fixture.tenant))

	report := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 2})
	require.True(t, report.Complete)
	assert.EqualValues(t, 3, report.BatchesCompleted)
	requireVerified(t, db, fixture.tenant)
}

func TestRequestChildStorageBackfillReplaysCompletedBatchesWithoutEffect(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	first := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 2})
	before := checkpointRow(t, db, fixture.tenant)

	// Rewind the high-water mark: every finished batch runs again.
	_, err := db.NewRaw(`UPDATE enrollment.request_child_storage_backfill_checkpoints SET high_water_mark = 0 WHERE tenant_id = ?`, fixture.tenant).Exec(t.Context())
	require.NoError(t, err)
	second := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 2})

	assert.Equal(t, first.RowsCopied, second.RowsCopied, "replayed batches copy nothing")
	assert.EqualValues(t, 5, second.RowsSkipped-first.RowsSkipped)
	assert.EqualValues(t, 0, second.RowsDeleted)
	assert.Equal(t, first.HighWaterMark, second.HighWaterMark)
	assert.True(t, second.Complete)
	assert.Equal(t, first.Verification.SourceSelectionsChecksum, second.Verification.SourceSelectionsChecksum)
	after := checkpointRow(t, db, fixture.tenant)
	assert.Equal(t, before["target_bookings_checksum"], after["target_bookings_checksum"])
	assert.Equal(t, before["target_selections_checksum"], after["target_selections_checksum"])
	var ids []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.care_offering_bookings WHERE tenant_id = ? ORDER BY id`, fixture.tenant).Scan(t.Context(), &ids))
	assert.Equal(t, fixture.legacyIDs, ids, "replay keeps the original target rows")
}

func TestRequestChildStorageBackfillRetriesInjectedDeadlockAndSerializationFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	injections := map[int]string{1: "40P01", 2: "40001"}
	var batchIndex int
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 2, BeforeCommit: func(_ int64, attempt int) error {
		if attempt == 1 {
			batchIndex++
			if code, ok := injections[batchIndex]; ok {
				return sqlStateError(code)
			}
		}
		return nil
	}})
	require.True(t, report.Complete)
	assert.EqualValues(t, 2, report.BatchesRetried)
	assert.EqualValues(t, 1, report.Deadlocks)
	assert.EqualValues(t, 1, report.SerializationFailures)
	assert.EqualValues(t, 3, report.BatchesCompleted)
	assert.EqualValues(t, 5, report.RowsScanned, "an aborted attempt counts nothing")
	requireVerified(t, db, fixture.tenant)
	checkpoint := checkpointRow(t, db, fixture.tenant)
	assert.EqualValues(t, 2, checkpoint["batches_retried"])
	assert.EqualValues(t, 1, checkpoint["deadlocks"])
	assert.EqualValues(t, 1, checkpoint["serialization_failures"])

	// Exhausted retries surface the error and leave the checkpoint at the last commit.
	_, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{
		TenantIDs: []int64{fixture.tenant}, Restart: true, MaxRetries: 1, BatchSize: 2,
		BeforeCommit: func(int64, int) error { return sqlStateError("40P01") },
	})
	require.ErrorContains(t, err, "after 2 attempts")
	require.ErrorContains(t, err, "40P01")
	checkpoint = checkpointRow(t, db, fixture.tenant)
	assert.EqualValues(t, 0, checkpoint["high_water_mark"])
	assert.EqualValues(t, 0, checkpoint["batches_completed"])
	assert.EqualValues(t, 2, checkpoint["deadlocks"], "terminal failure is counted and persisted")
	assert.EqualValues(t, 1, checkpoint["batches_retried"], "only the second attempt is a retry")
	assert.Zero(t, countRows(t, db, "enrollment.care_offering_bookings", fixture.tenant), "restart truncated and nothing committed")

	// A non-retryable error is returned at once.
	_, err = RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{
		TenantIDs: []int64{fixture.tenant}, BeforeCommit: func(int64, int) error { return sqlStateError("23505") },
	})
	require.ErrorContains(t, err, "after 1 attempts")
}

func TestRequestChildStorageBackfillWaitsOutApplicationLocks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	// A legacy change makes the copied booking and the Anmeldung selection
	// stale; the reconcile must delete both. selected_days follows the
	// write-path invariant selected = manual ∪ automatic.
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET manual_selected_days = '["fri"]', selected_days = '["tue","fri"]' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	holder, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	_, err = holder.ExecContext(t.Context(), `SELECT id FROM enrollment.care_offering_bookings WHERE id = ? FOR UPDATE`, fixture.legacyIDs[0])
	require.NoError(t, err)
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(400 * time.Millisecond)
		_ = holder.Rollback()
	}()
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{LockTimeout: 50 * time.Millisecond, MaxRetries: 50})
	<-released
	require.True(t, report.Complete)
	assert.GreaterOrEqual(t, report.LockTimeouts, int64(1), "the batch waited on the application lock and retried")
	assert.Equal(t, report.LockTimeouts, report.BatchesRetried)
	assert.EqualValues(t, 2, report.RowsDeleted, "stale booking and stale selection")
	requireVerified(t, db, fixture.tenant)
}

func TestRequestChildStorageBackfillReconcilesChangedLegacyRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})

	// Verify-only reports drift without repairing it.
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET notes = 'edited submission', updated_at = updated_at + interval '1 hour' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE id = ?`, fixture.legacyIDs[3]).Exec(t.Context())
	require.NoError(t, err)
	newID := insertLegacyOffering(t, db, fixture.tenant, legacyOffering{child: fixture.children[1], offering: fixture.offerings[0],
		selected: `["thu"]`, validFrom: new("2026-09-01"), validUntil: new("2026-12-01")})
	drift := runBackfill(t, db, RequestChildStorageBackfillOptions{VerifyOnly: true})
	require.False(t, drift.Complete)
	assert.EqualValues(t, 4, drift.Verification.MissingOrDifferent, "changed booking and selection, new booking and selection")
	assert.EqualValues(t, 2, drift.Verification.Orphans, "the deleted row leaves a booking and its pair's selection")
	assert.Greater(t, drift.Verification.OldestUnmigrated, time.Duration(0))
	assert.Equal(t, 5, countRows(t, db, "enrollment.care_offering_bookings", fixture.tenant), "verify-only writes nothing")
	checkpoint := checkpointRow(t, db, fixture.tenant)
	assert.Equal(t, false, checkpoint["complete"], "verify-only invalidates acceptance on drift")
	assert.EqualValues(t, 6, checkpoint["mismatch_count"])
	assert.EqualValues(t, 6, checkpoint["verify_only_mismatch_count"])
	assert.NotNil(t, checkpoint["verify_only_at"])
	assert.EqualValues(t, 1, checkpoint["runs"])

	report := runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, report.Complete)
	assert.EqualValues(t, 2, checkpointRow(t, db, fixture.tenant)["runs"])
	assert.Equal(t, 2, report.Passes, "one repairing pass and one empty pass")
	assert.EqualValues(t, 4, report.RowsDeleted, "stale and orphan booking, stale and orphan selection")
	assert.Equal(t, newID, report.HighWaterMark)
	requireVerified(t, db, fixture.tenant)
	assert.Equal(t, 3, countRows(t, db, "enrollment.request_child_offering_selections", fixture.tenant))
	assert.Equal(t, "edited submission", *selectionRows(t, db, fixture.tenant)[0].Notes, "notes edited before acceptance belong to the Anmeldung")

	// Deleting the oldest interval makes the next surviving one the earliest.
	_, err = db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	report = runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.True(t, report.Complete)
	selections := selectionRows(t, db, fixture.tenant)
	require.Len(t, selections, 3)
	assert.Equal(t, `["mon", "wed"]`, *selections[0].SelectedDays, "the selection follows the earliest surviving interval")
	assert.Equal(t, 4, countRows(t, db, "enrollment.care_offering_bookings", fixture.tenant))
	requireVerified(t, db, fixture.tenant)

	// A moved boundary: the new interval of the earlier row overlaps the stale
	// copy of the later row. One-id batches must still commit because a batch
	// removes every stale booking of the pairs it touches first.
	_, err = db.NewRaw(`UPDATE enrollment.request_child_offerings SET valid_from = '2027-02-01' WHERE id = ?`, fixture.legacyIDs[2]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE enrollment.request_child_offerings SET valid_until = '2027-02-01' WHERE id = ?`, fixture.legacyIDs[1]).Exec(t.Context())
	require.NoError(t, err)
	report = runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 1})
	require.True(t, report.Complete)
	requireVerified(t, db, fixture.tenant)
}

func TestRequestChildStorageBackfillIsolatesTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixtures := make([]requestChildStorageFixture, 2)
	for i := range fixtures {
		require.True(t, t.Run(fmt.Sprintf("tenant_%d", i), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
			fixtures[i] = createRequestChildStorageFixture(t, db)
		}))
	}
	_, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{TenantIDs: []int64{fixtures[0].tenant}})
	require.NoError(t, err)
	assert.Equal(t, 5, countRows(t, db, "enrollment.care_offering_bookings", fixtures[0].tenant))
	assert.Zero(t, countRows(t, db, "enrollment.care_offering_bookings", fixtures[1].tenant), "a tenant batch never touches another school")
	var checkpoints int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id IN (?, ?)`, fixtures[0].tenant, fixtures[1].tenant).Scan(t.Context(), &checkpoints))
	assert.Equal(t, 1, checkpoints)

	report, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{TenantIDs: []int64{fixtures[0].tenant, fixtures[1].tenant}})
	require.NoError(t, err)
	require.Empty(t, report.IncompleteTenants())
	require.Len(t, report.Tenants, 2)
	assert.EqualValues(t, 1, report.Tenants[0].BatchesCompleted, "the finished tenant has nothing above its high-water mark")
	assert.EqualValues(t, 8, report.Tenants[0].RowsCopied)
	assert.EqualValues(t, 8, report.Tenants[1].RowsCopied)
	for _, fixture := range fixtures {
		requireVerified(t, db, fixture.tenant)
	}
	_, err = RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{TenantIDs: []int64{fixtures[0].tenant, -1}})
	require.ErrorContains(t, err, "unknown tenant")

	// The tenant role sees only its own copies and has no access to checkpoints.
	for i, own := range fixtures {
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(t.Context(), `SET LOCAL ROLE phoenix_tenant`)
		require.NoError(t, err)
		_, err = tx.ExecContext(t.Context(), `SELECT set_config('app.current_tenant_id', ?, true)`, strconv.FormatInt(own.tenant, 10))
		require.NoError(t, err)
		for table, want := range map[string]int{"enrollment.care_offering_bookings": 5, "enrollment.request_child_offering_selections": 3} {
			var visible int
			require.NoError(t, tx.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s`, table)).Scan(t.Context(), &visible))
			assert.Equal(t, want, visible, "tenant %d sees only its rows in %s", i, table)
			var foreign int
			require.NoError(t, tx.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s WHERE tenant_id = ?`, table), fixtures[1-i].tenant).Scan(t.Context(), &foreign))
			assert.Zero(t, foreign)
		}
		_, err = tx.ExecContext(t.Context(), `SELECT count(*) FROM enrollment.request_child_storage_backfill_checkpoints`)
		require.ErrorContains(t, err, "SQLSTATE=42501", "checkpoints are migration bookkeeping")
		require.NoError(t, tx.Rollback())
	}
}

func TestRequestChildStorageBackfillMigrationUpDownAndRestart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	var legacyBefore string
	legacySnapshot := `SELECT string_agg(row_to_json(o)::text, E'\n' ORDER BY id) FROM enrollment.request_child_offerings o WHERE tenant_id = ?`
	require.NoError(t, db.NewRaw(legacySnapshot, fixture.tenant).Scan(t.Context(), &legacyBefore))

	require.NoError(t, runPresenceMigration(t.Context(), db, "001015383", true))
	requireVerified(t, db, fixture.tenant)
	assert.Equal(t, true, checkpointRow(t, db, fixture.tenant)["complete"])
	require.NoError(t, requestChildStorageBackfillUp(t.Context(), db), "a repeated Up resumes and verifies again")
	assert.EqualValues(t, 5, checkpointRow(t, db, fixture.tenant)["rows_scanned"])
	assert.EqualValues(t, 1, checkpointRow(t, db, fixture.tenant)["batches_completed"])

	// Restart empties only target rows and copies from zero.
	before := checkpointRow(t, db, fixture.tenant)
	report := runBackfill(t, db, RequestChildStorageBackfillOptions{Restart: true, BatchSize: 3})
	assert.EqualValues(t, 2, report.BatchesCompleted)
	assert.EqualValues(t, 5, report.RowsScanned)
	assert.Equal(t, before["target_bookings_checksum"], checkpointRow(t, db, fixture.tenant)["target_bookings_checksum"])
	requireVerified(t, db, fixture.tenant)
	_, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{Restart: true, VerifyOnly: true})
	require.ErrorContains(t, err, "mutually exclusive")

	// Rollback keeps legacy rows, empties targets, and drops the checkpoints.
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015383", false))
	var legacyAfter string
	require.NoError(t, db.NewRaw(legacySnapshot, fixture.tenant).Scan(t.Context(), &legacyAfter))
	assert.Equal(t, legacyBefore, legacyAfter, "rollback never mutates the authoritative rows")
	for _, table := range []string{"enrollment.care_offering_bookings", "enrollment.request_child_offering_selections"} {
		var count int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s`, table)).Scan(t.Context(), &count))
		assert.Zero(t, count, "%s must be empty after rollback", table)
	}
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('enrollment.request_child_storage_backfill_checkpoints') IS NOT NULL`).Scan(t.Context(), &exists))
	assert.False(t, exists)
	require.NoError(t, requestChildStorageExpandDown(t.Context(), db), "Expand can roll back once the backfill is rolled back")
	require.NoError(t, requestChildStorageExpandUp(t.Context(), db))
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015383", true))
	requireVerified(t, db, fixture.tenant)
}

func TestRequestChildStorageBackfillSchemaAndPlans(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenant := testpkg.Tenant(t)
	phaseID, requestID, _ := testpkg.CreateAuditAdjustmentChain(t, db)
	var children []int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_children
		(tenant_id, request_id, first_name, last_name, date_of_birth, custom_data, status, activation_mode)
		SELECT ?, ?, 'Plan', 'Child ' || g, DATE '2018-04-15', '{}'::jsonb, 'approved', 'scheduled'
		FROM generate_series(1, 200) AS g RETURNING id`, tenant, requestID).Scan(t.Context(), &children))
	var offerings []int64
	for i := range 5 {
		offerings = append(offerings, testpkg.CreateTestCareOffering(t, db, phaseID, "Plan offering "+strconv.Itoa(i)).ID)
	}
	_, err := db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		(tenant_id, request_child_id, care_offering_id, selected_days, notes, valid_from, valid_until)
		SELECT ?, child, offering, '["mon"]'::jsonb, 'plan', '2026-08-01', '2027-08-01'
		FROM unnest(?) AS child CROSS JOIN unnest(?) AS offering`, tenant, sqlBigintArray(children), sqlBigintArray(offerings)).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `ANALYZE enrollment.request_child_offerings, enrollment.care_offering_bookings, enrollment.request_child_offering_selections`)
	require.NoError(t, err)

	report := runBackfill(t, db, RequestChildStorageBackfillOptions{BatchSize: 300})
	require.True(t, report.Complete)
	assert.EqualValues(t, 1000, report.Verification.TargetBookings)
	assert.EqualValues(t, 1000, report.Verification.TargetSelections)
	assert.EqualValues(t, 4, report.BatchesCompleted)
	assert.GreaterOrEqual(t, report.BatchMax, report.BatchP95)

	explain := func(query string, args ...any) string {
		t.Helper()
		var lines []string
		require.NoError(t, db.NewRaw("EXPLAIN "+query, args...).Scan(t.Context(), &lines))
		return strings.Join(lines, "\n")
	}
	batchPlan := explain(`SELECT id FROM enrollment.request_child_offerings WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, tenant, report.HighWaterMark/2, 300)
	assert.Contains(t, batchPlan, "request_child_offerings_pkey", "the sweep walks the primary key in id order")
	assert.NotContains(t, batchPlan, "Seq Scan")
	ids := make([]int64, 0, 300)
	for id := report.HighWaterMark - 299; id <= report.HighWaterMark; id++ {
		ids = append(ids, id)
	}
	for name, query := range map[string]string{
		"stale bookings":     staleBookingDelete,
		"missing bookings":   missingBookingInsert,
		"stale selections":   staleSelectionDelete,
		"missing selections": missingSelectionInsert,
	} {
		plan := explain(query, tenant, sqlBigintArray(ids), sqlBigintArray(nil), sqlBigintArray(nil))
		assert.Contains(t, plan, "Index Scan using request_child_offerings_pkey", "%s must look legacy batch rows up by id", name)
	}
	// Bookings are keyed by legacy id; pair lookups on the legacy table go
	// through the exclusion index. Neither statement may scan a target table.
	for name, query := range map[string]string{"stale bookings": staleBookingDelete, "missing bookings": missingBookingInsert} {
		plan := explain(query, tenant, sqlBigintArray(ids[:5]), sqlBigintArray(nil), sqlBigintArray(nil))
		assert.NotContains(t, plan, "Seq Scan on care_offering_bookings", "%s must look bookings up by key", name)
	}

	// FK, constraints, indexes, and RLS of the targets survive the copy.
	var constraints []string
	require.NoError(t, db.NewRaw(`SELECT conname FROM pg_constraint WHERE conrelid = ANY(ARRAY['enrollment.care_offering_bookings'::regclass,
		'enrollment.request_child_offering_selections'::regclass, 'enrollment.request_child_storage_backfill_checkpoints'::regclass]) ORDER BY conname`).Scan(t.Context(), &constraints))
	for _, want := range []string{"care_offering_bookings_nonempty_validity", "care_offering_bookings_non_overlapping_validity",
		"care_offering_bookings_tenant_id_request_child_id_fkey", "care_offering_bookings_tenant_id_care_offering_id_fkey",
		"request_child_offering_select_tenant_id_request_child_id_ca_key",
		"request_child_storage_backfill_checkpoints_tenant_id_fkey"} {
		assert.Contains(t, constraints, want)
	}
	var indexes []string
	require.NoError(t, db.NewRaw(`SELECT indexname FROM pg_indexes WHERE schemaname = 'enrollment'
		AND tablename IN ('care_offering_bookings', 'request_child_offering_selections')`).Scan(t.Context(), &indexes))
	assert.Contains(t, indexes, "care_offering_bookings_active_validity")
	assert.Contains(t, indexes, "offering_selections_offering")
	for _, table := range []string{"enrollment.care_offering_bookings", "enrollment.request_child_offering_selections", "enrollment.request_child_storage_backfill_checkpoints"} {
		var enabled, forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, table).Scan(t.Context(), &enabled, &forced))
		assert.True(t, enabled && forced, "%s keeps forced RLS", table)
	}
	var orphans int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.care_offering_bookings b
		WHERE NOT EXISTS (SELECT 1 FROM enrollment.request_children c WHERE c.id = b.request_child_id AND c.tenant_id = b.tenant_id)
		OR NOT EXISTS (SELECT 1 FROM enrollment.care_offerings o WHERE o.id = b.care_offering_id AND o.tenant_id = b.tenant_id)`).Scan(t.Context(), &orphans))
	assert.Zero(t, orphans)
	_, err = db.NewRaw(`DELETE FROM enrollment.request_children WHERE id = ?`, children[0]).Exec(t.Context())
	require.NoError(t, err, "child deletion cascades through legacy and target rows alike")
	assert.EqualValues(t, 995, countRows(t, db, "enrollment.care_offering_bookings", tenant))
	requireVerified(t, db, tenant)
}

func TestRequestChildStorageBackfillRequiresDatabase(t *testing.T) {
	t.Parallel()
	_, err := RunRequestChildStorageBackfill(context.Background(), nil, RequestChildStorageBackfillOptions{})
	require.ErrorContains(t, err, "database is required")
	assert.Equal(t, time.Duration(0), percentile95(nil))
	assert.Equal(t, 3*time.Millisecond, percentile95([]time.Duration{3 * time.Millisecond, time.Millisecond, 2 * time.Millisecond}))
	code, retryable := classifyRetryableSQLError(fmt.Errorf("wrapped: %w", sqlStateError("55P03")))
	assert.True(t, retryable)
	assert.Equal(t, "55P03", code)
	_, retryable = classifyRetryableSQLError(errors.New("plain"))
	assert.False(t, retryable)
}

// The retry classifier must recognise the driver errors PostgreSQL itself
// raises, not only the synthetic ones the batch tests inject.
func TestRequestChildStorageBackfillClassifiesRealPostgresFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	first, second := fixture.legacyIDs[0], fixture.legacyIDs[1]

	t.Run("deadlock", func(t *testing.T) {
		left, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = left.Rollback() }()
		right, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = right.Rollback() }()
		lock := `SELECT id FROM enrollment.care_offering_bookings WHERE id = ? FOR UPDATE`
		_, err = left.ExecContext(t.Context(), lock, first)
		require.NoError(t, err)
		_, err = right.ExecContext(t.Context(), lock, second)
		require.NoError(t, err)
		errs := make(chan error, 2)
		go func() { _, err := left.ExecContext(t.Context(), lock, second); errs <- err }()
		go func() { _, err := right.ExecContext(t.Context(), lock, first); errs <- err }()
		var failure error
		for range 2 {
			if err := <-errs; err != nil {
				failure = err
			}
		}
		require.Error(t, failure, "one side must be chosen as the deadlock victim")
		code, retryable := classifyRetryableSQLError(fmt.Errorf("batch: %w", failure))
		assert.True(t, retryable)
		assert.Equal(t, "40P01", code)
	})

	t.Run("serialization failure", func(t *testing.T) {
		left, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = left.Rollback() }()
		right, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() { _ = right.Rollback() }()
		for _, tx := range []*testpkg.Tx{&left, &right} {
			_, err = tx.ExecContext(t.Context(), `SET TRANSACTION ISOLATION LEVEL SERIALIZABLE`)
			require.NoError(t, err)
		}
		read := `SELECT count(*) FROM enrollment.care_offering_bookings WHERE tenant_id = ?`
		write := `UPDATE enrollment.care_offering_bookings SET updated_at = updated_at WHERE id = ?`
		var count int
		require.NoError(t, left.NewRaw(read, fixture.tenant).Scan(t.Context(), &count))
		require.NoError(t, right.NewRaw(read, fixture.tenant).Scan(t.Context(), &count))
		_, err = left.ExecContext(t.Context(), write, first)
		require.NoError(t, err)
		_, err = right.ExecContext(t.Context(), write, second)
		require.NoError(t, err)
		require.NoError(t, left.Commit())
		err = right.Commit()
		require.Error(t, err, "write skew must be rejected under SERIALIZABLE")
		code, retryable := classifyRetryableSQLError(fmt.Errorf("batch: %w", err))
		assert.True(t, retryable)
		assert.Equal(t, "40001", code)
	})
}
