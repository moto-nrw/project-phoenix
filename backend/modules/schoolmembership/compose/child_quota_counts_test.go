package compose

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestChildQuotaCountsCountTheKontingentzahlOfEverySchool pins the operator
// overview's Kontingentzahl (#3568): the rule every membership write is
// checked against, actively managed children plus pending ones whose care
// starts after the count day. Inactive, graduated, deleted and ended children
// and a pending child whose start has come do not count, and another school's
// children stay in that school's entry.
func TestChildQuotaCountsCountTheKontingentzahlOfEverySchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	_, otherTenantID := otherTenantContext(t, db)

	testpkg.CreateTestStudent(t, db, "Aktiv", "Zählt", "1a")
	pending := testpkg.CreateTestStudent(t, db, "Vorgemerkt", "Zählt", "1a")
	setMembership(t, db, tenantID, pending.ID, "status = 'pending', enrolled_from = '2099-08-01'")
	pendingStarted := testpkg.CreateTestStudent(t, db, "Vorgemerkt", "Begonnen", "1a")
	setMembership(t, db, tenantID, pendingStarted.ID, "status = 'pending', enrolled_from = '2099-06-15'")
	for _, status := range []string{"inactive", "alumnus"} {
		student := testpkg.CreateTestStudent(t, db, "Frei", status, "1a")
		setMembership(t, db, tenantID, student.ID, "status = ?", status)
	}
	deleted := testpkg.CreateTestStudent(t, db, "Frei", "Gelöscht", "1a")
	setMembership(t, db, tenantID, deleted.ID, "deleted_at = NOW()")
	ended := testpkg.CreateTestStudent(t, db, "Frei", "Beendet", "1a")
	setMembership(t, db, tenantID, ended.ID, "enrolled_until = '2099-06-14'")
	testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Andere", "Schule", "1a")

	counts := childQuotaCounts{today: func() string { return "2099-06-15" }}
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		byTenant, err := counts.CountChildQuotaByTenant(adminCtx)
		require.NoError(t, err)
		require.Equal(t, 2, byTenant[tenantID])
		require.Equal(t, 1, byTenant[otherTenantID])
		return nil
	}))
}

// TestChildQuotaCountsRefuseATenantTransaction pins that the count fails
// instead of silently counting one school outside the administrative
// transaction.
func TestChildQuotaCountsRefuseATenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	counts := NewChildQuotaCounts()

	_, err := counts.CountChildQuotaByTenant(testpkg.Ctx(t))
	require.ErrorContains(t, err, "transaction is required")

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(tenantCtx context.Context) error {
		_, err := counts.CountChildQuotaByTenant(tenantCtx)
		require.ErrorContains(t, err, "administrative transaction")
		return nil
	}))
}
