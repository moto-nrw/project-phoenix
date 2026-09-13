package migrations

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Historical Expand/Backfill contracts need the pre-cutover schema, not the
// latest compatibility view. This reversal is confined to a disposable clone;
// production rollback deliberately retains the compatibility layer.
func setupRequestChildStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	_, err := db.ExecContext(t.Context(), `
		DROP VIEW enrollment.request_child_offerings;
		DROP FUNCTION enrollment.route_request_child_offering_compatibility();
		DROP FUNCTION enrollment.request_child_effective_days(jsonb, jsonb);
		DROP FUNCTION enrollment.request_child_legacy_manual(jsonb, jsonb, jsonb);
		DROP SEQUENCE enrollment.request_child_compatibility_reads, enrollment.request_child_compatibility_writes;
		ALTER TABLE enrollment.request_child_offerings_legacy RENAME TO request_child_offerings;
		ALTER TABLE enrollment.request_child_offerings ADD CONSTRAINT request_child_offerings_non_overlapping_validity
			EXCLUDE USING gist (request_child_id WITH =, care_offering_id WITH =,
				daterange(COALESCE(valid_from, '-infinity'::date), COALESCE(valid_until, 'infinity'::date), '[)') WITH &&);
	`)
	require.NoError(t, err)
	return db
}

func TestRequestChildStorageCutoverFinalDeltaRollback(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	checkpointBefore := checkpointRow(t, db, fixture.tenant)
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET notes = 'final submission' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	injected := errors.New("cutover failed after final delta")
	err = finalizeRequestChildStorage(t.Context(), db, func(ctx context.Context, tx testpkg.Tx) error {
		verification, err := verifyRequestChildStorageTenant(ctx, tx, fixture.tenant, nil)
		require.NoError(t, err)
		require.True(t, verification.Equal())
		return injected
	})
	require.ErrorIs(t, err, injected)
	require.Equal(t, checkpointBefore, checkpointRow(t, db, fixture.tenant), "failed cutover must roll back its checkpoint evidence")
	verification, err := verifyRequestChildStorageTenant(t.Context(), db, fixture.tenant, nil)
	require.NoError(t, err)
	require.False(t, verification.Equal(), "failed cutover must roll back its final delta")
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, func(ctx context.Context, tx testpkg.Tx) error {
		verification, err := verifyRequestChildStorageTenant(ctx, tx, fixture.tenant, nil)
		require.NoError(t, err)
		require.True(t, verification.Equal(), "retry must reconcile the final source write")
		return nil
	}))
	verification, err = verifyRequestChildStorageTenant(t.Context(), db, fixture.tenant, nil)
	require.NoError(t, err)
	require.True(t, verification.Equal())
	checkpointAfter := checkpointRow(t, db, fixture.tenant)
	require.Equal(t, verification.SourceSelectionsChecksum, checkpointAfter["source_selections_checksum"])
	require.Equal(t, verification.TargetSelectionsChecksum, checkpointAfter["target_selections_checksum"])
}

func TestRequestChildStorageCutoverRejectsChangedEffectiveDays(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET selected_days = '["fri"]'::jsonb WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	called := false
	err = finalizeRequestChildStorage(t.Context(), db, func(context.Context, testpkg.Tx) error {
		called = true
		return nil
	})
	require.ErrorContains(t, err, "checksum drift")
	require.False(t, called, "schema must not switch when manual and automatic days disagree with the legacy effective days")
}

func TestRequestChildStorageCutoverRequiresCompletedBackfill(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	createRequestChildStorageFixture(t, db)
	called := false
	err := finalizeRequestChildStorage(t.Context(), db, func(context.Context, testpkg.Tx) error {
		called = true
		return nil
	})
	require.ErrorContains(t, err, "requires a complete backfill checkpoint")
	require.False(t, called)
}

func TestRequestChildStorageCutoverFinalDeltaDeletesAndInserts(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	_, err := db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE request_child_id = ? AND care_offering_id = ?`, fixture.children[0], fixture.offerings[0]).Exec(t.Context())
	require.NoError(t, err)
	insertLegacyOffering(t, db, fixture.tenant, legacyOffering{
		child: fixture.children[1], offering: fixture.offerings[0], selected: `["thu"]`,
	})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, func(ctx context.Context, tx testpkg.Tx) error {
		verification, err := verifyRequestChildStorageTenant(ctx, tx, fixture.tenant, nil)
		require.NoError(t, err)
		require.True(t, verification.Equal())
		require.EqualValues(t, 3, verification.TargetBookings)
		require.EqualValues(t, 3, verification.TargetSelections)
		return nil
	}))
}

func TestRequestChildStorageCompatibilityPreservesContractGolden(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	var before, after string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(o) ORDER BY id)::text FROM enrollment.request_child_offerings o WHERE tenant_id = ?`, fixture.tenant).Scan(t.Context(), &before))
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(o) ORDER BY id)::text FROM enrollment.request_child_offerings o WHERE tenant_id = ?`, fixture.tenant).Scan(t.Context(), &after))
	require.JSONEq(t, before, after, "the previous image must read the same row shape, NULLs, original notes and day arrays")
	_, err := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{Restart: true})
	require.ErrorContains(t, err, "legacy source is not a base table")
}

func TestRequestChildStorageCompatibilityRoutesPreviousImageWrites(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	require.NoError(t, withPhoenixTenantTx(t, db, fixture.tenant, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.NewRaw(`SELECT link.id FROM enrollment.request_child_offerings link WHERE link.request_child_id = ? FOR UPDATE OF link`, fixture.children[0]).Exec(ctx)
		return err
	}), "the previous-image care-exit row lock must remain supported")
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		(tenant_id, request_child_id, care_offering_id, selected_days, notes)
		VALUES (?, ?, ?, '["thu"]', 'rollback submission') RETURNING id`, fixture.tenant, fixture.children[1], fixture.offerings[0]).Scan(t.Context(), &id))
	var manual, notes string
	require.NoError(t, db.NewRaw(`SELECT manual_selected_days::text FROM enrollment.care_offering_bookings WHERE id = ?`, id).Scan(t.Context(), &manual))
	require.JSONEq(t, `["thu"]`, manual)
	require.NoError(t, db.NewRaw(`SELECT notes FROM enrollment.request_child_offering_selections WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?`, fixture.tenant, fixture.children[1], fixture.offerings[0]).Scan(t.Context(), &notes))
	require.Equal(t, "rollback submission", notes)
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET selected_days = '["fri"]', notes = 'rollback adjustment', valid_until = '2031-08-01' WHERE id = ?`, id).Exec(t.Context())
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT manual_selected_days::text FROM enrollment.care_offering_bookings WHERE id = ?`, id).Scan(t.Context(), &manual))
	require.JSONEq(t, `["fri"]`, manual)
	require.NoError(t, db.NewRaw(`SELECT notes FROM enrollment.request_child_offering_selections WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?`, fixture.tenant, fixture.children[1], fixture.offerings[0]).Scan(t.Context(), &notes))
	require.Equal(t, "rollback submission", notes, "compatibility changes must not rewrite submitted choices")
	var snapshot string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(o)::text FROM enrollment.request_child_offerings o WHERE id = ?`, id).Scan(t.Context(), &snapshot))
	_, err = db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE id = ?`, id).Exec(t.Context())
	require.NoError(t, err)
	for range 2 {
		_, err = db.NewRaw(`INSERT INTO enrollment.request_child_offerings SELECT (jsonb_populate_record(NULL::enrollment.request_child_offerings, ?::jsonb)).* ON CONFLICT DO NOTHING`, snapshot).Exec(t.Context())
		require.NoError(t, err, "previous-image care-exit restore SQL must remain supported")
	}
	var restored string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(o)::text FROM enrollment.request_child_offerings o WHERE id = ?`, id).Scan(t.Context(), &restored))
	require.JSONEq(t, snapshot, restored)
}

func TestRequestChildStorageCompatibilityReadsTargetsAndCountsHits(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	var archivedBefore, archivedAfter string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(o)::text FROM enrollment.request_child_offerings_legacy o WHERE id = ?`, fixture.legacyIDs[0]).Scan(t.Context(), &archivedBefore))
	_, err := db.NewRaw(`UPDATE enrollment.care_offering_bookings SET manual_selected_days = '["fri"]', automatic_selected_days = NULL WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	var days string
	require.NoError(t, db.NewRaw(`SELECT selected_days::text FROM enrollment.request_child_offerings WHERE id = ?`, fixture.legacyIDs[0]).Scan(t.Context(), &days))
	require.JSONEq(t, `["fri"]`, days)
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(o)::text FROM enrollment.request_child_offerings_legacy o WHERE id = ?`, fixture.legacyIDs[0]).Scan(t.Context(), &archivedAfter))
	require.Equal(t, archivedBefore, archivedAfter, "new application writes must not maintain the legacy shape")
	var readHits int64
	require.NoError(t, db.NewRaw(`SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM enrollment.request_child_compatibility_reads`).Scan(t.Context(), &readHits))
	require.Positive(t, readHits)
	_, err = db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	var writeHits int64
	require.NoError(t, db.NewRaw(`SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM enrollment.request_child_compatibility_writes`).Scan(t.Context(), &writeHits))
	require.Positive(t, writeHits)
}

func TestRequestChildStorageCompatibilityRejectsEffectiveDayDrift(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET selected_days = '["fri"]' WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.ErrorContains(t, err, "effective days disagree")
	var days string
	require.NoError(t, db.NewRaw(`SELECT selected_days::text FROM enrollment.request_child_offerings WHERE id = ?`, fixture.legacyIDs[0]).Scan(t.Context(), &days))
	require.JSONEq(t, `["mon","tue"]`, days, "a failed compatibility write must leave the original effective care intact")
}

func TestRequestChildStorageCompatibilityEnforcesTwoTenantRLS(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	var schools []requestChildStorageFixture
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
			schools = append(schools, createRequestChildStorageFixture(t, db))
			runBackfill(t, db, RequestChildStorageBackfillOptions{})
		})
	}
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	for i, own := range schools {
		foreign := schools[1-i]
		require.NoError(t, withPhoenixTenantTx(t, db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
			var count int
			if err := tx.NewRaw(`SELECT count(*) FROM enrollment.request_child_offerings`).Scan(ctx, &count); err != nil {
				return err
			}
			require.Equal(t, 5, count)
			result, err := tx.NewRaw(`UPDATE enrollment.request_child_offerings SET notes = 'foreign update' WHERE id = ?`, foreign.legacyIDs[0]).Exec(ctx)
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			require.NoError(t, err)
			require.Zero(t, changed)
			_, err = tx.NewRaw(`INSERT INTO enrollment.request_child_offerings (tenant_id, request_child_id, care_offering_id, selected_days) VALUES (?, ?, ?, '["fri"]')`, own.tenant, own.children[1], own.offerings[0]).Exec(ctx)
			return err
		}))
		err := withPhoenixTenantTx(t, db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
			_, err := tx.NewRaw(`INSERT INTO enrollment.request_child_offerings (tenant_id, request_child_id, care_offering_id) VALUES (?, ?, ?)`, foreign.tenant, foreign.children[1], foreign.offerings[0]).Exec(ctx)
			return err
		})
		require.ErrorContains(t, err, "row-level security")
		err = withPhoenixTenantTx(t, db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
			_, err := tx.NewRaw(`INSERT INTO enrollment.request_child_offerings (tenant_id, request_child_id, care_offering_id) VALUES (?, ?, ?)`, own.tenant, foreign.children[1], own.offerings[0]).Exec(ctx)
			return err
		})
		require.ErrorContains(t, err, "SQLSTATE=23503")
	}
}
