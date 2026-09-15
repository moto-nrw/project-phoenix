package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsenceDateRangeContract(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds absences in date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.NewDate(2026, 8, 24)
		tomorrow := today.AddDays(1)
		nextWeek := today.AddDays(7)

		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   tomorrow.String(),
			DateEnd:     nextWeek.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		absence, err := repo.CreateStaffAbsence(ctx, absence)
		require.NoError(t, err)

		absences, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
			StaffID: staff.ID, OverlapFrom: today.String(), OverlapTo: nextWeek.String(),
			Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}},
		})
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
		futureDate := timezone.NewDate(2026, 8, 24).AddDays(365)
		absences, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
			StaffID: staff.ID, OverlapFrom: futureDate.String(), OverlapTo: futureDate.String(),
			Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}},
		})
		require.NoError(t, err)
		assert.Empty(t, absences)
	})

	t.Run("finds overlapping absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)
		tomorrow := today.AddDays(1)

		// Create absence spanning yesterday to tomorrow
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeVacation,
			DateStart:   yesterday.String(),
			DateEnd:     tomorrow.String(),
			Status:      workforce.AbsenceStatusApproved,
			CreatedBy:   staff.ID,
		}
		_, err := repo.CreateStaffAbsence(ctx, absence)
		require.NoError(t, err)

		// Query just for today should find the overlapping absence
		absences, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{
			StaffID: staff.ID, OverlapFrom: today.String(), OverlapTo: today.String(),
			Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})
}
