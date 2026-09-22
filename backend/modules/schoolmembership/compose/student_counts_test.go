package compose

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestActiveStudentCountsCountOnlyLiveActiveMemberships pins the billing rule
// of #2791: only a live membership in status active whose enrollment has not
// ended before the capture date counts, including an immediately activated
// child whose formal start is still ahead. Pending, inactive (care ended),
// alumnus and soft-deleted memberships do not.
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
	future := testpkg.CreateTestStudent(t, db, "Zukünftig", "Aktiv", "1a")
	_, err = db.ExecContext(ctx, "UPDATE users.student_school_memberships SET enrolled_from = '2099-06-16' WHERE tenant_id = ? AND student_profile_id = ?", tenantID, future.ID)
	require.NoError(t, err)
	ended := testpkg.CreateTestStudent(t, db, "Beendet", "Aktiv", "1a")
	_, err = db.ExecContext(ctx, "UPDATE users.student_school_memberships SET enrolled_until = '2099-06-14' WHERE tenant_id = ? AND student_profile_id = ?", tenantID, ended.ID)
	require.NoError(t, err)
	boundary := testpkg.CreateTestStudent(t, db, "Am", "Stichtag", "1a")
	_, err = db.ExecContext(ctx, "UPDATE users.student_school_memberships SET enrolled_from = '2099-06-15', enrolled_until = '2099-06-15' WHERE tenant_id = ? AND student_profile_id = ?", tenantID, boundary.ID)
	require.NoError(t, err)

	counts := NewActiveStudentCounts()
	capturedAt := time.Date(2099, time.June, 15, 12, 0, 0, 0, time.UTC)
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		byTenant, err := counts.CountActiveStudentsByTenant(adminCtx, capturedAt)
		require.NoError(t, err)
		require.Equal(t, 4, byTenant[tenantID])
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

	_, err := counts.CountActiveStudentsByTenant(testpkg.Ctx(t), time.Time{})
	require.ErrorContains(t, err, "transaction is required")

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(tenantCtx context.Context) error {
		_, err := counts.CountActiveStudentsByTenant(tenantCtx, time.Time{})
		require.ErrorContains(t, err, "administrative transaction")
		return nil
	}))
}
