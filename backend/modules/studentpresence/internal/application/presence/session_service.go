package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CheckActivityConflict reports whether a kiosk start of activityID would
// conflict. An independent room stay of that activity in the destination room
// is joinable, matching StartActivitySessionWithSupervisors, so the preflight
// does not push the tablet into a force-start that would end the stay.
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Check if device is already running another session
	existingDeviceSession, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	if existingDeviceSession != nil {
		deviceIDStr := fmt.Sprintf("%d", deviceID)
		return &ActivityConflictInfo{
			HasConflict:       true,
			ConflictingGroup:  existingDeviceSession,
			ConflictMessage:   fmt.Sprintf("Device %d is already running another session", deviceID),
			ConflictingDevice: &deviceIDStr,
			CanOverride:       true, // Administrative override is always possible
		}, nil
	}

	existingActivitySessions, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	activityIsSystem, err := s.systemActivity(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}
	plannedRoomID, err := s.getPlannedRoomID(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}
	destinationRoomID := activityConflictDestination(existingActivitySessions, plannedRoomID)
	if !activityRunningElsewhere(existingActivitySessions, destinationRoomID, deviceID, activityIsSystem) {
		return &ActivityConflictInfo{HasConflict: false, CanOverride: true}, nil
	}

	existingSession := existingActivitySessions[0]
	var conflictDeviceStr *string
	if existingSession != nil && existingSession.DeviceID != nil {
		deviceIDStr := fmt.Sprintf("%d", *existingSession.DeviceID)
		conflictDeviceStr = &deviceIDStr
	}
	var devicePtr *int64
	if existingSession != nil {
		devicePtr = existingSession.DeviceID
	}
	return &ActivityConflictInfo{
		HasConflict:       true,
		ConflictingGroup:  existingSession,
		ConflictMessage:   fmt.Sprintf("Activity is already active on device %s", getDeviceIDString(devicePtr)),
		ConflictingDevice: conflictDeviceStr,
		CanOverride:       true, // Administrative override is always possible
	}, nil
}

// getDeviceIDString returns a string representation of device ID or "unknown" if nil
func getDeviceIDString(deviceID *int64) string {
	if deviceID == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *deviceID)
}

// EndActivitySession ends an active activity session
func (s *service) EndActivitySession(ctx context.Context, activeGroupID int64) error {
	var broadcasts sessionEndSSEData
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		group, err := s.GroupRepo.FindByIDForUpdate(txCtx, activeGroupID)
		if err != nil || group == nil {
			return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupNotFound}
		}
		if !group.IsActive() {
			return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupAlreadyEnded}
		}
		broadcasts, err = s.endActivitySessionLocked(txCtx, group)
		return err
	})
	if err != nil {
		return err
	}
	s.queueActivitySessionEndBroadcasts(ctx, activeGroupID, broadcasts)
	return nil
}

// sessionEndSSEData is everything the session-end SSE events need, read
// inside the ending transaction. The after-commit hook that emits the events
// runs without a tenant transaction, on the pool connection as phoenix_auth,
// which has no rights until SET ROLE. So it must not touch the database
// (#2951).
type sessionEndSSEData struct {
	Visits []visitSSEData
	End    activityEndSSEData
}

// activityEndSSEData is the activity_end event payload.
type activityEndSSEData struct {
	RoomID       int64
	ActivityName string
	RoomName     string
}

// collectActivityEndSSE resolves the activity_end payload for group. It only
// queries when there is a broadcaster to deliver the event to. Without the
// name repositories (the shape unit tests build) the event carries empty
// names, as broadcastActivityEndEvent always did on a failed lookup.
func (s *service) collectActivityEndSSE(ctx context.Context, group *active.Group) (activityEndSSEData, error) {
	if s.Broadcaster == nil || s.RoomRepo == nil || s.ActivityGroupRepo == nil {
		return activityEndSSEData{RoomID: group.RoomID}, nil
	}

	activityName, err := s.getActivityEndActivityName(ctx, group.GroupID)
	if err != nil {
		return activityEndSSEData{}, err
	}
	roomName, err := s.getActivityEndRoomName(ctx, group.RoomID)
	if err != nil {
		return activityEndSSEData{}, err
	}
	return activityEndSSEData{RoomID: group.RoomID, ActivityName: activityName, RoomName: roomName}, nil
}

func (s *service) endActivitySessionLocked(ctx context.Context, group *active.Group) (sessionEndSSEData, error) {
	activeGroupID := group.ID
	// Collect active visits before mutating them for the SSE payloads.
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, activeGroupID)
	if err != nil {
		return sessionEndSSEData{}, &ActiveError{Op: "EndActivitySession", Err: ErrDatabaseOperation}
	}

	if _, err := s.closeGroupSession(ctx, "EndActivitySession", activeGroupID); err != nil {
		return sessionEndSSEData{}, err
	}
	endData, err := s.collectActivityEndSSE(ctx, group)
	if err != nil {
		return sessionEndSSEData{}, &ActiveError{Op: "EndActivitySession", Err: err}
	}
	return sessionEndSSEData{Visits: visitsToNotify, End: endData}, nil
}

// closeGroupSession is the one presence write behind every session end in
// this service: the Student Presence owner closes the group's open visits,
// supervisions, and the group in one command (#2697). The slot check-outs of
// the closed visits are then mirrored into the timetable in one statement,
// and the guardians of every checked-out child are woken after the commit.
// The caller holds the group lock and has verified the group is open.
func (s *service) closeGroupSession(ctx context.Context, op string, activeGroupID int64) (studentpresence.EndedGroupSession, error) {
	now := s.now()
	ended, err := s.SchoolPresence.EndGroupSession(ctx, activeGroupID, now)
	switch {
	case errors.Is(err, studentpresence.ErrGroupNotFound):
		return ended, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
	case errors.Is(err, studentpresence.ErrGroupEnded):
		return ended, &ActiveError{Op: op, Err: ErrActiveGroupAlreadyEnded}
	case err != nil:
		return ended, &ActiveError{Op: op, Err: err}
	}
	if s.AttendanceSyncer != nil && len(ended.ClosedVisits) > 0 {
		visits := make([]*studentpresence.Visit, 0, len(ended.ClosedVisits))
		for i := range ended.ClosedVisits {
			visits = append(visits, &ended.ClosedVisits[i])
		}
		if err := s.AttendanceSyncer.MirrorCheckOutForVisits(ctx, visits, now); err != nil {
			return ended, &ActiveError{Op: op, Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}
	for _, visit := range ended.ClosedVisits {
		s.wakeGuardiansAfterCommit(ctx, visit.StudentID)
	}
	return ended, nil
}

// queueActivitySessionEndBroadcasts emits the session-end SSE events once the
// surrounding transaction has committed. The hook performs no database access:
// every lookup lives in data, gathered inside the transaction (#2951).
func (s *service) queueActivitySessionEndBroadcasts(ctx context.Context, activeGroupID int64, data sessionEndSSEData) {
	if s.Broadcaster == nil {
		return
	}
	broadcastCtx := tenant.ContextWithoutTransaction(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		s.broadcastStudentCheckoutEvents(broadcastCtx, activeGroupIDStr, data.Visits)
		s.broadcastActivityEndEvent(broadcastCtx, activeGroupIDStr, data.End)
	})
}

// GetDeviceCurrentSession gets the current active session for a device
func (s *service) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error) {
	session, err := s.GroupRepo.FindActiveByDeviceIDWithNames(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: err}
	}

	if session == nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: ErrNoActiveSession}
	}

	return session, nil
}

// runInSessionTx runs fn inside a transaction, joining the caller's when one is
// already in context. Without a database handle — the shape unit tests build,
// where every repository is a double and there is nothing to commit — fn runs
// directly; production wiring always supplies one via services.NewFactory.
func (s *service) runInSessionTx(ctx context.Context, fn func(context.Context) error) error {
	if s.DB == nil {
		return fn(ctx)
	}
	if _, hasTransaction := tenant.TransactionFromContext(ctx); !hasTransaction && s.tenantRuntime != nil {
		ctx = tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
	}
	return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return fn(txCtx)
	})
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *service) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	err := s.GroupRepo.UpdateLastActivity(ctx, activeGroupID, time.Now())
	if err == nil {
		return nil
	}
	if isUpdateLastActivitySessionMiss(err) {
		session, findErr := s.GroupRepo.FindByID(ctx, activeGroupID)
		if missErr := sessionActivityMissError(session, findErr); missErr != nil {
			return missErr
		}
	}
	return &ActiveError{Op: "UpdateSessionActivity", Err: err}
}

// sessionActivityMissError explains why no running session was touched: the
// session is gone or has ended. It returns nil for a session that still runs.
func sessionActivityMissError(session *active.Group, findErr error) error {
	if findErr != nil {
		if isFindByIDNoRows(findErr) {
			return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "UpdateSessionActivity", Err: findErr}
	}
	if session == nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
	}
	if !session.IsActive() {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupAlreadyEnded}
	}
	return nil
}

func isUpdateLastActivitySessionMiss(err error) bool {
	if errors.Is(err, studentpresence.ErrGroupNotOpen) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	return errors.As(err, &dbErr) && dbErr.Op == "update last activity - session not found"
}

func isFindByIDNoRows(err error) bool {
	var dbErr *modelBase.DatabaseError
	return errors.As(err, &dbErr) && dbErr.Op == "find by id" && modelBase.IsNoRows(dbErr.Err)
}
