package active_test

import (
	"context"
	"testing"
	"time"

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("creates staff absence with valid data", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		tomorrow := today.AddDate(0, 0, 1)
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

		testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)
	})

	t.Run("creates absence with vacation type", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		nextWeek := today.AddDate(0, 0, 7)
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

		testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)
	})

	t.Run("creates half-day absence", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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

		testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)
	})

	t.Run("create with nil absence should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid absence type should fail", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
}

// ============================================================================
// Query Tests
// ============================================================================

func TestStaffAbsenceRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("lists all staff absences", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)

		absences, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})

	t.Run("lists with query options", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
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
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)

		absences, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(absences), 10)
	})
}

func TestStaffAbsenceRepository_GetByStaffAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds absences in date range", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		tomorrow := today.AddDate(0, 0, 1)
		nextWeek := today.AddDate(0, 0, 7)

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
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)

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
		futureDate := timezone.DateOfUTC(time.Now().AddDate(1, 0, 0))
		absences, err := repo.GetByStaffAndDateRange(ctx, staff.ID, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, absences)
	})

	t.Run("finds overlapping absences", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		yesterday := today.AddDate(0, 0, -1)
		tomorrow := today.AddDate(0, 0, 1)

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
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)

		// Query just for today should find the overlapping absence
		absences, err := repo.GetByStaffAndDateRange(ctx, staff.ID, today, today)
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})
}

func TestStaffAbsenceRepository_GetByStaffAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID)

	t.Run("finds absence for specific date", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		tomorrow := today.AddDate(0, 0, 1)

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
		defer testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence.ID)

		found, err := repo.GetByStaffAndDate(ctx, staff.ID, today)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, absence.ID, found.ID)
	})

	t.Run("returns nil when no absence for date", func(t *testing.T) {
		futureDate := timezone.DateOfUTC(time.Now().AddDate(1, 0, 0))
		found, err := repo.GetByStaffAndDate(ctx, staff.ID, futureDate)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestStaffAbsenceRepository_GetTodayAbsenceMap(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
	staff3 := testpkg.CreateTestStaff(t, db, "Staff", "Three")
	defer func() {
		testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, staff3.ID)
	}()

	t.Run("returns absence map for today", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())

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
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence1.ID)
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence2.ID)
		}()

		absenceMap, err := repo.GetTodayAbsenceMap(ctx)
		require.NoError(t, err)
		assert.Equal(t, active.AbsenceTypeSick, absenceMap[staff1.ID])
		assert.Equal(t, active.AbsenceTypeVacation, absenceMap[staff2.ID])
	})

	t.Run("prioritizes sick over vacation", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())

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
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence1.ID)
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence2.ID)
		}()

		absenceMap, err := repo.GetTodayAbsenceMap(ctx)
		require.NoError(t, err)
		// Sick should take priority over vacation
		assert.Equal(t, active.AbsenceTypeSick, absenceMap[staff3.ID])
	})
}

func TestStaffAbsenceRepository_GetByDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsence
	ctx := context.Background()

	staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
	defer func() {
		testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID)
		testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID)
	}()

	t.Run("finds all absences in date range", func(t *testing.T) {
		today := timezone.DateOfUTC(time.Now())
		tomorrow := today.AddDate(0, 0, 1)
		nextWeek := today.AddDate(0, 0, 7)

		absence1 := &active.StaffAbsence{
			StaffID:     staff1.ID,
			AbsenceType: active.AbsenceTypeSick,
			DateStart:   today,
			DateEnd:     tomorrow,
			Status:      active.AbsenceStatusReported,
			CreatedBy:   staff1.ID,
		}

		absence2 := &active.StaffAbsence{
			StaffID:     staff2.ID,
			AbsenceType: active.AbsenceTypeVacation,
			DateStart:   tomorrow,
			DateEnd:     nextWeek,
			Status:      active.AbsenceStatusApproved,
			CreatedBy:   staff2.ID,
		}

		err := repo.Create(ctx, absence1)
		require.NoError(t, err)
		err = repo.Create(ctx, absence2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence1.ID)
			testpkg.CleanupTableRecords(t, db, "active.staff_absences", absence2.ID)
		}()

		absences, err := repo.GetByDateRange(ctx, today, nextWeek)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(absences), 2)

		// Verify both absences are in the results
		var found1, found2 bool
		for _, a := range absences {
			if a.ID == absence1.ID {
				found1 = true
			}
			if a.ID == absence2.ID {
				found2 = true
			}
		}
		assert.True(t, found1)
		assert.True(t, found2)
	})

	t.Run("returns empty for date range with no absences", func(t *testing.T) {
		futureDate := timezone.DateOfUTC(time.Now().AddDate(2, 0, 0))
		absences, err := repo.GetByDateRange(ctx, futureDate, futureDate)
		require.NoError(t, err)
		assert.Empty(t, absences)
	})
}
