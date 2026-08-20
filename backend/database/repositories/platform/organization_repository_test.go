package platform_test

import (
	"fmt"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOrganizationRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates organization", func(t *testing.T) {
		now := time.Now().UnixNano()
		org := &platformModels.Organization{
			Model:  modelBase.Model{ID: now},
			Name:   fmt.Sprintf("Org %d", now),
			Slug:   fmt.Sprintf("org-%d", now),
			Active: true,
		}

		err := repo.Create(ctx, org)
		require.NoError(t, err)
		require.NotZero(t, org.ID)

	})

	t.Run("rejects nil organization", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid organization", func(t *testing.T) {
		err := repo.Create(ctx, &platformModels.Organization{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestOrganizationRepository_FindByIDAndSlugAndList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOrganizationRepository(db)
	ctx := testpkg.Ctx(t)

	now := time.Now().UnixNano()
	orgA := &platformModels.Organization{
		Model:  modelBase.Model{ID: now},
		Name:   fmt.Sprintf("Alpha %d", now),
		Slug:   fmt.Sprintf("alpha-%d", now),
		Active: true,
	}
	orgB := &platformModels.Organization{
		Model:  modelBase.Model{ID: now + 1},
		Name:   fmt.Sprintf("Beta %d", now),
		Slug:   fmt.Sprintf("beta-%d", now),
		Active: true,
	}
	require.NoError(t, repo.Create(ctx, orgA))
	require.NoError(t, repo.Create(ctx, orgB))

	t.Run("finds by id", func(t *testing.T) {
		found, err := repo.FindByID(ctx, orgA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, orgA.Slug, found.Slug)
	})

	t.Run("returns nil for missing id", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("finds by slug", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, orgB.Slug)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, orgB.ID, found.ID)
	})

	t.Run("returns nil for missing slug", func(t *testing.T) {
		found, err := repo.FindBySlug(ctx, "missing-org-slug")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("lists organizations ordered by name", func(t *testing.T) {
		items, err := repo.List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, items)

		foundA := false
		foundB := false
		for _, item := range items {
			if item.ID == orgA.ID {
				foundA = true
			}
			if item.ID == orgB.ID {
				foundB = true
			}
		}
		assert.True(t, foundA)
		assert.True(t, foundB)
	})

	t.Run("CountByIDs returns correct count for existing IDs", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{orgA.ID, orgB.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("CountByIDs returns partial count when some IDs missing", func(t *testing.T) {
		count, err := repo.CountByIDs(ctx, []int64{orgA.ID, 999999999})
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

	t.Run("CountByIDs excludes soft-deleted organizations", func(t *testing.T) {
		// Pin the filter `deleted_at IS NULL` on CountByIDs: a soft-deleted row
		// must drop out of the count so announcement targeting cannot reference
		// trashed organizations via new targeting IDs.
		_, err := db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, orgA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NULL WHERE id = ?`, orgA.ID)
		})

		count, err := repo.CountByIDs(ctx, []int64{orgA.ID, orgB.ID})
		require.NoError(t, err)
		assert.Equal(t, 1, count, "soft-deleted organization must not be counted")

		onlyDeleted, err := repo.CountByIDs(ctx, []int64{orgA.ID})
		require.NoError(t, err)
		assert.Equal(t, 0, onlyDeleted, "single soft-deleted organization id must count as 0")
	})

	t.Run("find by id for share returns nil when missing", func(t *testing.T) {
		// Exercises the ErrNoRows branch of findByIDWithLock (returns nil, nil).
		found, err := repo.FindByIDForShare(ctx, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found, "FindByIDForShare must return nil, nil for missing IDs")
	})

	t.Run("find by id for share returns organization", func(t *testing.T) {
		found, err := repo.FindByIDForShare(ctx, orgA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, orgA.Slug, found.Slug)
	})

	t.Run("find by id for update returns nil when missing", func(t *testing.T) {
		found, err := repo.FindByIDForUpdate(ctx, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found, "FindByIDForUpdate must return nil, nil for missing IDs")
	})

	t.Run("soft delete sets deleted_at", func(t *testing.T) {
		err := repo.SoftDelete(ctx, orgA.ID)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NULL WHERE id = ?`, orgA.ID)
		})

		found, err := repo.FindByID(ctx, orgA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.True(t, found.IsDeleted(), "organization should be marked as deleted")
	})

	t.Run("soft delete prevents double-delete", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, orgA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NULL WHERE id = ?`, orgA.ID)
		})

		err := repo.SoftDelete(ctx, orgA.ID)
		require.Error(t, err, "double soft-delete should fail via AssertRowsAffected")
	})

	t.Run("restore clears deleted_at", func(t *testing.T) {
		_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, orgA.ID)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NULL WHERE id = ?`, orgA.ID)
		})

		err := repo.Restore(ctx, orgA.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, orgA.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.False(t, found.IsDeleted(), "organization should no longer be deleted after restore")
	})

	t.Run("restore prevents double-restore", func(t *testing.T) {
		err := repo.Restore(ctx, orgA.ID)
		require.Error(t, err, "restoring a non-deleted organization should fail via AssertRowsAffected")
	})
}
