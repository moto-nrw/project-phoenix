package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// TestSameLegacyAttendanceRestoreComparesValues pins the comparison the
// update path skips a write on: the restore shape carries pointers, so an
// address comparison would call every update a change and rewrite the
// attendance row of an unchanged participant.
func TestSameLegacyAttendanceRestoreComparesValues(t *testing.T) {
	t.Parallel()

	checkedIn := time.Date(2026, 9, 22, 8, 15, 0, 0, time.UTC)
	// restore builds the Student Presence restore shape through the adapter's
	// own mapping, with fresh pointers on every call.
	restore := func() *scheduleModels.InstanceStudent {
		substatus, note, dayID := "spaeter", "Kommt nach dem Arzttermin", int64(7)
		at := checkedIn
		return &scheduleModels.InstanceStudent{
			Status:    scheduleModels.AttendanceStatusPresent,
			Substatus: &substatus, Note: &note, CheckedInAt: &at,
			StudentStatusDayID: &dayID,
		}
	}
	base := legacyAttendanceRestore(42, restore())

	require.True(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, restore())),
		"equal values behind different pointers are the same attendance")

	relocated := restore()
	berlin := relocated.CheckedInAt.In(time.FixedZone("CEST", 2*60*60))
	relocated.CheckedInAt = &berlin
	require.True(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, relocated)),
		"the instant decides, not the location the stored row arrives in")

	cleared := restore()
	cleared.Note = nil
	require.False(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, cleared)), "a cleared note is a change")

	otherSubstatus := restore()
	other := "krank"
	otherSubstatus.Substatus = &other
	require.False(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, otherSubstatus)), "a different substatus is a change")

	later := restore()
	laterAt := later.CheckedInAt.Add(time.Minute)
	later.CheckedInAt = &laterAt
	require.False(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, later)), "a different check-in instant is a change")

	expected := restore()
	expected.Status = scheduleModels.AttendanceStatusExpected
	require.False(t, sameLegacyAttendanceRestore(base, legacyAttendanceRestore(42, expected)), "a different status is a change")
}
