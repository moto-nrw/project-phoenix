// Package application holds the session end decision: what closes, in which
// order, inside one UnitOfWork, and what is announced afterwards.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/ports"
)

// Dependencies are the owner capabilities and runtime the command coordinates.
type Dependencies struct {
	Presence   ports.Presence
	Timetable  ports.Timetable
	Completion ports.InstanceCompletion
	Students   ports.Students
	Rooms      ports.Rooms
	Notifier   ports.Notifier
	Runtime    ports.Runtime
	Observe    func(ports.Observation)
}

type command struct {
	deps Dependencies
}

// NewCommand builds the session end command. Every dependency is required;
// a missing one is a composition error, not a runtime fallback.
func NewCommand(deps Dependencies) sessionend.Command {
	if deps.Presence == nil || deps.Timetable == nil || deps.Completion == nil || deps.Students == nil ||
		deps.Rooms == nil || deps.Notifier == nil || deps.Observe == nil ||
		deps.Runtime.TenantID == nil || deps.Runtime.WithinTenant == nil || deps.Runtime.AfterCommit == nil || deps.Runtime.Now == nil {
		panic("session end: all dependencies are required")
	}
	return &command{deps: deps}
}

// EndSession closes the session. Order inside the unit:
//
//  1. lock the group (Presence) and reject a missing or already ended one;
//  2. close open visits and supervisions, end the group (Presence);
//  3. when the session was mirrored into the timetable: stamp the slot
//     check-outs of the children just sent home and finalize the instance
//     (Timetable), so the block never stays "running" without its session;
//  4. gather the announcement data while the tenant role is still set;
//  5. queue the announcements for after the commit.
//
// A failure anywhere returns before anything is announced; the transaction
// owner rolls the writes back.
func (c *command) EndSession(ctx context.Context, activeGroupID int64) (sessionend.Result, error) {
	started := time.Now()
	var result sessionend.Result
	var note ports.Notification
	err := c.deps.Runtime.WithinTenant(ctx, func(txCtx context.Context) error {
		if activeGroupID <= 0 {
			return fmt.Errorf("session end: invalid active group ID %d", activeGroupID)
		}
		group, err := c.deps.Presence.LockGroup(txCtx, activeGroupID)
		if err != nil {
			return presenceError(err)
		}
		if !group.IsOpen() {
			return sessionend.ErrSessionAlreadyEnded
		}
		now := c.deps.Runtime.Now()

		ended, err := c.deps.Presence.EndGroupSession(txCtx, activeGroupID, now)
		if err != nil {
			return presenceError(err)
		}

		completed, err := c.completeMirroredInstance(txCtx, activeGroupID, now)
		if err != nil {
			return err
		}

		students, err := c.endedStudents(txCtx, ended.ClosedVisits)
		if err != nil {
			return err
		}
		activityName, err := c.activityName(txCtx, group.ActivityGroupID)
		if err != nil {
			return err
		}
		roomName, err := c.roomName(txCtx, group.RoomID)
		if err != nil {
			return err
		}

		note = ports.Notification{
			TenantID:      c.deps.Runtime.TenantID(txCtx),
			ActiveGroupID: activeGroupID,
			Students:      students,
			RoomID:        group.RoomID,
			ActivityName:  activityName,
			RoomName:      roomName,
			Instance:      completed,
		}
		result = sessionend.Result{
			ActiveGroupID:      activeGroupID,
			EndedAt:            now,
			StudentsCheckedOut: len(ended.ClosedVisits),
			SupervisorsEnded:   len(ended.EndedSupervisorIDs),
		}
		if completed != nil {
			id := completed.ID
			result.MirroredInstanceID = &id
		}
		notifier := c.deps.Notifier
		c.deps.Runtime.AfterCommit(txCtx, func() { notifier.SessionEnded(note) })
		return nil
	})
	c.deps.Observe(ports.Observation{
		Operation:          "end_session",
		Duration:           time.Since(started),
		Err:                err,
		StudentsCheckedOut: result.StudentsCheckedOut,
		SupervisorsEnded:   result.SupervisorsEnded,
		InstanceCompleted:  result.MirroredInstanceID != nil,
	})
	if err != nil {
		return sessionend.Result{}, err
	}
	return result, nil
}

func presenceError(err error) error {
	switch {
	case errors.Is(err, studentpresence.ErrGroupNotFound):
		return sessionend.ErrSessionNotFound
	case errors.Is(err, studentpresence.ErrGroupEnded):
		return sessionend.ErrSessionAlreadyEnded
	}
	return fmt.Errorf("session end: presence: %w", err)
}

// completeMirroredInstance closes the timetable side of a mirrored session.
// A session nobody mirrored (started before mirroring existed, or by a path
// that does not mirror) has nothing to complete. An instance another path
// already completed is left alone and not re-announced.
func (c *command) completeMirroredInstance(ctx context.Context, activeGroupID int64, now time.Time) (*ports.CompletedInstance, error) {
	instances, err := c.deps.Timetable.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{ActiveGroupID: &activeGroupID})
	if err != nil {
		return nil, fmt.Errorf("session end: find mirrored instance: %w", err)
	}
	if len(instances) == 0 {
		return nil, nil
	}
	if _, err := c.deps.Timetable.CloseOpenCheckoutsByActiveGroupIDs(ctx, []int64{activeGroupID}, now); err != nil {
		return nil, fmt.Errorf("session end: close slot check-outs: %w", err)
	}
	changed, err := c.deps.Completion.CompleteActiveByActiveGroupIDs(ctx, []int64{activeGroupID}, now)
	if err != nil {
		return nil, fmt.Errorf("session end: complete mirrored instance %d: %w", instances[0].ID, err)
	}
	if changed == 0 {
		return nil, nil
	}
	instance := instances[0]
	return &ports.CompletedInstance{ID: instance.ID, Date: instance.Date, StartTime: instance.StartTime, RoomID: instance.RoomID}, nil
}

func (c *command) endedStudents(ctx context.Context, visits []studentpresence.Visit) ([]ports.EndedStudent, error) {
	if len(visits) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(visits))
	seen := make(map[int64]bool, len(visits))
	for _, visit := range visits {
		if !seen[visit.StudentID] {
			seen[visit.StudentID] = true
			ids = append(ids, visit.StudentID)
		}
	}
	rows, err := c.deps.Students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("session end: resolve students: %w", err)
	}
	groups := make(map[int64]*int64, len(rows))
	for _, row := range rows {
		groups[row.ID] = row.GroupID
	}
	result := make([]ports.EndedStudent, 0, len(ids))
	for _, id := range ids {
		result = append(result, ports.EndedStudent{StudentID: id, EducationGroupID: groups[id]})
	}
	return result, nil
}

// activityName is empty for a spontaneous session without a template and for
// a template that no longer exists; any other read failure aborts the close,
// because a failed statement has already poisoned the transaction.
func (c *command) activityName(ctx context.Context, activityGroupID *int64) (string, error) {
	if activityGroupID == nil || *activityGroupID <= 0 {
		return "", nil
	}
	group, err := c.deps.Timetable.FindGroup(ctx, *activityGroupID)
	if errors.Is(err, timetable.ErrGroupNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("session end: resolve activity name: %w", err)
	}
	return group.Name, nil
}

func (c *command) roomName(ctx context.Context, roomID int64) (string, error) {
	if roomID <= 0 {
		return "", nil
	}
	room, err := c.deps.Rooms.FindRoom(ctx, roomID)
	if errors.Is(err, facilities.ErrRoomNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("session end: resolve room name: %w", err)
	}
	return room.Name, nil
}
