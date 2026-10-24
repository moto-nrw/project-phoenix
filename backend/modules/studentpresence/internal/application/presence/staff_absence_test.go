package presence_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

	repo := api.NewStaffAbsenceSuiteRepositories(testpkg.NewDatabase(t, db)).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("creates staff absence with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		tomorrow := today.AddDays(1)
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     tomorrow.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)

	})

	t.Run("creates absence with vacation type", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		nextWeek := today.AddDays(7)
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeVacation,
			DateStart:   today.String(),
			DateEnd:     nextWeek.String(),
			Status:      workforce.AbsenceStatusApproved,
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)
		assert.Equal(t, workforce.AbsenceTypeVacation, absence.AbsenceType)

	})

	t.Run("creates half-day absence", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeTraining,
			DateStart:   today.String(),
			DateEnd:     today.String(),
			HalfDay:     true,
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)
		assert.NotZero(t, absence.ID)
		assert.True(t, absence.HalfDay)

	})

	t.Run("create with nil absence should fail", func(t *testing.T) {
		_, err := repo.CreateStaffAbsence(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid absence type should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: "invalid_type",
			DateStart:   today.String(),
			DateEnd:     today.String(),
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		assert.Error(t, err)
	})

	t.Run("create with invalid status should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     today.String(),
			Status:      "invalid_status",
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid absence status")
	})

	t.Run("create with missing staff ID should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     0, // Invalid
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     today.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("create with date_start after date_end should fail", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		yesterday := today.AddDays(-1)
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     yesterday.String(), // Before start
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}

		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		assert.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestStaffAbsenceRepository_GetByStaffAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewStaffAbsenceSuiteRepositories(testpkg.NewDatabase(t, db)).StaffAbsence
	ctx := testpkg.Ctx(t)

	t.Run("finds absence for specific date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		tomorrow := today.AddDays(1)

		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     tomorrow.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		absence, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)

		values, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staff.ID, OverlapFrom: today.String(), OverlapTo: today.String(), Statuses: []string{workforce.AbsenceStatusReported, workforce.AbsenceStatusApproved}, Limit: 1})
		var found *workforce.StaffAbsence
		if len(values) > 0 {
			found = &values[0]
		}
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, absence.ID, found.ID)
	})

	t.Run("returns nil when no absence for date", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		futureDate := calendar.TodayDate().AddDays(365)
		values, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staff.ID, OverlapFrom: futureDate.String(), OverlapTo: futureDate.String(), Statuses: []string{workforce.AbsenceStatusReported, workforce.AbsenceStatusApproved}, Limit: 1})
		var found *workforce.StaffAbsence
		if len(values) > 0 {
			found = &values[0]
		}
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("ignores requested and canceled absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		for _, status := range []string{
			workforce.AbsenceStatusRequested,
			workforce.AbsenceStatusCanceled,
			workforce.AbsenceStatusDeclined,
		} {
			absence := workforce.StaffAbsence{
				StaffID:     staff.ID,
				AbsenceType: workforce.AbsenceTypeVacation,
				DateStart:   today.String(),
				DateEnd:     today.String(),
				Status:      status,
				CreatedBy:   staff.ID,
			}
			absence, err := repo.CreateStaffAbsence(ctx, &absence)
			require.NoError(t, err)
		}

		values, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staff.ID, OverlapFrom: today.String(), OverlapTo: today.String(), Statuses: []string{workforce.AbsenceStatusReported, workforce.AbsenceStatusApproved}, Limit: 1})
		var found *workforce.StaffAbsence
		if len(values) > 0 {
			found = &values[0]
		}
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}
