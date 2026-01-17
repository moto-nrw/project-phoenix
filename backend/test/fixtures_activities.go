package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Activities Domain Fixtures
// ============================================================================

// CreateTestActivityCategory creates a real activity category in the database
func CreateTestActivityCategory(tb testing.TB, db *bun.DB, name string) *activities.Category {
	tb.Helper()

	// Make name unique to avoid conflicts with seeded data
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	category := &activities.Category{
		Name:  uniqueName,
		Color: "#CCCCCC",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.NewInsert().
		Model(category).
		ModelTableExpr(`activities.categories`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity category")

	return category
}

// CreateTestActivityGroup creates a real activity group in the database
// Activity groups require a category, so this helper creates one automatically
func CreateTestActivityGroup(tb testing.TB, db *bun.DB, name string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First create a category (activities.groups.category_id is required)
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, time.Now().UnixNano()))

	// Create the activity group
	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
	}

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity group")

	return group
}
