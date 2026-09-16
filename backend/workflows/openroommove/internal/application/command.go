// Package application holds the open-room move decision: which room, which
// room session, in which lock order, inside one UnitOfWork.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/ports"
)

// Dependencies are the owner capabilities and runtime the command coordinates.
type Dependencies struct {
	Rooms        ports.Rooms
	Activities   ports.Activities
	RoomSessions ports.RoomSessions
	Moves        ports.Moves
	Runtime      ports.Runtime
	Observe      func(ports.Observation)
}

type command struct {
	deps Dependencies
}

// NewCommand builds the open-room move command. Every dependency is required;
// a missing one is a composition error, not a runtime fallback.
func NewCommand(deps Dependencies) openroommove.Command {
	if deps.Rooms == nil || deps.Activities == nil || deps.RoomSessions == nil || deps.Moves == nil ||
		deps.Observe == nil || deps.Runtime.TenantID == nil || deps.Runtime.WithinTenant == nil ||
		deps.Runtime.AcquireLock == nil {
		panic("open room move: all dependencies are required")
	}
	return &command{deps: deps}
}

// MoveToOpenRoom records the move. Order inside the unit:
//
//  1. lock the room row (Facilities) and require its release, so a release
//     removed concurrently rejects the move instead of racing it;
//  2. resolve the room's system activity (Timetable & Activities),
//     provisioning it on first use under a tenant-wide lock;
//  3. find or create the room's session for that activity (Student Presence);
//  4. move the children into it (Student Presence), which locks the children,
//     checks the caller's source-side rights and the room capacity, and ends
//     each current visit before recording the new one.
//
// A failure anywhere returns an error and the transaction owner rolls back.
func (c *command) MoveToOpenRoom(ctx context.Context, move openroommove.Move) (openroommove.Result, error) {
	started := time.Now()
	var result openroommove.Result
	err := c.move(ctx, move, &result)
	c.deps.Observe(ports.Observation{
		Operation: "move_to_open_room",
		Duration:  time.Since(started),
		Err:       err,
		Moved:     len(result.Moved),
	})
	if err != nil {
		return openroommove.Result{}, err
	}
	return result, nil
}

func (c *command) move(ctx context.Context, move openroommove.Move, result *openroommove.Result) error {
	studentIDs := uniquePositive(move.StudentIDs)
	if move.RoomID <= 0 || len(studentIDs) == 0 {
		return openroommove.ErrInvalidMove
	}
	return c.deps.Runtime.WithinTenant(ctx, func(txCtx context.Context) error {
		room, err := c.deps.Rooms.FindRoomForUpdate(txCtx, move.RoomID)
		if err != nil {
			if errors.Is(err, facilities.ErrRoomNotFound) {
				return openroommove.ErrRoomNotFound
			}
			return fmt.Errorf("open room move: lock room %d: %w", move.RoomID, err)
		}
		if !room.IsOpenRoom {
			return openroommove.ErrRoomNotReleased
		}

		activityID, err := c.roomActivity(txCtx, room)
		if err != nil {
			return err
		}
		sessionID, err := c.deps.RoomSessions.EnsureOpenRoomSession(txCtx, room.ID, activityID)
		if err != nil {
			return err
		}
		outcome, err := c.deps.Moves.MoveIntoRoomSession(txCtx, sessionID, studentIDs, move.Actor)
		if err != nil {
			return err
		}
		*result = openroommove.Result{
			RoomID:        room.ID,
			RoomSessionID: sessionID,
			Moved:         outcome.Moved,
			Unchanged:     outcome.Unchanged,
			Skipped:       outcome.Skipped,
		}
		return nil
	})
}

// activitySpec describes the system activity a room session runs under.
type activitySpec struct {
	name string
	// plannedRoomID pins the activity to one room; nil marks the shared
	// activity that serves every other released room.
	plannedRoomID       *int64
	maxParticipants     int
	categoryName        string
	categoryDescription string
	categoryColor       string
}

// specFor keeps the canonical Schulhof on its existing system activity, so
// phone moves and the Schulhof kiosk journey share one room session there.
// Every other released room uses the shared open-room activity.
func specFor(room facilities.Room) activitySpec {
	if room.IsSystem && room.Name == facilities.SchulhofRoomName {
		roomID := room.ID
		return activitySpec{
			name:                facilities.SchulhofActivityName,
			plannedRoomID:       &roomID,
			maxParticipants:     facilities.SchulhofMaxParticipants,
			categoryName:        facilities.SchulhofCategoryName,
			categoryDescription: facilities.SchulhofCategoryDescription,
			categoryColor:       facilities.SchulhofColor,
		}
	}
	return activitySpec{
		name:                timetable.OpenRoomActivityName,
		maxParticipants:     timetable.OpenRoomMaxParticipants,
		categoryName:        timetable.OpenRoomCategoryName,
		categoryDescription: timetable.OpenRoomCategoryDescription,
		categoryColor:       timetable.OpenRoomCategoryColor,
	}
}

// roomActivity returns the system activity of the room, provisioning it on
// first use. The lookup runs twice: once without a lock for the common case,
// and again under a tenant-wide lock, so concurrent first moves into different
// rooms cannot create the activity twice.
func (c *command) roomActivity(ctx context.Context, room facilities.Room) (int64, error) {
	spec := specFor(room)
	if id, found, err := c.findActivity(ctx, spec); err != nil || found {
		return id, err
	}
	key := fmt.Sprintf("open-room-activity:%d", c.deps.Runtime.TenantID(ctx))
	if err := c.deps.Runtime.AcquireLock(ctx, key); err != nil {
		return 0, fmt.Errorf("open room move: lock activity provisioning: %w", err)
	}
	if id, found, err := c.findActivity(ctx, spec); err != nil || found {
		return id, err
	}
	categoryID, err := c.systemCategory(ctx, spec)
	if err != nil {
		return 0, err
	}
	created, err := c.deps.Activities.CreateGroup(ctx, timetable.GroupInput{
		Name:            spec.name,
		MaxParticipants: spec.maxParticipants,
		IsOpen:          true,
		CategoryID:      categoryID,
		PlannedRoomID:   spec.plannedRoomID,
		IsSystem:        true,
	})
	if err != nil {
		return 0, fmt.Errorf("open room move: create %q activity: %w", spec.name, err)
	}
	return created.ID, nil
}

func (c *command) findActivity(ctx context.Context, spec activitySpec) (int64, bool, error) {
	isSystem := true
	groups, err := c.deps.Activities.ListGroups(ctx, timetable.GroupFilter{
		Name: spec.name, IsSystem: &isSystem, OrderByID: true,
	})
	if err != nil {
		return 0, false, fmt.Errorf("open room move: find %q activity: %w", spec.name, err)
	}
	for _, group := range groups {
		if group.Name != spec.name || !group.IsSystem || group.ArchivedAt != nil {
			continue
		}
		if !sameRoom(group.PlannedRoomID, spec.plannedRoomID) {
			continue
		}
		return group.ID, true, nil
	}
	return 0, false, nil
}

func (c *command) systemCategory(ctx context.Context, spec activitySpec) (int64, error) {
	categories, err := c.deps.Activities.ListCategories(ctx)
	if err != nil {
		return 0, fmt.Errorf("open room move: list activity categories: %w", err)
	}
	for _, category := range categories {
		if category.Name == spec.categoryName && category.IsSystem && !category.IsArchived() {
			return category.ID, nil
		}
	}
	created, err := c.deps.Activities.CreateCategory(ctx, timetable.CreateCategory{
		Name:        spec.categoryName,
		Description: spec.categoryDescription,
		Color:       spec.categoryColor,
		IsSystem:    true,
	})
	if err != nil {
		return 0, fmt.Errorf("open room move: create %q category: %w", spec.categoryName, err)
	}
	return created.ID, nil
}

func sameRoom(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func uniquePositive(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
