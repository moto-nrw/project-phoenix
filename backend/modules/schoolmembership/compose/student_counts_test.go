package compose

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestActiveStudentCountsCountOnlyLiveActiveMemberships pins the billing rule
// of #2791: only a live membership in status active counts. Pending,
// inactive (care ended), alumnus and soft-deleted memberships do not.
func TestActiveStudentCountsCountOnlyLiveActiveMemberships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	testpkg.CreateTestStudent(t, db, "Aktiv", "Eins", "1a")
	testpkg.CreateTestStudent(t, db, "Aktiv", "Zwei", "1a")
	for _, status := range []string{"pending", "inactive", "alumnus"} {
		student := testpkg.CreateTestStudent(t, db, "Nicht", status, "1a")
		_, err := db.ExecContext(ctx, "UPDATE users.student_school_memberships SET status = ? WHERE tenant_id = ? AND student_profile_id = ?", status, tenantID, student.ID)
		require.NoError(t, err)
	}
	deleted := testpkg.CreateTestStudent(t, db, "Gelöscht", "Aktiv", "1a")
	_, err := db.ExecContext(ctx, "UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND student_profile_id = ?", tenantID, deleted.ID)
	require.NoError(t, err)

	counts := NewActiveStudentCounts()
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		byTenant, err := counts.CountActiveStudentsByTenant(adminCtx)
		require.NoError(t, err)
		require.Equal(t, 2, byTenant[tenantID])
		return nil
	}))
}

// TestActiveStudentCountsRefuseATenantTransaction pins that the count fails
// instead of silently counting one school when it runs outside the
// administrative transaction.
func TestActiveStudentCountsRefuseATenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	counts := NewActiveStudentCounts()

	_, err := counts.CountActiveStudentsByTenant(testpkg.Ctx(t))
	require.ErrorContains(t, err, "transaction is required")

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(tenantCtx context.Context) error {
		_, err := counts.CountActiveStudentsByTenant(tenantCtx)
		require.ErrorContains(t, err, "administrative transaction")
		return nil
	}))
}
