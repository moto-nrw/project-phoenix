package activities_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityGroupRepositoryUpdateTemplateFieldsPlanningTrackPresence(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	defer testpkg.CleanupTenantTestData(t, db, scope.TenantID)
	ctx := scope.Context()
	factory := repositories.NewFactory(db)

	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "PlanningTrackPresence")
	room := testpkg.CreateTestRoomForTenant(t, db, scope.TenantID, "PlanningTrackPresence")
	track := &scheduleModel.PlanningTrack{Name: "Bestehend", Color: "#5080D8", SortOrder: 0}
	require.NoError(t, factory.PlanningTrack.Create(ctx, track))
	_, err := db.NewUpdate().Table("activities.groups").
		Set("is_template = true").
		Set("planned_room_id = ?", room.ID).
		Set("planning_track_id = ?", track.ID).
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", group.ID).
		Exec(ctx)
	require.NoError(t, err)

	persisted, err := factory.ActivityGroup.FindByID(ctx, group.ID)
	require.NoError(t, err)
	fields := activities.TemplateFieldsUpdate{
		Name:              persisted.Name,
		Type:              persisted.Type,
		CategoryID:        persisted.CategoryID,
		RoomID:            room.ID,
		EducationGroupID:  persisted.EducationGroupID,
		MaxParticipants:   persisted.MaxParticipants,
		RequiredStaff:     persisted.RequiredStaff,
		CalendarPeriodID:  persisted.CalendarPeriodID,
		TargetGroupType:   persisted.TargetGroupType,
		TargetGradeLevel:  persisted.TargetGradeLevel,
		TargetSchoolClass: persisted.TargetSchoolClass,
		ListKind:          persisted.ListKind,
		Notes:             persisted.Notes,
	}

	_, err = factory.ActivityGroup.UpdateTemplateFields(ctx, group.ID, fields)
	require.NoError(t, err)
	persisted, err = factory.ActivityGroup.FindByID(ctx, group.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted.PlanningTrackID)
	assert.Equal(t, track.ID, *persisted.PlanningTrackID)

	fields.PlanningTrackIDProvided = true
	fields.PlanningTrackID = nil
	_, err = factory.ActivityGroup.UpdateTemplateFields(ctx, group.ID, fields)
	require.NoError(t, err)
	persisted, err = factory.ActivityGroup.FindByID(ctx, group.ID)
	require.NoError(t, err)
	assert.Nil(t, persisted.PlanningTrackID)
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestActivityGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("creates activity group with valid data", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "GroupCreate")
		staff := testpkg.CreateTestStaff(t, db, "GroupCreate", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("TestGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 20,
			IsOpen:          true,
			CreatedBy:       &staff.ID,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)
		assert.Equal(t, activities.TargetGroupTypeNone, group.TargetGroupType)

		persisted, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, activities.TargetGroupTypeNone, persisted.TargetGroupType)

		testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
	})

	t.Run("creates closed activity group", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ClosedGroup")
		staff := testpkg.CreateTestStaff(t, db, "ClosedGroup", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("ClosedGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 15,
			IsOpen:          false,
			CreatedBy:       &staff.ID,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.False(t, group.IsOpen)

		testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
	})
}

func TestActivityGroupTargets_AreTenantScoped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	defer testpkg.CleanupTenantTestData(t, db, tenantA, tenantB)

	groupA := testpkg.CreateTestActivityGroupForTenant(t, db, tenantA, "Target-A")
	groupB := testpkg.CreateTestActivityGroupForTenant(t, db, tenantB, "Target-B")
	repo, ok := repositories.NewFactory(db).ActivityGroup.(activities.GroupTargetRepository)
	require.True(t, ok)
	classA := "1a"
	classB := "2a"
	require.NoError(t, repo.ReplaceTargets(testpkg.TenantContext(tenantA), groupA.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classA},
	}))
	require.NoError(t, repo.ReplaceTargets(testpkg.TenantContext(tenantB), groupB.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classB},
	}))

	visible, err := repo.FindTargetsByGroupIDs(testpkg.TenantContext(tenantA), []int64{groupA.ID, groupB.ID})
	require.NoError(t, err)
	require.Len(t, visible[groupA.ID], 1)
	assert.Empty(t, visible[groupB.ID])

	err = repo.ReplaceTargets(testpkg.TenantContext(tenantA), groupB.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classA},
	})
	require.Error(t, err)
}

func TestActivityGroupTargets_ReplaceIsAtomic(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Atomic-Targets")
	repo, ok := repositories.NewFactory(db).ActivityGroup.(activities.GroupTargetRepository)
	require.True(t, ok)
	ctx := testpkg.TenantContext(tenantID)
	classA := "1a"
	require.NoError(t, repo.ReplaceTargets(ctx, group.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classA},
	}))

	gradeOne := int16(1)
	err := repo.ReplaceTargets(ctx, group.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classA},
		{TargetGroupType: activities.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeOne},
	})
	require.ErrorContains(t, err, "same type")

	classB := "2a"
	err = repo.ReplaceTargets(ctx, group.ID, []*activities.GroupTarget{
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classB},
		{TargetGroupType: activities.TargetGroupTypeKlasse, TargetSchoolClass: &classB},
	})
	require.Error(t, err)

	stored, err := repo.FindTargetsByGroupIDs(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Len(t, stored[group.ID], 1)
	assert.Equal(t, classA, *stored[group.ID][0].TargetSchoolClass)
}

func TestActivityGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("updates activity group name", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		newName := fmt.Sprintf("UpdatedName-%d", time.Now().UnixNano())
		group.Name = newName
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})

	t.Run("updates activity group open status", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateIsOpen")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		group.IsOpen = false
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.False(t, found.IsOpen)
	})

	t.Run("canonicalizes an empty target group type before a generic update", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateEmptyTargetGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		group.TargetGroupType = ""
		require.NoError(t, repo.Update(ctx, group))
		assert.Equal(t, activities.TargetGroupTypeNone, group.TargetGroupType)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, activities.TargetGroupTypeNone, found.TargetGroupType)
	})
}

func TestActivityGroupRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("deletes existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Delete")
		categoryID := group.CategoryID
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, categoryID, 0)

		err := repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, group.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestActivityGroupRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("lists all activity groups", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("lists with filter using id field", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ListFilter")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		// Create filter with id field - this tests the table alias fix
		// to avoid ambiguous column reference with the category join
		options := base.NewQueryOptions()
		options.Filter.Equal("id", group.ID)

		groups, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, group.ID, groups[0].ID)
	})
}

func TestActivityGroupRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds groups by category ID", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ByCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		groups, err := repo.FindByCategory(ctx, group.CategoryID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for category with no groups", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "EmptyCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		groups, err := repo.FindByCategory(ctx, category.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActivityGroupRepository_FindOpenGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds only open groups", func(t *testing.T) {
		// Create an open group
		openGroup := testpkg.CreateTestActivityGroup(t, db, "IsOpenGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, openGroup.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", openGroup.ID)

		groups, err := repo.FindOpenGroups(ctx)
		require.NoError(t, err)

		// All returned groups should be open
		for _, g := range groups {
			assert.True(t, g.IsOpen)
		}

		// Our open group should be in the results
		var found bool
		for _, g := range groups {
			if g.ID == openGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestActivityGroupRepository_FindAllTemplates(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns only groups flagged as templates", func(t *testing.T) {
		// Template group — should be returned.
		templateGroup := testpkg.CreateTestActivityGroup(t, db, "TemplateGroup")
		templateGroup.IsTemplate = true
		require.NoError(t, repo.Update(ctx, templateGroup))
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, templateGroup.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", templateGroup.ID)

		// Non-template group — should NOT be returned.
		regularGroup := testpkg.CreateTestActivityGroup(t, db, "RegularGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, regularGroup.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", regularGroup.ID)

		templates, err := repo.FindAllTemplates(ctx)
		require.NoError(t, err)

		// All returned groups must be templates.
		for _, g := range templates {
			assert.True(t, g.IsTemplate, "FindAllTemplates must only return is_template=true rows")
		}

		// Template group must be present; non-template must not.
		var foundTemplate, foundRegular bool
		for _, g := range templates {
			if g.ID == templateGroup.ID {
				foundTemplate = true
			}
			if g.ID == regularGroup.ID {
				foundRegular = true
			}
		}
		assert.True(t, foundTemplate, "template group must be returned")
		assert.False(t, foundRegular, "non-template group must not be returned")
	})

	t.Run("returns empty slice when no templates exist", func(t *testing.T) {
		// Isolate this case in a fresh tenant so seed/other templates don't leak in.
		isoCtx := testpkg.TenantContext(999999)
		templates, err := repo.FindAllTemplates(isoCtx)
		require.NoError(t, err)
		assert.Empty(t, templates, "no templates must return empty slice, not error")
	})
}

func TestActivityGroupRepository_FindWithEnrollmentCounts(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns groups with enrollment counts", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WithEnrollments")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		// Create some enrollments
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, 0, 0, 0, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student2.ID, 0, 0, 0, 0)

		// Add enrollments directly
		enrollment1 := &activities.StudentEnrollment{
			StudentID:       student1.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
		}
		enrollment1.SetTenantID(1)
		_, _ = db.NewInsert().
			Model(enrollment1).
			ModelTableExpr(`activities.student_enrollments`).
			Exec(ctx)

		enrollment2 := &activities.StudentEnrollment{
			StudentID:       student2.ID,
			ActivityGroupID: group.ID,
			ValidFrom:       timezone.TodayDate(),
		}
		enrollment2.SetTenantID(1)
		_, _ = db.NewInsert().
			Model(enrollment2).
			ModelTableExpr(`activities.student_enrollments`).
			Exec(ctx)

		groups, counts, err := repo.FindWithEnrollmentCounts(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
		assert.NotNil(t, counts)

		// Check that our group has the correct count
		count, exists := counts[group.ID]
		assert.True(t, exists)
		assert.Equal(t, 2, count)
	})

	t.Run("returns empty map when no groups exist", func(t *testing.T) {
		// This test assumes at least some groups exist from seeding
		// We're just checking the function doesn't fail
		groups, counts, err := repo.FindWithEnrollmentCounts(ctx)
		require.NoError(t, err)
		assert.NotNil(t, groups)
		assert.NotNil(t, counts)
	})
}

func TestActivityGroupRepository_FindWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns group with supervisors", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WithSupervisors")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		// Add a supervisor
		sup := &activities.SupervisorPlanned{
			GroupID:   group.ID,
			StaffID:   staff.ID,
			IsPrimary: true,
		}
		sup.SetTenantID(1)
		_, _ = db.NewInsert().
			Model(sup).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)

		foundGroup, supervisors, err := repo.FindWithSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.NotNil(t, foundGroup)
		assert.Equal(t, group.ID, foundGroup.ID)
		assert.NotEmpty(t, supervisors)
		assert.Equal(t, staff.ID, supervisors[0].StaffID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, _, err := repo.FindWithSupervisors(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_FindByStaffSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds groups supervised by staff member", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "BySupervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		staff := testpkg.CreateTestStaff(t, db, "Finding", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		// Add supervisor assignment
		sup := &activities.SupervisorPlanned{
			GroupID:   group.ID,
			StaffID:   staff.ID,
			IsPrimary: true,
		}
		sup.SetTenantID(1)
		_, _ = db.NewInsert().
			Model(sup).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)

		groups, err := repo.FindByStaffSupervisor(ctx, staff.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for staff with no supervised groups", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NoGroups", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		groups, err := repo.FindByStaffSupervisor(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

// ============================================================================
// Edge Cases and Validation Tests
// ============================================================================

func TestActivityGroupRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when group is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActivityGroupRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns error when group is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActivityGroupRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("does not error when deleting non-existent group", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}

func TestActivityGroupRepository_FindByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := testpkg.TenantContext(1)

	t.Run("empty input short-circuits without hitting the DB", func(t *testing.T) {
		groups, err := repo.FindByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("returns the groups matching the ids", func(t *testing.T) {
		a := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("FindByIDs-A-%d", time.Now().UnixNano()))
		b := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("FindByIDs-B-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", a.ID, b.ID)

		groups, err := repo.FindByIDs(ctx, []int64{a.ID, b.ID, 9_999_999})
		require.NoError(t, err)
		require.Len(t, groups, 2, "the non-existent id is silently absent")

		names := map[int64]string{}
		for _, g := range groups {
			names[g.ID] = g.Name
		}
		assert.Equal(t, a.Name, names[a.ID])
		assert.Equal(t, b.Name, names[b.ID])
	})
}
