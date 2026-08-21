package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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

	repo := repositories.NewFactory(db).StaffAbsence
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

func TestStaffAbsenceRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("lists all staff absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, absence)
		require.NoError(t, err)

		absences, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})

	t.Run("lists with query options", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, absence)
		require.NoError(t, err)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		absences, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(absences), 10)
	})
}

func TestStaffAbsenceRepository_GetByStaffAndDateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("finds absences in date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		tomorrow := today.AddDays(1)
		nextWeek := today.AddDays(7)

		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   tomorrow,
			DateEnd:     nextWeek,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, absence)
		require.NoError(t, err)

		absences, err := repo.GetByStaffAndDateRange(ctx, staff.ID, today, nextWeek)
		require.NoError(t, err)
		assert.NotEmpty(t, absences)

		var found bool
		for _, a := range absences {
			if a.ID == absence.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for date range with no absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		futureDate := timezone.TodayDate().AddDays(365)
		absences, err := repo.GetByStaffAndDateRange(ctx, staff.ID, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, absences)
	})

	t.Run("finds overlapping absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		tomorrow := today.AddDays(1)

		// Create absence spanning yesterday to tomorrow
		absence := &active.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: active.AbsenceTypeVacation,
			DateStart:   yesterday,
			DateEnd:     tomorrow,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff.ID,
		}
		err := repo.Create(ctx, absence)
		require.NoError(t, err)

		// Query just for today should find the overlapping absence
		absences, err := repo.GetByStaffAndDateRange(ctx, staff.ID, today, today)
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})
}

func TestStaffAbsenceRepository_GetByStaffAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffAbsence
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

func TestStaffAbsenceRepository_GetAbsenceMapForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("returns absence map for today", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
		today := timezone.TodayDate()

		absence1 := &active.StaffAbsence{
			StaffID:     staff1.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff1.ID,
		}

		absence2 := &active.StaffAbsence{
			StaffID:     staff2.ID,
			AbsenceType: active.AbsenceTypeVacation,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff2.ID,
		}

		err := repo.Create(ctx, absence1)
		require.NoError(t, err)
		err = repo.Create(ctx, absence2)
		require.NoError(t, err)

		absenceMap, err := repo.GetAbsenceMapForDate(ctx, today)
		require.NoError(t, err)
		assert.Equal(t, active.AbsenceTypeSick, absenceMap[staff1.ID])
		assert.Equal(t, active.AbsenceTypeVacation, absenceMap[staff2.ID])
	})

	t.Run("prioritizes sick over vacation", func(t *testing.T) {
		staff3 := testpkg.CreateTestStaff(t, db, "Staff", "Three")
		today := timezone.TodayDate()

		// Create two overlapping absences for same staff
		absence1 := &active.StaffAbsence{
			StaffID:     staff3.ID,
			AbsenceType: active.AbsenceTypeVacation,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff3.ID,
		}

		absence2 := &active.StaffAbsence{
			StaffID:     staff3.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff3.ID,
		}

		err := repo.Create(ctx, absence1)
		require.NoError(t, err)
		err = repo.Create(ctx, absence2)
		require.NoError(t, err)

		absenceMap, err := repo.GetAbsenceMapForDate(ctx, today)
		require.NoError(t, err)
		// Sick should take priority over vacation
		assert.Equal(t, active.AbsenceTypeSick, absenceMap[staff3.ID])
	})

	t.Run("uses the requested date", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		requestedDate := timezone.TodayDate().AddDays(30)
		absence := &active.StaffAbsence{
			StaffID:     staff1.ID,
			AbsenceType: active.AbsenceTypeTraining,
			DateStart:   requestedDate,
			DateEnd:     requestedDate,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff1.ID,
		}

		require.NoError(t, repo.Create(ctx, absence))

		absenceMap, err := repo.GetAbsenceMapForDate(ctx, requestedDate)
		require.NoError(t, err)
		assert.Equal(t, active.AbsenceTypeTraining, absenceMap[staff1.ID])
	})

	t.Run("uses the same ID tie-breaker for type and label maps", func(t *testing.T) {
		today := timezone.TodayDate()
		absenceType := &active.StaffAbsenceType{
			Name:     "Regenerationstag",
			BaseType: active.AbsenceTypeOther,
			IsActive: true,
		}
		require.NoError(t, repositories.NewFactory(db).StaffAbsenceType.Create(ctx, absenceType))
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absence_types", absenceType.ID)

		customAbsence := &active.StaffAbsence{
			StaffID:       staff1.ID,
			AbsenceType:   active.AbsenceTypeOther,
			AbsenceTypeID: &absenceType.ID,
			DateStart:     today,
			DateEnd:       today,
			Status:        active.AbsenceStatusApproved,
			CreatedBy:     staff1.ID,
		}
		standardAbsence := &active.StaffAbsence{
			StaffID:     staff1.ID,
			AbsenceType: active.AbsenceTypeOther,
			DateStart:   today,
			DateEnd:     today,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff1.ID,
		}
		require.NoError(t, repo.Create(ctx, customAbsence))
		require.NoError(t, repo.Create(ctx, standardAbsence))
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", customAbsence.ID)
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", standardAbsence.ID)

		absenceMap, err := repo.GetAbsenceMapForDate(ctx, today)
		require.NoError(t, err)
		typeIDMap, err := repo.GetAbsenceTypeIDMapForDate(ctx, today)
		require.NoError(t, err)

		assert.Equal(t, active.AbsenceTypeOther, absenceMap[staff1.ID])
		assert.Equal(t, absenceType.ID, typeIDMap[staff1.ID])
	})
}
