// Package ports declares the consumer-owned seams of the open-room move
// workflow. Owners satisfy them with their public capabilities; nothing here
// reaches a repository or a model.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
)

// Rooms is the Facilities surface the move uses: the room row stays locked
// until the move commits, so its release cannot change underneath it.
type Rooms interface {
	FindRoomForUpdate(context.Context, int64) (facilities.Room, error)
}

// Activities is the Timetable & Activities surface that resolves, and on first
// use provisions, the system activity a room session runs under.
type Activities interface {
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
	CreateGroup(context.Context, timetable.GroupInput) (timetable.Group, error)
	ListCategories(context.Context) ([]timetable.Category, error)
	CreateCategory(context.Context, timetable.CreateCategory) (timetable.Category, error)
}

// RoomSessions returns the ID of the room's device-less session for the given
// system activity, creating the session when none runs.
type RoomSessions interface {
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (int64, error)
}

// MoveOutcome is what the Student Presence move changed.
type MoveOutcome struct {
	Moved     []int64
	Unchanged []int64
	Skipped   []openroommove.Skipped
}

// Moves transfers children into a room session with every source-side right
// and validation of the ordinary move, without requiring supervision there.
type Moves interface {
	MoveIntoRoomSession(ctx context.Context, roomSessionID int64, studentIDs []int64, actor openroommove.Actor) (MoveOutcome, error)
}

// Runtime binds the workflow to the tenant UnitOfWork.
type Runtime struct {
	// TenantID reads the tenant of the request.
	TenantID func(context.Context) int64
	// WithinTenant joins the ambient tenant transaction or opens one.
	WithinTenant func(context.Context, func(context.Context) error) error
	// AcquireLock takes an exclusive lock held until the transaction ends.
	AcquireLock func(ctx context.Context, key string) error
}

// Observation is the runtime evidence of one command run.
type Observation struct {
	Operation string
	Duration  time.Duration
	Err       error
	Moved     int
}
