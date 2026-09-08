// Package ports declares the consumer-owned seams of the session end
// workflow. Owners satisfy them with their public capabilities; nothing here
// reaches a repository or a model.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Presence is the Student Presence command surface the close uses: lock the
// group first, then close everything still open in it.
type Presence interface {
	LockGroup(context.Context, int64) (studentpresence.LiveGroup, error)
	EndGroupSession(context.Context, int64, time.Time) (studentpresence.EndedGroupSession, error)
}

// Timetable is the Timetable & Activities surface the close uses: find the
// instance mirrored from the session, stamp the slot check-outs of the
// children the close sent home, and resolve the activity name for the
// announcement.
type Timetable interface {
	ListActivityInstances(context.Context, timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error)
	CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error)
	FindGroup(context.Context, int64) (timetable.Group, error)
}

// InstanceCompletion finalizes the attendance of every still-active instance
// mirrored from the given sessions and marks them completed. The Timetable
// owner guarantees the ordering inside: non-bookings are stamped, expected
// children flip to absent, then the instance closes. It returns how many
// instances changed.
type InstanceCompletion interface {
	CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error)
}

// Students resolves the education group of every checked-out child so the
// announcement can be scoped to the affected groups.
type Students interface {
	ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
}

// Rooms resolves the room name for the announcement.
type Rooms interface {
	FindRoom(context.Context, int64) (facilities.Room, error)
}

// GuardianWaker tells the guardians of a child that its presence changed.
// Best effort, after commit.
type GuardianWaker interface {
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}

// EndedStudent is one child the close checked out.
type EndedStudent struct {
	StudentID        int64
	EducationGroupID *int64
}

// CompletedInstance is the timetable instance the close completed.
type CompletedInstance struct {
	ID        int64
	Date      string // YYYY-MM-DD
	StartTime string // HH:MM:SS
	RoomID    int64
}

// Notification carries everything the after-commit announcements need. It is
// gathered inside the transaction; the notifier must not touch the database.
type Notification struct {
	TenantID      int64
	ActiveGroupID int64
	Students      []EndedStudent
	RoomID        int64
	ActivityName  string
	RoomName      string
	Instance      *CompletedInstance
}

// Notifier announces a committed session end.
type Notifier interface {
	SessionEnded(Notification)
}

// Runtime binds the workflow to the tenant UnitOfWork.
type Runtime struct {
	// TenantID reads the tenant of the request or job.
	TenantID func(context.Context) int64
	// WithinTenant joins the ambient tenant transaction or opens one.
	WithinTenant func(context.Context, func(context.Context) error) error
	// AfterCommit queues fn until the surrounding transaction committed.
	AfterCommit func(context.Context, func())
	// Now is the close instant; visits, supervisions, group, slot check-outs
	// and the instance completion all carry the same value.
	Now func() time.Time
}

// Observation is the runtime evidence of one command run.
type Observation struct {
	Operation          string
	Duration           time.Duration
	Err                error
	StudentsCheckedOut int
	SupervisorsEnded   int
	InstanceCompleted  bool
}
