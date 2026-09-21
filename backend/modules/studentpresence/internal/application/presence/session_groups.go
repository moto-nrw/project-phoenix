package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrDatabaseOperation}
	}
	if group == nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Ensure we always have room metadata so downstream callers
	// (location resolver, SSE payloads) can render friendly labels.
	if group != nil && group.Room == nil && group.RoomID > 0 {
		if room, roomErr := s.RoomRepo.FindByID(ctx, group.RoomID); roomErr == nil {
			group.Room = room
		}
	}

	return group, nil
}

func (s *service) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	if len(groupIDs) == 0 {
		return map[int64]*active.Group{}, nil
	}

	groups, err := s.GroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupsByIDs", Err: ErrDatabaseOperation}
	}

	if groups == nil {
		groups = make(map[int64]*active.Group)
	}

	return groups, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.createActiveGroupLocked(txCtx, group)
	})
}

func (s *service) createActiveGroupLocked(ctx context.Context, group *active.Group) error {
	// Check for room conflicts if room is assigned
	if group.RoomID > 0 {
		if err := s.SchoolPresence.LockRoomSessionWrites(ctx, group.RoomID); err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrDatabaseOperation}
		}
		hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
	}

	group.SetTenantID(tenant.FromContext(ctx))
	if err := s.GroupRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("create failed: %w", err)}
	}

	return nil
}

func (s *service) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrInvalidData}
	}
	return s.runInSessionTx(ctx, func(txCtx context.Context) error {
		return s.updateActiveGroupLocked(txCtx, group)
	})
}

func (s *service) updateActiveGroupLocked(ctx context.Context, group *active.Group) error {
	if group.RoomID > 0 {
		if err := s.SchoolPresence.LockRoomSessionWrites(ctx, group.RoomID); err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrDatabaseOperation}
		}
	}
	existing, err := s.GroupRepo.FindByIDForUpdate(ctx, group.ID)
	if err != nil {
		if base.IsNoRows(err) {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrDatabaseOperation}
	}
	if existing == nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Check for room conflicts if room is assigned (exclude current group)
	if group.RoomID > 0 {
		if err := s.checkActiveGroupRoomChange(ctx, existing, group); err != nil {
			return err
		}
	}

	if err := s.GroupRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("update failed: %w", err)}
	}

	return nil
}

// checkActiveGroupRoomChange rejects a room another session occupies and,
// when the session changes rooms, a room without space for its children.
func (s *service) checkActiveGroupRoomChange(ctx context.Context, existing, group *active.Group) error {
	hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
	if err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
	}
	if hasConflict {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
	}
	if existing.RoomID == group.RoomID {
		return nil
	}
	incoming, err := s.SchoolPresence.CountOpenVisitsInGroup(ctx, group.ID)
	if err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrDatabaseOperation}
	}
	return s.ensureRoomCapacity(ctx, group.RoomID, incoming)
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{id}})
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find visits: %w", err)}
	}

	// Check if any of the visits are still active
	for _, visit := range visits {
		if visit.ExitTime == nil {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrCannotDeleteActiveGroup}
		}
	}

	// Delete the active group
	_, err = s.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find group: %w", err)}
	}

	if err := s.GroupRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("delete failed: %w", err)}
	}

	return nil
}

// EnsureOpenRoomSession returns the room session of a released room: the open
// session of the room's system activity. Independent room stays (#3066) live
// in it, apart from any other activity session in the same room, so ending
// that activity leaves them untouched.
//
// The canonical Schulhof keeps Schulhof Freispiel, so a phone move joins the
// running kiosk session when that activity is already open on a device. A
// device-less stay is reused the same way, so a later kiosk start can attach
// to it instead of opening a second group.
//
// It deliberately skips CreateActiveGroup's one-session-per-room conflict: a
// room session coexists with activity sessions by design. The room's session
// write lock serializes concurrent first moves, so the room never gets a
// second room session; the lock is held until the caller's transaction ends.
func (s *service) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*active.Group, error) {
	const op = "EnsureOpenRoomSession"
	if roomID <= 0 || activityID <= 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}
	var session *active.Group
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.SchoolPresence.LockRoomSessionWrites(txCtx, roomID); err != nil {
			return &ActiveError{Op: op, Err: ErrDatabaseOperation}
		}
		groups, err := s.GroupRepo.FindActiveByRoomID(txCtx, roomID)
		if err != nil {
			return &ActiveError{Op: op, Err: fmt.Errorf("find room sessions: %w", err)}
		}
		if existing := openSessionForActivity(groups, activityID); existing != nil {
			session = existing
			return nil
		}
		now := time.Now()
		created := &active.Group{StartTime: now, LastActivity: now, GroupID: &activityID, RoomID: roomID}
		created.SetTenantID(tenant.FromContext(txCtx))
		if err := s.GroupRepo.Create(txCtx, created); err != nil {
			return &ActiveError{Op: op, Err: fmt.Errorf("create room session: %w", err)}
		}
		session = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// openSessionForActivity returns the open session of activityID in the given
// groups. A device-owned row (kiosk) wins over a device-less stay so phone
// moves join the running Schulhof journey.
func openSessionForActivity(groups []*active.Group, activityID int64) *active.Group {
	var deviceLess *active.Group
	for _, group := range groups {
		if group == nil {
			continue
		}
		templateID, ok := group.TemplateID()
		if !ok || templateID != activityID {
			continue
		}
		if group.DeviceID != nil {
			return group
		}
		if deviceLess == nil {
			deviceLess = group
		}
	}
	return deviceLess
}

// joinableOpenSession is the session a kiosk start may attach to: a running
// kiosk copy of the activity, or an independent room stay. Device-less
// planner and app sessions of the same activity are not joinable.
func joinableOpenSession(groups []*active.Group, activityID int64, activityIsSystem bool) *active.Group {
	existing := openSessionForActivity(groups, activityID)
	if existing == nil || existing.DeviceID != nil || existing.IsIndependentRoomSession(activityIsSystem) {
		return existing
	}
	return nil
}

func (s *service) systemActivity(ctx context.Context, activityID int64) (bool, error) {
	if s.ActivityGroupRepo == nil {
		return false, nil
	}
	activity, err := s.ActivityGroupRepo.FindByID(ctx, activityID)
	if err != nil {
		return false, err
	}
	return activity != nil && activity.IsSystem, nil
}

func (s *service) FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	group, err := s.GroupRepo.FindActiveByRoomIDAndDeviceID(ctx, roomID, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "FindDeviceActiveGroupInRoom", Err: fmt.Errorf("find by room and device: %w", err)}
	}
	return group, nil
}

func (s *service) EndActiveGroupSession(ctx context.Context, id int64) error {
	// Ending a group by hand (POST /active/groups/{id}/end, and the Schulhof
	// stale-session sweep) is a session end like any other, so it closes the
	// timetable side too (#1747 review). Skipping it leaves the linked instance
	// active with its expected rows unfinalized, and nothing repairs that
	// afterwards: the nightly bridge only looks at active.groups that are still
	// running, and this one is not.
	//
	// The state check runs BEFORE the bridge on purpose. EndActivitySession
	// rejects an already-ended group as ErrActiveGroupAlreadyEnded, which the
	// handler renders as 4xx — and the tenant middleware only rolls back on its
	// own for 5xx, so bridging first would commit a completed instance beside
	// that rejection.
	// Both writes are one transition (#1747 review). runInSessionTx joins the
	// caller's transaction when there is one (the HTTP path) and opens its own
	// when there is none — the Schulhof sweep calls this straight from the
	// scheduler, where a committed bridge write beside a failed session end
	// would leave a completed instance next to an open group that no later run
	// repairs.
	var broadcasts sessionEndSSEData
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		group, err := s.GroupRepo.FindByIDForUpdate(txCtx, id)
		if err != nil || group == nil {
			return &ActiveError{Op: "EndActiveGroupSession", Err: ErrActiveGroupNotFound}
		}
		if !group.IsActive() {
			return &ActiveError{Op: "EndActiveGroupSession", Err: ErrActiveGroupAlreadyEnded}
		}
		// Complete the failure-prone timetable half before mutating the active
		// session. Broadcasts are queued only after both halves succeed.
		if err := s.completeTimetableMirrorsForEndedSessions(txCtx, []int64{id}); err != nil {
			return &ActiveError{Op: "EndActiveGroupSession", Err: err}
		}

		broadcasts, err = s.endActivitySessionLocked(txCtx, group)
		if err != nil {
			// The bridge already completed the mirrored instance, so the
			// transaction has to go. Joining the request's transaction means
			// nothing here can roll it back, and the tenant middleware only
			// rolls back on its own for 5xx — ErrActiveGroupAlreadyEnded and
			// friends render as 4xx and would commit exactly that split state.
			tenant.MarkRollback(txCtx)
			// Wrap the error with our operation name for clarity
			if activeErr, ok := err.(*ActiveError); ok {
				return &ActiveError{Op: "EndActiveGroupSession", Err: activeErr.Err}
			}
			return &ActiveError{Op: "EndActiveGroupSession", Err: err}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.queueActivitySessionEndBroadcasts(ctx, id, broadcasts)
	return nil
}
