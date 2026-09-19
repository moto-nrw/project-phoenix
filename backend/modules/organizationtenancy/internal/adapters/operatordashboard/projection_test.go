package operatordashboard_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/operatordashboard"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Base role names as Identity & Access seeds them. They are literals because
// this package may not import that owner's models.
const (
	roleUser     = "user"
	roleGuardian = "guardian"
)

// ambientTransaction resolves the ambient transaction exactly like the
// Organisation & Tenancy composition does.
func ambientTransaction(ctx context.Context) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("unsupported transaction %T", transaction)
	}
	return tx, nil
}

// activeMemberships spells the Identity & Access active-membership statement
// (AccountTenantRepository.ActiveMemberships, #2721) the root binds; this
// test scope may not import that owner's adapter.
func activeMemberships(ctx context.Context) *bun.SelectQuery {
	db, err := ambientTransaction(ctx)
	if err != nil {
		panic(err)
	}
	return db.NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`"account_tenant".account_id`).
		ColumnExpr(`"account_tenant".tenant_id`).
		Where(`"account_tenant".status = ?`, "active")
}

func newProjection() *operatordashboard.Projection {
	return operatordashboard.New(ambientTransaction, activeMemberships)
}

// TestAccountCountsFailClosedWithoutMembershipQuery pins that the account
// counting reads report an error when composed without the Identity &
// Access membership statement, instead of counting nothing.
func TestAccountCountsFailClosedWithoutMembershipQuery(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	projection := operatordashboard.New(ambientTransaction, nil)
	organizationID := testpkg.Tenant(t)

	withinAdmin(t, db, func(ctx context.Context) error {
		_, err := projection.Counts(ctx)
		require.ErrorContains(t, err, "active membership query is not bound")
		_, err = projection.OrganizationSummaries(ctx)
		require.ErrorContains(t, err, "active membership query is not bound")
		_, err = projection.SchoolSummaries(ctx, nil)
		require.ErrorContains(t, err, "active membership query is not bound")
		_, err = projection.SchoolSummaries(ctx, &organizationID)
		require.ErrorContains(t, err, "active membership query is not bound")
		return nil
	})
}

// withinAdmin runs fn in the administrative transaction production uses for
// the cross-tenant operator reads.
func withinAdmin(t *testing.T, db *bun.DB, fn func(context.Context) error) {
	t.Helper()
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, fn))
}

func createOrganization(t *testing.T, db *bun.DB, name, slug string) *platformModels.Organization {
	t.Helper()
	org := &platformModels.Organization{Name: name, Slug: slug, Active: true}
	org.ID = testpkg.UniqueTestTenantID(t)
	return testpkg.CreateTestOrganization(t, db, org)
}

// createSchool inserts a school under an organisation created through
// testpkg.CreateTestOrganization, whose cleanup owns the school's rows.
func createSchool(t *testing.T, db *bun.DB, organizationID int64, name, slug string) int64 {
	t.Helper()
	id := testpkg.UniqueTestTenantID(t)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		 VALUES (?, ?, ?, ?, ?, TRUE)`,
		id, organizationID, name, slug, slug)
	require.NoError(t, err)
	return id
}

func softDelete(t *testing.T, db *bun.DB, table string, id int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`UPDATE ? SET deleted_at = NOW() WHERE id = ?`, bun.Ident(table), id)
	require.NoError(t, err)
}

// summariesFixture stages two organisations, a soft-deleted organisation,
// three live schools, one soft-deleted school and two accounts so each
// projection read has at least one matched and one unmatched row.
type summariesFixture struct {
	OrgA, OrgB, OrgADeleted                   *platformModels.Organization
	SchoolA1, SchoolA2, SchoolB1, SchoolADead int64
	AccountID1, AccountID2                    int64
}

func setupSummariesFixture(t *testing.T, db *bun.DB) *summariesFixture {
	t.Helper()
	ctx := context.Background()
	now := testpkg.UniqueSuffix()

	orgA := createOrganization(t, db, fmt.Sprintf("Summaries Alpha %d", now), fmt.Sprintf("sum-alpha-%d", now))
	orgB := createOrganization(t, db, fmt.Sprintf("Summaries Beta %d", now), fmt.Sprintf("sum-beta-%d", now))
	orgADel := createOrganization(t, db, fmt.Sprintf("Summaries Trash %d", now), fmt.Sprintf("sum-trash-%d", now))
	softDelete(t, db, "platform.organizations", orgADel.ID)

	schoolA1 := createSchool(t, db, orgA.ID, fmt.Sprintf("Alpha One %d", now), fmt.Sprintf("alpha-one-%d", now))
	schoolA2 := createSchool(t, db, orgA.ID, fmt.Sprintf("Alpha Two %d", now), fmt.Sprintf("alpha-two-%d", now))
	schoolB1 := createSchool(t, db, orgB.ID, fmt.Sprintf("Beta One %d", now), fmt.Sprintf("beta-one-%d", now))
	schoolADel := createSchool(t, db, orgA.ID, fmt.Sprintf("Alpha Trash %d", now), fmt.Sprintf("alpha-trash-%d", now))
	softDelete(t, db, "platform.schools", schoolADel)

	// Account 1 is active in BOTH schoolA1 and schoolA2 to pin the
	// DISTINCT-account semantics. Account 2 is active in schoolB1 only; its
	// inactive mapping to schoolA1 must not contribute to any count.
	acct1 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("sum-acct-1-%d", now))
	acct2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("sum-acct-2-%d", now))
	testpkg.MapAccountToTenant(t, db, acct1.ID, schoolA1)
	testpkg.MapAccountToTenant(t, db, acct1.ID, schoolA2)
	testpkg.MapAccountToTenant(t, db, acct2.ID, schoolB1)
	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		VALUES (?, ?, 'inactive', NOW(), NOW())
		ON CONFLICT (account_id, tenant_id) DO UPDATE SET status = 'inactive'`,
		acct2.ID, schoolA1)
	require.NoError(t, err)

	return &summariesFixture{
		OrgA: orgA, OrgB: orgB, OrgADeleted: orgADel,
		SchoolA1: schoolA1, SchoolA2: schoolA2, SchoolB1: schoolB1, SchoolADead: schoolADel,
		AccountID1: acct1.ID, AccountID2: acct2.ID,
	}
}

func counts(t *testing.T, db *bun.DB, projection *operatordashboard.Projection) domain.DashboardCounts {
	t.Helper()
	var result domain.DashboardCounts
	withinAdmin(t, db, func(ctx context.Context) error {
		var err error
		result, err = projection.Counts(ctx)
		return err
	})
	return result
}

func TestProjection_Counts(t *testing.T) {
	t.Parallel()
	// The counts are platform-wide: a disposable clone keeps other tests'
	// inserts out of the two snapshots, so the deltas are exact.
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	projection := newProjection()
	// Materialize this test's own tenant before the first snapshot; fixtures
	// would otherwise create its organisation and school between snapshots.
	testpkg.Tenant(t)

	before := counts(t, db, projection)
	setupSummariesFixture(t, db)
	after := counts(t, db, projection)

	assert.Equal(t, before.Organizations+2, after.Organizations, "fixture adds 2 non-deleted organisations (deleted excluded)")
	assert.Equal(t, before.Schools+3, after.Schools, "fixture adds 3 non-deleted schools (deleted excluded)")
	assert.Equal(t, before.Accounts+2, after.Accounts, "fixture adds 2 DISTINCT active accounts")
}

func TestProjection_OrganizationSummaries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	projection := newProjection()
	fix := setupSummariesFixture(t, db)

	var all []domain.OrganizationSummary
	withinAdmin(t, db, func(ctx context.Context) error {
		var err error
		all, err = projection.OrganizationSummaries(ctx)
		return err
	})
	require.NotEmpty(t, all)

	byID := map[int64]domain.OrganizationSummary{}
	for _, o := range all {
		byID[o.ID] = o
	}

	// orgA: 2 live schools (A1, A2; the trashed one is excluded) and 1
	// distinct active account (acct1 is active in both).
	a, ok := byID[fix.OrgA.ID]
	require.True(t, ok, "orgA must appear in summaries")
	assert.Equal(t, fix.OrgA.Name, a.Name)
	assert.Equal(t, fix.OrgA.Slug, a.Slug)
	assert.True(t, a.Active)
	assert.Equal(t, 2, a.SchoolCount, "soft-deleted school must not count")
	assert.Equal(t, 1, a.AccountCount, "DISTINCT account in scope; inactive mapping ignored")
	assert.Nil(t, a.DeletedAt)

	b, ok := byID[fix.OrgB.ID]
	require.True(t, ok)
	assert.Equal(t, 1, b.SchoolCount)
	assert.Equal(t, 1, b.AccountCount)

	// A soft-deleted organisation still appears (Papierkorb), but its counts
	// reflect non-deleted children only (none).
	d, ok := byID[fix.OrgADeleted.ID]
	require.True(t, ok, "soft-deleted organisation must still appear")
	require.NotNil(t, d.DeletedAt)
	assert.Equal(t, 0, d.SchoolCount)
	assert.Equal(t, 0, d.AccountCount)

	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].Name, all[i].Name, "organisations are ordered by name")
	}
}

func schoolSummaries(t *testing.T, db *bun.DB, projection *operatordashboard.Projection, organizationID *int64) []domain.SchoolSummary {
	t.Helper()
	var rows []domain.SchoolSummary
	withinAdmin(t, db, func(ctx context.Context) error {
		var err error
		rows, err = projection.SchoolSummaries(ctx, organizationID)
		return err
	})
	return rows
}

func TestProjection_SchoolSummaries_Global(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	projection := newProjection()
	fix := setupSummariesFixture(t, db)

	all := schoolSummaries(t, db, projection, nil)
	require.NotEmpty(t, all)

	byID := map[int64]domain.SchoolSummary{}
	for _, s := range all {
		byID[s.ID] = s
	}

	// schoolA1: 1 active mapping (acct1); acct2's inactive mapping is ignored.
	a1, ok := byID[fix.SchoolA1]
	require.True(t, ok)
	assert.Equal(t, fix.OrgA.ID, a1.OrganizationID)
	assert.Equal(t, fix.OrgA.Name, a1.OrganizationName, "OrganizationName must be denormalized")
	assert.Equal(t, 1, a1.AccountCount, "inactive mapping must not count")
	assert.Nil(t, a1.DeletedAt)

	a2, ok := byID[fix.SchoolA2]
	require.True(t, ok)
	assert.Equal(t, 1, a2.AccountCount)

	b1, ok := byID[fix.SchoolB1]
	require.True(t, ok)
	assert.Equal(t, fix.OrgB.Name, b1.OrganizationName)
	assert.Equal(t, 1, b1.AccountCount)

	// The soft-deleted school appears in the global list with deleted_at set.
	del, ok := byID[fix.SchoolADead]
	require.True(t, ok)
	require.NotNil(t, del.DeletedAt)
	assert.Equal(t, 0, del.AccountCount)
}

func TestProjection_SchoolSummaries_ByOrganization(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	projection := newProjection()
	fix := setupSummariesFixture(t, db)

	t.Run("scopes to organization", func(t *testing.T) {
		rows := schoolSummaries(t, db, projection, &fix.OrgA.ID)

		byID := map[int64]domain.SchoolSummary{}
		for _, s := range rows {
			byID[s.ID] = s
			assert.Equal(t, fix.OrgA.ID, s.OrganizationID)
			assert.Equal(t, fix.OrgA.Name, s.OrganizationName)
		}
		require.Contains(t, byID, fix.SchoolA1)
		require.Contains(t, byID, fix.SchoolA2)
		require.Contains(t, byID, fix.SchoolADead, "soft-deleted school must still appear in org-scoped list (Papierkorb)")
		assert.NotContains(t, byID, fix.SchoolB1, "schools from other orgs must not leak into the response")
		assert.Len(t, rows, 3)

		assert.Equal(t, 1, byID[fix.SchoolA1].AccountCount, "inactive mapping must not count")
		assert.Equal(t, 1, byID[fix.SchoolA2].AccountCount)
		assert.NotNil(t, byID[fix.SchoolADead].DeletedAt)

		for i := 1; i < len(rows); i++ {
			assert.LessOrEqual(t, rows[i-1].Name, rows[i].Name, "schools are ordered by name")
		}
	})

	t.Run("returns empty slice when org has no schools", func(t *testing.T) {
		empty := createOrganization(t, db, fmt.Sprintf("Summaries Empty %d", testpkg.UniqueSuffix()),
			fmt.Sprintf("sum-empty-%d", testpkg.UniqueSuffix()))
		rows := schoolSummaries(t, db, projection, &empty.ID)
		assert.NotNil(t, rows, "must return [] not nil so JSON encodes as array")
		assert.Empty(t, rows)
	})
}

func assignRoleForTenant(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	ctx := context.Background()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		OrderExpr("id").
		Limit(1).
		Scan(ctx, &roleID))
	_, err := db.ExecContext(ctx,
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

func usageRowFor(rows []domain.PWAUsageRow, tenantID int64, portal string) *domain.PWAUsageRow {
	for i := range rows {
		if rows[i].TenantID == tenantID && rows[i].Portal == portal {
			return &rows[i]
		}
	}
	return nil
}

func pwaUsage(t *testing.T, db *bun.DB, projection *operatordashboard.Projection, tenantID int64, window time.Duration) []domain.PWAUsageRow {
	t.Helper()
	var rows []domain.PWAUsageRow
	withinAdmin(t, db, func(ctx context.Context) error {
		var err error
		rows, err = projection.PWAUsage(ctx, tenantID, window)
		return err
	})
	return rows
}

// TestProjection_PWAUsage pins the #2189 aggregate: the denominator buckets
// active mappings by the same guardian-role predicate the push audience
// filters use, and the numerator only counts accounts inside the window AND
// still in the matching bucket.
func TestProjection_PWAUsage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	projection := newProjection()
	now := testpkg.UniqueSuffix()

	org := createOrganization(t, db, fmt.Sprintf("PWA Usage Org %d", now), fmt.Sprintf("pwa-usage-%d", now))
	schoolX := createSchool(t, db, org.ID, fmt.Sprintf("PWA x %d", now), fmt.Sprintf("pwa-x-%d", now))
	schoolY := createSchool(t, db, org.ID, fmt.Sprintf("PWA y %d", now), fmt.Sprintf("pwa-y-%d", now))
	schoolZ := createSchool(t, db, org.ID, fmt.Sprintf("PWA z %d", now), fmt.Sprintf("pwa-z-%d", now))

	acctStaff := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-staff-%d", now))
	acctGuardian := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-guardian-%d", now))
	acctDual := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-dual-%d", now))
	acctInactive := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-inactive-%d", now))
	acctDeletedSchool := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-deleted-school-%d", now))

	testpkg.MapAccountToTenant(t, db, acctStaff.ID, schoolX)
	testpkg.MapAccountToTenant(t, db, acctGuardian.ID, schoolX)
	testpkg.MapAccountToTenant(t, db, acctDual.ID, schoolX)
	testpkg.MapAccountToTenant(t, db, acctDual.ID, schoolY)
	testpkg.MapAccountToTenant(t, db, acctInactive.ID, schoolX)
	testpkg.MapAccountToTenant(t, db, acctDeletedSchool.ID, schoolZ)

	assignRoleForTenant(t, db, acctStaff.ID, schoolX, roleUser)
	assignRoleForTenant(t, db, acctGuardian.ID, schoolX, roleGuardian)
	// acctDual: staff in school X, guardian in school Y — the buckets must
	// follow the per-tenant role, not any role anywhere.
	assignRoleForTenant(t, db, acctDual.ID, schoolX, roleUser)
	assignRoleForTenant(t, db, acctDual.ID, schoolY, roleGuardian)
	assignRoleForTenant(t, db, acctInactive.ID, schoolX, roleUser)
	assignRoleForTenant(t, db, acctDeletedSchool.ID, schoolZ, roleUser)
	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, acctInactive.ID)
	require.NoError(t, err)

	// Usage: acctStaff fresh in X (staff), acctDual stale in X (staff, outside
	// the window) and fresh in Y (parent). acctGuardian never reported.
	// The projection compares against its own clock, so the rows are
	// instants relative to now, not calendar dates.
	fresh := time.Now()
	stale := fresh.Add(-40 * 24 * time.Hour)
	recordUsageRow(t, db, schoolX, acctStaff.ID, "staff", fresh)
	recordUsageRow(t, db, schoolX, acctDual.ID, "staff", stale)
	recordUsageRow(t, db, schoolY, acctDual.ID, "parent", fresh)
	recordUsageRow(t, db, schoolX, acctInactive.ID, "staff", fresh)
	recordUsageRow(t, db, schoolZ, acctDeletedSchool.ID, "staff", fresh)
	softDelete(t, db, "platform.schools", schoolZ)

	window := 30 * 24 * time.Hour

	t.Run("single school buckets and window", func(t *testing.T) {
		rows := pwaUsage(t, db, projection, schoolX, window)
		require.Len(t, rows, 2, "one row per portal bucket of school X")

		staff := usageRowFor(rows, schoolX, "staff")
		require.NotNil(t, staff)
		assert.Equal(t, 2, staff.EligibleUsers, "inactive accounts are excluded")
		assert.Equal(t, 1, staff.StandaloneUsers, "acctDual's report is outside the window")

		parent := usageRowFor(rows, schoolX, "parent")
		require.NotNil(t, parent)
		assert.Equal(t, 1, parent.EligibleUsers, "acctGuardian is guardian in X")
		assert.Equal(t, 0, parent.StandaloneUsers, "acctGuardian never reported")

		for _, row := range rows {
			assert.Equal(t, schoolX, row.TenantID, "tenant filter must scope the result")
			assert.LessOrEqual(t, row.StandaloneUsers, row.EligibleUsers)
		}
	})

	t.Run("wider window counts the stale report", func(t *testing.T) {
		rows := pwaUsage(t, db, projection, schoolX, 60*24*time.Hour)
		staff := usageRowFor(rows, schoolX, "staff")
		require.NotNil(t, staff)
		assert.Equal(t, 2, staff.EligibleUsers)
		assert.Equal(t, 2, staff.StandaloneUsers, "acctDual's 40-day-old report is inside a 60-day window")
	})

	t.Run("all schools includes the second school's guardian bucket", func(t *testing.T) {
		rows := pwaUsage(t, db, projection, 0, window)

		parentY := usageRowFor(rows, schoolY, "parent")
		require.NotNil(t, parentY)
		assert.Equal(t, 1, parentY.EligibleUsers)
		assert.Equal(t, 1, parentY.StandaloneUsers)

		assert.Nil(t, usageRowFor(rows, schoolY, "staff"), "no staff roles exist in Y")
		assert.NotNil(t, usageRowFor(rows, schoolX, "staff"), "the unfiltered read covers school X too")
		assert.Nil(t, usageRowFor(rows, schoolZ, "staff"), "soft-deleted schools are excluded")
	})

	t.Run("soft-deleted school yields no buckets", func(t *testing.T) {
		rows := pwaUsage(t, db, projection, schoolZ, window)
		assert.NotNil(t, rows)
		assert.Empty(t, rows)
	})
}
