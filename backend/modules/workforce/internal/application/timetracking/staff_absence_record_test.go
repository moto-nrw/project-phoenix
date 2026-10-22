package timetracking

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsence_Validate(t *testing.T) {
	t.Parallel()

	validAbsence := func() *StaffAbsence {
		return &StaffAbsence{
			StaffID:     1,
			AbsenceType: workforce.AbsenceTypeSick,
			DateStart:   calendar.NewDate(2024, 3, 1),
			DateEnd:     calendar.NewDate(2024, 3, 3),
			Status:      workforce.AbsenceStatusReported,
			CreatedBy:   1,
		}
	}

	t.Run("valid absence", func(t *testing.T) {
		assert.NoError(t, validAbsence().Validate())
	})

	t.Run("all valid absence types", func(t *testing.T) {
		for _, at := range ValidAbsenceTypes {
			a := validAbsence()
			a.AbsenceType = at
			assert.NoError(t, a.Validate(), "type %s should be valid", at)
		}
	})

	t.Run("all valid absence statuses", func(t *testing.T) {
		for _, s := range ValidAbsenceStatuses {
			a := validAbsence()
			a.Status = s
			assert.NoError(t, a.Validate(), "status %s should be valid", s)
		}
	})

	t.Run("missing staff ID", func(t *testing.T) {
		a := validAbsence()
		a.StaffID = 0
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("invalid absence type", func(t *testing.T) {
		a := validAbsence()
		a.AbsenceType = "invalid_type"
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid absence type")
	})

	t.Run("invalid status", func(t *testing.T) {
		a := validAbsence()
		a.Status = "invalid_status"
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid absence status")
	})

	t.Run("missing date_start", func(t *testing.T) {
		a := validAbsence()
		a.DateStart = calendar.Date("")
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_start is required")
	})

	t.Run("missing date_end", func(t *testing.T) {
		a := validAbsence()
		a.DateEnd = calendar.Date("")
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_end is required")
	})

	t.Run("date_start after date_end", func(t *testing.T) {
		a := validAbsence()
		a.DateStart = calendar.NewDate(2024, 3, 5)
		a.DateEnd = calendar.NewDate(2024, 3, 1)
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_start must be before or equal to date_end")
	})

	t.Run("same start and end date is valid", func(t *testing.T) {
		a := validAbsence()
		d := calendar.NewDate(2024, 3, 1)
		a.DateStart = d
		a.DateEnd = d
		assert.NoError(t, a.Validate())
	})

	t.Run("missing created_by", func(t *testing.T) {
		a := validAbsence()
		a.CreatedBy = 0
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "created_by is required")
	})
}

func TestAbsenceDurationDays(t *testing.T) {
	t.Parallel()

	t.Run("single day", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: calendar.NewDate(2024, 3, 1),
			DateEnd:   calendar.NewDate(2024, 3, 1),
		}
		assert.Equal(t, 1, absenceDurationDays(a))
	})

	t.Run("three days", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: calendar.NewDate(2024, 3, 1),
			DateEnd:   calendar.NewDate(2024, 3, 3),
		}
		assert.Equal(t, 3, absenceDurationDays(a))
	})

	t.Run("week", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: calendar.NewDate(2024, 3, 4),
			DateEnd:   calendar.NewDate(2024, 3, 10),
		}
		assert.Equal(t, 7, absenceDurationDays(a))
	})

	t.Run("minimum is 1 day", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: calendar.NewDate(2024, 3, 1),
			DateEnd:   calendar.NewDate(2024, 3, 1),
		}
		assert.GreaterOrEqual(t, absenceDurationDays(a), 1)
	})
}

func TestAbsenceTypeConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "sick", workforce.AbsenceTypeSick)
	assert.Equal(t, "vacation", workforce.AbsenceTypeVacation)
	assert.Equal(t, "training", workforce.AbsenceTypeTraining)
	assert.Equal(t, "other", workforce.AbsenceTypeOther)
}

func TestAbsenceStatusConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "reported", workforce.AbsenceStatusReported)
	assert.Equal(t, "approved", workforce.AbsenceStatusApproved)
	assert.Equal(t, "declined", workforce.AbsenceStatusDeclined)
}
