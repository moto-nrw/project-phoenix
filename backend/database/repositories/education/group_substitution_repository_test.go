package education_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupGroupSubstitutionRepo(t *testing.T, db *bun.DB) education.GroupSubstitutionRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.GroupSubstitution
}

// cleanupSubstitutionRecords removes group substitutions directly
func cleanupSubstitutionRecords(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	if len(ids) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("education.group_substitution").
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup group substitutions: %v", err)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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

	repo := setupGroupSubstitutionRepo(t, db)
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
