package migrations

import (
	"context"
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
