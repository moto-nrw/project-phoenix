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
	restored := false
	var firstID, secondID int64

	require.NoError(t, activityCategoryArchivalDown(ctx, db))
	defer func() {
		if !restored {
			require.NoError(t, activityCategoryArchivalUp(ctx, db))
		}
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

func TestActivityCategoryArchivalUpCanonicalizesReservedCaseVariants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	const (
		variantTenantID   int64 = 9258
		duplicateTenantID int64 = 9259
	)
	testpkg.EnsureTestTenant(t, db, variantTenantID)
	testpkg.EnsureTestTenant(t, db, duplicateTenantID)
	defer testpkg.CleanupTenantTestData(t, db, variantTenantID)
	defer testpkg.CleanupTenantTestData(t, db, duplicateTenantID)

	restored := false
	require.NoError(t, activityCategoryArchivalDown(ctx, db))
	defer func() {
		if !restored {
			require.NoError(t, activityCategoryArchivalUp(ctx, db))
		}
	}()

	insertCategory := func(tenantID int64, name string, isSystem bool) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO activities.categories (tenant_id, name, is_system)
			VALUES (?, ?, ?)
			RETURNING id
		`, tenantID, name, isSystem).Scan(ctx, &id))
		return id
	}

	wcVariantID := insertCategory(variantTenantID, "wc", false)
	schulhofVariantID := insertCategory(variantTenantID, "sCHulHOf", false)
	canonicalWCID := insertCategory(duplicateTenantID, "WC", true)
	duplicateWCID := insertCategory(duplicateTenantID, "wC", false)

	require.NoError(t, activityCategoryArchivalUp(ctx, db))
	restored = true

	assertCategory := func(id int64, expectedName string, expectedSystem bool) {
		t.Helper()
		var row struct {
			Name     string `bun:"name"`
			IsSystem bool   `bun:"is_system"`
		}
		require.NoError(t, db.NewRaw(`
			SELECT name, is_system
			FROM activities.categories
			WHERE id = ?
		`, id).Scan(ctx, &row))
		assert.Equal(t, expectedName, row.Name)
		assert.Equal(t, expectedSystem, row.IsSystem)
	}

	assertCategory(wcVariantID, "WC", true)
	assertCategory(schulhofVariantID, "Schulhof", true)
	assertCategory(canonicalWCID, "WC", true)
	assertCategory(duplicateWCID, fmt.Sprintf("wC (Duplikat #%d)", duplicateWCID), false)

	_, err := db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name)
		VALUES (?, 'Wc')
	`, variantTenantID).Exec(ctx)
	require.Error(t, err, "the active-name index must reserve case variants of WC")
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
