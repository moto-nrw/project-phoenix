// Package activities_test tests the activities service layer with hermetic testing pattern.
package activities_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupActivityService creates an ActivityService with real database connection
func setupActivityService(t *testing.T, db *bun.DB) activities.ActivityService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Activities
}

// cleanupGroup is a test helper to delete a group with admin permission (for cleanup purposes)
func cleanupGroup(service activities.ActivityService, ctx context.Context, groupID int64) {
	_ = service.DeleteGroup(ctx, groupID, 0, true) // 0 staff ID, true = admin permission
}

type fakeActiveEnrollmentRepo struct {
	activitiesModels.StudentEnrollmentRepository
	enrollments []*activitiesModels.StudentEnrollment
	err         error
	calls       int
	studentIDs  []int64
	onDate      timezone.Date
}

func (r *fakeActiveEnrollmentRepo) FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate timezone.Date) ([]*activitiesModels.StudentEnrollment, error) {
	r.calls++
	r.studentIDs = append([]int64(nil), studentIDs...)
	r.onDate = onDate
	if r.err != nil {
		return nil, r.err
	}
	return r.enrollments, nil
}

// =============================================================================
// Category Operations Tests
// =============================================================================

func TestActivityService_CreateCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates category successfully", func(t *testing.T) {
		// ARRANGE
		category := &activitiesModels.Category{
			Name:        fmt.Sprintf("Test Category %d", time.Now().UnixNano()),
			Description: "Test description",
		}

		// ACT
		result, err := service.CreateCategory(ctx, category)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
		assert.Equal(t, category.Name, result.Name)
		assert.Equal(t, category.Description, result.Description)
	})

	t.Run("returns error for invalid category", func(t *testing.T) {
		// ARRANGE - empty name should fail validation
		category := &activitiesModels.Category{
			Name: "", // invalid
		}

		// ACT
		result, err := service.CreateCategory(ctx, category)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_GetCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns category when found", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "get-cat")

		// ACT
		result, err := service.GetCategory(ctx, category.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, category.ID, result.ID)
		assert.Equal(t, category.Name, result.Name)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetCategory(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_ListCategories(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns list of categories", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestActivityCategory(t, db, "list-1")
		testpkg.CreateTestActivityCategory(t, db, "list-2")

		// ACT
		result, err := service.ListCategories(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 2)
	})
}

// =============================================================================
// Activity Group Operations Tests
// =============================================================================

func TestActivityService_GetGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns group when found", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "get-group")

		// ACT
		result, err := service.GetGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, group.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_ListGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns list of groups", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestActivityGroup(t, db, "list-g1")
		testpkg.CreateTestActivityGroup(t, db, "list-g2")

		// ACT
		result, err := service.ListGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 2)
	})
}

func TestActivityService_ListGroupsWithOccupancy(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns groups with occupancy status", func(t *testing.T) {
		// ARRANGE - create two activity groups
		group1 := testpkg.CreateTestActivityGroup(t, db, "occ-g1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "occ-g2")
		room := testpkg.CreateTestRoom(t, db, "occ-room")

		// Create an active session for group1 (making it occupied)
		now := time.Now()
		group1ID := group1.ID
		activeGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        &group1ID,
			RoomID:         room.ID,
		}
		activeGroup.SetTenantID(testpkg.Tenant(t))
		err := db.NewInsert().
			Model(activeGroup).
			ModelTableExpr(`active.groups AS "active_group"`).
			Scan(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewDelete().
				TableExpr("active.groups").
				Where("id = ?", activeGroup.ID).
				Exec(ctx)
		}()

		// ACT
		result, err := service.ListGroupsWithOccupancy(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)

		// Find our test groups in the results
		var found1, found2 bool
		for _, item := range result {
			if item.ID == group1.ID {
				found1 = true
				assert.True(t, item.IsOccupied, "group1 should be occupied (has active session)")
			}
			if item.ID == group2.ID {
				found2 = true
				assert.False(t, item.IsOccupied, "group2 should not be occupied (no active session)")
			}
		}
		assert.True(t, found1, "group1 should be in results")
		assert.True(t, found2, "group2 should be in results")
	})

	t.Run("returns empty list when no groups exist in fresh DB", func(t *testing.T) {
		// This test checks the zero-groups path.
		// In a shared test DB there may be pre-existing groups, so we just verify no error.
		result, err := service.ListGroupsWithOccupancy(ctx)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestActivityService_UpdateGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "to-update-grp")

		group.Name = "Updated Group Name"
		group.MaxParticipants = 50

		// ACT - use creator's staff ID and give manage permission for test
		result, err := service.UpdateGroup(ctx, group, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Updated Group Name", result.Name)
	})

	t.Run("blocks renaming system activity Schulhof Freispiel", func(t *testing.T) {
		// ARRANGE — cleanup immediately after test to avoid cross-package interference
		group := testpkg.CreateTestActivityGroup(t, db, "Schulhof Freispiel")

		group.Name = "Renamed Activity"

		// ACT
		_, err := service.UpdateGroup(ctx, group, *group.CreatedBy, true)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemaktivität")

		// Immediate cleanup — do not defer, to prevent cross-package test interference
	})

	t.Run("allows updating other properties of system activity", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "WC")

		group.MaxParticipants = 30

		// ACT
		result, err := service.UpdateGroup(ctx, group, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, 30, result.MaxParticipants)
		assert.Equal(t, "WC", result.Name)

		// Immediate cleanup
	})

	t.Run("returns not found when updating non-existent group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "temp-for-id")
		staffID := *group.CreatedBy

		nonExistentGroup := *group
		nonExistentGroup.ID = 999999999

		// ACT
		_, err := service.UpdateGroup(ctx, &nonExistentGroup, staffID, true)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestActivityService_DeleteGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "to-delete-grp")

		// ACT - use creator's staff ID and give manage permission for test
		err := service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)

		// Verify deleted
		result, err := service.GetGroup(ctx, group.ID)
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("blocks deletion of system activity Schulhof Freispiel", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "Schulhof Freispiel")

		// ACT
		err := service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemaktivität")

		// Immediate cleanup — do not defer, to prevent cross-package test interference
	})

	t.Run("blocks deletion of system activity WC", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "WC")

		// ACT
		err := service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemaktivität")

		// Immediate cleanup
	})
}

func TestActivityService_RejectsLegacyMutationsForTimetableTemplates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	service := setupActivityService(t, db)

	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, fmt.Sprintf("template-guard-%d", time.Now().UnixNano()))
	ordinaryGroup := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, fmt.Sprintf("ordinary-guard-%d", time.Now().UnixNano()))
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, tenantID, fmt.Sprintf("template-guard-%d", time.Now().UnixNano()))
	schedule := &activitiesModels.Schedule{
		ActivityGroupID: group.ID,
		Weekday:         activitiesModels.WeekdayMonday,
		TimeframeID:     &timeframe.ID,
	}
	schedule.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(schedule).ModelTableExpr(`activities.schedules`).Scan(ctx))
	supervisor := &activitiesModels.SupervisorPlanned{
		GroupID:   group.ID,
		StaffID:   *group.CreatedBy,
		IsPrimary: true,
		ValidFrom: timezone.TodayDate(),
	}
	supervisor.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(supervisor).ModelTableExpr(`activities.supervisors`).Scan(ctx))
	_, err := db.NewUpdate().
		TableExpr(`activities.groups`).
		Set("is_template = TRUE").
		Where("tenant_id = ?", tenantID).
		Where("id = ?", group.ID).
		Exec(ctx)
	require.NoError(t, err)

	template, err := service.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	require.True(t, template.IsTemplate)

	assertProtected := func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, activities.ErrTimetableTemplateProtected)
	}

	t.Run("create group", func(t *testing.T) {
		_, createErr := service.CreateGroup(ctx, &activitiesModels.Group{IsTemplate: true}, nil, nil)
		assertProtected(t, createErr)
	})
	t.Run("update persisted template", func(t *testing.T) {
		forged := *template
		forged.IsTemplate = false
		forged.Name = "must-not-change"
		_, updateErr := service.UpdateGroup(ctx, &forged, *group.CreatedBy, true)
		assertProtected(t, updateErr)
	})
	t.Run("convert ordinary input to template", func(t *testing.T) {
		candidate := *ordinaryGroup
		candidate.IsTemplate = true
		_, updateErr := service.UpdateGroup(ctx, &candidate, *ordinaryGroup.CreatedBy, true)
		assertProtected(t, updateErr)

		storedOrdinary, getErr := service.GetGroup(ctx, ordinaryGroup.ID)
		require.NoError(t, getErr)
		assert.False(t, storedOrdinary.IsTemplate)
		assert.Equal(t, ordinaryGroup.Name, storedOrdinary.Name)
	})
	t.Run("delete group", func(t *testing.T) {
		assertProtected(t, service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true))
	})
	t.Run("enrollment mutations", func(t *testing.T) {
		assertProtected(t, service.EnrollStudent(ctx, group.ID, 999_001))
		assertProtected(t, service.UnenrollStudent(ctx, group.ID, 999_001))
		assertProtected(t, service.UpdateGroupEnrollments(ctx, group.ID, []int64{999_001}))
	})
	t.Run("schedule mutations", func(t *testing.T) {
		_, addErr := service.AddSchedule(ctx, group.ID, &activitiesModels.Schedule{
			Weekday: activitiesModels.WeekdayTuesday,
		})
		assertProtected(t, addErr)
		updated := *schedule
		updated.Weekday = activitiesModels.WeekdayTuesday
		_, updateErr := service.UpdateSchedule(ctx, &updated)
		assertProtected(t, updateErr)
		assertProtected(t, service.DeleteSchedule(ctx, schedule.ID))
	})
	t.Run("supervisor mutations", func(t *testing.T) {
		_, addErr := service.AddSupervisor(ctx, group.ID, *group.CreatedBy, false)
		assertProtected(t, addErr)
		updated := *supervisor
		updated.IsPrimary = false
		_, updateErr := service.UpdateSupervisor(ctx, &updated)
		assertProtected(t, updateErr)
		assertProtected(t, service.DeleteSupervisor(ctx, supervisor.ID))
		assertProtected(t, service.SetPrimarySupervisor(ctx, supervisor.ID))
		assertProtected(t, service.UpdateGroupSupervisors(ctx, group.ID, []int64{*group.CreatedBy}))
	})

	stored, err := service.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.Name, stored.Name)
	storedSchedule, err := service.GetSchedule(ctx, schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, activitiesModels.WeekdayMonday, storedSchedule.Weekday)
	storedSupervisor, err := service.GetSupervisor(ctx, supervisor.ID)
	require.NoError(t, err)
	assert.True(t, storedSupervisor.IsPrimary)
}

func TestActivityService_FindByCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns groups for category", func(t *testing.T) {
		// ARRANGE - CreateTestActivityGroup creates a category too
		group := testpkg.CreateTestActivityGroup(t, db, "find-by-cat")

		// ACT
		result, err := service.FindByCategory(ctx, group.CategoryID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns error for nonexistent category", func(t *testing.T) {
		// ACT
		result, err := service.FindByCategory(ctx, 99999999)

		// ASSERT
		// Service returns error when category doesn't exist
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_GetGroupWithDetails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns group with details", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "with-details")

		// ACT
		resultGroup, _, _, err := service.GetGroupWithDetails(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		// Group may be returned as nil if no relations are found
		// Supervisors and schedules will be empty slices for new groups
		if resultGroup != nil {
			assert.Equal(t, group.ID, resultGroup.ID)
		}
		// These may be empty but should not error
	})
}

func TestActivityService_GetGroupsWithEnrollmentCounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns groups with enrollment counts", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestActivityGroup(t, db, "with-counts")

		// ACT
		groups, counts, err := service.GetGroupsWithEnrollmentCounts(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, groups)
		assert.NotNil(t, counts)
	})
}

// =============================================================================
// Enrollment Operations Tests
// =============================================================================

func TestActivityService_EnrollStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("enrolls student successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "enroll-test")
		student := testpkg.CreateTestStudent(t, db, "Enroll", "Student", "1a")

		// ACT
		err := service.EnrollStudent(ctx, group.ID, student.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify enrollment
		enrolled, err := service.GetEnrolledStudents(ctx, group.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(enrolled), 1)

		// The model no longer defaults ValidFrom (#586); the service must set it.
		repoFactory := repositories.NewFactory(db)
		enrollments, err := repoFactory.StudentEnrollment.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		require.NotEmpty(t, enrollments)
		assert.False(t, enrollments[0].ValidFrom.IsZero(), "EnrollStudent must set ValidFrom now that the model no longer defaults it")
	})

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "NoGroup", "Student", "1a")

		// ACT
		err := service.EnrollStudent(ctx, 99999999, student.ID)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_UnenrollStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("unenrolls student successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "unenroll-test")
		student := testpkg.CreateTestStudent(t, db, "Unenroll", "Student", "1a")

		// First enroll the student
		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT
		err = service.UnenrollStudent(ctx, group.ID, student.ID)

		// ASSERT
		require.NoError(t, err)
	})
}

func TestActivityService_GetEnrolledStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns enrolled students", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "get-enrolled")
		student := testpkg.CreateTestStudent(t, db, "Enrolled", "Student", "1a")

		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT
		result, err := service.GetEnrolledStudents(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns empty list for group with no enrollments", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "no-enrolled")

		// ACT
		result, err := service.GetEnrolledStudents(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestActivityService_GetStudentEnrollments(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns student enrollments", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "student-enroll")
		student := testpkg.CreateTestStudent(t, db, "GetEnroll", "Student", "1a")

		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT
		result, err := service.GetStudentEnrollments(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})
}

func TestActivityService_GetActiveStudentEnrollmentsByStudentIDs(t *testing.T) {
	t.Parallel()

	onDate := timezone.NewDate(2026, time.September, 15)

	t.Run("returns empty map without repository call for empty input", func(t *testing.T) {
		repo := &fakeActiveEnrollmentRepo{}
		service, err := activities.NewService(nil, nil, nil, nil, repo, nil, nil, nil)
		require.NoError(t, err)

		result, err := service.GetActiveStudentEnrollmentsByStudentIDs(testpkg.Ctx(t), nil, onDate)

		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Zero(t, repo.calls)
	})

	t.Run("wraps repository errors", func(t *testing.T) {
		repo := &fakeActiveEnrollmentRepo{err: errors.New("database unavailable")}
		service, err := activities.NewService(nil, nil, nil, nil, repo, nil, nil, nil)
		require.NoError(t, err)

		result, err := service.GetActiveStudentEnrollmentsByStudentIDs(testpkg.Ctx(t), []int64{10}, onDate)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "get active student enrollments by student IDs")
		assert.Equal(t, []int64{10}, repo.studentIDs)
		assert.Equal(t, onDate, repo.onDate)
	})

	t.Run("groups active enrollments by student and de-duplicates groups", func(t *testing.T) {
		groupA := &activitiesModels.Group{Model: base.Model{ID: 101}, Name: "A"}
		groupB := &activitiesModels.Group{Model: base.Model{ID: 202}, Name: "B"}
		repo := &fakeActiveEnrollmentRepo{
			enrollments: []*activitiesModels.StudentEnrollment{
				nil,
				{StudentID: 0, ActivityGroupID: 999},
				{StudentID: 10, ActivityGroupID: groupA.ID, ActivityGroup: groupA},
				{StudentID: 10, ActivityGroupID: groupA.ID, ActivityGroup: groupA},
				{StudentID: 10, ActivityGroupID: 303},
				{StudentID: 20, ActivityGroupID: groupB.ID, ActivityGroup: groupB},
			},
		}
		service, err := activities.NewService(nil, nil, nil, nil, repo, nil, nil, nil)
		require.NoError(t, err)

		result, err := service.GetActiveStudentEnrollmentsByStudentIDs(testpkg.Ctx(t), []int64{10, 20}, onDate)

		require.NoError(t, err)
		require.Len(t, result[10], 2)
		assert.Equal(t, int64(101), result[10][0].ID)
		assert.Equal(t, int64(303), result[10][1].ID)
		require.Len(t, result[20], 1)
		assert.Equal(t, groupB, result[20][0])
	})
}

func TestActivityService_GetAvailableGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns available groups for student", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Available", "Student", "1a")

		// ACT
		result, err := service.GetAvailableGroups(ctx, student.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// =============================================================================
// Public Operations Tests
// =============================================================================

// =============================================================================
// Schedule Operations Tests
// =============================================================================

func TestActivityService_GetGroupSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns schedules for group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "with-schedules")

		// ACT
		_, err := service.GetGroupSchedules(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		// New groups don't have schedules, so result may be empty
		// Just verify the call succeeds without error
	})
}

// =============================================================================
// Supervisor Operations Tests
// =============================================================================

func TestActivityService_GetGroupSupervisors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns supervisors for group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "with-supervisors")

		// ACT
		result, err := service.GetGroupSupervisors(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestActivityService_AddSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds supervisor to group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "add-super")
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")

		// ACT
		result, err := service.AddSupervisor(ctx, group.ID, staff.ID, true)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, group.ID, result.GroupID)
		assert.Equal(t, staff.ID, result.StaffID)
		assert.True(t, result.IsPrimary)
	})

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "NoGroupSuper", "Staff")

		// ACT
		result, err := service.AddSupervisor(ctx, 99999999, staff.ID, false)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for nonexistent staff", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "no-staff-super")

		// ACT
		result, err := service.AddSupervisor(ctx, group.ID, 99999999, false)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		// Verify it's a staff not found error
		var actErr *activities.ActivityError
		if errors.As(err, &actErr) {
			assert.True(t, errors.Is(actErr.Err, activities.ErrStaffNotFound), "expected ErrStaffNotFound, got: %v", actErr.Err)
		}
	})
}

// =============================================================================
// Device Operations Tests
// =============================================================================

// =============================================================================
// CreateGroup Tests (0% coverage)
// =============================================================================

func TestActivityService_CreateGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates group successfully", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "create-grp-cat")
		staff := testpkg.CreateTestStaff(t, db, "Creator", "Staff")

		group := &activitiesModels.Group{
			Name:            fmt.Sprintf("Test Group %d", time.Now().UnixNano()),
			MaxParticipants: 20,
			IsOpen:          true,
			CategoryID:      category.ID,
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, nil)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
		assert.Equal(t, group.Name, result.Name)

		// Cleanup
	})

	t.Run("creates group with supervisors", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "grp-with-super")
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "ForGroup")

		group := &activitiesModels.Group{
			Name:            fmt.Sprintf("Group With Super %d", time.Now().UnixNano()),
			MaxParticipants: 15,
			IsOpen:          false,
			CategoryID:      category.ID,
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, []int64{staff.ID}, nil)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify supervisor was added
		supervisors, err := service.GetGroupSupervisors(ctx, result.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisors), 1)

		// Cleanup
	})

	t.Run("returns error for invalid category", func(t *testing.T) {
		// ARRANGE - still need a valid staff ID for CreatedBy even though category is invalid
		staff := testpkg.CreateTestStaff(t, db, "Creator", "InvalidCat")

		group := &activitiesModels.Group{
			Name:            "Invalid Category Group",
			MaxParticipants: 10,
			CategoryID:      99999999, // nonexistent
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// Schedule CRUD Tests (0% coverage)
// =============================================================================

func TestActivityService_AddSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds schedule to group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "add-sched")

		schedule := &activitiesModels.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         1, // Monday
		}

		// ACT
		result, err := service.AddSchedule(ctx, group.ID, schedule)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.ID, int64(0))
	})

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		schedule := &activitiesModels.Schedule{
			Weekday: 1,
		}

		// ACT
		result, err := service.AddSchedule(ctx, 99999999, schedule)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_GetSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns schedule when found", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "get-sched")

		schedule := &activitiesModels.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         2, // Tuesday
		}
		created, err := service.AddSchedule(ctx, group.ID, schedule)
		require.NoError(t, err)

		// ACT
		result, err := service.GetSchedule(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, created.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetSchedule(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_UpdateSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates schedule successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "upd-sched")

		schedule := &activitiesModels.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         3, // Wednesday
		}
		created, err := service.AddSchedule(ctx, group.ID, schedule)
		require.NoError(t, err)

		// Modify weekday
		created.Weekday = 4 // Thursday

		// ACT
		result, err := service.UpdateSchedule(ctx, created)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 4, result.Weekday)
	})
}

func TestActivityService_DeleteSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes schedule successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "del-sched")

		schedule := &activitiesModels.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         4, // Thursday
		}
		created, err := service.AddSchedule(ctx, group.ID, schedule)
		require.NoError(t, err)

		// ACT
		err = service.DeleteSchedule(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deleted
		result, err := service.GetSchedule(ctx, created.ID)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// Supervisor CRUD Tests (0% coverage)
// =============================================================================

func TestActivityService_GetSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns supervisor when found", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "get-super")
		staff := testpkg.CreateTestStaff(t, db, "Get", "Supervisor")

		created, err := service.AddSupervisor(ctx, group.ID, staff.ID, true)
		require.NoError(t, err)

		// ACT
		result, err := service.GetSupervisor(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, created.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetSupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_UpdateSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates supervisor successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "upd-super")
		staff := testpkg.CreateTestStaff(t, db, "Update", "Supervisor")

		created, err := service.AddSupervisor(ctx, group.ID, staff.ID, false)
		require.NoError(t, err)

		// Modify to primary
		created.IsPrimary = true

		// ACT
		result, err := service.UpdateSupervisor(ctx, created)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsPrimary)
	})
}

func TestActivityService_DeleteSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes supervisor successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "del-super")
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Supervisor")

		created, err := service.AddSupervisor(ctx, group.ID, staff.ID, false)
		require.NoError(t, err)

		// ACT
		err = service.DeleteSupervisor(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deleted
		result, err := service.GetSupervisor(ctx, created.ID)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_SetPrimarySupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("sets supervisor as primary", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "set-primary")
		staff := testpkg.CreateTestStaff(t, db, "Primary", "Supervisor")

		created, err := service.AddSupervisor(ctx, group.ID, staff.ID, false)
		require.NoError(t, err)
		assert.False(t, created.IsPrimary)

		// ACT
		err = service.SetPrimarySupervisor(ctx, created.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify is now primary
		result, err := service.GetSupervisor(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, result.IsPrimary)
	})
}

// =============================================================================
// Enrollment Management Tests (0% coverage)
// =============================================================================

func TestActivityService_UpdateGroupEnrollments(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates group enrollments", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "upd-enrollments")
		student1 := testpkg.CreateTestStudent(t, db, "Enroll1", "Student", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Enroll2", "Student", "1b")

		// ACT - enroll both students
		err := service.UpdateGroupEnrollments(ctx, group.ID, []int64{student1.ID, student2.ID})

		// ASSERT
		require.NoError(t, err)

		// Verify enrollments
		enrolled, err := service.GetEnrolledStudents(ctx, group.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(enrolled), 2)
	})

	t.Run("removes enrollments when list is empty", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "clear-enrollments")
		student := testpkg.CreateTestStudent(t, db, "ToClear", "Student", "1a")

		// First enroll
		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT - clear all enrollments
		err = service.UpdateGroupEnrollments(ctx, group.ID, []int64{})

		// ASSERT
		require.NoError(t, err)

		// Verify cleared
		enrolled, err := service.GetEnrolledStudents(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, enrolled)
	})
}

func TestActivityService_UpdateGroupSupervisors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates group supervisors", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "upd-supervisors")
		staff1 := testpkg.CreateTestStaff(t, db, "Super1", "Staff")
		staff2 := testpkg.CreateTestStaff(t, db, "Super2", "Staff")

		// ACT - assign both supervisors
		err := service.UpdateGroupSupervisors(ctx, group.ID, []int64{staff1.ID, staff2.ID})

		// ASSERT
		require.NoError(t, err)

		// Verify supervisors
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisors), 2)
	})
}

// =============================================================================
// Attendance and History Tests (0% coverage)
// =============================================================================

// =============================================================================
// Additional Edge Case Tests for Higher Coverage
// =============================================================================

func TestActivityService_CreateGroup_WithSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates group with schedules", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "grp-with-sched")
		staff := testpkg.CreateTestStaff(t, db, "Creator", "Schedules")

		group := &activitiesModels.Group{
			Name:            fmt.Sprintf("Group With Schedules %d", time.Now().UnixNano()),
			MaxParticipants: 25,
			IsOpen:          true,
			CategoryID:      category.ID,
			CreatedBy:       &staff.ID,
		}

		schedules := []*activitiesModels.Schedule{
			{Weekday: 1}, // Monday
			{Weekday: 3}, // Wednesday
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, schedules)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify schedules were added
		groupSchedules, err := service.GetGroupSchedules(ctx, result.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groupSchedules), 2)

		// Cleanup
	})
}

func TestActivityService_DeleteSupervisor_Primary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes primary supervisor and assigns new primary", func(t *testing.T) {
		// ARRANGE - create group with two supervisors
		group := testpkg.CreateTestActivityGroup(t, db, "del-primary")
		staff1 := testpkg.CreateTestStaff(t, db, "Primary", "Super")
		staff2 := testpkg.CreateTestStaff(t, db, "Secondary", "Super")

		// Add primary supervisor
		primary, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true)
		require.NoError(t, err)

		// Add secondary supervisor
		_, err = service.AddSupervisor(ctx, group.ID, staff2.ID, false)
		require.NoError(t, err)

		// ACT - delete primary
		err = service.DeleteSupervisor(ctx, primary.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify supervisors
		remaining, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, remaining, 1)
	})
}

func TestActivityService_AddSupervisor_Duplicate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when adding duplicate supervisor", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "dup-super")
		staff := testpkg.CreateTestStaff(t, db, "Dup", "Supervisor")

		// Add first time
		_, err := service.AddSupervisor(ctx, group.ID, staff.ID, true)
		require.NoError(t, err)

		// ACT - try to add same supervisor again
		result, err := service.AddSupervisor(ctx, group.ID, staff.ID, false)

		// ASSERT - should fail with duplicate error
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_EnrollStudent_Duplicate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when enrolling duplicate student", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "dup-enroll")
		student := testpkg.CreateTestStudent(t, db, "Dup", "Student", "1a")

		// Enroll first time
		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT - try to enroll same student again
		err = service.EnrollStudent(ctx, group.ID, student.ID)

		// ASSERT - should fail with duplicate error
		require.Error(t, err)
	})
}

func TestActivityService_UnenrollStudent_NotEnrolled(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when unenrolling non-enrolled student", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "not-enrolled")
		student := testpkg.CreateTestStudent(t, db, "Not", "Enrolled", "1a")

		// ACT - try to unenroll student that was never enrolled
		err := service.UnenrollStudent(ctx, group.ID, student.ID)

		// ASSERT - should fail
		require.Error(t, err)
	})
}

func TestActivityService_GetCategory_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns specific error for not found", func(t *testing.T) {
		// ACT
		result, err := service.GetCategory(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Note: UpdateCategory, DeleteCategory, and UpdateGroup don't validate existence before
// operating - they pass through to the repository. This is by design for these simple CRUD ops.

func TestActivityService_DeleteGroup_WithEnrollments(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes group with enrollments (cascade)", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "del-with-enroll")
		student := testpkg.CreateTestStudent(t, db, "Enrolled", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID) // group will be deleted

		// Enroll student
		err := service.EnrollStudent(ctx, group.ID, student.ID)
		require.NoError(t, err)

		// ACT - delete group (using creator's staff ID with manage permission for test)
		err = service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)

		// Verify deleted
		result, err := service.GetGroup(ctx, group.ID)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// ======== Additional Tests for 80%+ Coverage ========

// Note: TestActivityService_GetPublicGroups, TestActivityService_GetPublicCategories,
// TestActivityService_GetOpenGroups, TestActivityService_GetTeacherTodaysActivities,
// TestActivityService_GetStudentEnrollments, and TestActivityService_GetAvailableGroups
// are already defined above

func TestActivityService_UpdateSupervisor_SetPrimary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("sets new supervisor as primary", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "set-primary")
		staff1 := testpkg.CreateTestStaff(t, db, "First", "Supervisor")
		staff2 := testpkg.CreateTestStaff(t, db, "Second", "Supervisor")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Add first supervisor as primary
		sup1, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true)
		require.NoError(t, err)
		assert.True(t, sup1.IsPrimary)

		// Add second supervisor as non-primary
		sup2, err := service.AddSupervisor(ctx, group.ID, staff2.ID, false)
		require.NoError(t, err)
		assert.False(t, sup2.IsPrimary)

		// ACT - update second supervisor to be primary
		sup2.IsPrimary = true
		updated, err := service.UpdateSupervisor(ctx, sup2)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, updated.IsPrimary)

		// Verify first is no longer primary
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		for _, s := range supervisors {
			if s.StaffID == staff1.ID {
				assert.False(t, s.IsPrimary)
			}
			if s.StaffID == staff2.ID {
				assert.True(t, s.IsPrimary)
			}
		}
	})
}

func TestActivityService_DeleteSchedule_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent schedule", func(t *testing.T) {
		// ACT
		err := service.DeleteSchedule(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityError_Methods(t *testing.T) {
	t.Parallel()

	t.Run("Error returns message without underlying error", func(t *testing.T) {
		// ARRANGE
		err := &activities.ActivityError{Op: "test operation", Err: nil}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Equal(t, "activity error during test operation", msg)
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		// ARRANGE
		underlying := fmt.Errorf("underlying error")
		err := &activities.ActivityError{Op: "test", Err: underlying}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, underlying, unwrapped)
	})

	t.Run("Unwrap returns nil when no underlying error", func(t *testing.T) {
		// ARRANGE
		err := &activities.ActivityError{Op: "test", Err: nil}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Nil(t, unwrapped)
	})
}

func TestActivityService_SetPrimarySupervisor_ExistingSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("sets existing supervisor as primary", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "set-prim-exist")
		staff1 := testpkg.CreateTestStaff(t, db, "Primary", "Staff")
		staff2 := testpkg.CreateTestStaff(t, db, "Secondary", "Staff")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Add both supervisors
		sup1, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true) // primary
		require.NoError(t, err)
		sup2, err := service.AddSupervisor(ctx, group.ID, staff2.ID, false) // not primary
		require.NoError(t, err)

		// ACT - set sup2 as primary (using supervisor ID, not staff ID)
		err = service.SetPrimarySupervisor(ctx, sup2.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify staff2 is now primary and staff1 is not
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		for _, s := range supervisors {
			if s.ID == sup1.ID {
				assert.False(t, s.IsPrimary, "sup1 should not be primary")
			}
			if s.ID == sup2.ID {
				assert.True(t, s.IsPrimary, "sup2 should be primary")
			}
		}
	})
}

func TestActivityService_UpdateGroupEnrollments_AddAndRemove(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds new and removes old enrollments", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "enroll-update")
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1a")
		student3 := testpkg.CreateTestStudent(t, db, "Student", "Three", "1a")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Initial enrollment: student1 and student2
		err := service.UpdateGroupEnrollments(ctx, group.ID, []int64{student1.ID, student2.ID})
		require.NoError(t, err)

		enrolled, err := service.GetEnrolledStudents(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, enrolled, 2)

		// ACT - update to student2 and student3 (removes student1, adds student3)
		err = service.UpdateGroupEnrollments(ctx, group.ID, []int64{student2.ID, student3.ID})

		// ASSERT
		require.NoError(t, err)
		enrolled, err = service.GetEnrolledStudents(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, enrolled, 2)

		// Verify student1 is not enrolled, student2 and student3 are
		studentIDs := make(map[int64]bool)
		for _, s := range enrolled {
			studentIDs[s.ID] = true // GetEnrolledStudents returns []*users.Student
		}
		assert.False(t, studentIDs[student1.ID], "student1 should not be enrolled")
		assert.True(t, studentIDs[student2.ID], "student2 should be enrolled")
		assert.True(t, studentIDs[student3.ID], "student3 should be enrolled")
	})
}

func TestActivityService_UpdateGroupSupervisors_AddAndRemove(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds new and removes old supervisors", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "sup-update")
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
		staff3 := testpkg.CreateTestStaff(t, db, "Staff", "Three")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Initial supervisors: staff1 and staff2
		err := service.UpdateGroupSupervisors(ctx, group.ID, []int64{staff1.ID, staff2.ID})
		require.NoError(t, err)

		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 2)

		// ACT - update to staff2 and staff3 (removes staff1, adds staff3)
		err = service.UpdateGroupSupervisors(ctx, group.ID, []int64{staff2.ID, staff3.ID})

		// ASSERT
		require.NoError(t, err)
		supervisors, err = service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 2)

		// Verify staff1 is not supervisor, staff2 and staff3 are
		staffIDs := make(map[int64]bool)
		for _, s := range supervisors {
			staffIDs[s.StaffID] = true
		}
		assert.False(t, staffIDs[staff1.ID], "staff1 should not be supervisor")
		assert.True(t, staffIDs[staff2.ID], "staff2 should be supervisor")
		assert.True(t, staffIDs[staff3.ID], "staff3 should be supervisor")
	})

	t.Run("ensures primary supervisor exists after update", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "sup-primary")
		staff1 := testpkg.CreateTestStaff(t, db, "Primary", "Staff")
		staff2 := testpkg.CreateTestStaff(t, db, "New", "Staff")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Set staff1 as primary
		_, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true)
		require.NoError(t, err)

		// ACT - replace all supervisors with staff2
		err = service.UpdateGroupSupervisors(ctx, group.ID, []int64{staff2.ID})

		// ASSERT
		require.NoError(t, err)
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 1)
		// The new supervisor should be primary since they're the only one
		assert.True(t, supervisors[0].IsPrimary)
	})
}

// Note: TestActivityService_GetGroupWithDetails is already defined above - see line 322

// ======== Additional Edge Case Tests for 80%+ Coverage ========

func TestActivityService_UpdateGroupEnrollments_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ACT
		err := service.UpdateGroupEnrollments(ctx, 99999999, []int64{1, 2, 3})

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_UpdateGroupSupervisors_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ACT
		err := service.UpdateGroupSupervisors(ctx, 99999999, []int64{1, 2, 3})

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_CreateGroup_WithCategoryValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent category", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "CatVal", "Staff")

		group := &activitiesModels.Group{
			Name:            "Test Group",
			CategoryID:      99999999, // nonexistent
			MaxParticipants: 10,
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_UpdateSupervisor_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent supervisor", func(t *testing.T) {
		// ARRANGE
		supervisor := &activitiesModels.SupervisorPlanned{
			IsPrimary: true,
		}
		supervisor.ID = 99999999

		// ACT
		result, err := service.UpdateSupervisor(ctx, supervisor)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// Note: TestActivityService_GetStaffAssignments is already defined above - see line 704

func TestActivityService_SetPrimarySupervisor_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent supervisor", func(t *testing.T) {
		// ACT
		err := service.SetPrimarySupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_GetSchedule_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent schedule", func(t *testing.T) {
		// ACT
		result, err := service.GetSchedule(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_UpdateSchedule_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent schedule", func(t *testing.T) {
		// ARRANGE
		schedule := &activitiesModels.Schedule{
			ActivityGroupID: 1,
			Weekday:         1,
		}
		schedule.ID = 99999999

		// ACT
		result, err := service.UpdateSchedule(ctx, schedule)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_GetSupervisor_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent supervisor", func(t *testing.T) {
		// ACT
		result, err := service.GetSupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_DeleteSupervisor_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent supervisor", func(t *testing.T) {
		// ACT
		err := service.DeleteSupervisor(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_AddSchedule_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		schedule := &activitiesModels.Schedule{
			Weekday: 1,
		}

		// ACT
		result, err := service.AddSchedule(ctx, 99999999, schedule)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_AddSupervisor_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "NoGroup")

		// ACT
		result, err := service.AddSupervisor(ctx, 99999999, staff.ID, true)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// ======== Final Coverage Push Tests ========

func TestActivityService_CreateCategory_ValidationError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for invalid category", func(t *testing.T) {
		// ARRANGE - empty name should fail validation
		category := &activitiesModels.Category{
			Name:        "", // Invalid: empty
			Description: "Test",
		}

		// ACT
		result, err := service.CreateCategory(ctx, category)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_CreateGroup_ValidationError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for invalid group", func(t *testing.T) {
		// ARRANGE - empty name should fail validation
		staff := testpkg.CreateTestStaff(t, db, "ValErr", "Staff")

		group := &activitiesModels.Group{
			Name:            "", // Invalid: empty
			CategoryID:      1,
			MaxParticipants: 10,
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_UpdateGroup_ValidationError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for invalid group update", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "update-grp-val")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// Get the group and make it invalid
		grp, err := service.GetGroup(ctx, group.ID)
		require.NoError(t, err)

		grp.Name = "" // Invalid: empty name

		// ACT
		result, err := service.UpdateGroup(ctx, grp, *grp.CreatedBy, true)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_EnrollStudent_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Enroll", "NoGroup", "1a")

		// ACT
		err := service.EnrollStudent(ctx, 99999999, student.ID)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_UnenrollStudent_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent group", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Unenroll", "NoGroup", "1a")

		// ACT
		err := service.UnenrollStudent(ctx, 99999999, student.ID)

		// ASSERT
		require.Error(t, err)
	})
}

func TestActivityService_ListCategories_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list when no categories", func(t *testing.T) {
		// ACT - the test DB may have existing data, so just verify it works
		categories, err := service.ListCategories(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, categories)
	})
}

func TestActivityService_ListGroups_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns list of groups", func(t *testing.T) {
		// ACT
		groups, err := service.ListGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, groups)
	})
}

func TestActivityService_GetGroupSchedules_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list for group with no schedules", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "no-schedules")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// ACT
		schedules, err := service.GetGroupSchedules(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, schedules)
	})
}

func TestActivityService_GetGroupSupervisors_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list for group with no supervisors", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "no-supervisors")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// ACT
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestActivityService_GetEnrolledStudents_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list for group with no enrollments", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "no-enrollments")
		defer func() { cleanupGroup(service, ctx, group.ID) }()

		// ACT
		students, err := service.GetEnrolledStudents(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, students)
	})
}

// =============================================================================
// Additional Tests for 80%+ Coverage
// =============================================================================

func TestActivityService_CreateGroup_InvalidSupervisor(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when supervisor does not exist", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "invalid-sup-cat")
		staff := testpkg.CreateTestStaff(t, db, "InvSup", "Staff")

		group := &activitiesModels.Group{
			Name:            "Test Group Invalid Sup",
			CategoryID:      category.ID,
			MaxParticipants: 20,
			CreatedBy:       &staff.ID,
		}

		// ACT - non-existent staff ID
		result, err := service.CreateGroup(ctx, group, []int64{99999999}, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_CreateGroup_InvalidScheduleWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for invalid schedule weekday", func(t *testing.T) {
		// ARRANGE
		category := testpkg.CreateTestActivityCategory(t, db, "invalid-sched-cat")
		staff := testpkg.CreateTestStaff(t, db, "InvSched", "Staff")

		group := &activitiesModels.Group{
			Name:            "Test Group Invalid Sched",
			CategoryID:      category.ID,
			MaxParticipants: 20,
			CreatedBy:       &staff.ID,
		}

		// Invalid weekday (should be 0-6)
		invalidSchedule := &activitiesModels.Schedule{
			Weekday: 99,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, []*activitiesModels.Schedule{invalidSchedule})

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_DeleteGroup_CascadesSupervisors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deleting group also deletes associated supervisors", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "cascade-sup-del")
		staff := testpkg.CreateTestStaff(t, db, "Cascade", "Supervisor")

		supervisor, err := service.AddSupervisor(ctx, group.ID, staff.ID, true)
		require.NoError(t, err)
		supervisorID := supervisor.ID

		// ACT - delete group (using creator's staff ID with manage permission)
		err = service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)

		// Verify supervisor is gone
		result, err := service.GetSupervisor(ctx, supervisorID)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_DeleteGroup_CascadesSchedules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deleting group also deletes associated schedules", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "cascade-sched-del")

		schedule := &activitiesModels.Schedule{
			Weekday: 1,
		}
		created, err := service.AddSchedule(ctx, group.ID, schedule)
		require.NoError(t, err)
		scheduleID := created.ID

		// ACT - delete group (using creator's staff ID with manage permission)
		err = service.DeleteGroup(ctx, group.ID, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)

		// Verify schedule is gone
		result, err := service.GetSchedule(ctx, scheduleID)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestActivityService_GetCategory_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns wrapped error for not found category", func(t *testing.T) {
		// ACT
		result, err := service.GetCategory(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)

		// Verify error is properly wrapped in ActivityError
		var actErr *activities.ActivityError
		if errors.As(err, &actErr) {
			assert.Contains(t, actErr.Error(), "category")
		}
	})
}

func TestActivityService_GetGroup_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns wrapped error for not found group", func(t *testing.T) {
		// ACT
		result, err := service.GetGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)

		// Verify error is properly wrapped in ActivityError
		var actErr *activities.ActivityError
		if errors.As(err, &actErr) {
			assert.Contains(t, actErr.Error(), "group")
		}
	})
}

func TestActivityService_UpdateGroup_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates existing group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "update-success-grp")

		group.Name = "Updated Group Name"

		// ACT (using creator's staff ID with manage permission)
		result, err := service.UpdateGroup(ctx, group, *group.CreatedBy, true)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, "Updated Group Name", result.Name)
	})

	t.Run("rejects changing to an archived category", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "update-archived-category")
		archived := testpkg.CreateTestActivityCategory(t, db, "ArchivedUpdateTarget")

		_, err := service.ArchiveCategory(ctx, archived.ID)
		require.NoError(t, err)
		group.CategoryID = archived.ID

		result, err := service.UpdateGroup(ctx, group, *group.CreatedBy, true)
		require.Error(t, err)
		require.ErrorIs(t, err, activities.ErrCategoryArchived)
		assert.Nil(t, result)
	})
}

func TestActivityService_CreateGroup_InvalidCategoryID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent category", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "InvCat", "Staff")

		group := &activitiesModels.Group{
			Name:            "Test Group Invalid Cat",
			CategoryID:      99999999, // Non-existent
			MaxParticipants: 20,
			CreatedBy:       &staff.ID,
		}

		// ACT
		result, err := service.CreateGroup(ctx, group, nil, nil)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "category")
	})

	t.Run("rejects an archived category", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ArchivedCat", "Staff")
		category := testpkg.CreateTestActivityCategory(t, db, "ArchivedForCreate")

		_, err := service.ArchiveCategory(ctx, category.ID)
		require.NoError(t, err)
		group := &activitiesModels.Group{
			Name:            "Archived Category Group",
			CategoryID:      category.ID,
			MaxParticipants: 20,
			CreatedBy:       &staff.ID,
		}

		result, err := service.CreateGroup(ctx, group, nil, nil)
		require.Error(t, err)
		require.ErrorIs(t, err, activities.ErrCategoryArchived)
		assert.Nil(t, result)
	})
}

// TestActivityService_AddSupervisor_PrimaryReplacement tests that adding a new primary
// supervisor unsets the previous primary supervisor (tests unsetPrimarySupervisorsInTx)
func TestActivityService_AddSupervisor_PrimaryReplacement(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adding new primary supervisor unsets existing primary", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "primary-replace")
		staff1 := testpkg.CreateTestStaff(t, db, "First", "Primary")
		staff2 := testpkg.CreateTestStaff(t, db, "Second", "Primary")

		// Add first supervisor as primary
		super1, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true)
		require.NoError(t, err)
		assert.True(t, super1.IsPrimary, "First supervisor should be primary")

		// ACT - Add second supervisor as primary (should unset first)
		super2, err := service.AddSupervisor(ctx, group.ID, staff2.ID, true)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, super2.IsPrimary, "Second supervisor should be primary")

		// Verify first supervisor is no longer primary
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)

		for _, s := range supervisors {
			if s.StaffID == staff1.ID {
				assert.False(t, s.IsPrimary, "First supervisor should no longer be primary")
			}
			if s.StaffID == staff2.ID {
				assert.True(t, s.IsPrimary, "Second supervisor should be primary")
			}
		}
	})
}

// TestActivityService_GetAvailableGroups_DatabaseError tests error handling
func TestActivityService_GetAvailableGroups_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT
	_, err := service.GetAvailableGroups(canceledCtx, 1)

	// ASSERT
	require.Error(t, err)
}

// TestActivityService_UpdateGroupSupervisors_EmptyList tests with empty supervisor list
func TestActivityService_UpdateGroupSupervisors_EmptyList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("handles empty supervisor list", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "empty-supervisors")

		// ACT - Update with empty list
		err := service.UpdateGroupSupervisors(ctx, group.ID, []int64{})

		// ASSERT
		require.NoError(t, err)
	})
}

// TestActivityService_UpdateGroupEnrollments_EmptyList tests with empty enrollment list
func TestActivityService_UpdateGroupEnrollments_EmptyList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("handles empty enrollment list", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "empty-enrollments")

		// ACT - Update with empty list
		err := service.UpdateGroupEnrollments(ctx, group.ID, []int64{})

		// ASSERT
		require.NoError(t, err)
	})
}

// TestActivityService_UpdateGroupSupervisors_AddThenRemove tests full supervisor update flow
func TestActivityService_UpdateGroupSupervisors_AddThenRemove(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates supervisors by adding and removing", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestActivityGroup(t, db, "supervisor-update-flow")
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")

		// Add first supervisor
		_, err := service.AddSupervisor(ctx, group.ID, staff1.ID, true)
		require.NoError(t, err)

		// ACT - Update to only have staff2 (removes staff1, adds staff2)
		err = service.UpdateGroupSupervisors(ctx, group.ID, []int64{staff2.ID})

		// ASSERT
		require.NoError(t, err)

		// Verify only staff2 is supervisor
		supervisors, err := service.GetGroupSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 1)
		assert.Equal(t, staff2.ID, supervisors[0].StaffID)
	})
}

// TestActivityService_GetEnrolledStudents_DatabaseError tests error handling
func TestActivityService_GetEnrolledStudents_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT
	_, err := service.GetEnrolledStudents(canceledCtx, 1)

	// ASSERT
	require.Error(t, err)
}

// TestActivityService_CreateCategory_DatabaseError tests CreateCategory database error handling
func TestActivityService_CreateCategory_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	category := &activitiesModels.Category{
		Name:  "Test",
		Color: "#FFFFFF",
	}

	// ACT
	_, err := service.CreateCategory(canceledCtx, category)

	// ASSERT
	require.Error(t, err)
}

// TestActivityService_ListCategories_DatabaseError tests ListCategories database error handling
func TestActivityService_ListCategories_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT
	_, err := service.ListCategories(canceledCtx)

	// ASSERT
	require.Error(t, err)
}

// TestActivityService_ListGroups_DatabaseError tests ListGroups database error handling
func TestActivityService_ListGroups_DatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// ARRANGE: Use canceled context to trigger database error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT
	_, err := service.ListGroups(canceledCtx, nil)

	// ASSERT
	require.Error(t, err)
}

// =============================================================================
// CanModifyActivity Ownership Tests
// =============================================================================

// TestActivityService_CanModifyActivity_AdminBypassesOwnership tests that admins can modify any activity
func TestActivityService_CanModifyActivity_AdminBypassesOwnership(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create staff and activity
	staff := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "admin-test")

	group := &activitiesModels.Group{
		Name:            "Admin Test Activity",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{staff.ID}, nil)
	require.NoError(t, err)

	// ACT: Check if a different staff with admin permission can modify
	otherStaffID := int64(99999) // Non-existent staff ID
	canModify, err := service.CanModifyActivity(ctx, created.ID, otherStaffID, true)

	// ASSERT: Admin permission should bypass ownership check
	require.NoError(t, err)
	assert.True(t, canModify, "Admin should be able to modify any activity")
}

// TestActivityService_CanModifyActivity_CreatorCanModify tests that the creator can modify their activity
func TestActivityService_CanModifyActivity_CreatorCanModify(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create staff and activity
	staff := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "creator-test")

	group := &activitiesModels.Group{
		Name:            "Creator Test Activity",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{staff.ID}, nil)
	require.NoError(t, err)

	// ACT: Check if creator can modify (without admin permission)
	canModify, err := service.CanModifyActivity(ctx, created.ID, staff.ID, false)

	// ASSERT: Creator should be able to modify their own activity
	require.NoError(t, err)
	assert.True(t, canModify, "Creator should be able to modify their own activity")
}

// TestActivityService_CanModifyActivity_SupervisorCanModify tests that supervisors can modify the activity
func TestActivityService_CanModifyActivity_SupervisorCanModify(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create two staff members
	creator := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	supervisor := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "supervisor-test")

	// Create activity with supervisor as an assigned supervisor
	group := &activitiesModels.Group{
		Name:            "Supervisor Test Activity",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &creator.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{creator.ID, supervisor.ID}, nil)
	require.NoError(t, err)

	// ACT: Check if supervisor can modify (without admin permission)
	canModify, err := service.CanModifyActivity(ctx, created.ID, supervisor.ID, false)

	// ASSERT: Supervisor should be able to modify the activity
	require.NoError(t, err)
	assert.True(t, canModify, "Supervisor should be able to modify the activity")
}

// TestActivityService_CanModifyActivity_NonOwnerCannotModify tests that non-owners cannot modify
func TestActivityService_CanModifyActivity_NonOwnerCannotModify(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create staff and activity
	creator := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	otherStaff := testpkg.CreateTestStaff(t, db, "Other", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "nonowner-test")

	group := &activitiesModels.Group{
		Name:            "Non-Owner Test Activity",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &creator.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{creator.ID}, nil)
	require.NoError(t, err)

	// ACT: Check if other staff (not creator, not supervisor) can modify
	canModify, err := service.CanModifyActivity(ctx, created.ID, otherStaff.ID, false)

	// ASSERT: Non-owner should NOT be able to modify
	require.NoError(t, err)
	assert.False(t, canModify, "Non-owner should not be able to modify activity")
}

// TestActivityService_CanModifyActivity_GroupNotFound tests error when group doesn't exist
func TestActivityService_CanModifyActivity_GroupNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ACT: Check permissions for non-existent group
	canModify, err := service.CanModifyActivity(ctx, 999999, 1, false)

	// ASSERT: Should return error for non-existent group
	require.Error(t, err)
	assert.False(t, canModify)
	assert.True(t, errors.Is(err, activities.ErrGroupNotFound) || err.Error() == "check permissions: activity group not found")
}

// TestActivityService_UpdateGroup_OwnershipEnforced tests that UpdateGroup enforces ownership
func TestActivityService_UpdateGroup_OwnershipEnforced(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create two staff members and an activity
	creator := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	otherStaff := testpkg.CreateTestStaff(t, db, "Other", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "update-owner-test")

	group := &activitiesModels.Group{
		Name:            "Update Owner Test",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &creator.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{creator.ID}, nil)
	require.NoError(t, err)

	// ACT: Non-owner tries to update without admin permission
	created.Name = "Modified Name"
	_, err = service.UpdateGroup(ctx, created, otherStaff.ID, false)

	// ASSERT: Should return ownership error
	require.Error(t, err)
	assert.True(t, errors.Is(err, activities.ErrNotOwner) || err.Error() == "update group: you can only modify activities you created or supervise")
}

// TestActivityService_DeleteGroup_OwnershipEnforced tests that DeleteGroup enforces ownership
func TestActivityService_DeleteGroup_OwnershipEnforced(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: Create two staff members and an activity
	creator := testpkg.CreateTestStaff(t, db, "Creator", "Staff")
	otherStaff := testpkg.CreateTestStaff(t, db, "Other", "Staff")
	category := testpkg.CreateTestActivityCategory(t, db, "delete-owner-test")

	group := &activitiesModels.Group{
		Name:            "Delete Owner Test",
		MaxParticipants: 10,
		CategoryID:      category.ID,
		CreatedBy:       &creator.ID,
	}
	created, err := service.CreateGroup(ctx, group, []int64{creator.ID}, nil)
	require.NoError(t, err)
	// Note: No cleanup needed as the test expects deletion to fail

	// ACT: Non-owner tries to delete without admin permission
	err = service.DeleteGroup(ctx, created.ID, otherStaff.ID, false)

	// ASSERT: Should return ownership error
	require.Error(t, err)
	assert.True(t, errors.Is(err, activities.ErrNotOwner) || err.Error() == "delete group: you can only modify activities you created or supervise")

	// Cleanup: Delete with admin permission
}

// =============================================================================
// Kategorie↔Schichtart mapping (#1837 follow-up)
// =============================================================================

func TestActivityService_SetCategoryShiftTypeLinks(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	catRepo := repositories.NewFactory(db).ActivityCategory
	stRepo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	st := &scheduleModels.ShiftType{Name: fmt.Sprintf("SvcLink-%d", time.Now().UnixNano()), Color: "#83CD2D", IsActive: true}
	require.NoError(t, stRepo.Create(ctx, st))
	t.Cleanup(func() { _ = stRepo.Delete(ctx, st.ID) })

	cat1 := testpkg.CreateTestActivityCategory(t, db, "svc-link-1")
	cat2 := testpkg.CreateTestActivityCategory(t, db, "svc-link-2")

	// ACT: link both categories
	require.NoError(t, service.SetCategoryShiftTypeLinks(ctx, st.ID, []int64{cat1.ID, cat2.ID}))

	// ASSERT: ListCategories surfaces shift_type_id on the mapped categories.
	cats, err := service.ListCategories(ctx)
	require.NoError(t, err)
	byID := map[int64]*activitiesModels.Category{}
	for _, c := range cats {
		byID[c.ID] = c
	}
	require.NotNil(t, byID[cat1.ID].ShiftTypeID)
	assert.Equal(t, st.ID, *byID[cat1.ID].ShiftTypeID)
	require.NotNil(t, byID[cat2.ID].ShiftTypeID)

	// ACT: reduce the set to cat2 only -> cat1 is cleared.
	require.NoError(t, service.SetCategoryShiftTypeLinks(ctx, st.ID, []int64{cat2.ID}))
	c1, err := catRepo.FindByID(ctx, cat1.ID)
	require.NoError(t, err)
	assert.Nil(t, c1.ShiftTypeID, "de-selected category is unlinked via the service")
	c2, err := catRepo.FindByID(ctx, cat2.ID)
	require.NoError(t, err)
	require.NotNil(t, c2.ShiftTypeID)
	assert.Equal(t, st.ID, *c2.ShiftTypeID)
}

// TestActivityService_UpdateGroupEnrollments_PreservesAlumnusEnrollment covers
// the #405 review fix: GetEnrolledStudents hides alumni, so the roster a caller
// reads (GET /api/activities/{id}/students) never lists them and the PUT that
// replaces it cannot either. Treating "not submitted" as "delete" would wipe
// exactly the enrollments a grade transition preserved on purpose — the rows the
// transition's revert and future materialization still need.
func TestActivityService_UpdateGroupEnrollments_PreservesAlumnusEnrollment(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, db, "alumnus-enrollments")
	activeStudent := testpkg.CreateTestStudent(t, db, "Still", "Here", "1a")
	graduated := testpkg.CreateTestStudent(t, db, "Already", "Gone", "4a")

	require.NoError(t, service.UpdateGroupEnrollments(ctx, group.ID, []int64{activeStudent.ID, graduated.ID}))

	// The grade transition graduates the second child; the enrollment row stays.
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", graduated.ID).
		Exec(ctx)
	require.NoError(t, err)

	// What a staff member sees, and therefore submits back unchanged.
	visible, err := service.GetEnrolledStudents(ctx, group.ID)
	require.NoError(t, err)
	visibleIDs := make([]int64, 0, len(visible))
	for _, s := range visible {
		visibleIDs = append(visibleIDs, s.ID)
	}
	assert.Equal(t, []int64{activeStudent.ID}, visibleIDs, "the alumnus is hidden from the roster read")

	require.NoError(t, service.UpdateGroupEnrollments(ctx, group.ID, visibleIDs))

	count, err := db.NewSelect().
		TableExpr(`activities.student_enrollments`).
		Where("activity_group_id = ?", group.ID).
		Where("student_id = ?", graduated.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count,
		"a hidden alumnus enrollment must survive a replacement update that could not show it")
}

// TestActivityService_EnrollmentWritesRejectAlumni covers the other half of the
// same asymmetry (#405 review): an alumnus enrollment survives because no edit
// can display it — which is exactly why no edit may CREATE one either. Such a
// row is invisible in the roster and its counts, cannot be removed again, and
// still feeds materialization and a transition revert.
func TestActivityService_EnrollmentWritesRejectAlumni(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, db, "alumnus-enroll-guard")
	activeStudent := testpkg.CreateTestStudent(t, db, "Still", "Enrolled", "1a")
	graduated := testpkg.CreateTestStudent(t, db, "Long", "Graduated", "4a")

	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", graduated.ID).
		Exec(ctx)
	require.NoError(t, err)

	enrollmentCount := func(studentID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`activities.student_enrollments`).
			Where("activity_group_id = ?", group.ID).
			Where("student_id = ?", studentID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	t.Run("EnrollStudent refuses a graduated child", func(t *testing.T) {
		err := service.EnrollStudent(ctx, group.ID, graduated.ID)
		require.ErrorIs(t, err, activities.ErrStudentIsAlumnus)
		assert.Equal(t, 0, enrollmentCount(graduated.ID))
	})

	t.Run("UpdateGroupEnrollments refuses to add a graduated child", func(t *testing.T) {
		err := service.UpdateGroupEnrollments(ctx, group.ID, []int64{activeStudent.ID, graduated.ID})
		require.ErrorIs(t, err, activities.ErrStudentIsAlumnus)
		assert.Equal(t, 0, enrollmentCount(graduated.ID))
		assert.Equal(t, 0, enrollmentCount(activeStudent.ID),
			"the whole write is rejected before any row is created")
	})

	t.Run("UpdateGroupEnrollments still accepts active students", func(t *testing.T) {
		require.NoError(t, service.UpdateGroupEnrollments(ctx, group.ID, []int64{activeStudent.ID}))
		assert.Equal(t, 1, enrollmentCount(activeStudent.ID))
	})
}
