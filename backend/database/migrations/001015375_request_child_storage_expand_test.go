package migrations

import (
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRequestChildStorageExpandStartsEmpty(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, table := range []string{"enrollment.request_child_offering_selections", "enrollment.care_offering_bookings"} {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(t.Context(), &exists))
		require.True(t, exists, "%s must exist after Expand", table)
		var count int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s`, table)).Scan(t.Context(), &count))
		require.Zero(t, count, "%s must not be backfilled or dual-written", table)
	}
}

func TestRequestChildStorageExpandColumnOwnership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	common := map[string]string{
		"id": "bigint:NO", "tenant_id": "bigint:NO", "request_child_id": "bigint:NO",
		"care_offering_id": "bigint:NO", "created_at": "timestamp with time zone:NO",
	}
	for table, owned := range map[string]map[string]string{
		"request_child_offering_selections": {"selected_days": "jsonb:YES", "notes": "text:YES"},
		"care_offering_bookings": {
			"manual_selected_days": "jsonb:YES", "automatic_selected_days": "jsonb:YES",
			"valid_from": "date:YES", "valid_until": "date:YES", "updated_at": "timestamp with time zone:NO",
		},
	} {
		var columns []struct {
			Name string
			Kind string
		}
		require.NoError(t, db.NewRaw(`SELECT column_name AS name, data_type || ':' || is_nullable AS kind
			FROM information_schema.columns WHERE table_schema = 'enrollment' AND table_name = ?`, table).Scan(t.Context(), &columns))
		actual := make(map[string]string)
		for _, column := range columns {
			actual[column.Name] = column.Kind
		}
		for name, kind := range common {
			owned[name] = kind
		}
		require.Equal(t, owned, actual, "field ownership for %s", table)
	}
}

func TestRequestChildStorageExpandDoesNotCopyOldTraffic(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, requestChildStorageExpandDown(t.Context(), db))
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Expand compatibility")
	var legacyID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		(tenant_id, request_child_id, care_offering_id, selected_days, manual_selected_days,
		 automatic_selected_days, notes, valid_from, valid_until)
		VALUES (?, ?, ?, '["mon","tue"]', '["mon"]', '["tue"]', 'submitted note', '2026-08-01', '2027-08-01')
		RETURNING id`, testpkg.Tenant(t), childID, offering.ID).Scan(t.Context(), &legacyID))
	var before, after string
	require.NoError(t, db.NewRaw(`SELECT row_to_json(o)::text FROM enrollment.request_child_offerings o WHERE id = ?`, legacyID).Scan(t.Context(), &before))
	require.NoError(t, requestChildStorageExpandUp(t.Context(), db))
	require.NoError(t, db.NewRaw(`SELECT row_to_json(o)::text FROM enrollment.request_child_offerings o WHERE id = ?`, legacyID).Scan(t.Context(), &after))
	require.Equal(t, before, after, "Expand must preserve every legacy column")
	_, err := db.NewRaw(`UPDATE enrollment.request_child_offerings SET notes = 'legacy update' WHERE id = ?`, legacyID).Exec(t.Context())
	require.NoError(t, err)
	var note string
	require.NoError(t, db.NewRaw(`SELECT notes FROM enrollment.request_child_offerings WHERE id = ?`, legacyID).Scan(t.Context(), &note))
	require.Equal(t, "legacy update", note)
	for _, table := range strings.Fields("enrollment.request_child_offering_selections enrollment.care_offering_bookings") {
		var count int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s`, table)).Scan(t.Context(), &count))
		require.Zero(t, count, "old traffic must not populate %s", table)
	}
	require.NoError(t, requestChildStorageExpandDown(t.Context(), db))
	for _, relation := range []string{"enrollment.request_child_offering_selections", "enrollment.care_offering_bookings", "enrollment.request_children_expand_tenant_id", "enrollment.care_offerings_expand_tenant_id"} {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, relation).Scan(t.Context(), &exists))
		require.False(t, exists, "rollback must remove %s", relation)
	}
	require.NoError(t, db.NewRaw(`SELECT notes FROM enrollment.request_child_offerings WHERE id = ?`, legacyID).Scan(t.Context(), &note))
	require.Equal(t, "legacy update", note)
	require.NoError(t, requestChildStorageExpandUp(t.Context(), db), "migration must apply again after rollback")
}

func TestRequestChildStorageExpandConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Storage constraints")
	insertSelection := `INSERT INTO enrollment.request_child_offering_selections
		(tenant_id, request_child_id, care_offering_id, selected_days, notes) VALUES (?, ?, ?, '["mon"]', 'original submission')`
	_, err := db.NewRaw(insertSelection, testpkg.Tenant(t), childID, offering.ID).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(insertSelection, testpkg.Tenant(t), childID, offering.ID).Exec(t.Context())
	require.ErrorContains(t, err, "SQLSTATE=23505", "one immutable choice per child and offering")
	require.False(t, tenantHasPrivilege(t, db, "enrollment.request_child_offering_selections", "UPDATE"))
	require.False(t, tenantHasPrivilege(t, db, "enrollment.request_child_offering_selections", "TRUNCATE"))
	insertBooking := `INSERT INTO enrollment.care_offering_bookings
		(tenant_id, request_child_id, care_offering_id, manual_selected_days, automatic_selected_days, valid_from, valid_until)
		VALUES (?, ?, ?, '["mon"]', '["tue"]', ?::date, ?::date)`
	for _, interval := range [][2]any{{nil, "2026-08-01"}, {"2026-08-01", "2027-08-01"}, {"2027-08-01", nil}} {
		_, err = db.NewRaw(insertBooking, testpkg.Tenant(t), childID, offering.ID, interval[0], interval[1]).Exec(t.Context())
		require.NoError(t, err, "adjacent half-open intervals and unbounded endpoints must work")
	}
	for _, interval := range [][2]string{{"2026-08-01", "2026-08-01"}, {"2027-08-01", "2026-08-01"}} {
		_, err = db.NewRaw(insertBooking, testpkg.Tenant(t), childID, offering.ID, interval[0], interval[1]).Exec(t.Context())
		require.Error(t, err, "empty and reversed intervals must fail")
	}
	_, err = db.NewRaw(insertBooking, testpkg.Tenant(t), childID, offering.ID, "2026-09-01", "2026-10-01").Exec(t.Context())
	require.ErrorContains(t, err, "SQLSTATE=23P01", "overlapping intervals must fail")
	var manual, automatic, selected, notes string
	require.NoError(t, db.NewRaw(`SELECT manual_selected_days::text, automatic_selected_days::text
		FROM enrollment.care_offering_bookings WHERE request_child_id = ? AND valid_from = '2026-08-01'`, childID).Scan(t.Context(), &manual, &automatic))
	require.JSONEq(t, `["mon"]`, manual)
	require.JSONEq(t, `["tue"]`, automatic)
	require.NoError(t, db.NewRaw(`SELECT selected_days::text, notes FROM enrollment.request_child_offering_selections WHERE request_child_id = ?`, childID).Scan(t.Context(), &selected, &notes))
	require.JSONEq(t, `["mon"]`, selected)
	require.Equal(t, "original submission", notes)
	require.ErrorContains(t, requestChildStorageExpandDown(t.Context(), db), "target tables are not empty")
	_, err = db.NewRaw(`DELETE FROM enrollment.care_offerings WHERE id = ?`, offering.ID).Exec(t.Context())
	require.ErrorContains(t, err, "SQLSTATE=23503", "referenced offering deletion must be restricted")
	_, err = db.NewRaw(`DELETE FROM enrollment.request_children WHERE id = ?`, childID).Exec(t.Context())
	require.NoError(t, err, "child deletion cascades to both target tables")
	require.NoError(t, requestChildStorageExpandDown(t.Context(), db))
}

func TestRequestChildStorageExpandTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	type tenantFixture struct {
		tenantID, childID, offeringID int64
	}
	fixtures := make([]tenantFixture, 2)
	for i := range fixtures {
		require.True(t, t.Run(fmt.Sprintf("tenant_%d", i), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
			phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
			offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Tenant isolation")
			fixtures[i] = tenantFixture{testpkg.Tenant(t), childID, offering.ID}
		}))
	}
	for _, table := range []string{"enrollment.request_child_offering_selections", "enrollment.care_offering_bookings"} {
		t.Run(table, func(t *testing.T) {
			var enabled, forced bool
			require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class
				WHERE oid = ?::regclass`, table).Scan(t.Context(), &enabled, &forced))
			require.True(t, enabled, "RLS must be enabled")
			require.True(t, forced, "RLS must be forced")
			insert := fmt.Sprintf(`INSERT INTO %s (tenant_id, request_child_id, care_offering_id) VALUES (?, ?, ?)`, table)
			for _, fixture := range fixtures {
				_, err := db.NewRaw(insert, fixture.tenantID, fixture.childID, fixture.offeringID).Exec(t.Context())
				require.NoError(t, err)
			}
			for i, own := range fixtures {
				other := fixtures[1-i]
				assertTenantRowsIsolated(t, db, table, own.tenantID, insert, other.tenantID, other.childID, other.offeringID)
				// RLS alone does not prevent a row labelled with the current
				// tenant from referencing a different school's child or offering.
				// Composite foreign keys must also reject this under superuser.
				_, err := db.NewRaw(insert, own.tenantID, other.childID, own.offeringID).Exec(t.Context())
				require.ErrorContains(t, err, "SQLSTATE=23503", "cross-school child reference")
				_, err = db.NewRaw(insert, own.tenantID, own.childID, other.offeringID).Exec(t.Context())
				require.ErrorContains(t, err, "SQLSTATE=23503", "cross-school offering reference")
			}
		})
	}
}
