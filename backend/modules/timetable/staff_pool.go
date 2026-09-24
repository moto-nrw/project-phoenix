package timetable

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Staff pool categories (#1884), mutually exclusive per staff member.
// Precedence: a non-absent assignment on the target block wins
// (assigned_here); otherwise any absent row on the day wins (absence is
// day-wide, #1840 — including when the target's own row carries it); then
// assigned_elsewhere > on_shift_free > not_on_shift.
const (
	StaffPoolAssignedHere      = "assigned_here"
	StaffPoolAbsent            = "absent"
	StaffPoolAssignedElsewhere = "assigned_elsewhere"
	StaffPoolOnShiftFree       = "on_shift_free"
	StaffPoolNotOnShift        = "not_on_shift"
)

// StaffPoolAssignment is one same-day block assignment overlapping the target
// window — the move source the planner offers.
type StaffPoolAssignment struct {
	InstanceID   int64
	Title        string
	StartTime    string // HH:MM
	EndTime      string // HH:MM
	IsSubstitute bool
}

// StaffPoolEntry is one staff member categorized against the target window.
type StaffPoolEntry struct {
	StaffID     int64
	DisplayName string
	Category    string
	// OnShift reports whether any non-cancelled shift overlaps the window;
	// CoversWindow whether the union of that day's shifts covers it entirely.
	OnShift      bool
	CoversWindow bool
	ShiftWindows []string // "HH:MM–HH:MM" per non-cancelled shift that day
	// AbsenceReason carries the day-wide absence reason for the absent
	// category (first non-empty reason of the day's absent rows).
	AbsenceReason *string
	// Assignments lists the overlapping same-day blocks the person is already
	// planned on (assigned_elsewhere; empty otherwise).
	Assignments []StaffPoolAssignment
}

// StaffPool is the categorized pool for one block's window. StartTime and
// EndTime are the block's wall-clock times.
type StaffPool struct {
	InstanceID int64
	Title      string
	Date       calendar.Date
	StartTime  time.Time
	EndTime    time.Time
	// DienstplanInUse mirrors the #1873 semantics: false when the block's
	// calendar week has no shifts at all, so "not on shift" carries no signal.
	DienstplanInUse bool
	Entries         []StaffPoolEntry
}
