package users

import (
	"context"
	"time"
)

// StudentDeletionCounts groups every child-owned or child-linked row affected
// by a permanent student deletion. The groups are deliberately user-facing:
// the API and confirmation dialog expose these names instead of leaking the
// physical table layout.
type StudentDeletionCounts struct {
	TimetableAssignments int `bun:"timetable_assignments" json:"timetable_assignments"`
	ActivityEnrollments  int `bun:"activity_enrollments" json:"activity_enrollments"`
	AttendanceRecords    int `bun:"attendance_records" json:"attendance_records"`
	CareSchedules        int `bun:"care_schedules" json:"care_schedules"`
	GuardianLinks        int `bun:"guardian_links" json:"guardian_links"`
	CompanionLinks       int `bun:"companion_links" json:"companion_links"`
	Communications       int `bun:"communications" json:"communications"`
	Consents             int `bun:"consents" json:"consents"`
	EnrollmentReferences int `bun:"enrollment_references" json:"enrollment_references"`
	OtherRecords         int `bun:"other_records" json:"other_records"`
}

// Total is the number of dependent rows the deletion removes or unlinks. The
// student and person rows themselves are intentionally not included.
func (c StudentDeletionCounts) Total() int {
	return c.TimetableAssignments + c.ActivityEnrollments +
		c.AttendanceRecords + c.CareSchedules + c.GuardianLinks +
		c.CompanionLinks + c.Communications + c.Consents +
		c.EnrollmentReferences + c.OtherRecords
}

// StudentDeletionRepository owns the cross-schema read model and the one
// explicit cleanup required before users.students can be deleted. Every other
// child relation either cascades safely or is unlinked via ON DELETE SET NULL.
type StudentDeletionRepository interface {
	Preview(ctx context.Context, studentID int64) (*StudentDeletionCounts, error)
	DeleteTimetableAssignments(ctx context.Context, studentID int64) (int64, error)
	DeleteLegacyGuardianLinks(ctx context.Context, personID int64) error
	AnonymizePersonIfUnchanged(ctx context.Context, personID int64, updatedAt time.Time) (bool, error)
}
