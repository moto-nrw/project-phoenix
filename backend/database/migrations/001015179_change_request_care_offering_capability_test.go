package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestChangeRequestCareOfferingCapabilityMigration_BackfillsLegacyRowsEnabled(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// A migration-reset test database is already on the post-1.15.179 schema.
	// Insert a row without naming the new column, which is the same operation
	// PostgreSQL performs for pre-existing rows when ADD COLUMN installs a
	// non-null default. Do not run the down migration here: dropping a shared
	// schema column races service packages when `go test ./...` runs in parallel.

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	phaseID := insertRequestChildSourcePhase(t, db, tenantID)
	requestID := insertRequestChildSourceRequest(t, db, tenantID, phaseID)

	var changeRequestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.change_requests (
			tenant_id, request_id, status, base_snapshot, proposed_snapshot, diff_json
		)
		VALUES (?, ?, 'pending_review', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)
		RETURNING id
	`, tenantID, requestID).Scan(ctx, &changeRequestID))

	var pinnedEnabled bool
	require.NoError(t, db.NewRaw(`
		SELECT care_offerings_enabled_at_creation
		FROM enrollment.change_requests
		WHERE id = ?
	`, changeRequestID).Scan(ctx, &pinnedEnabled))
	require.True(t, pinnedEnabled)
}

func insertRequestChildSourcePhase(t *testing.T, db *testpkg.DB, tenantID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.phases (
			tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active
		)
		VALUES (?, ?, 'school_year', '2026-09-01', '2027-07-31', 'waitlist', 'optional', TRUE)
		RETURNING id
	`, tenantID, "Manual Backfill Guard").Scan(context.Background(), &id))
	return id
}

func insertRequestChildSourceRequest(t *testing.T, db *testpkg.DB, tenantID, phaseID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests (
			tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, custom_data, status_token
		)
		VALUES (?, ?, 'Eva', 'Roster', ?, '{}'::jsonb, '{}'::jsonb, ?)
		RETURNING id
	`, tenantID, phaseID, "manual-roster@example.test", fmt.Sprintf("manual-roster-token-%d", tenantID)).Scan(context.Background(), &id))
	return id
}
