package platform_test

import (
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
)

func TestSchoolRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platformRepo.NewSchoolRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates school", func(t *testing.T) {
		now := time.Now().UnixNano()
		org := &platformModels.Organization{
			Model:  modelBase.Model{ID: now},
			Name:   fmt.Sprintf("Org %d", now),
			Slug:   fmt.Sprintf("org-%d", now),
			Active: true,
		}
		require.NoError(t, platformRepo.NewOrganizationRepository(db).Create(ctx, org))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE organization_id = ?`, org.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, org.ID)
		})

		school := &platformModels.School{
			Model:          modelBase.Model{ID: now + 1},
			OrganizationID: org.ID,
			Name:           fmt.Sprintf("School %d", now),
			Slug:           fmt.Sprintf("school-%d", now),
			Subdomain:      fmt.Sprintf("school-%d", now),
			Active:         true,
		}

		err := repo.Create(ctx, school)
		require.NoError(t, err)
		assert.NotZero(t, school.ID)
	})

	t.Run("rejects nil school", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "school cannot be nil")
	})

	t.Run("rejects invalid school", func(t *testing.T) {
		err := repo.Create(ctx, &platformModels.School{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestSchoolRepository_QueryMethods(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platformRepo.NewSchoolRepository(db)
	orgRepo := platformRepo.NewOrganizationRepository(db)
	ctx := testpkg.TenantContext(1)
	now := time.Now().UnixNano()

	orgA := &platformModels.Organization{Model: modelBase.Model{ID: now}, Name: fmt.Sprintf("OrgA %d", now), Slug: fmt.Sprintf("orga-%d", now), Active: true}
	orgB := &platformModels.Organization{Model: modelBase.Model{ID: now + 1}, Name: fmt.Sprintf("OrgB %d", now), Slug: fmt.Sprintf("orgb-%d", now), Active: true}
	require.NoError(t, orgRepo.Create(ctx, orgA))
	require.NoError(t, orgRepo.Create(ctx, orgB))

	schoolA := &platformModels.School{
		Model:          modelBase.Model{ID: now + 2},
		OrganizationID: orgA.ID,
		Name:           fmt.Sprintf("Alpha School %d", now),
		Slug:           fmt.Sprintf("shared-slug-%d", now),
		Subdomain:      fmt.Sprintf("school-a-%d", now),
		Active:         true,
	}
	schoolB := &platformModels.School{
		Model:          modelBase.Model{ID: now + 3},
		OrganizationID: orgB.ID,
		Name:           fmt.Sprintf("Beta School %d", now),
		Slug:           fmt.Sprintf("shared-slug-%d", now),
		Subdomain:      fmt.Sprintf("school-b-%d", now),
		Active:         false,
	}
	require.NoError(t, repo.Create(ctx, schoolA))
	require.NoError(t, repo.Create(ctx, schoolB))
	_, err := db.ExecContext(ctx, `UPDATE platform.schools SET active = false WHERE id = ?`, schoolB.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE tenant_id IN (?, ?)`, schoolA.ID, schoolB.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id IN (?, ?)`, schoolA.ID, schoolB.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id IN (?, ?)`, orgA.ID, orgB.ID)
	})

	t.Run("find by id returns school", func(t *testing.T) {
		found, err := repo.FindByID(ctx, schoolA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, schoolA.Subdomain, found.Subdomain)
	})

	t.Run("find by id wraps not found", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999999)
		require.Error(t, err)
		assert.Nil(t, found)
	})

	t.Run("find by slug returns first matching scoped row", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, schoolA.Slug)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, schoolA.Slug, found.Slug)
	})

	t.Run("find by organization and slug respects organization", func(t *testing.T) {
		found, err := repo.FindByOrganizationAndSlug(ctx, orgB.ID, schoolB.Slug)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, schoolB.ID, found.ID)
	})

	t.Run("find by organization and slug returns nil when missing", func(t *testing.T) {
		found, err := repo.FindByOrganizationAndSlug(ctx, orgA.ID, "missing-school")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("find by subdomain preloads organization", func(t *testing.T) {
		found, err := repo.FindBySubdomain(ctx, schoolA.Subdomain)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.NotNil(t, found.Organization)
		assert.Equal(t, orgA.ID, found.Organization.ID)
	})

	t.Run("find by subdomain returns nil when missing", func(t *testing.T) {
		found, err := repo.FindBySubdomain(ctx, "missing-subdomain")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("find by slug returns nil when missing", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, "missing-slug")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("list returns both schools", func(t *testing.T) {
		items, err := repo.List(ctx)
		require.NoError(t, err)
		var foundA, foundB bool
		for _, item := range items {
			if item.ID == schoolA.ID {
				foundA = true
			}
			if item.ID == schoolB.ID {
				foundB = true
			}
		}
		assert.True(t, foundA)
		assert.True(t, foundB)
	})

	t.Run("list active filters inactive schools", func(t *testing.T) {
		items, err := repo.ListActive(ctx)
		require.NoError(t, err)
		foundA := false
		foundB := false
		for _, item := range items {
			if item.ID == schoolA.ID {
				foundA = true
			}
			if item.ID == schoolB.ID {
				foundB = true
			}
		}
		assert.True(t, foundA)
		assert.False(t, foundB)
	})

	t.Run("list active filters hidden schools", func(t *testing.T) {
		// Create a hidden but active school
		hiddenSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 10},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("Hidden School %d", now),
			Slug:           fmt.Sprintf("hidden-school-%d", now),
			Subdomain:      fmt.Sprintf("hidden-school-%d", now),
			Active:         true,
			Hidden:         true,
		}
		require.NoError(t, repo.Create(ctx, hiddenSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, hiddenSchool.ID)
		})

		items, err := repo.ListActive(ctx)
		require.NoError(t, err)
		foundA := false
		foundHidden := false
		for _, item := range items {
			if item.ID == schoolA.ID {
				foundA = true
			}
			if item.ID == hiddenSchool.ID {
				foundHidden = true
			}
		}
		assert.True(t, foundA, "active non-hidden school should be listed")
		assert.False(t, foundHidden, "active but hidden school should not be listed")
	})

	t.Run("list returns hidden schools", func(t *testing.T) {
		// Create own hidden school to avoid depending on prior subtests
		listHiddenSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 11},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("ListHidden School %d", now),
			Slug:           fmt.Sprintf("list-hidden-%d", now),
			Subdomain:      fmt.Sprintf("list-hidden-%d", now),
			Active:         true,
			Hidden:         true,
		}
		require.NoError(t, repo.Create(ctx, listHiddenSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, listHiddenSchool.ID)
		})

		// List (used by operator dashboard) should return all schools including hidden
		items, err := repo.List(ctx)
		require.NoError(t, err)
		foundHidden := false
		for _, item := range items {
			if item.ID == listHiddenSchool.ID {
				foundHidden = true
				break
			}
		}
		assert.True(t, foundHidden, "List should include hidden schools for operator management")
	})

	t.Run("soft delete sets deleted_at", func(t *testing.T) {
		sdSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 20},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("SoftDel School %d", now),
			Slug:           fmt.Sprintf("softdel-%d", now),
			Subdomain:      fmt.Sprintf("softdel-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, sdSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, sdSchool.ID)
		})

		err := repo.SoftDelete(ctx, sdSchool.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, sdSchool.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.True(t, found.IsDeleted(), "school should be soft-deleted")
		assert.True(t, found.Active, "active flag should be preserved")
	})

	t.Run("soft delete fails on already-deleted school", func(t *testing.T) {
		sdSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 21},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("SoftDel2 School %d", now),
			Slug:           fmt.Sprintf("softdel2-%d", now),
			Subdomain:      fmt.Sprintf("softdel2-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, sdSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, sdSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, sdSchool.ID))

		err := repo.SoftDelete(ctx, sdSchool.ID)
		require.Error(t, err, "second soft-delete should fail")
	})

	t.Run("restore clears deleted_at", func(t *testing.T) {
		rSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 22},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("Restore School %d", now),
			Slug:           fmt.Sprintf("restore-%d", now),
			Subdomain:      fmt.Sprintf("restore-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, rSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, rSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, rSchool.ID))
		err := repo.Restore(ctx, rSchool.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, rSchool.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.False(t, found.IsDeleted(), "school should be restored")
	})

	t.Run("restore fails on non-deleted school", func(t *testing.T) {
		err := repo.Restore(ctx, schoolA.ID)
		require.Error(t, err, "restore on non-deleted school should fail")
	})

	t.Run("find deleted older than returns matching schools", func(t *testing.T) {
		fdSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 23},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("FindDel School %d", now),
			Slug:           fmt.Sprintf("finddel-%d", now),
			Subdomain:      fmt.Sprintf("finddel-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, fdSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, fdSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, fdSchool.ID))

		// Backdate deleted_at to 100 days ago
		_, err := db.ExecContext(ctx,
			`UPDATE platform.schools SET deleted_at = NOW() - INTERVAL '100 days' WHERE id = ?`,
			fdSchool.ID)
		require.NoError(t, err)

		// Should find with 90-day window
		results, err := repo.FindDeletedOlderThan(ctx, 90*24*time.Hour)
		require.NoError(t, err)
		found := false
		for _, s := range results {
			if s.ID == fdSchool.ID {
				found = true
			}
		}
		assert.True(t, found, "should find school deleted >90 days ago")

		// Should NOT find with 200-day window
		results, err = repo.FindDeletedOlderThan(ctx, 200*24*time.Hour)
		require.NoError(t, err)
		found = false
		for _, s := range results {
			if s.ID == fdSchool.ID {
				found = true
			}
		}
		assert.False(t, found, "should not find school deleted <200 days ago")
	})

	t.Run("soft-deleted school excluded from find by slug", func(t *testing.T) {
		exSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 24},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("Excluded School %d", now),
			Slug:           fmt.Sprintf("excluded-%d", now),
			Subdomain:      fmt.Sprintf("excluded-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, exSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, exSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, exSchool.ID))

		found, err := repo.FindBySlug(ctx, exSchool.Slug)
		require.NoError(t, err)
		assert.Nil(t, found, "soft-deleted school should not be found by slug")
	})

	t.Run("soft-deleted school excluded from find by subdomain", func(t *testing.T) {
		exSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 25},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("ExSub School %d", now),
			Slug:           fmt.Sprintf("exsub-%d", now),
			Subdomain:      fmt.Sprintf("exsub-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, exSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, exSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, exSchool.ID))

		found, err := repo.FindBySubdomain(ctx, exSchool.Subdomain)
		require.NoError(t, err)
		assert.Nil(t, found, "soft-deleted school should not be found by subdomain")
	})

	t.Run("soft-deleted school excluded from list active", func(t *testing.T) {
		exSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 26},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("ExList School %d", now),
			Slug:           fmt.Sprintf("exlist-%d", now),
			Subdomain:      fmt.Sprintf("exlist-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, exSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, exSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, exSchool.ID))

		items, err := repo.ListActive(ctx)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotEqual(t, exSchool.ID, item.ID, "soft-deleted school should not appear in ListActive")
		}
	})

	t.Run("soft-deleted school included in list (operator)", func(t *testing.T) {
		inSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 27},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("InList School %d", now),
			Slug:           fmt.Sprintf("inlist-%d", now),
			Subdomain:      fmt.Sprintf("inlist-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, inSchool))
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, inSchool.ID)
		})

		require.NoError(t, repo.SoftDelete(ctx, inSchool.ID))

		items, err := repo.List(ctx)
		require.NoError(t, err)
		found := false
		for _, item := range items {
			if item.ID == inSchool.ID {
				found = true
			}
		}
		assert.True(t, found, "soft-deleted school should appear in List for operator")
	})

	t.Run("find active by account id returns active school memberships only", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "school-query")
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		})

		_, err := db.ExecContext(ctx,
			`INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW()), (?, ?, ?, NOW(), NOW())`,
			account.ID, schoolA.ID, authModels.AccountTenantStatusActive,
			account.ID, schoolB.ID, authModels.AccountTenantStatusActive)
		require.NoError(t, err)

		items, err := repo.FindActiveByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.NotEmpty(t, items)
		foundA := false
		foundB := false
		for _, item := range items {
			if item.ID == schoolA.ID {
				foundA = true
			}
			if item.ID == schoolB.ID {
				foundB = true
			}
		}
		assert.True(t, foundA)
		assert.False(t, foundB)
	})

	t.Run("find active by account id excludes soft-deleted schools", func(t *testing.T) {
		delSchool := &platformModels.School{
			Model:          modelBase.Model{ID: now + 28},
			OrganizationID: orgA.ID,
			Name:           fmt.Sprintf("DelAcct School %d", now),
			Slug:           fmt.Sprintf("delacct-%d", now),
			Subdomain:      fmt.Sprintf("delacct-%d", now),
			Active:         true,
		}
		require.NoError(t, repo.Create(ctx, delSchool))
		acct := testpkg.CreateTestAccount(t, db, "del-school-query")
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, acct.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, acct.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, delSchool.ID)
		})

		_, err := db.ExecContext(ctx,
			`INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`,
			acct.ID, delSchool.ID, authModels.AccountTenantStatusActive)
		require.NoError(t, err)

		// Before soft-delete: should find the school
		items, err := repo.FindActiveByAccountID(ctx, acct.ID)
		require.NoError(t, err)
		found := false
		for _, item := range items {
			if item.ID == delSchool.ID {
				found = true
			}
		}
		assert.True(t, found, "active school should be found")

		// After soft-delete: should NOT find the school
		require.NoError(t, repo.SoftDelete(ctx, delSchool.ID))
		items, err = repo.FindActiveByAccountID(ctx, acct.ID)
		require.NoError(t, err)
		found = false
		for _, item := range items {
			if item.ID == delSchool.ID {
				found = true
			}
		}
		assert.False(t, found, "soft-deleted school should not be found via FindActiveByAccountID")
	})
}
