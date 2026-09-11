package audit

import (
	"context"
	"time"
)

// StudentDeletion is an append-only, data-minimal tombstone for a permanently
// deleted child. It keeps the internal student ID for accountability, but no
// name, birthday, contact information or snapshot of the erased data.
type StudentDeletion struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	StudentID      int64                 `bun:"student_id,notnull" json:"student_id"`
	ActorAccountID int64                 `bun:"actor_account_id,notnull" json:"actor_account_id"`
	Reason         string                `bun:"reason,notnull" json:"reason"`
	Counts         StudentDeletionCounts `bun:"counts,type:jsonb,notnull" json:"counts"`
	DeletedAt      time.Time             `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
}

// StudentDeletionCounts is the immutable dependency count stored with a
// student tombstone. It is copied at the command boundary from the people
// directory's deletion preview.
type StudentDeletionCounts struct {
	TimetableAssignments int `json:"timetable_assignments"`
	ActivityEnrollments  int `json:"activity_enrollments"`
	AttendanceRecords    int `json:"attendance_records"`
	CareSchedules        int `json:"care_schedules"`
	GuardianLinks        int `json:"guardian_links"`
	CompanionLinks       int `json:"companion_links"`
	Communications       int `json:"communications"`
	Consents             int `json:"consents"`
	EnrollmentReferences int `json:"enrollment_references"`
	OtherRecords         int `json:"other_records"`
}

type StudentDeletionRepository interface {
	Create(ctx context.Context, event *StudentDeletion) error
	CountStudentReferences(ctx context.Context, studentID int64) (int, error)
}
