package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkSessionEdit_Validate(t *testing.T) {
	validEdit := func() *WorkSessionEdit {
		return &WorkSessionEdit{
			SessionID: 1,
			StaffID:   2,
			EditedBy:  3,
			FieldName: FieldCheckInTime,
			CreatedAt: time.Now(),
		}
	}

	t.Run("valid edit", func(t *testing.T) {
		assert.NoError(t, validEdit().Validate())
	})

	t.Run("all valid field names", func(t *testing.T) {
		fields := []string{
			FieldCheckInTime, FieldCheckOutTime, FieldBreakMinutes,
			FieldBreakDuration, FieldStatus, FieldNotes,
		}
		for _, f := range fields {
			e := validEdit()
			e.FieldName = f
			assert.NoError(t, e.Validate(), "field %s should be valid", f)
		}
	})

	t.Run("missing session ID", func(t *testing.T) {
		e := validEdit()
		e.SessionID = 0
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session ID is required")
	})

	t.Run("missing staff ID", func(t *testing.T) {
		e := validEdit()
		e.StaffID = 0
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID is required")
	})

	t.Run("missing edited by", func(t *testing.T) {
		e := validEdit()
		e.EditedBy = 0
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "edited by is required")
	})

	t.Run("empty field name", func(t *testing.T) {
		e := validEdit()
		e.FieldName = ""
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "field name is required")
	})

	t.Run("invalid field name", func(t *testing.T) {
		e := validEdit()
		e.FieldName = "invalid_field"
		err := e.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field name")
	})

	t.Run("zero created_at gets set to now", func(t *testing.T) {
		e := validEdit()
		e.CreatedAt = time.Time{}
		before := time.Now()
		err := e.Validate()
		assert.NoError(t, err)
		assert.False(t, e.CreatedAt.IsZero())
		assert.True(t, e.CreatedAt.After(before) || e.CreatedAt.Equal(before))
	})
}

func TestWorkSessionEdit_TableName(t *testing.T) {
	e := &WorkSessionEdit{}
	assert.Equal(t, "audit.work_session_edits", e.TableName())
}

func TestWorkSessionEdit_FieldConstants(t *testing.T) {
	assert.Equal(t, "check_in_time", FieldCheckInTime)
	assert.Equal(t, "check_out_time", FieldCheckOutTime)
	assert.Equal(t, "break_minutes", FieldBreakMinutes)
	assert.Equal(t, "break_duration", FieldBreakDuration)
	assert.Equal(t, "status", FieldStatus)
	assert.Equal(t, "notes", FieldNotes)
}
