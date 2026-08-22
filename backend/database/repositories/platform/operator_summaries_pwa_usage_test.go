package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func assignRoleForTenant(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		Scan(context.Background(), &roleID))
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())`,
		accountID, roleID, tenantID)
	require.NoError(t, err)
}

func recordUsageRow(t *testing.T, db *bun.DB, tenantID, accountID int64, portal string, lastSeen time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO iot.pwa_standalone_usage (tenant_id, account_id, portal, first_seen_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		tenantID, accountID, portal, lastSeen, lastSeen)
	require.NoError(t, err)
}

func usageRowFor(rows []platformModels.SchoolPWAUsageRow, tenantID int64, portal string) *platformModels.SchoolPWAUsageRow {
	for i := range rows {
		if rows[i].TenantID == tenantID && rows[i].Portal == portal {
			return &rows[i]
		}
	}
	return nil
}

// TestOperatorSummariesRepository_PWAUsage pins the #2189 aggregate: the
// denominator buckets active mappings by the same guardian-role predicate the
// push audience filters use, and the numerator only counts accounts inside
// the window AND still in the matching bucket.
func TestOperatorSummariesRepository_PWAUsage(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := platformRepo.NewOperatorSummariesRepository(db)
	ctx := context.Background()
	now := time.Now().UnixNano()

	orgRepo := platformRepo.NewOrganizationRepository(db)
	schoolRepo := platformRepo.NewSchoolRepository(db)

	org := &platformModels.Organization{
		Model:  modelBase.Model{ID: now},
		Name:   fmt.Sprintf("PWA Usage Org %d", now),
		Slug:   fmt.Sprintf("pwa-usage-%d", now),
		Active: true,
	}
	require.NoError(t, orgRepo.Create(ctx, org))

	mkSchool := func(id int64, tag string) *platformModels.School {
		s := &platformModels.School{
			Model:          modelBase.Model{ID: id},
			OrganizationID: org.ID,
			Name:           fmt.Sprintf("PWA %s %d", tag, now),
			Slug:           fmt.Sprintf("pwa-%s-%d", tag, now),
			Subdomain:      fmt.Sprintf("pwa-%s-%d", tag, now),
			Active:         true,
		}
		require.NoError(t, schoolRepo.Create(ctx, s))
		return s
	}
	schoolX := mkSchool(now+10, "x")
	schoolY := mkSchool(now+11, "y")

	acctStaff := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-staff-%d", now))
	acctGuardian := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-guardian-%d", now))
	acctDual := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-dual-%d", now))
	acctInactive := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-inactive-%d", now))

	testpkg.MapAccountToTenant(t, db, acctStaff.ID, schoolX.ID)
	testpkg.MapAccountToTenant(t, db, acctGuardian.ID, schoolX.ID)
	testpkg.MapAccountToTenant(t, db, acctDual.ID, schoolX.ID)
	testpkg.MapAccountToTenant(t, db, acctDual.ID, schoolY.ID)
	testpkg.MapAccountToTenant(t, db, acctInactive.ID, schoolX.ID)

	assignRoleForTenant(t, db, acctStaff.ID, schoolX.ID, authModels.BaseRoleUser)
	assignRoleForTenant(t, db, acctGuardian.ID, schoolX.ID, authModels.BaseRoleGuardian)
	// acctDual: staff in school X, guardian in school Y — the buckets must
	// follow the per-tenant role, not any role anywhere.
	assignRoleForTenant(t, db, acctDual.ID, schoolX.ID, authModels.BaseRoleUser)
	assignRoleForTenant(t, db, acctDual.ID, schoolY.ID, authModels.BaseRoleGuardian)
	assignRoleForTenant(t, db, acctInactive.ID, schoolX.ID, authModels.BaseRoleUser)
	_, err := db.ExecContext(ctx, `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, acctInactive.ID)
	require.NoError(t, err)

	// Usage: acctStaff fresh in X (staff), acctDual stale in X (staff, outside
	// the window) and fresh in Y (parent). acctGuardian never reported.
	recordUsageRow(t, db, schoolX.ID, acctStaff.ID, "staff", time.Now())
	recordUsageRow(t, db, schoolX.ID, acctDual.ID, "staff", time.Now().AddDate(0, 0, -40))
	recordUsageRow(t, db, schoolY.ID, acctDual.ID, "parent", time.Now())
	recordUsageRow(t, db, schoolX.ID, acctInactive.ID, "staff", time.Now())

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM iot.pwa_standalone_usage WHERE tenant_id IN (?, ?)`, schoolX.ID, schoolY.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id IN (?, ?, ?, ?)`, acctStaff.ID, acctGuardian.ID, acctDual.ID, acctInactive.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id IN (?, ?, ?, ?)`, acctStaff.ID, acctGuardian.ID, acctDual.ID, acctInactive.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id IN (?, ?, ?, ?)`, acctStaff.ID, acctGuardian.ID, acctDual.ID, acctInactive.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id IN (?, ?)`, schoolX.ID, schoolY.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, org.ID)
	})

	window := 30 * 24 * time.Hour

	t.Run("single school buckets and window", func(t *testing.T) {
		rows, err := repo.PWAUsage(ctx, schoolX.ID, window)
		require.NoError(t, err)

		staff := usageRowFor(rows, schoolX.ID, "staff")
		require.NotNil(t, staff)
		assert.Equal(t, 2, staff.EligibleUsers, "inactive accounts are excluded")
		assert.Equal(t, 1, staff.StandaloneUsers, "acctDual's report is outside the window")

		parent := usageRowFor(rows, schoolX.ID, "parent")
		require.NotNil(t, parent)
		assert.Equal(t, 1, parent.EligibleUsers, "acctGuardian is guardian in X")
		assert.Equal(t, 0, parent.StandaloneUsers, "acctGuardian never reported")

		for _, row := range rows {
			assert.Equal(t, schoolX.ID, row.TenantID, "tenant filter must scope the result")
			assert.LessOrEqual(t, row.StandaloneUsers, row.EligibleUsers)
		}
	})

	t.Run("all schools includes the second school's guardian bucket", func(t *testing.T) {
		rows, err := repo.PWAUsage(ctx, 0, window)
		require.NoError(t, err)

		parentY := usageRowFor(rows, schoolY.ID, "parent")
		require.NotNil(t, parentY)
		assert.Equal(t, 1, parentY.EligibleUsers)
		assert.Equal(t, 1, parentY.StandaloneUsers)

		assert.Nil(t, usageRowFor(rows, schoolY.ID, "staff"), "no staff roles exist in Y")
	})
}
