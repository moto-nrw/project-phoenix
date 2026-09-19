package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStaffAbsenceRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("creates staff absence with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		tomorrow := today.AddDays(1)
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     tomorrow,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)

	})

	t.Run("creates absence with vacation type", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		nextWeek := today.AddDays(7)
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeVacation,
			DateStart:   today,
			DateEnd:     nextWeek,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)
		assert.Equal(t, active.AbsenceTypeVacation, absence.AbsenceType)

	})

	t.Run("creates half-day absence", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeTraining,
			DateStart:   today,
			DateEnd:     today,
			HalfDay:     true,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)
		assert.True(t, absence.HalfDay)

	})

	t.Run("create with nil absence should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid absence type should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: "invalid_type",
			DateStart:   today,
			DateEnd:     today,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		assert.Error(t, err)
	})

	t.Run("create with invalid status should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      "invalid_status",
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid absence status")
	})

	t.Run("create with missing staff ID should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     0, // Invalid
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("create with date_start after date_end should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     yesterday, // Before start
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		err := repo.Create(ctx, absence)
		assert.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestStaffAbsenceRepository_GetByStaffAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("finds absence for specific date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		tomorrow := today.AddDays(1)

		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     tomorrow,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, absence)
		require.NoError(t, err)

		found, err := repo.GetByStaffAndDate(ctx, staff.ID, today)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, absence.ID, found.ID)
	})

	t.Run("returns nil when no absence for date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		futureDate := timezone.TodayDate().AddDays(365)
		found, err := repo.GetByStaffAndDate(ctx, staff.ID, futureDate)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("ignores requested and canceled absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		for _, status := range []string{
			active.AbsenceStatusRequested,
			active.AbsenceStatusCanceled,
			active.AbsenceStatusDeclined,
		} {
			absence := &active.StaffAbsence{
				StaffID:     staff.ID,
				AbsenceType: active.AbsenceTypeVacation,
				DateStart:   today,
				DateEnd:     today,
				Status:      status,
				CreatedBy:   staff.ID,
			}
			err := repo.Create(ctx, absence)
			require.NoError(t, err)
		}

		found, err := repo.GetByStaffAndDate(ctx, staff.ID, today)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}
