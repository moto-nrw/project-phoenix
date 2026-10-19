// Package compose wires the student deletion workflow over the owner
// capabilities and the shared tenant runtime (#2710).
package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/uptrace/bun"
)

// Feedback is the Feedback owner's deletion count.
type Feedback interface {
	CountForStudent(context.Context, int64) (int, error)
}

// Dependencies name the owner capabilities the composition does not build
// itself: the bound People Directory, Care Plan and Timetable of the running
// graph, the Feedback module, and the runtime hooks.
type Dependencies struct {
	DB        *bun.DB
	Directory studentdeletion.Directory
	CarePlan  studentdeletion.CarePlan
	Timetable studentdeletion.Timetable
	Feedback  Feedback
	// IsVerifiedStaff reports whether the caller has a staff record in the
	// current tenant; guests and guardians authenticate against the same
	// portal, so a permission alone never authorizes a deletion.
	IsVerifiedStaff       func(context.Context) (bool, error)
	LockCareBookingWrites func(context.Context) error
	// UnlinkPhoto schedules the removal of a stored photo after the outer
	// transaction commits. Optional: nil keeps the bytes (bare compositions).
	UnlinkPhoto func(context.Context, string) error
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	// Audit overrides the append command of Owners.
	Audit AuditLog
	Today func() string
	Now   func() time.Time
	// Owners are the owners the root builds for the deletion; the composition
	// builds no other owner's composition (#4004, ADR 0059 point 6).
	Owners Owners
}

// Owners are the School Structure, Communication, Enrollment, Student
// Presence, Appointments and Identity & Access capabilities the deletion
// counts in and deletes through.
type Owners struct {
	Structure     studentdeletion.Structure
	Conversations studentdeletion.Conversations
	Enrollment    studentdeletion.Enrollment
	// The counters read the child's Student Presence attendance, visits and
	// consents, appointment recipients and pending guardian invitations.
	Attendance, Visits, Consents      studentdeletion.Counter
	Appointments, GuardianInvitations studentdeletion.Counter
	// Audit appends the deletion's audit records, as DeletionRecords builds
	// them; AuditReferences counts the child's audit rows.
	Audit           AuditLog
	DeletionRecords func(studentdeletion.Actor, studentdeletion.Result) []any
	AuditReferences studentdeletion.Counter
}

// AuditLog is the Audit owner's append command.
type AuditLog interface {
	Append(context.Context, any) error
}

// New constructs the workflow over the ports already bound by the root.
func New(deps studentdeletion.Dependencies) (*studentdeletion.Workflow, error) {
	return studentdeletion.New(deps)
}
