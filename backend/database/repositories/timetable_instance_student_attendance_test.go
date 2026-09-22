package repositories

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/require"
)

// TestSameLegacyAttendanceRestoreComparesValues pins the comparison the
// update path skips a write on: the restore shape carries pointers, so an
// address comparison would call every update a change and rewrite the
// attendance row of an unchanged participant.
func TestSameLegacyAttendanceRestoreComparesValues(t *testing.T) {
	t.Parallel()

	checkedIn := time.Date(2026, 9, 22, 8, 15, 0, 0, time.UTC)
	restore := func(mutate func(*studentpresence.SessionAttendanceRestore)) studentpresence.SessionAttendanceRestore {
		substatus, note, dayID := "spaeter", "Kommt nach dem Arzttermin", int64(7)
		at := checkedIn
		row := studentpresence.SessionAttendanceRestore{
			ParticipantID: 42, Status: studentpresence.SessionAttendancePresent,
			Substatus: &substatus, Note: &note, CheckedInAt: &at,
			StudentStatusDayID: &dayID,
		}
		if mutate != nil {
			mutate(&row)
		}
		return row
	}

	require.True(t, sameLegacyAttendanceRestore(restore(nil), restore(nil)),
		"equal values behind different pointers are the same attendance")
	require.True(t, sameLegacyAttendanceRestore(restore(nil), restore(func(row *studentpresence.SessionAttendanceRestore) {
		berlin := row.CheckedInAt.In(time.FixedZone("CEST", 2*60*60))
		row.CheckedInAt = &berlin
	})), "the instant decides, not the location the stored row arrives in")

	require.False(t, sameLegacyAttendanceRestore(restore(nil), restore(func(row *studentpresence.SessionAttendanceRestore) {
		row.Note = nil
	})), "a cleared note is a change")
	require.False(t, sameLegacyAttendanceRestore(restore(nil), restore(func(row *studentpresence.SessionAttendanceRestore) {
		other := "krank"
		row.Substatus = &other
	})), "a different substatus is a change")
	require.False(t, sameLegacyAttendanceRestore(restore(nil), restore(func(row *studentpresence.SessionAttendanceRestore) {
		later := row.CheckedInAt.Add(time.Minute)
		row.CheckedInAt = &later
	})), "a different check-in instant is a change")
	require.False(t, sameLegacyAttendanceRestore(restore(nil), restore(func(row *studentpresence.SessionAttendanceRestore) {
		row.Status = studentpresence.SessionAttendanceExpected
	})), "a different status is a change")
}
