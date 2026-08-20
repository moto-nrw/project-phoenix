// Package active_test tests the analytics service using the hermetic testing pattern.
package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetDashboardAnalytics Tests
// =============================================================================

func TestGetDashboardAnalytics(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns dashboard analytics without error", func(t *testing.T) {
		// ACT
		analytics, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, analytics)
		assert.False(t, analytics.LastUpdated.IsZero(), "LastUpdated should be set")
		assert.GreaterOrEqual(t, analytics.TotalRooms, 0)
	})

	t.Run("includes active groups in analytics", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "dashboard-active")
		room := testpkg.CreateTestRoom(t, db, "Dashboard Room")
		testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		// ACT
		analytics, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, analytics.ActiveActivities, 1, "Should have at least 1 active activity")
	})

	t.Run("counts planned excused students for today", func(t *testing.T) {
		// ARRANGE
		before, err := service.GetDashboardAnalytics(ctx)
		require.NoError(t, err)

		statusRepo := repositories.NewFactory(db).StudentStatusDay
		plannedExcusedStudent := testpkg.CreateTestStudent(t, db, "PlannedExcused", "Dashboard", "PE1")
		legacyOverlapStudent := testpkg.CreateTestStudent(t, db, "LegacyOverlap", "Dashboard", "PE2")

		flagTrue := true
		_, err = db.NewUpdate().
			Model(legacyOverlapStudent).
			Set("excused = ?", flagTrue).
			Where("id = ?", legacyOverlapStudent.ID).
			Exec(ctx)
		require.NoError(t, err)

		now := time.Now()
		for _, studentID := range []int64{plannedExcusedStudent.ID, legacyOverlapStudent.ID} {
			require.NoError(t, statusRepo.UpsertReported(ctx, &activeModels.StudentStatusDay{
				StudentID:  studentID,
				Date:       timezone.TodayDate(),
				Status:     activeModels.StudentStatusDayExcused,
				ReportedAt: now,
				Source:     activeModels.StudentStatusSourcePlanned,
			}))
		}

		// ACT
		after, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, before.StudentsExcused+2, after.StudentsExcused)
	})

	t.Run("counts class trip students as excused without counting sick overlaps", func(t *testing.T) {
		// ARRANGE
		before, err := service.GetDashboardAnalytics(ctx)
		require.NoError(t, err)

		classTripStudent := testpkg.CreateTestStudent(t, db, "ClassTrip", "Dashboard", "CT1")
		alreadyExcusedStudent := testpkg.CreateTestStudent(t, db, "ExcusedClassTrip", "Dashboard", "CT2")
		sickClassTripStudent := testpkg.CreateTestStudent(t, db, "SickClassTrip", "Dashboard", "CT3")

		flagTrue := true
		_, err = db.NewUpdate().
			Model(alreadyExcusedStudent).
			Set("excused = ?", flagTrue).
			Where("id = ?", alreadyExcusedStudent.ID).
			Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewUpdate().
			Model(sickClassTripStudent).
			Set("sick = ?", flagTrue).
			Where("id = ?", sickClassTripStudent.ID).
			Exec(ctx)
		require.NoError(t, err)

		now := time.Now()
		for _, studentID := range []int64{classTripStudent.ID, alreadyExcusedStudent.ID, sickClassTripStudent.ID} {
			_, err = db.NewRaw(`
				INSERT INTO active.student_status_days
					(tenant_id, student_id, date, status, reported_at, source, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`,
				classTripStudent.GetTenantID(),
				studentID,
				// The DATE column must hold the Berlin calendar date; the driver
				// binds raw time.Time values as UTC, which is one day behind
				// Berlin between 00:00 and 02:00 local time.
				timezone.TodayDate(),
				activeModels.StudentStatusDayClassTrip,
				now,
				activeModels.StudentStatusSourcePlanned,
				now,
				now,
			).Exec(ctx)
			require.NoError(t, err)
		}

		// ACT
		after, err := service.GetDashboardAnalytics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, before.StudentsExcused+2, after.StudentsExcused)
	})

	t.Run("scopes class trip count to tenant context", func(t *testing.T) {
		tenantA := testpkg.UniqueTestTenantID(t)
		tenantB := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, tenantA)
		testpkg.EnsureTestTenant(t, db, tenantB)
		ctxA := testpkg.TenantContext(tenantA)
		ctxB := testpkg.TenantContext(tenantB)
		before, err := service.GetDashboardAnalytics(ctxA)
		require.NoError(t, err)

		studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "TenantA", "ClassTrip", "CTA")
		studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "TenantB", "ClassTrip", "CTB")

		statusRepo := repositories.NewFactory(db).StudentStatusDay
		now := time.Now()
		require.NoError(t, statusRepo.UpsertReported(ctxA, &activeModels.StudentStatusDay{
			StudentID:  studentA.ID,
			Date:       timezone.DateFromTime(now),
			Status:     activeModels.StudentStatusDayClassTrip,
			ReportedAt: now,
			Source:     activeModels.StudentStatusSourcePlanned,
		}))
		require.NoError(t, statusRepo.UpsertReported(ctxB, &activeModels.StudentStatusDay{
			StudentID:  studentB.ID,
			Date:       timezone.DateFromTime(now),
			Status:     activeModels.StudentStatusDayClassTrip,
			ReportedAt: now,
			Source:     activeModels.StudentStatusSourcePlanned,
		}))

		after, err := service.GetDashboardAnalytics(ctxA)
		require.NoError(t, err)
		assert.Equal(t, before.StudentsExcused+1, after.StudentsExcused)
	})
}
