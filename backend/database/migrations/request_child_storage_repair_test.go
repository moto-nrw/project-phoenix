package migrations

import (
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCompatibilityRepairRollsBackFailedBatchAndRepairsSchema(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	for _, id := range fixture.legacyIDs[:2] {
		_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings_legacy SET selected_days = '["sun"]'::jsonb WHERE id = ?`, id).Exec(t.Context())
		require.NoError(t, err)
	}
	archiveSnapshot := func() string {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM enrollment.request_child_offerings_legacy a`).Scan(t.Context(), &snapshot))
		return snapshot
	}
	before := archiveSnapshot()
	_, err := db.ExecContext(t.Context(), `
		CREATE SEQUENCE public.repair_test_attempts;
		CREATE FUNCTION public.fail_second_repair_row() RETURNS trigger LANGUAGE plpgsql AS $fn$
		BEGIN
			IF nextval('public.repair_test_attempts') = 2 THEN RAISE EXCEPTION 'injected repair batch failure'; END IF;
			RETURN NEW;
		END $fn$;
		CREATE TRIGGER fail_second_repair_row BEFORE UPDATE ON enrollment.request_child_offerings_legacy
		FOR EACH ROW EXECUTE FUNCTION public.fail_second_repair_row();`)
	require.NoError(t, err)
	options := CompatibilityRepairOptions{BatchSize: 2, TenantIDs: []int64{fixture.tenant}}
	report, err := RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.ErrorContains(t, err, "injected repair batch failure")
	require.Zero(t, report.RepairedRows)
	require.JSONEq(t, before, archiveSnapshot(), "a failed batch cannot commit its earlier row")
	report, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.NoError(t, err)
	require.EqualValues(t, 2, report.RepairedRows)
	// A current-image booking change makes this row use the live view formula.
	_, err = db.NewRaw(`UPDATE enrollment.care_offering_bookings SET manual_selected_days = '["thu"]'::jsonb WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `CREATE OR REPLACE FUNCTION enrollment.request_child_effective_days(manual jsonb, automatic jsonb)
		RETURNS jsonb LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path = pg_catalog AS $fn$ SELECT '[]'::jsonb $fn$`)
	require.NoError(t, err)
	options.VerifyOnly = true
	_, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.ErrorContains(t, err, "checksum drift", "verification must not trust the compatibility normalization function")
	options.VerifyOnly = false
	report, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.NoError(t, err)
	require.Zero(t, report.RepairedRows)
	require.True(t, report.Tenants[0].EffectiveChecksumsEqual)
}

func TestCompatibilityRepairResumesWithoutChangingOwnerData(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	var schools []requestChildStorageFixture
	for _, name := range []string{"first", "second"} {
		require.True(t, t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
			schools = append(schools, createRequestChildStorageFixture(t, db))
		}))
	}
	_, backfillErr := RunRequestChildStorageBackfill(t.Context(), db, RequestChildStorageBackfillOptions{})
	require.NoError(t, backfillErr)
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	ownerSnapshot := func() string {
		var snapshot string
		require.NoError(t, db.NewRaw(`SELECT jsonb_build_object(
			'bookings', (SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM enrollment.care_offering_bookings b),
			'selections', (SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM enrollment.request_child_offering_selections s),
			'checkpoints', (SELECT jsonb_agg(to_jsonb(c) ORDER BY tenant_id) FROM enrollment.request_child_storage_backfill_checkpoints c))::text`).Scan(t.Context(), &snapshot))
		return snapshot
	}
	before := ownerSnapshot()
	for _, id := range []int64{schools[0].legacyIDs[0], schools[0].legacyIDs[1], schools[1].legacyIDs[0]} {
		_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings_legacy SET selected_days = '["sun"]'::jsonb WHERE id = ?`, id).Exec(t.Context())
		require.NoError(t, err)
	}
	options := CompatibilityRepairOptions{BatchSize: 1, TenantIDs: []int64{schools[0].tenant}, VerifyOnly: true}
	report, err := RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.ErrorContains(t, err, "checksum drift")
	require.Len(t, report.Tenants, 1)
	require.False(t, report.Tenants[0].EffectiveChecksumsEqual)
	options.VerifyOnly = false
	interrupted := errors.New("interrupted after committed batch")
	report, err = repairRequestChildStorageCompatibility(t.Context(), db, options, func() error { return interrupted })
	require.ErrorIs(t, err, interrupted)
	require.EqualValues(t, 1, report.RepairedRows)
	require.Equal(t, 1, report.Batches)
	require.JSONEq(t, before, ownerSnapshot())
	report, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.NoError(t, err)
	require.EqualValues(t, 1, report.RepairedRows, "resume skips the already repaired row")
	require.True(t, report.Tenants[0].EffectiveChecksumsEqual)
	report, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.NoError(t, err)
	require.Zero(t, report.RepairedRows, "identical retry does not rewrite healthy rollback metadata")
	options.TenantIDs = []int64{schools[1].tenant}
	options.VerifyOnly = true
	_, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.ErrorContains(t, err, "checksum drift", "repair must not cross its school filter")
	options.VerifyOnly = false
	_, err = RepairRequestChildStorageCompatibility(t.Context(), db, options)
	require.NoError(t, err)
	require.JSONEq(t, before, ownerSnapshot(), "repair never changes owner data or the frozen cutover evidence")
}
