package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// MorningAttendanceStore persists the morning attendance (#4790) in
// active.morning_attendance, apart from active.attendance.
type MorningAttendanceStore interface {
	// ListMorningAttendance returns the marks of date for the given students
	// and class-list entries.
	ListMorningAttendance(ctx context.Context, date Date, studentIDs, classListEntryIDs []int64) ([]studentpresence.MorningAttendanceEntry, Stats, error)
	// UpsertMorningAttendance stores the marks of date; an existing mark of
	// the same child and date is replaced.
	UpsertMorningAttendance(ctx context.Context, date Date, recordedBy int64, marks []studentpresence.MorningAttendanceMark) (Stats, error)
	// DeleteMorningAttendanceBefore deletes the tenant's marks dated before
	// cutoff.
	DeleteMorningAttendanceBefore(ctx context.Context, cutoff Date) (Stats, error)
}
