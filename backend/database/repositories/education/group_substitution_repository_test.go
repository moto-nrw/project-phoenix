package education_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupSubstitutionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("creates substitution with substitute only", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubCreate")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)

		sub := &education.GroupSubstitution{
			TargetType:        education.GroupSubstitutionTypeGroupHandover,
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         startDate,
			EndDate:           endDate,
			Reason:            "Test substitution",
		}

		err := repo.Create(ctx, sub)
		require.NoError(t, err)
		assert.NotZero(t, sub.ID)

	})

	t.Run("creates substitution with regular and substitute staff", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubCreateFull")
		regular := testpkg.CreateTestStaff(t, db, "Regular", "Staff")
		substitute := testpkg.CreateTestStaff(t, db, "Substitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)

		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, &regular.ID, substitute.ID, startDate, endDate)

		assert.NotZero(t, sub.ID)
		assert.Equal(t, group.ID, sub.GroupID)
		require.NotNil(t, sub.RegularStaffID)
		assert.Equal(t, regular.ID, *sub.RegularStaffID)
		assert.Equal(t, substitute.ID, sub.SubstituteStaffID)
	})
}

func TestGroupSubstitutionRepository_DeleteActiveOrFutureByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestEducationGroup(t, db, "SubDelOffboard")
	staff := testpkg.CreateTestStaff(t, db, "Offboarded", "Staff")
	otherStaff := testpkg.CreateTestStaff(t, db, "Other", "Staff")

	today := timezone.TodayDate()
	// Past substitution (ended yesterday) — must stay as history.
	past := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID,
		today.AddDays(-10), today.AddDays(-1))
	// Active substitution (running today) where staff is the substitute — must go.
	activeSub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID,
		today.AddDays(-2), today.AddDays(2))
	// Future substitution where staff is the regular (being substituted) — must go.
	futureRegular := testpkg.CreateTestGroupSubstitution(t, db, group.ID, &staff.ID, otherStaff.ID,
		today.AddDays(5), today.AddDays(10))
	// Future substitution of an unrelated staff member — must stay.
	otherFuture := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, otherStaff.ID,
		today.AddDays(5), today.AddDays(10))

	affected, err := repo.DeleteActiveOrFutureByStaffID(ctx, staff.ID, today)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	remainingIDs := func() map[int64]bool {
		subs, listErr := repo.FindByGroup(ctx, group.ID)
		require.NoError(t, listErr)
		ids := make(map[int64]bool, len(subs))
		for _, s := range subs {
			ids[s.ID] = true
		}
		return ids
	}()
	assert.True(t, remainingIDs[past.ID], "past substitution must stay as history")
	assert.False(t, remainingIDs[activeSub.ID], "active substitution must be deleted")
	assert.False(t, remainingIDs[futureRegular.ID], "future substitution naming staff as regular must be deleted")
	assert.True(t, remainingIDs[otherFuture.ID], "unrelated staff's substitution must stay")
}

func TestGroupSubstitutionRepository_BlockersExcludeLegacyPersonnelRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestEducationGroup(t, db, "TypedBlockers")
	target := testpkg.CreateTestStaff(t, db, "Typed", "Target")
	other := testpkg.CreateTestStaff(t, db, "Legacy", "Target")
	today := timezone.TodayDate()
	handover := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, target.ID, today, today)
	testpkg.CreateTestGroupSubstitution(t, db, group.ID, &target.ID, other.ID, today, today)

	blockers, err := repo.ListActiveSubstitutionBlockers(ctx, target.ID, testpkg.Tenant(t))

	require.NoError(t, err)
	require.Len(t, blockers, 1)
	assert.Equal(t, handover.ID, blockers[0].ID)
}

func TestGroupSubstitutionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds existing substitution", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubFindByID")
		substitute := testpkg.CreateTestStaff(t, db, "FindSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("updates substitution reason", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubUpdate")
		substitute := testpkg.CreateTestStaff(t, db, "UpdateSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		sub.Reason = "Updated reason"
		err := repo.Update(ctx, sub)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated reason", found.Reason)
	})
}

func TestGroupSubstitutionRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing substitution", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubDelete")
		substitute := testpkg.CreateTestStaff(t, db, "DeleteSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("lists all substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubList")
		substitute := testpkg.CreateTestStaff(t, db, "ListSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		subs, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_ListWithOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("lists with pagination", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubListOpts")
		substitute := testpkg.CreateTestStaff(t, db, "ListOptsSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		subs, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(subs), 10)
	})
}

func TestGroupSubstitutionRepository_FindByGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds substitutions by group ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubByGroup")
		substitute := testpkg.CreateTestStaff(t, db, "ByGroupSubstitute", "Staff")

		startDate := timezone.TodayDate()
		endDate := startDate.AddDays(7)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

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

func TestGroupSubstitutionRepository_FindActive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds active substitutions for date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActive")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubstitute", "Staff")

		// Create substitution that's active today
		today := timezone.TodayDate()
		startDate := today.AddDays(-1) // Yesterday
		endDate := today.AddDays(7)    // Week from now
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		subs, err := repo.FindActive(ctx, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindOverlapping(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds overlapping substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubOverlap")
		substitute := testpkg.CreateTestStaff(t, db, "OverlapSubstitute", "Staff")

		// Create substitution from today for 7 days
		today := timezone.NewDate(2026, 8, 24)
		startDate := today
		endDate := today.AddDays(7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		// Check for overlapping period (3 days in the middle)
		checkStart := today.AddDays(2)
		checkEnd := today.AddDays(5)

		subs, err := repo.FindOverlapping(ctx, substitute.ID, checkStart, checkEnd)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("returns empty for non-overlapping period", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubNoOverlap")
		substitute := testpkg.CreateTestStaff(t, db, "NoOverlapSubstitute", "Staff")

		// Create substitution for next week
		today := timezone.NewDate(2026, 8, 24)
		startDate := today.AddDays(7)
		endDate := today.AddDays(14)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		// Check for this week (should not overlap)
		checkStart := today
		checkEnd := today.AddDays(3)

		subs, err := repo.FindOverlapping(ctx, substitute.ID, checkStart, checkEnd)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestGroupSubstitutionRepository_FindActiveBySubstitute(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds active substitutions by substitute staff and date", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveSubstitute")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubstitute", "Staff")

		today := timezone.TodayDate()
		startDate := today.AddDays(-1)
		endDate := today.AddDays(7)
		sub := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

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
		today := timezone.TodayDate()
		startDate := today.AddDays(-14)
		endDate := today.AddDays(-7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		subs, err := repo.FindActiveBySubstitute(ctx, substitute.ID, today)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupSubstitutionRepository_Create_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nil substitution", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for invalid date range", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubValidation")
		substitute := testpkg.CreateTestStaff(t, db, "ValidationSub", "Staff")

		today := timezone.TodayDate()
		sub := &education.GroupSubstitution{
			TargetType:        education.GroupSubstitutionTypeGroupHandover,
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         today,
			EndDate:           today.AddDays(-7), // End before start
		}

		err := repo.Create(ctx, sub)
		require.Error(t, err)
	})
}

func TestGroupSubstitutionRepository_Update_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("filters by active status", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveFilter")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveFilterSub", "Staff")

		startDate := timezone.NewDate(2000, 1, 1)
		endDate := timezone.NewDate(2100, 1, 1)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

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

		today := timezone.NewDate(2026, 8, 24)
		startDate := today
		endDate := today.AddDays(7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		filters := map[string]interface{}{
			"date": today.AddDays(3), // Middle of range
		}

		subs, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("filters by reason_like", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubReasonFilter")
		substitute := testpkg.CreateTestStaff(t, db, "ReasonFilterSub", "Staff")

		today := timezone.NewDate(2026, 8, 24)
		startDate := today
		endDate := today.AddDays(7)

		sub := &education.GroupSubstitution{
			TargetType:        education.GroupSubstitutionTypeGroupHandover,
			GroupID:           group.ID,
			SubstituteStaffID: substitute.ID,
			StartDate:         startDate,
			EndDate:           endDate,
			Reason:            "Sick leave emergency",
		}
		err := repo.Create(ctx, sub)
		require.NoError(t, err)

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

func TestGroupSubstitutionRepository_ListWithRelations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("loads relations for multiple substitutions", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubListRelations")
		substitute1 := testpkg.CreateTestStaff(t, db, "Sub1", "Person")
		substitute2 := testpkg.CreateTestStaff(t, db, "Sub2", "Person")

		today := timezone.TodayDate()
		startDate := today
		endDate := today.AddDays(7)

		sub1 := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute1.ID, startDate, endDate)
		sub2 := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute2.ID, startDate, endDate)

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
				// Staff.Person is attached by the substitution service through
				// the People Directory (#2661); the repository stops at staff.
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

func TestGroupSubstitutionRepository_FindActiveBySubstituteWithRelations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSubstitution
	ctx := testpkg.Ctx(t)

	t.Run("finds active substitutions by substitute with relations", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "SubActiveSubRel")
		substitute := testpkg.CreateTestStaff(t, db, "ActiveSubRel", "Person")

		today := timezone.TodayDate()
		startDate := today.AddDays(-1)
		endDate := today.AddDays(7)
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, substitute.ID, startDate, endDate)

		subs, err := repo.FindActiveBySubstituteWithRelations(ctx, substitute.ID, today)
		require.NoError(t, err)
		assert.NotEmpty(t, subs)

		// Verify relations are loaded
		found := subs[0]
		assert.NotNil(t, found.Group)
		assert.NotNil(t, found.SubstituteStaff)
	})
}
