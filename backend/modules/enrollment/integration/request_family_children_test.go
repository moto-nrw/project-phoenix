package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// familyChildrenRequest inserts a request of a school with a phase of its
// own and answers its ID.
func familyChildrenRequest(t *testing.T, db *bun.DB, tenantID int64, email string) int64 {
	t.Helper()
	ctx := context.Background()
	var phaseID, requestID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', DATE '2027-08-01', DATE '2028-07-31', 'waitlist', 'optional', true) RETURNING id`,
		tenantID, fmt.Sprintf("Kinderzahl %d", testpkg.UniqueSuffix())).Scan(ctx, &phaseID))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, legal_blocks_snapshot, custom_data, source_metadata, status_token, submitted_at)
		VALUES (?, ?, 'Eva', 'Muster', ?, '{}'::jsonb, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, ?, NOW()) RETURNING id`,
		tenantID, phaseID, email, fmt.Sprintf("family-children-%d", testpkg.UniqueSuffix())).Scan(ctx, &requestID))
	return requestID
}

// SetRequestFamilyChildren names the school in its own filter (#6335): run
// below row level security, a request of another school stays untouched.
// Removing the tenant filter turns this test red.
func TestSetRequestFamilyChildrenKeepsTheSchoolBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own, other := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, own)
	testpkg.EnsureTestTenant(t, db, other)
	ownRequest := familyChildrenRequest(t, db, own, "eva@example.org")
	otherRequest := familyChildrenRequest(t, db, other, "olga@example.org")
	store := postgres.NewStore(postgres.NewDatabase(), func(ctx context.Context) (int64, error) { return tenant.FromContext(ctx), nil })

	stored := func(id int64) *int {
		var children *int
		require.NoError(t, db.NewRaw(`SELECT family_children FROM enrollment.requests WHERE id = ?`, id).Scan(context.Background(), &children))
		return children
	}
	err := testpkg.WithinTenantContext(t, context.Background(), db, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, ok := testpkg.TransactionFromContext(ctx)
		require.True(t, ok)
		var bypasses bool
		require.NoError(t, tx.(bun.Tx).NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		require.ErrorIs(t, store.SetRequestFamilyChildren(ctx, otherRequest, 4), enrollment.ErrRequestNotFound)
		return store.SetRequestFamilyChildren(ctx, ownRequest, 3)
	})
	require.NoError(t, err)
	assert.Nil(t, stored(otherRequest), "the other school's request keeps no number")
	require.NotNil(t, stored(ownRequest))
	assert.Equal(t, 3, *stored(ownRequest))
}
