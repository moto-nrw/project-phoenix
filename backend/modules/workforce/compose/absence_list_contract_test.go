package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsenceListContract(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all staff absences", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     today.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		_, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)

		absences, err := repo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{})
		require.NoError(t, err)
		assert.NotEmpty(t, absences)
	})

	t.Run("lists with query options", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
		today := calendar.TodayDate()
		absence := workforce.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   today.String(),
			DateEnd:     today.String(),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   staff.ID,
		}
		_, err := repo.CreateStaffAbsence(ctx, &absence)
		require.NoError(t, err)

		options := workforce.StaffAbsenceFilter{Limit: 10}

		absences, err := repo.ListStaffAbsences(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(absences), 10)
	})
}
