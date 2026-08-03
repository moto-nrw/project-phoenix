package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityCategoryArchivalUpDisambiguatesExistingCaseVariants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	group := testpkg.CreateTestActivityGroup(t, db, "ArchivalUpgrade")
	originalCategoryID := group.CategoryID
	restored := false
	var firstID, secondID int64

	require.NoError(t, activityCategoryArchivalDown(ctx, db))
	defer func() {
		if !restored {
			require.NoError(t, activityCategoryArchivalUp(ctx, db))
		}
		testpkg.CleanupActivityFixtures(t, db, group.ID, *group.CreatedBy, originalCategoryID, firstID, secondID)
	}()

	baseName := fmt.Sprintf("UpgradeCase-%d", time.Now().UnixNano())
	require.NoError(t, db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name)
		VALUES (?, ?)
		RETURNING id
	`, group.TenantID, baseName).Scan(ctx, &firstID))
	require.NoError(t, db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name)
		VALUES (?, ?)
		RETURNING id
	`, group.TenantID, strings.ToLower(baseName)).Scan(ctx, &secondID))
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET category_id = ?
		WHERE id = ?
	`, secondID, group.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, activityCategoryArchivalUp(ctx, db))
	restored = true

	var firstName, secondName string
	require.NoError(t, db.NewRaw(
		"SELECT name FROM activities.categories WHERE id = ?",
		firstID,
	).Scan(ctx, &firstName))
	require.NoError(t, db.NewRaw(
		"SELECT name FROM activities.categories WHERE id = ?",
		secondID,
	).Scan(ctx, &secondName))
	assert.Equal(t, baseName, firstName)
	assert.Contains(t, secondName, "Duplikat")
	assert.NotEqual(t, strings.ToLower(firstName), strings.ToLower(secondName))

	var referencedCategoryID int64
	require.NoError(t, db.NewRaw(
		"SELECT category_id FROM activities.groups WHERE id = ?",
		group.ID,
	).Scan(ctx, &referencedCategoryID))
	assert.Equal(t, secondID, referencedCategoryID)
}

func TestActivityCategoryArchivalDownPreservesReferencedNameConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	group := testpkg.CreateTestActivityGroup(t, db, "ArchivalRollback")
	category := new(activitiesModel.Category)
	require.NoError(t, db.NewSelect().
		Model(category).
		ModelTableExpr(`activities.categories AS "category"`).
		Where("id = ?", group.CategoryID).
		Scan(ctx))
	replacementName := category.Name

	now := time.Now()
	_, err := db.NewUpdate().
		Table("activities.categories").
		Set("archived_at = ?", now).
		Where("id = ?", category.ID).
		Exec(ctx)
	require.NoError(t, err)
	replacement := &activitiesModel.Category{Name: replacementName}
	replacement.SetTenantID(category.TenantID)
	require.NoError(t, db.NewInsert().
		Model(replacement).
		ModelTableExpr("activities.categories").
		Scan(ctx))

	defer func() {
		require.NoError(t, activityCategoryArchivalUp(ctx, db))
		testpkg.CleanupActivityFixtures(t, db, 0, 0, group.ID, group.CategoryID, 0)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, replacement.ID, 0)
	}()

	require.NoError(t, activityCategoryArchivalDown(ctx, db))

	var referencedCategoryID int64
	require.NoError(t, db.NewRaw(
		"SELECT category_id FROM activities.groups WHERE id = ?",
		group.ID,
	).Scan(ctx, &referencedCategoryID))
	assert.Equal(t, group.CategoryID, referencedCategoryID)

	var rolledBackName string
	require.NoError(t, db.NewRaw(
		"SELECT name FROM activities.categories WHERE id = ?",
		group.CategoryID,
	).Scan(ctx, &rolledBackName))
	assert.Contains(t, rolledBackName, "archiviert")
	assert.NotEqual(t, replacementName, rolledBackName)
}
