package parent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ensureGuardianProfile returns the guardian_profile id for the
// (account, tenant) pair, creating it on first call. Uses the existing
// row on subsequent calls to satisfy the (tenant_id, account_id) unique
// index — multiple children under one parent share a single profile.
func ensureGuardianProfile(t *testing.T, db *bun.DB, accountID, tenantID int64) int64 {
	t.Helper()
	bg := context.Background()

	// account_tenants is the membership precondition.
	testpkg.EnsureAccountTenant(t, db, accountID, tenantID)

	var profileID int64
	err := db.NewRaw(`SELECT id FROM users.guardian_profiles WHERE tenant_id = ? AND account_id = ?`,
		tenantID, accountID).Scan(bg, &profileID)
	if err == nil && profileID != 0 {
		return profileID
	}

	uniqueEmail := fmt.Sprintf("link-%d@test", time.Now().UnixNano())
	err = db.NewRaw(`
		INSERT INTO users.guardian_profiles
		  (tenant_id, account_id, has_account, first_name, last_name, email,
		   preferred_contact_method, language_preference)
		VALUES (?, ?, TRUE, 'Anna', 'Beispiel', ?, 'email', 'de')
		RETURNING id
	`, tenantID, accountID, uniqueEmail).Scan(bg, &profileID)
	require.NoError(t, err)
	return profileID
}

// linkChildToAccount adds a students_guardians row tying the student
// to the (account, tenant) guardian profile. Idempotent on the
// (tenant, student, profile) unique index; safe to call multiple times
// for the same triple.
func linkChildToAccount(t *testing.T, db *bun.DB, accountID, tenantID, studentID int64) int64 {
	return linkChildToAccountWithPermissions(t, db, accountID, tenantID, studentID, map[string]bool{
		authorize.GuardianPermissionPortalAccess: true,
	})
}

func linkChildToAccountWithPermissions(
	t *testing.T,
	db *bun.DB,
	accountID, tenantID, studentID int64,
	permissions map[string]bool,
) int64 {
	t.Helper()
	bg := context.Background()
	profileID := ensureGuardianProfile(t, db, accountID, tenantID)
	permissionsJSON, err := json.Marshal(permissions)
	require.NoError(t, err)

	_, err = db.NewRaw(`
		INSERT INTO users.students_guardians
		  (tenant_id, student_id, guardian_profile_id, relationship_type, is_primary,
		   can_pickup, permissions)
		VALUES (?, ?, ?, 'parent', TRUE, TRUE, ?::jsonb)
		ON CONFLICT (tenant_id, student_id, guardian_profile_id) DO NOTHING
	`, tenantID, studentID, profileID, string(permissionsJSON)).Exec(bg)
	require.NoError(t, err)
	return profileID
}

func runAsAdmin(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

// --- ListByAccount ----------------------------------------------------

func TestChildRepository_ListByAccount_RejectsNonPositive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := parentRepo.NewChildRepository(db)

	_, err := repo.ListByAccount(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	_, err = repo.ListByAccount(context.Background(), -1)
	require.Error(t, err)
}

func TestChildRepository_ListByAccount_HappyPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-happy")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lara", "Beispiel", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").
			Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").
			Where("id = ?", student.PersonID).Exec(context.Background())
	})

	linkChildToAccount(t, db, account.ID, tenantID, student.ID)

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	err := runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, student.ID, list[0].StudentID)
	assert.Equal(t, tenantID, list[0].TenantID)
	assert.Equal(t, "Lara", list[0].FirstName)
	assert.Equal(t, "1a", list[0].SchoolClass)
	assert.NotEmpty(t, list[0].SchoolName, "school name must be JOINed in from platform.schools")
	assert.NotEmpty(t, list[0].SchoolSlug)
}

func TestChildRepository_ListByAccount_FiltersInactiveMembership(t *testing.T) {
	t.Parallel()

	// A parent who lost access to a school (account_tenants.status !=
	// 'active') MUST NOT continue to see its children.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-inactive")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lara", "Inactive", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").
			Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").
			Where("id = ?", student.PersonID).Exec(context.Background())
	})
	linkChildToAccount(t, db, account.ID, tenantID, student.ID)

	// Flip the mapping to 'inactive'.
	_, err := db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`,
		account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	assert.Empty(t, list, "inactive account_tenants membership MUST hide the child")
}

func TestChildRepository_ListByAccount_FiltersMissingPortalAccess(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-noportal")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lara", "Noportal", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").
			Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").
			Where("id = ?", student.PersonID).Exec(context.Background())
	})
	linkChildToAccountWithPermissions(t, db, account.ID, tenantID, student.ID, map[string]bool{})

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	assert.Empty(t, list, "linked guardians without parent_portal.access must not see children")
}

func TestChildRepository_ListByAccount_FiltersSoftDeletedPerson(t *testing.T) {
	t.Parallel()

	// Soft-deleted persons (deleted_at IS NOT NULL) must be excluded.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-softdel")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lara", "Deleted", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").
			Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").
			Where("id = ?", student.PersonID).Exec(context.Background())
	})
	linkChildToAccount(t, db, account.ID, tenantID, student.ID)

	// Soft-delete the underlying person.
	_, err := db.NewRaw(`UPDATE users.persons SET deleted_at = NOW() WHERE id = ?`, student.PersonID).
		Exec(context.Background())
	require.NoError(t, err)

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	assert.Empty(t, list, "soft-deleted person MUST be filtered out")
}

func TestChildRepository_ListByAccount_CrossTenant(t *testing.T) {
	t.Parallel()

	// One account → two children at two different tenants. Both rows
	// must come back in a single call (cross-tenant join via
	// account_tenants).
	db := testpkg.SetupTestDB(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	account := testpkg.CreateTestAccount(t, db, "parentchild-cross")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Lara", "Cross-A", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Tim", "Cross-B", "2b")
	t.Cleanup(func() {
		bg := context.Background()
		for _, s := range []struct {
			studentID, personID int64
		}{{studentA.ID, studentA.PersonID}, {studentB.ID, studentB.PersonID}} {
			_, _ = db.NewDelete().Table("users.students").Where("id = ?", s.studentID).Exec(bg)
			_, _ = db.NewDelete().Table("users.persons").Where("id = ?", s.personID).Exec(bg)
		}
	})

	linkChildToAccount(t, db, account.ID, tenantA, studentA.ID)
	linkChildToAccount(t, db, account.ID, tenantB, studentB.ID)

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	require.Len(t, list, 2, "both children must come back in a single cross-tenant query")
	seenTenants := map[int64]bool{}
	for _, c := range list {
		seenTenants[c.TenantID] = true
	}
	assert.True(t, seenTenants[tenantA])
	assert.True(t, seenTenants[tenantB])
}

func TestChildRepository_ListByAccount_OrdersBySchoolThenName(t *testing.T) {
	t.Parallel()

	// Result ordering is school_name ASC, first_name ASC, last_name
	// ASC. Verify with two children at the same school.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-order")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	// Insert Z first, A second, so the test really exercises the
	// ORDER BY rather than insertion order.
	studentZ := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Zara", "Last", "3c")
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Anna", "First", "1a")
	t.Cleanup(func() {
		bg := context.Background()
		for _, s := range []struct {
			studentID, personID int64
		}{{studentZ.ID, studentZ.PersonID}, {studentA.ID, studentA.PersonID}} {
			_, _ = db.NewDelete().Table("users.students").Where("id = ?", s.studentID).Exec(bg)
			_, _ = db.NewDelete().Table("users.persons").Where("id = ?", s.personID).Exec(bg)
		}
	})
	linkChildToAccount(t, db, account.ID, tenantID, studentZ.ID)
	linkChildToAccount(t, db, account.ID, tenantID, studentA.ID)

	repo := parentRepo.NewChildRepository(db)
	var list []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	require.Len(t, list, 2)
	assert.Equal(t, "Anna", list[0].FirstName, "first_name ASC means Anna comes before Zara")
	assert.Equal(t, "Zara", list[1].FirstName)
}

func TestChildRepository_ListByAccount_UnknownAccountEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := parentRepo.NewChildRepository(db)

	var list []*parentModels.ChildSummary
	err := runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListByAccount(ctx, 9_999_999)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- FindForAccount ---------------------------------------------------

// findChild runs the single-child resolver inside the required admin tx.
func findChild(t *testing.T, db *bun.DB, accountID, studentID int64) *parentModels.ChildSummary {
	t.Helper()
	repo := parentRepo.NewChildRepository(db)
	var got *parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		c, e := repo.FindForAccount(ctx, accountID, studentID)
		got = c
		return e
	}))
	return got
}

func TestChildRepository_FindForAccount_RejectsNonPositive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := parentRepo.NewChildRepository(db)

	_, err := repo.FindForAccount(context.Background(), 0, 5)
	require.Error(t, err)
	_, err = repo.FindForAccount(context.Background(), 5, 0)
	require.Error(t, err)
}

func TestChildRepository_FindForAccount_OwnedChild(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "find-owned")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Felix", "Owned", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").Where("id = ?", student.PersonID).Exec(context.Background())
	})
	linkChildToAccount(t, db, account.ID, tenantID, student.ID)

	got := findChild(t, db, account.ID, student.ID)
	require.NotNil(t, got, "guardian's own child must resolve")
	assert.Equal(t, student.ID, got.StudentID)
	assert.Equal(t, tenantID, got.TenantID, "tenant must be returned for downstream scoping")
}

func TestChildRepository_FindForAccount_NotLinkedReturnsNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "find-notlinked")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})
	// account is a guardian of `mine` but NOT of `other`.
	mine := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Mine", "Child", "1a")
	other := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Other", "Child", "2b")
	t.Cleanup(func() {
		bg := context.Background()
		for _, s := range []struct{ sid, pid int64 }{{mine.ID, mine.PersonID}, {other.ID, other.PersonID}} {
			_, _ = db.NewDelete().Table("users.students").Where("id = ?", s.sid).Exec(bg)
			_, _ = db.NewDelete().Table("users.persons").Where("id = ?", s.pid).Exec(bg)
		}
	})
	linkChildToAccount(t, db, account.ID, tenantID, mine.ID)

	assert.Nil(t, findChild(t, db, account.ID, other.ID),
		"a student the account is not a guardian of must not resolve")
}

func TestChildRepository_FindForAccount_InactiveMappingReturnsNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "find-inactive")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Felix", "Inactive", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").Where("id = ?", student.PersonID).Exec(context.Background())
	})
	linkChildToAccount(t, db, account.ID, tenantID, student.ID)

	_, err := db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`,
		account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)

	assert.Nil(t, findChild(t, db, account.ID, student.ID),
		"an inactive tenant mapping must drop the child")
}

// A grade transition graduates a child by flipping their status to 'alumnus' —
// a soft delete that hides them from every staff-facing read path. The parent
// portal must follow: the guardian relation survives graduation, so without this
// filter a guardian keeps seeing the departed child AND keeps write access,
// because FindForAccount is the authorization gate for every per-child parent
// write (#405 review).
func TestChildRepository_AlumnusChildIsHiddenAndUnwritable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "parentchild-alumnus")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	graduate := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lena", "Abgang", "4a")
	sibling := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Timo", "Bleibt", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("users.students").
			Where("id IN (?, ?)", graduate.ID, sibling.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("users.persons").
			Where("id IN (?, ?)", graduate.PersonID, sibling.PersonID).Exec(context.Background())
	})
	linkChildToAccount(t, db, account.ID, tenantID, graduate.ID)
	linkChildToAccount(t, db, account.ID, tenantID, sibling.ID)

	// Before graduation both children are the guardian's to see and write.
	repo := parentRepo.NewChildRepository(db)
	var before []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		before, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	require.Len(t, before, 2)
	require.NotNil(t, findChild(t, db, account.ID, graduate.ID))

	_, err := db.NewRaw(`UPDATE users.students SET status = 'alumnus' WHERE id = ?`, graduate.ID).
		Exec(context.Background())
	require.NoError(t, err)

	var after []*parentModels.ChildSummary
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		after, lErr = repo.ListByAccount(ctx, account.ID)
		return lErr
	}))
	require.Len(t, after, 1, "the graduate must drop out of the parent dashboard")
	assert.Equal(t, sibling.ID, after[0].StudentID, "the sibling still at the school stays")

	assert.Nil(t, findChild(t, db, account.ID, graduate.ID),
		"the authorization gate must refuse the graduate, so no parent write can target them")
	assert.NotNil(t, findChild(t, db, account.ID, sibling.ID),
		"graduating one child must not revoke access to the others")
}
