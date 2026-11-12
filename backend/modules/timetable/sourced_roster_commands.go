package timetable

import "context"

// SourcedRosterCommands applies the validated offering-roster diffs without
// growing the general timetable facade. The caller holds the recurrence gate.
type SourcedRosterCommands interface {
	SaveSourcedRosterChanges(context.Context, SourcedRosterChanges) error
	UpdateGroupOfferingSources(context.Context, map[int64]OfferingSourceInput) error
}

// SourcedInstanceStudents applies the corresponding planned-participant diff.
type SourcedInstanceStudents interface {
	SaveSourcedInstanceStudents(context.Context, []InstanceStudentInput, []int64) error
}
