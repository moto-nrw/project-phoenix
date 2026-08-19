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

	repo := platformRepo.NewSchoolRepository(db)
	ctx := testpkg.Ctx(t)

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

	repo := platformRepo.NewSchoolRepository(db)
	orgRepo := platformRepo.NewOrganizationRepository(db)
	ctx := testpkg.Ctx(t)
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

	t.Run("find by id for update returns school", func(t *testing.T) {
		found, err := repo.FindByIDForUpdate(ctx, schoolA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, schoolA.Subdomain, found.Subdomain)
	})

	t.Run("find by id for update wraps not found", func(t *testing.T) {
		found, err := repo.FindByIDForUpdate(ctx, 999999999)
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

	t.Run("list public excludes hidden schools", func(t *testing.T) {
		// Make schoolA hidden
		_, err := db.ExecContext(ctx, `UPDATE platform.schools SET hidden = true WHERE id = ?`, schoolA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET hidden = false WHERE id = ?`, schoolA.ID)
		})

		items, err := repo.ListPublic(ctx)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotEqual(t, schoolA.ID, item.ID, "ListPublic must not return hidden schools")
		}
	})

	t.Run("list public returns active non-hidden schools", func(t *testing.T) {
		items, err := repo.ListPublic(ctx)
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
		assert.True(t, foundA, "ListPublic should return active non-hidden schools")
		assert.False(t, foundB, "ListPublic should not return inactive schools")
	})

	t.Run("list public excludes deleted schools", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		items, err := repo.ListPublic(ctx)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotEqual(t, schoolA.ID, item.ID, "ListPublic must not return soft-deleted schools")
		}
	})

	t.Run("list active still includes hidden schools", func(t *testing.T) {
		// Make schoolA hidden — ListActive must still return it (scheduler depends on this)
		_, err := db.ExecContext(ctx, `UPDATE platform.schools SET hidden = true WHERE id = ?`, schoolA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET hidden = false WHERE id = ?`, schoolA.ID)
		})

		items, err := repo.ListActive(ctx)
		require.NoError(t, err)
		found := false
		for _, item := range items {
			if item.ID == schoolA.ID {
				found = true
			}
		}
		assert.True(t, found, "ListActive must still return hidden schools (scheduler dependency)")
	})

	t.Run("soft delete sets deleted_at", func(t *testing.T) {
		err := repo.SoftDelete(ctx, schoolA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		found, err := repo.FindByID(ctx, schoolA.ID)
		require.NoError(t, err)
		assert.True(t, found.IsDeleted(), "school should be marked as deleted")
	})

	t.Run("soft delete prevents double-delete", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		err := repo.SoftDelete(ctx, schoolA.ID)
		require.Error(t, err, "double soft-delete should fail via AssertRowsAffected")
	})

	t.Run("restore clears deleted_at", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		err := repo.Restore(ctx, schoolA.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, schoolA.ID)
		require.NoError(t, err)
		assert.False(t, found.IsDeleted(), "school should no longer be deleted after restore")
	})

	t.Run("restore prevents double-restore", func(t *testing.T) {
		err := repo.Restore(ctx, schoolA.ID)
		require.Error(t, err, "restoring a non-deleted school should fail via AssertRowsAffected")
	})

	t.Run("find by slug excludes deleted schools", func(t *testing.T) {
		uniqueSlug := fmt.Sprintf("deleted-only-%d", time.Now().UnixNano())
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET slug = ?, deleted_at = NOW() WHERE id = ?`, uniqueSlug, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET slug = ?, deleted_at = NULL WHERE id = ?`, schoolA.Slug, schoolA.ID)
		})

		found, err := repo.FindBySlug(ctx, uniqueSlug)
		require.NoError(t, err)
		assert.Nil(t, found, "FindBySlug should not return soft-deleted schools")
	})

	t.Run("find by subdomain includes deleted schools", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		found, err := repo.FindBySubdomain(ctx, schoolA.Subdomain)
		require.NoError(t, err)
		require.NotNil(t, found, "FindBySubdomain should still return soft-deleted schools")
		assert.True(t, found.IsDeleted())
	})

	t.Run("list active excludes deleted schools", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		items, err := repo.ListActive(ctx)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotEqual(t, schoolA.ID, item.ID, "ListActive must not return soft-deleted schools")
		}
	})

	t.Run("list includes deleted schools", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		items, err := repo.List(ctx)
		require.NoError(t, err)
		found := false
		for _, item := range items {
			if item.ID == schoolA.ID {
				found = true
			}
		}
		assert.True(t, found, "List should include soft-deleted schools (operator trash view)")
	})

	t.Run("find active by account id excludes deleted schools", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		account := testpkg.CreateTestAccount(t, db, "deleted-school-test")
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
			_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		})
		_, err := db.ExecContext(ctx,
			`INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`,
			account.ID, schoolA.ID, authModels.AccountTenantStatusActive)
		require.NoError(t, err)

		items, err := repo.FindActiveByAccountID(ctx, account.ID)
		require.NoError(t, err)
		for _, item := range items {
			assert.NotEqual(t, schoolA.ID, item.ID, "FindActiveByAccountID must not return soft-deleted schools")
		}
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

	t.Run("CountByIDs returns correct count for existing IDs", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{schoolA.ID, schoolB.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("CountByIDs returns partial count when some IDs missing", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{schoolA.ID, 999999999})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("CountByIDs returns zero for nonexistent IDs", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{999999999})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("CountByIDs returns zero for empty slice", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("CountByIDs excludes soft-deleted schools", func(t *testing.T) {
		// Pin the filter `deleted_at IS NULL` on CountByIDs: a soft-deleted row
		// must drop out of the count so announcement targeting cannot reference
		// trashed schools via new targeting IDs.
		_, err := db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		count, err := repo.CountByIDs(ctx, []int64{schoolA.ID, schoolB.ID})
		require.NoError(t, err)
		assert.Equal(t, 1, count, "soft-deleted school must not be counted")

		onlyDeleted, err := repo.CountByIDs(ctx, []int64{schoolA.ID})
		require.NoError(t, err)
		assert.Equal(t, 0, onlyDeleted, "single soft-deleted school id must count as 0")
	})

	t.Run("find by id for share returns school", func(t *testing.T) {
		found, err := repo.FindByIDForShare(ctx, schoolA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, schoolA.Subdomain, found.Subdomain)
	})

	t.Run("find by id for share wraps not found", func(t *testing.T) {
		found, err := repo.FindByIDForShare(ctx, 999999999)
		require.Error(t, err)
		assert.Nil(t, found)
	})

	t.Run("CountNonDeletedByOrganizationID counts live schools", func(t *testing.T) {
		count, err := repo.CountNonDeletedByOrganizationID(ctx, orgA.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "orgA has one live school (schoolA)")
	})

	t.Run("CountNonDeletedByOrganizationID excludes soft-deleted schools", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, schoolA.ID)
		})

		count, err := repo.CountNonDeletedByOrganizationID(ctx, orgA.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "soft-deleted school must not count — allowing parent org to be deleted")
	})

	t.Run("CountNonDeletedByOrganizationID returns zero for organization with no schools", func(t *testing.T) {
		count, err := repo.CountNonDeletedByOrganizationID(ctx, 999999999)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
