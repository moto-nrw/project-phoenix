package compose

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type SessionLifecycle = devicescan.SessionLifecycle

// NewSessionLifecycle binds manual kiosk controls to the retained presence
// and people services without constructing a legacy service graph.
func NewSessionLifecycle(active studentpresence.Presence, presence SupervisionQuery, people PersonDirectory, heartbeat application.SessionHeartbeat, mirror application.SessionMirror, logger *slog.Logger, principal devicescan.DevicePrincipal) SessionLifecycle {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	return application.NewSessionLifecycle(lifecycleStore{active: active, presence: presence}, lifecyclePeople{people: people}, principal, heartbeat, mirror, clock{now: time.Now}, logger)
}

type SupervisionQuery interface{ devicescan.SupervisionQuery }

type nativeSupervisionQuery interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func NewSupervisionQuery(source nativeSupervisionQuery) SupervisionQuery {
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

type lifecycleStore struct {
	active   studentpresence.Presence
	presence SupervisionQuery
}

func lifecycleSession(group *studentpresence.SessionDetail) application.LifecycleSession {
	if group == nil {
		return application.LifecycleSession{}
	}
	result := application.LifecycleSession{ID: group.ID, RoomID: group.RoomID, ActivityID: group.ActivityGroupID, StartTime: group.StartTime, Supervisors: lifecycleSupervisors(group.Supervisors)}
	if group.Activity != nil {
		result.ActivityName = &group.Activity.Name
	}
	if group.Room != nil {
		result.RoomName = &group.Room.Name
	}
	return result
}

func lifecycleSupervisors(rows []studentpresence.GroupSupervision) []application.LifecycleSupervisor {
	today := calendar.TodayDate().String()
	result := make([]application.LifecycleSupervisor, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.LifecycleSupervisor{StaffID: row.StaffID, Role: row.Role, Ended: supervisionEnded(row.EndDate, today)})
	}
	return result
}

// supervisionEnded matches ActiveOn: a row is over only when end_date <= today.
// A planned future end_date is still supervising and must stay on the kiosk roster.
func supervisionEnded(endDate *string, today string) bool {
	return application.SupervisionEnded(endDate, today)
}

func (s lifecycleStore) Start(ctx context.Context, deviceID int64, command devicescan.StartSessionCommand) (application.LifecycleSession, error) {
	var group *studentpresence.SessionDetail
	var err error
	if command.Force {
		group, err = s.active.ForceStartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	} else {
		group, err = s.active.StartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	}
	if errors.Is(err, studentpresence.ErrSessionConflict) || errors.Is(err, studentpresence.ErrDeviceAlreadyActive) {
		return application.LifecycleSession{}, &application.SessionStartConflict{Cause: classifyPresence(err)}
	}
	return lifecycleSession(group), classifyPresence(err)
}

func (s lifecycleStore) Current(ctx context.Context, deviceID int64) (*application.LifecycleSession, error) {
	group, err := s.active.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		classified := classifyPresence(err)
		if errors.Is(err, studentpresence.ErrNoActiveSession) {
			classified = mapError(classified, application.ErrLifecycleSessionAbsent)
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

func (s lifecycleStore) Supervisors(ctx context.Context, sessionID int64) ([]application.LifecycleSupervisor, error) {
	return application.LifecycleSupervisorsBinding(ctx, s.presence, sessionID)
}

func (s lifecycleStore) ReplaceSupervisors(ctx context.Context, sessionID int64, ids []int64) (application.LifecycleSession, error) {
	group, err := s.active.UpdateActiveGroupSupervisors(ctx, sessionID, ids)
	return lifecycleSession(group), classifyPresence(err)
}

func (s lifecycleStore) CountStudents(ctx context.Context, sessionID int64) (int, error) {
	return s.active.CountActiveVisitsByActiveGroupID(ctx, sessionID)
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

type lifecyclePeople struct{ people PersonDirectory }

func (p lifecyclePeople) StaffNames(ctx context.Context, ids []int64) (map[int64]ports.Person, error) {
	return application.StaffNamesBinding(ctx, p.people, ids)
}
