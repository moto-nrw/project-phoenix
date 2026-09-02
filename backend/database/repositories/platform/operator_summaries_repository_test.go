package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// summariesFixture stages two organizations, three schools, two persons, two
// devices and two account-tenant mappings spanning both orgs so each repo
// method has at least one matched and one unmatched row to exercise.
type summariesFixture struct {
	OrgA, OrgB             *platformModels.Organization
	OrgADeleted            *platformModels.Organization
	SchoolA1, SchoolA2     *platformModels.School
	SchoolB1               *platformModels.School
	SchoolADeleted         *platformModels.School
	PersonA1, PersonA2     *userModels.Person
	PersonB1               *userModels.Person
	AccountID1, AccountID2 int64
	DeviceA1ID             int64
	DeviceB1ID             int64
}

func listOperatorSchoolPersons(t *testing.T, ctx context.Context, db *bun.DB, repo platformModels.OperatorSummariesRepository, schoolID int64) []platformModels.OperatorPersonInfo {
	t.Helper()
	var rows []platformModels.OperatorPersonInfo
	err := testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		var err error
		rows, err = repo.PersonsBySchool(adminCtx, schoolID)
		return err
	})
	require.NoError(t, err)
	return rows
}

func listOperatorOrganizationPersons(t *testing.T, ctx context.Context, db *bun.DB, repo platformModels.OperatorSummariesRepository, organizationID int64) []platformModels.OperatorPersonInfo {
	t.Helper()
	var rows []platformModels.OperatorPersonInfo
	err := testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		var err error
		rows, err = repo.PersonsByOrganization(adminCtx, organizationID)
		return err
	})
	require.NoError(t, err)
	return rows
}

func setupSummariesFixture(t *testing.T, db *bun.DB) *summariesFixture {
	t.Helper()
	ctx := testpkg.Ctx(t)
	now := time.Now().UnixNano()

	schoolRepo := platformRepo.NewSchoolRepository(db)

	orgA := &platformModels.Organization{
		Model:  modelBase.Model{ID: now},
		Name:   fmt.Sprintf("Summaries Alpha %d", now),
		Slug:   fmt.Sprintf("sum-alpha-%d", now),
		Active: true,
	}
	orgB := &platformModels.Organization{
		Model:  modelBase.Model{ID: now + 1},
		Name:   fmt.Sprintf("Summaries Beta %d", now),
		Slug:   fmt.Sprintf("sum-beta-%d", now),
		Active: true,
	}
	orgADel := &platformModels.Organization{
		Model:  modelBase.Model{ID: now + 2},
		Name:   fmt.Sprintf("Summaries Trash %d", now),
		Slug:   fmt.Sprintf("sum-trash-%d", now),
		Active: true,
	}
	testpkg.CreateTestOrganization(t, db, orgA)
	testpkg.CreateTestOrganization(t, db, orgB)
	testpkg.CreateTestOrganization(t, db, orgADel)
	_, err := db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, orgADel.ID)
	require.NoError(t, err)

	mkSchool := func(id, orgID int64, name, slug, sub string) *platformModels.School {
		s := &platformModels.School{
			Model:          modelBase.Model{ID: id},
			OrganizationID: orgID,
			Name:           name,
			Slug:           slug,
			Subdomain:      sub,
			Active:         true,
		}
		require.NoError(t, schoolRepo.Create(ctx, s))
		return s
	}

	schoolA1 := mkSchool(now+10, orgA.ID,
		fmt.Sprintf("Alpha One %d", now),
		fmt.Sprintf("alpha-one-%d", now),
		fmt.Sprintf("a1-%d", now))
	schoolA2 := mkSchool(now+11, orgA.ID,
		fmt.Sprintf("Alpha Two %d", now),
		fmt.Sprintf("alpha-two-%d", now),
		fmt.Sprintf("a2-%d", now))
	schoolB1 := mkSchool(now+12, orgB.ID,
		fmt.Sprintf("Beta One %d", now),
		fmt.Sprintf("beta-one-%d", now),
		fmt.Sprintf("b1-%d", now))
	schoolADel := mkSchool(now+13, orgA.ID,
		fmt.Sprintf("Alpha Trash %d", now),
		fmt.Sprintf("alpha-trash-%d", now),
		fmt.Sprintf("at-%d", now))
	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolADel.ID)
	require.NoError(t, err)

	// Two persons in schoolA1, one in schoolB1, plus one soft-deleted that must
	// not contribute to any count. personATrash sits in the soft-deleted school
	// so PersonsBySchool can assert that drilling into a Papierkorb school does
	// not surface persons.
	personA1 := testpkg.CreateTestPersonForTenant(t, db, schoolA1.ID, "Anna", fmt.Sprintf("AlphaOne-%d", now))
	personA2 := testpkg.CreateTestPersonForTenant(t, db, schoolA2.ID, "Adam", fmt.Sprintf("AlphaTwo-%d", now))
	personB1 := testpkg.CreateTestPersonForTenant(t, db, schoolB1.ID, "Bea", fmt.Sprintf("BetaOne-%d", now))
	personDeleted := testpkg.CreateTestPersonForTenant(t, db, schoolA1.ID, "Ghost", fmt.Sprintf("Deleted-%d", now))
	_, err = db.ExecContext(ctx, `UPDATE users.persons SET deleted_at = NOW() WHERE id = ?`, personDeleted.ID)
	require.NoError(t, err)
	personATrash := testpkg.CreateTestPersonForTenant(t, db, schoolADel.ID, "Trash", fmt.Sprintf("AlphaTrash-%d", now))

	// Two accounts. Account 1 is mapped active to BOTH schoolA1 and schoolA2 to
	// pin the DISTINCT-account semantics. Account 2 is mapped to schoolB1 only.
	// A third "inactive" mapping must NOT contribute to the active-count tally.
	acct1 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("sum-acct-1-%d", now))
	acct2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("sum-acct-2-%d", now))
	testpkg.MapAccountToTenant(t, db, acct1.ID, schoolA1.ID)
	testpkg.MapAccountToTenant(t, db, acct1.ID, schoolA2.ID)
	testpkg.MapAccountToTenant(t, db, acct2.ID, schoolB1.ID)
	// Inactive mapping for acct2 against schoolA1: must be ignored by all counts.
	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		VALUES (?, ?, 'inactive', NOW(), NOW())
		ON CONFLICT (account_id, tenant_id) DO UPDATE SET status = 'inactive'`,
		acct2.ID, schoolA1.ID)
	require.NoError(t, err)

	// One device per school under each org.
	devA1 := testpkg.CreateTestDeviceForTenant(t, db, schoolA1.ID, "sum-dev-a1")
	devB1 := testpkg.CreateTestDeviceForTenant(t, db, schoolB1.ID, "sum-dev-b1")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id IN (?, ?)`, acct1.ID, acct2.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id IN (?, ?)`, acct1.ID, acct2.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM iot.devices WHERE id IN (?, ?)`, devA1.ID, devB1.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users.persons WHERE id IN (?, ?, ?, ?, ?)`,
			personA1.ID, personA2.ID, personB1.ID, personDeleted.ID, personATrash.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id IN (?, ?, ?, ?)`,
			schoolA1.ID, schoolA2.ID, schoolB1.ID, schoolADel.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id IN (?, ?, ?)`,
			orgA.ID, orgB.ID, orgADel.ID)
	})

	return &summariesFixture{
		OrgA: orgA, OrgB: orgB, OrgADeleted: orgADel,
		SchoolA1: schoolA1, SchoolA2: schoolA2, SchoolB1: schoolB1, SchoolADeleted: schoolADel,
		PersonA1: personA1, PersonA2: personA2, PersonB1: personB1,
		AccountID1: acct1.ID, AccountID2: acct2.ID,
		DeviceA1ID: devA1.ID, DeviceB1ID: devB1.ID,
	}
}

func TestOperatorSummariesRepository_Stats(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	repo := platformRepo.NewOperatorSummariesRepository(db)
	ctx := context.Background()

	before, err := repo.Stats(ctx)
	require.NoError(t, err)
	require.NotNil(t, before)

	setupSummariesFixture(t, db)

	after, err := repo.Stats(ctx)
	require.NoError(t, err)
	require.NotNil(t, after)

	// Other test packages run in parallel against the same database, so the
	// global counts drift between the two snapshots. The contract under test
	// is that this fixture contributes AT LEAST its non-deleted entities while
	// soft-deleted rows are filtered out — assertions therefore use >= deltas
	// to stay stable against concurrent inserts elsewhere.
	assert.GreaterOrEqual(t, after.TraegerCount, before.TraegerCount+2, "fixture must add 2 non-deleted orgs (deleted excluded)")
	assert.GreaterOrEqual(t, after.SchulenCount, before.SchulenCount+3, "fixture must add 3 non-deleted schools (deleted excluded)")
	assert.GreaterOrEqual(t, after.KontenCount, before.KontenCount+2, "fixture must add 2 DISTINCT active accounts")
	assert.GreaterOrEqual(t, after.GeraeteCount, before.GeraeteCount+2, "fixture must add 2 devices")
}

func TestOperatorSummariesRepository_OrganizationSummaries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// Person counts come from the People Directory composition (#2661).
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.OperatorSummaries
	ctx := testpkg.Ctx(t)

	fix := setupSummariesFixture(t, db)

	all, err := repo.OrganizationSummaries(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)

	byID := map[int64]*platformModels.OrganizationSummary{}
	for _, o := range all {
		byID[o.ID] = o
	}

	// orgA: 2 active schools (A1, A2; trashed A_del excluded), 1 distinct active
	// account (acct1 active in both), 1 device (A1 only), 2 non-deleted persons
	// (the soft-deleted ghost in A1 must not count).
	a, ok := byID[fix.OrgA.ID]
	require.True(t, ok, "orgA must appear in summaries")
	assert.Equal(t, 2, a.SchulenCount, "soft-deleted school must not count")
	assert.Equal(t, 1, a.KontenCount, "DISTINCT account in scope")
	assert.Equal(t, 1, a.GeraeteCount)
	assert.Equal(t, 2, a.PersonenCount, "soft-deleted person must not count")
	assert.Nil(t, a.DeletedAt)

	b, ok := byID[fix.OrgB.ID]
	require.True(t, ok)
	assert.Equal(t, 1, b.SchulenCount)
	assert.Equal(t, 1, b.KontenCount)
	assert.Equal(t, 1, b.GeraeteCount)
	assert.Equal(t, 1, b.PersonenCount)

	// Soft-deleted org MUST appear so the operator can see it in Papierkorb,
	// but its counts must reflect non-deleted children only (none).
	d, ok := byID[fix.OrgADeleted.ID]
	require.True(t, ok, "soft-deleted org must still appear")
	require.NotNil(t, d.DeletedAt)
	assert.Equal(t, 0, d.SchulenCount)
	assert.Equal(t, 0, d.KontenCount)
	assert.Equal(t, 0, d.GeraeteCount)
	assert.Equal(t, 0, d.PersonenCount)
}

func TestOperatorSummariesRepository_SchoolSummaries_Global(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// Person counts come from the People Directory composition (#2661).
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.OperatorSummaries
	ctx := testpkg.Ctx(t)

	fix := setupSummariesFixture(t, db)

	all, err := repo.SchoolSummaries(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)

	byID := map[int64]*platformModels.SchoolSummary{}
	for _, s := range all {
		byID[s.ID] = s
	}

	// schoolA1: 1 active account mapping (acct1; the inactive acct2 row must be
	// ignored), 1 device, 1 non-deleted person.
	a1, ok := byID[fix.SchoolA1.ID]
	require.True(t, ok)
	assert.Equal(t, fix.OrgA.Name, a1.OrganizationName, "OrganizationName must be denormalized")
	assert.Equal(t, 1, a1.KontenCount, "inactive mapping must not count")
	assert.Equal(t, 1, a1.GeraeteCount)
	assert.Equal(t, 1, a1.PersonenCount, "soft-deleted person must not count")

	// schoolA2 has no devices, but has 1 active account (acct1) and 1 person.
	a2 := byID[fix.SchoolA2.ID]
	require.NotNil(t, a2)
	assert.Equal(t, 1, a2.KontenCount)
	assert.Equal(t, 0, a2.GeraeteCount)
	assert.Equal(t, 1, a2.PersonenCount)

	// schoolADeleted appears in the global list with deleted_at populated.
	del, ok := byID[fix.SchoolADeleted.ID]
	require.True(t, ok)
	require.NotNil(t, del.DeletedAt)
}

func TestOperatorSummariesRepository_SchoolSummariesByOrganization(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// Person counts come from the People Directory composition (#2661).
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.OperatorSummaries
	ctx := testpkg.Ctx(t)

	fix := setupSummariesFixture(t, db)

	t.Run("scopes to organization", func(t *testing.T) {
		rows, err := repo.SchoolSummariesByOrganization(ctx, fix.OrgA.ID)
		require.NoError(t, err)

		ids := map[int64]bool{}
		for _, s := range rows {
			ids[s.ID] = true
			assert.Equal(t, fix.OrgA.ID, s.OrganizationID)
		}
		assert.True(t, ids[fix.SchoolA1.ID])
		assert.True(t, ids[fix.SchoolA2.ID])
		assert.True(t, ids[fix.SchoolADeleted.ID], "soft-deleted school must still appear in org-scoped list (Papierkorb)")
		assert.False(t, ids[fix.SchoolB1.ID], "schools from other orgs must not leak into the response")
	})

	t.Run("returns empty slice when org has no schools", func(t *testing.T) {
		rows, err := repo.SchoolSummariesByOrganization(ctx, 999999999)
		require.NoError(t, err)
		assert.NotNil(t, rows, "must return [] not nil so JSON encodes as array")
		assert.Empty(t, rows)
	})
}

func TestOperatorSummariesRepository_PersonsBySchool(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.OperatorSummaries
	ctx := testpkg.Ctx(t)

	fix := setupSummariesFixture(t, db)

	t.Run("returns persons for active school with org context", func(t *testing.T) {
		rows := listOperatorSchoolPersons(t, ctx, db, repo, fix.SchoolA1.ID)

		ids := map[int64]platformModels.OperatorPersonInfo{}
		for _, p := range rows {
			ids[p.ID] = p
		}

		a1, ok := ids[fix.PersonA1.ID]
		require.True(t, ok)
		assert.Equal(t, fix.SchoolA1.ID, a1.SchoolID)
		assert.Equal(t, fix.OrgA.ID, a1.OrganizationID)
		assert.Equal(t, fix.OrgA.Name, a1.OrganizationName)

		// Persons from other schools must not appear.
		_, leaked := ids[fix.PersonA2.ID]
		assert.False(t, leaked, "persons from sibling schools must not leak")
		_, leakedB := ids[fix.PersonB1.ID]
		assert.False(t, leakedB, "persons from other orgs must not leak")
	})

	t.Run("returns empty for soft-deleted school", func(t *testing.T) {
		// Drilling into a Papierkorb school must not surface persons, matching
		// PersonsByOrganization which already excludes soft-deleted schools.
		rows := listOperatorSchoolPersons(t, ctx, db, repo, fix.SchoolADeleted.ID)
		assert.NotNil(t, rows, "must return [] not nil so JSON encodes as array")
		assert.Empty(t, rows, "soft-deleted school must not surface its persons")
	})

	t.Run("is_staff ignores soft-deleted staff rows", func(t *testing.T) {
		bg := context.Background()
		var staffID int64
		err := db.QueryRowContext(bg,
			`INSERT INTO users.staff (person_id, tenant_id, created_at, updated_at)
			 VALUES (?, ?, NOW(), NOW()) RETURNING id`,
			fix.PersonA1.ID, fix.SchoolA1.ID).Scan(&staffID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(bg, `DELETE FROM users.staff WHERE id = ?`, staffID)
		})

		isStaff := func() bool {
			rows := listOperatorSchoolPersons(t, ctx, db, repo, fix.SchoolA1.ID)
			for _, p := range rows {
				if p.ID == fix.PersonA1.ID {
					return p.IsStaff
				}
			}
			require.FailNow(t, "person not found in school person list")
			return false
		}
		assert.True(t, isStaff(), "live staff row must mark the person as staff")

		_, err = db.ExecContext(bg, `UPDATE users.staff SET deleted_at = NOW() WHERE id = ?`, staffID)
		require.NoError(t, err)
		assert.False(t, isStaff(), "soft-deleted (offboarded) staff row must not mark the person as staff")
	})
}

func TestOperatorSummariesRepository_PersonsByOrganization(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.OperatorSummaries
	ctx := context.Background()

	fix := setupSummariesFixture(t, db)

	rows := listOperatorOrganizationPersons(t, ctx, db, repo, fix.OrgA.ID)

	ids := map[int64]platformModels.OperatorPersonInfo{}
	for _, p := range rows {
		ids[p.ID] = p
	}

	// PersonA1 (in schoolA1) and PersonA2 (in schoolA2) must both surface.
	assert.Contains(t, ids, fix.PersonA1.ID)
	assert.Contains(t, ids, fix.PersonA2.ID)
	// PersonB1 belongs to orgB and must not leak.
	assert.NotContains(t, ids, fix.PersonB1.ID)

	t.Run("missing org returns empty slice", func(t *testing.T) {
		rows := listOperatorOrganizationPersons(t, ctx, db, repo, 999999999)
		assert.NotNil(t, rows)
		assert.Empty(t, rows)
	})
}
