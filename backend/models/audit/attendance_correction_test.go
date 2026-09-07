package audit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCorrection() *AttendanceCorrection {
	return &AttendanceCorrection{
		InstanceID: 7,
		StudentID:  9,
		FieldName:  AttendanceFieldNote,
		Reason:     "Notiz war einem falschen Kind zugeordnet",
	}
}

func TestAttendanceCorrection_Validate_AcceptsCompleteRow(t *testing.T) {
	t.Parallel()

	require.NoError(t, validCorrection().Validate())
}

func TestAttendanceCorrection_Validate_RequiresIdentifiers(t *testing.T) {
	t.Parallel()

	noInstance := validCorrection()
	noInstance.InstanceID = 0
	assert.ErrorContains(t, noInstance.Validate(), "instance ID is required")

	noStudent := validCorrection()
	noStudent.StudentID = 0
	assert.ErrorContains(t, noStudent.Validate(), "student ID is required")
}

func TestAttendanceCorrection_Validate_RejectsUnknownField(t *testing.T) {
	t.Parallel()

	// checked_in_at is a real column on the attendance row but is written by
	// the operational flow, never corrected by hand — it must not be audited
	// through this table.
	row := validCorrection()
	row.FieldName = "checked_in_at"
	assert.ErrorContains(t, row.Validate(), "invalid attendance field name")
}

// The reason is the integrity guarantee of this table: a correction of a
// closed record without a stated reason is weak evidence.
func TestAttendanceCorrection_Validate_RequiresReason(t *testing.T) {
	t.Parallel()

	for name, reason := range map[string]string{
		"empty":      "",
		"spaces":     "   ",
		"whitespace": "\t\n ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			row := validCorrection()
			row.Reason = reason
			assert.ErrorContains(t, row.Validate(), "reason is required")
		})
	}
}

func TestAttendanceCorrection_Validate_TrimsReason(t *testing.T) {
	t.Parallel()

	row := validCorrection()
	row.Reason = "  Übertragungsfehler  "
	require.NoError(t, row.Validate())
	assert.Equal(t, "Übertragungsfehler", row.Reason, "the stored reason must not carry padding")
}

func TestAttendanceCorrection_Validate_CapsReasonLength(t *testing.T) {
	t.Parallel()

	atLimit := validCorrection()
	// Umlauts: the cap counts runes, not bytes — 500 "ä" are 1000 bytes.
	atLimit.Reason = strings.Repeat("ä", CorrectionReasonMaxLength)
	require.NoError(t, atLimit.Validate())

	overLimit := validCorrection()
	overLimit.Reason = strings.Repeat("ä", CorrectionReasonMaxLength+1)
	assert.ErrorContains(t, overLimit.Validate(), "reason is too long")
}

// Clearing a value IS a correction, so a nil new value must stay valid.
func TestAttendanceCorrection_Validate_AllowsClearedValue(t *testing.T) {
	t.Parallel()

	old := "Kind war unruhig"
	row := validCorrection()
	row.OldValue = &old
	row.NewValue = nil
	require.NoError(t, row.Validate())
}
