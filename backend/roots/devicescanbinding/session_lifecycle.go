package devicescanbinding

import (
	"context"
	"errors"
	"strconv"

	scanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type nativeSupervisionQuery interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func NewSupervisionQuery(source nativeSupervisionQuery) devicescan.SupervisionQuery {
	return supervisionQuery{source: source}
}

type supervisionQuery struct{ source nativeSupervisionQuery }

func (q supervisionQuery) ActiveSupervisions(ctx context.Context, sessionID int64) ([]devicescan.Supervision, error) {
	today := calendar.TodayDate()
	day := today.String()
	rows, err := q.source.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{sessionID}, ActiveOn: &day})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "FindSupervisorsByActiveGroupID", Err: studentpresence.ErrDatabaseOperation}
	}
	result := make([]devicescan.Supervision, 0, len(rows))
	for _, row := range rows {
		result = append(result, devicescan.Supervision{StaffID: row.StaffID, Role: row.Role, Ended: supervisionEnded(row.EndDate, today.String())})
	}
	return result, nil
}

type lifecycleSupervisionQuery interface {
	ActiveSupervisions(context.Context, int64) ([]devicescan.Supervision, error)
}

// OpenVisitCounts is the Student Presence read of a session count with its
// visit revision (#5128).
type OpenVisitCounts interface {
	CountOpenVisitsInGroupWithRevision(context.Context, int64) (int, int64, error)
}

// lifecycleStore promotes CountOpenVisitsInGroupWithRevision unchanged; the
// other lifecycle steps go through active.
type lifecycleStore struct {
	OpenVisitCounts
	active   studentpresence.Presence
	presence lifecycleSupervisionQuery
}

func lifecycleSession(group *studentpresence.SessionDetail) ports.LifecycleSession {
	if group == nil {
		return ports.LifecycleSession{}
	}
	result := ports.LifecycleSession{ID: group.ID, RoomID: group.RoomID, ActivityID: group.ActivityGroupID, StartTime: group.StartTime, Supervisors: lifecycleSupervisors(group.Supervisors)}
	if group.Activity != nil {
		result.ActivityName = &group.Activity.Name
	}
	if group.Room != nil {
		result.RoomName = &group.Room.Name
	}
	return result
}

func lifecycleSupervisors(rows []studentpresence.GroupSupervision) []ports.LifecycleSupervisor {
	today := calendar.TodayDate().String()
	result := make([]ports.LifecycleSupervisor, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.LifecycleSupervisor{StaffID: row.StaffID, Role: row.Role, Ended: supervisionEnded(row.EndDate, today)})
	}
	return result
}

// supervisionEnded matches ActiveOn: a row is over only when end_date <= today.
// A planned future end_date is still supervising and must stay on the kiosk roster.
func supervisionEnded(endDate *string, today string) bool {
	return scanCompose.SupervisionEnded(endDate, today)
}

func (s lifecycleStore) Start(ctx context.Context, deviceID int64, command devicescan.StartSessionCommand) (ports.LifecycleSession, error) {
	var group *studentpresence.SessionDetail
	var err error
	if command.Force {
		group, err = s.active.ForceStartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	} else {
		group, err = s.active.StartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	}
	if errors.Is(err, studentpresence.ErrSessionConflict) || errors.Is(err, studentpresence.ErrDeviceAlreadyActive) {
		return ports.LifecycleSession{}, &ports.SessionStartConflict{Cause: classifyPresence(err)}
	}
	return lifecycleSession(group), classifyPresence(err)
}

func (s lifecycleStore) Current(ctx context.Context, deviceID int64) (*ports.LifecycleSession, error) {
	group, err := s.active.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		classified := classifyPresence(err)
		if errors.Is(err, studentpresence.ErrNoActiveSession) {
			classified = mapError(classified, ports.ErrLifecycleSessionAbsent)
		}
		return nil, classified
	}
	if group == nil {
		return nil, nil
	}
	result := lifecycleSession(group)
	return &result, nil
}

func (s lifecycleStore) Conflict(ctx context.Context, activityID, deviceID int64) (devicescan.ConflictInfoResponse, error) {
	info, err := s.active.CheckActivityConflict(ctx, activityID, deviceID)
	if err != nil {
		return devicescan.ConflictInfoResponse{}, classifyPresence(err)
	}
	result := devicescan.ConflictInfoResponse{HasConflict: info.HasConflict, ConflictMessage: info.ConflictMessage, CanOverride: info.CanOverride}
	if info.ConflictingDevice != nil {
		if id, parseErr := strconv.ParseInt(*info.ConflictingDevice, 10, 64); parseErr == nil {
			result.ConflictingDevice = &id
		}
	}
	return result, nil
}

func (s lifecycleStore) Supervisors(ctx context.Context, sessionID int64) ([]ports.LifecycleSupervisor, error) {
	return scanCompose.LifecycleSupervisorsBinding(ctx, s.presence, sessionID)
}

func (s lifecycleStore) ReplaceSupervisors(ctx context.Context, sessionID int64, ids []int64) (ports.LifecycleSession, error) {
	group, err := s.active.UpdateActiveGroupSupervisors(ctx, sessionID, ids)
	return lifecycleSession(group), classifyPresence(err)
}

func (s lifecycleStore) Touch(ctx context.Context, sessionID int64) error {
	return classifyPresence(s.active.UpdateSessionActivity(ctx, sessionID))
}

func (s lifecycleStore) Timeout(ctx context.Context, deviceID int64) (devicescan.SessionTimeoutResponse, error) {
	result, err := s.active.ProcessSessionTimeout(ctx, deviceID)
	if err != nil {
		classified := classifyPresence(err)
		if errors.Is(err, studentpresence.ErrNoActiveSession) {
			classified = mapError(classified, ports.ErrNoActiveSession)
		}
		return devicescan.SessionTimeoutResponse{}, classified
	}
	return devicescan.SessionTimeoutResponse{SessionID: result.SessionID, ActivityID: result.ActivityID, StudentsCheckedOut: result.StudentsCheckedOut, TimeoutAt: result.TimeoutAt}, nil
}

func (s lifecycleStore) ValidateTimeout(ctx context.Context, deviceID int64, minutes int) error {
	return classifyPresence(s.active.ValidateSessionTimeout(ctx, deviceID, minutes))
}

func (s lifecycleStore) TimeoutInfo(ctx context.Context, deviceID int64) (devicescan.SessionTimeoutInfoResponse, error) {
	info, err := s.active.GetSessionTimeoutInfo(ctx, deviceID)
	if err != nil {
		return devicescan.SessionTimeoutInfoResponse{}, classifyPresence(err)
	}
	return devicescan.SessionTimeoutInfoResponse{SessionID: info.SessionID, ActivityID: info.ActivityID, StartTime: info.StartTime, LastActivity: info.LastActivity, TimeoutMinutes: info.TimeoutMinutes, InactivitySeconds: int(info.InactivityDuration.Seconds()), TimeUntilTimeoutSeconds: int(info.TimeUntilTimeout.Seconds()), IsTimedOut: info.IsTimedOut, ActiveStudentCount: info.ActiveStudentCount}, nil
}
