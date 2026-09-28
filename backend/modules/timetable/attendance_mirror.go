package timetable

import (
	"context"
	"time"
)

// The attendance mirror (WP-B10, #3424 slice S3) keeps the slot attendance
// of schedule.instance_students in step with Student Presence's visits. The
// Timetable owner is the only runtime writer of those rows: Student Presence
// triggers the mirror through its attendance syncer port, bound at the
// composition root, inside its own tenant transaction, and rolls back its
// visit write together with the mirror when the mirror fails.

// AttendanceVisit is the part of a Student Presence visit the mirror reads.
type AttendanceVisit struct {
	StudentID     int64
	ActiveGroupID int64
	EntryTime     time.Time
	ExitTime      *time.Time
}

// AttendanceSnapshot is the slot attendance a mirror call left behind, the
// state the presence events carry.
type AttendanceSnapshot struct {
	Status      string
	Substatus   *string
	Note        *string
	InstanceID  int64
	IsUnplanned bool
}

// AttendanceMirror is the Timetable command Student Presence triggers on
// every visit write. Missing timetable assignments are valid no-ops (nil
// snapshot, nil error); read and write failures return errors so the caller
// rolls back its visit write with them.
type AttendanceMirror interface {
	// MirrorCheckInForVisit opens observed presence on the slot of the block
	// bridged to the visit's session, or records unplanned presence when the
	// child has no slot there.
	MirrorCheckInForVisit(ctx context.Context, visit AttendanceVisit) (*AttendanceSnapshot, error)
	// MirrorCheckInAt resolves a roomless check-in to exactly one currently
	// scheduled slot. Zero or multiple matches remain unassigned.
	MirrorCheckInAt(ctx context.Context, studentID int64, at time.Time) (*AttendanceSnapshot, error)
	// MirrorCheckOutForVisit stamps the slot's checkout without changing its
	// attendance status and returns the current snapshot.
	MirrorCheckOutForVisit(ctx context.Context, visit AttendanceVisit) (*AttendanceSnapshot, error)
	// MirrorVisitRevision replaces a slot interval only when it still matches
	// previous.
	MirrorVisitRevision(ctx context.Context, previous, updated AttendanceVisit) error
	// MirrorCheckOutAt closes the most recently checked-in open slot of a
	// roomless attendance flow.
	MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) error
	// MirrorCheckInAtBatch is MirrorCheckInAt for many students at one
	// shared instant, with one candidate query and one guarded update.
	MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error
	// MirrorCheckOutAtBatch is MirrorCheckOutAt for many students at one
	// shared instant.
	MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error
	// MirrorCheckOutForVisits is MirrorCheckOutForVisit for visits ended at
	// one shared instant, without snapshots.
	MirrorCheckOutForVisits(ctx context.Context, visits []AttendanceVisit, at time.Time) error
}
