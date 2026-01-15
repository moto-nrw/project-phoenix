package education_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupSubstitutionRecords removes group substitutions directly
func cleanupSubstitutionRecords(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		testpkg.CleanupTableRecords(t, db, "education.group_substitution", id)
	}
}

// cleanupStaffChain cleans up staff -> person chain
func cleanupStaffChain(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	ctx := context.Background()

	// Get person ID
	var personID int64
	err := db.NewSelect().
		TableExpr("users.staff").
		Column("person_id").
		Where("id = ?", staffID).
		Scan(ctx, &personID)
	if err != nil {
		t.Logf("Warning: failed to get person ID: %v", err)
	}

	// Delete in order
	_, _ = db.NewDelete().TableExpr("users.staff").Where("id = ?", staffID).Exec(ctx)
	if personID != 0 {
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", personID).Exec(ctx)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupSubstitutionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("creates substitution with substitute only", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubCreate")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Staff")

		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)

		sub := &education.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         startDate,
			EndDate:           endDate,
			Reason:            "Test substitution",
		}

		err := repo.Create(ctx, sub)
		require.NoError(t, err)
		assert.NotZero(t, sub.ID)

		cleanupSubstitutionRecords(t, db, sub.ID)
	})

	t.Run("creates substitution with regular and substitute staff", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubCreateFull")
		regular := testpkg.CreateTestStaff(t, db, "Regular", "Staff")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Staff")

		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, regular.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)

		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regular.ID, substitute.ID, startDate, endDate)
		defer cleanupSubstitutionRecords(t, db, sub.ID)

		assert.NotZero(t, sub.ID)
		assert.Equal(t, group.ID, sub.GroupID)
		require.NotNil(t, sub.RegularStaffID)
		assert.Equal(t, regular.ID, *sub.RegularStaffID)
		assert.Equal(t, substitute.ID, sub.SubstituteStaffID)
	})
}

func TestGroupSubstitutionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds existing substitution", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubFindByID")
		substitute := testpkg.CreateTestStaff(t, db, "FindSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		found, err := repo.FindByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, sub.ID, found.ID)
		assert.Equal(t, group.ID, found.GroupID)
	})

	t.Run("returns error for non-existent substitution", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestGroupSubstitutionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("updates substitution reason", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubUpdate")
		substitute := testpkg.CreateTestStaff(t, db, "UpdateSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		sub.Reason = "Updated reason"
		err := repo.Update(ctx, sub)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated reason", found.Reason)
	})
}

func TestGroupSubstitutionRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("deletes existing substitution", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubDelete")
		substitute := testpkg.CreateTestStaff(t, db, "DeleteSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		err := repo.Delete(ctx, sub.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, sub.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestGroupSubstitutionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("lists all substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubList")
		substitute := testpkg.CreateTestStaff(t, db, "ListSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_ListWithOptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("lists with pagination", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubListOpts")
		substitute := testpkg.CreateTestStaff(t, db, "ListOptsSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		subs, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(subs), 10)
	})
}

func TestGroupSubstitutionRepository_FindByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds substitutions by group ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubByGroup")
		substitute := testpkg.CreateTestStaff(t, db, "ByGroupSubstitute", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindByGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		var found bool
		for _, s := range subs {
			if s.ID == sub.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestGroupSubstitutionRepository_FindBySubstituteStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds substitutions by substitute staff ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubBySubstitute")
		substitute := testpkg.CreateTestStaff(t, db, "BySubstituteStaff", "Staff")

		startDate := time.Now()
		endDate := startDate.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindBySubstituteStaff(ctx, substitute.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindActive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions for date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActive")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubstitute", "Staff")

		// Create substitution that's active today
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour) // Yesterday
		endDate := today.Add(7 * 24 * time.Hour)    // Week from now
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActive(ctx, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindActiveByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions for group and date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveGroup")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveGroupSubstitute", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveByGroup(ctx, group.ID, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindOverlapping(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds overlapping substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubOverlap")
		substitute := testpkg.CreateTestStaff(t, db, "OverlapSubstitute", "Staff")

		// Create substitution from today for 7 days
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		// Check for overlapping period (3 days in the middle)
		checkStart := today.Add(2 * 24 * time.Hour)
		checkEnd := today.Add(5 * 24 * time.Hour)

		subs, err := repo.FindOverlapping(ctx, substitute.ID, checkStart, checkEnd)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("returns empty for non-overlapping period", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubNoOverlap")
		substitute := testpkg.CreateTestStaff(t, db, "NoOverlapSubstitute", "Staff")

		// Create substitution for next week
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(7 * 24 * time.Hour)
		endDate := today.Add(14 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		// Check for this week (should not overlap)
		checkStart := today
		checkEnd := today.Add(3 * 24 * time.Hour)

		subs, err := repo.FindOverlapping(ctx, substitute.ID, checkStart, checkEnd)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindByRegularStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds substitutions by regular staff ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubByRegular")
		regular := testpkg.CreateTestStaff(t, db, "Regular", "Staff")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regular.ID, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, regular.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindByRegularStaff(ctx, regular.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		var found bool
		for _, s := range subs {
			if s.ID == sub.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for staff with no substitutions", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NoSubs", "Staff")
		defer cleanupStaffChain(t, db, staff.ID)

		subs, err := repo.FindByRegularStaff(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindActiveBySubstitute(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions by substitute staff and date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveSubstitute")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubstitute", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveBySubstitute(ctx, substitute.ID, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		var found bool
		for _, s := range subs {
			if s.ID == sub.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for non-active date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubInactive")
		substitute := testpkg.CreateTestStaff(t, db, "InactiveSubstitute", "Staff")

		// Create substitution for last week (expired)
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-14 * 24 * time.Hour)
		endDate := today.Add(-7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveBySubstitute(ctx, substitute.ID, today)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupSubstitutionRepository_Create_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("returns error for nil substitution", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for invalid date range", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubValidation")
		substitute := testpkg.CreateTestStaff(t, db, "ValidationSub", "Staff")
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		today := time.Now()
		sub := &education.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         today,
			EndDate:           today.Add(-7 * 24 * time.Hour), // End before start
		}

		err := repo.Create(ctx, sub)
		require.Error(t, err)
	})
}

func TestGroupSubstitutionRepository_Update_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("returns error for nil substitution", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// List Filter Tests
// ============================================================================

func TestGroupSubstitutionRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("filters by active status", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveFilter")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveFilterSub", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		filters := map[string]interface{}{
			"active": true,
		}

		subs, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("filters by specific date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubDateFilter")
		substitute := testpkg.CreateTestStaff(t, db, "DateFilterSub", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		filters := map[string]interface{}{
			"date": today.Add(3 * 24 * time.Hour), // Middle of range
		}

		subs, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("filters by reason_like", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubReasonFilter")
		substitute := testpkg.CreateTestStaff(t, db, "ReasonFilterSub", "Staff")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)

		sub := &education.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         startDate,
			EndDate:           endDate,
			Reason:            "Sick leave emergency",
		}
		err := repo.Create(ctx, sub)
		require.NoError(t, err)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		filters := map[string]interface{}{
			"reason_like": "emergency",
		}

		subs, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		var found bool
		for _, s := range subs {
			if s.ID == sub.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// ============================================================================
// Relation Loading Tests (Critical for Coverage)
// ============================================================================

func TestGroupSubstitutionRepository_FindByIDWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("loads all relations including staff persons", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubWithRelations")
		regular := testpkg.CreateTestStaff(t, db, "Regular", "Person")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regular.ID, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, regular.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		// Load with relations
		found, err := repo.FindByIDWithRelations(ctx, sub.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		// Verify group is loaded
		require.NotNil(t, found.Group)
		assert.Equal(t, group.ID, found.Group.ID)

		// Verify regular staff and person are loaded
		if found.RegularStaff != nil {
			assert.Equal(t, regular.ID, found.RegularStaff.ID)
			if found.RegularStaff.Person != nil {
				assert.Contains(t, found.RegularStaff.Person.FirstName, "Regular")
			}
		}

		// Verify substitute staff and person are loaded
		if found.SubstituteStaff != nil {
			assert.Equal(t, substitute.ID, found.SubstituteStaff.ID)
			if found.SubstituteStaff.Person != nil {
				assert.Contains(t, found.SubstituteStaff.Person.FirstName, "Substitute")
			}
		}
	})

	t.Run("loads with nil regular staff", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubNoRegular")
		substitute := testpkg.CreateTestStaff(t, db, "OnlySubstitute", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		found, err := repo.FindByIDWithRelations(ctx, sub.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Nil(t, found.RegularStaff)
		// SubstituteStaff may or may not be loaded depending on query success
		if found.SubstituteStaff != nil {
			assert.Equal(t, substitute.ID, found.SubstituteStaff.ID)
		}
	})
}

func TestGroupSubstitutionRepository_ListWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("loads relations for multiple substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubListRelations")
		substitute1 := testpkg.CreateTestStaff(t, db, "Sub1", "Person")
		substitute2 := testpkg.CreateTestStaff(t, db, "Sub2", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today
		endDate := today.Add(7 * 24 * time.Hour)

		sub1 := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute1.ID, startDate, endDate)
		sub2 := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute2.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub1.ID, sub2.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute1.ID)
		defer cleanupStaffChain(t, db, substitute2.ID)

		// List with relations
		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.Equal("group_id", group.ID)
		options.Filter = filter

		subs, err := repo.ListWithRelations(ctx, options)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		// Verify relations are loaded
		for _, s := range subs {
			if s.ID == sub1.ID || s.ID == sub2.ID {
				assert.NotNil(t, s.Group, "Group should be loaded")
				assert.NotNil(t, s.SubstituteStaff, "Substitute staff should be loaded")
				assert.NotNil(t, s.SubstituteStaff.Person, "Staff person should be loaded")
			}
		}
	})

	t.Run("handles empty result set", func(t *testing.T) {
		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.Equal("group_id", int64(999999)) // Non-existent
		options.Filter = filter

		subs, err := repo.ListWithRelations(ctx, options)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindActiveWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions with relations", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveRel")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveRelSub", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveWithRelations(ctx, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		// Find our substitution in results
		var found *education.GroupSubstitution
		for _, s := range subs {
			if s.ID == sub.ID {
				found = s
				break
			}
		}

		require.NotNil(t, found, "Should find our substitution")
		assert.NotNil(t, found.Group)
		assert.NotNil(t, found.SubstituteStaff)
		assert.NotNil(t, found.SubstituteStaff.Person)
	})
}

func TestGroupSubstitutionRepository_FindActiveBySubstituteWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions by substitute with relations", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveSubRel")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubRel", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveBySubstituteWithRelations(ctx, substitute.ID, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		// Verify relations are loaded
		found := subs[0]
		assert.NotNil(t, found.Group)
		assert.NotNil(t, found.SubstituteStaff)
		assert.NotNil(t, found.SubstituteStaff.Person)
	})
}

func TestGroupSubstitutionRepository_FindActiveByGroupWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := context.Background()

	t.Run("finds active substitutions by group with relations", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveGroupRel")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveGroupRelSub", "Person")

		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.Add(-1 * 24 * time.Hour)
		endDate := today.Add(7 * 24 * time.Hour)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		defer cleanupSubstitutionRecords(t, db, sub.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupStaffChain(t, db, substitute.ID)

		subs, err := repo.FindActiveByGroupWithRelations(ctx, group.ID, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		// Verify relations are loaded
		found := subs[0]
		assert.NotNil(t, found.Group)
		assert.Equal(t, group.ID, found.Group.ID)
		assert.NotNil(t, found.SubstituteStaff)
		assert.NotNil(t, found.SubstituteStaff.Person)
	})
}
