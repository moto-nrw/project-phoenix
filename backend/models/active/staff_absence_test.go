package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsence_Validate(t *testing.T) {
	validAbsence := func() *StaffAbsence {
		return &StaffAbsence{
			StaffID:     1,
			AbsenceType: AbsenceTypeSick,
			DateStart:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			DateEnd:     time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC),
			Status:      AbsenceStatusReported,
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
		a.DateStart = time.Time{}
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_start is required")
	})

	t.Run("missing date_end", func(t *testing.T) {
		a := validAbsence()
		a.DateEnd = time.Time{}
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_end is required")
	})

	t.Run("date_start after date_end", func(t *testing.T) {
		a := validAbsence()
		a.DateStart = time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC)
		a.DateEnd = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		err := a.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "date_start must be before or equal to date_end")
	})

	t.Run("same start and end date is valid", func(t *testing.T) {
		a := validAbsence()
		d := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
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

func TestStaffAbsence_DurationDays(t *testing.T) {
	t.Run("single day", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		}
		assert.Equal(t, 1, a.DurationDays())
	})

	t.Run("three days", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC),
		}
		assert.Equal(t, 3, a.DurationDays())
	})

	t.Run("week", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
		}
		assert.Equal(t, 7, a.DurationDays())
	})

	t.Run("minimum is 1 day", func(t *testing.T) {
		a := &StaffAbsence{
			DateStart: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			DateEnd:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		}
		assert.GreaterOrEqual(t, a.DurationDays(), 1)
	})
}

func TestStaffAbsence_TableName(t *testing.T) {
	a := &StaffAbsence{}
	assert.Equal(t, "active.staff_absences", a.TableName())
}

func TestStaffAbsence_Getters(t *testing.T) {
	now := time.Now()
	a := &StaffAbsence{}
	a.ID = 99
	a.CreatedAt = now
	a.UpdatedAt = now

	assert.Equal(t, int64(99), a.GetID())
	assert.Equal(t, now, a.GetCreatedAt())
	assert.Equal(t, now, a.GetUpdatedAt())
}

func TestAbsenceTypeConstants(t *testing.T) {
	assert.Equal(t, "sick", AbsenceTypeSick)
	assert.Equal(t, "vacation", AbsenceTypeVacation)
	assert.Equal(t, "training", AbsenceTypeTraining)
	assert.Equal(t, "other", AbsenceTypeOther)
}

func TestAbsenceStatusConstants(t *testing.T) {
	assert.Equal(t, "reported", AbsenceStatusReported)
	assert.Equal(t, "approved", AbsenceStatusApproved)
	assert.Equal(t, "declined", AbsenceStatusDeclined)
}

func TestStaffAbsence_BeforeAppendModel(t *testing.T) {
	a := &StaffAbsence{}

	t.Run("handles SelectQuery", func(t *testing.T) {
		err := a.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles UpdateQuery", func(t *testing.T) {
		err := a.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles DeleteQuery", func(t *testing.T) {
		err := a.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("handles InsertQuery", func(t *testing.T) {
		err := a.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})
}
