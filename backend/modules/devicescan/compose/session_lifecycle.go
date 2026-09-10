package compose

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

type SessionLifecycle = devicescan.SessionLifecycle

// NewSessionLifecycle binds manual kiosk controls to the retained presence
// and people services without constructing a legacy service graph.
func NewSessionLifecycle(active activeSvc.Service, people usersSvc.PersonService, heartbeat application.SessionHeartbeat, mirror application.SessionMirror, logger *slog.Logger) SessionLifecycle {
	return application.NewSessionLifecycle(lifecycleStore{active: active}, lifecyclePeople{people}, principals{}, heartbeat, mirror, clock{now: time.Now}, logger)
}

type lifecycleStore struct{ active activeSvc.Service }

func lifecycleSession(group *active.Group) application.LifecycleSession {
	if group == nil {
		return application.LifecycleSession{}
	}
	result := application.LifecycleSession{ID: group.ID, RoomID: group.RoomID, ActivityID: group.GroupID, StartTime: group.StartTime, Supervisors: lifecycleSupervisors(group.Supervisors)}
	if group.ActualGroup != nil {
		result.ActivityName = &group.ActualGroup.Name
	}
	if group.Room != nil {
		result.RoomName = &group.Room.Name
	}
	return result
}

func lifecycleSupervisors(rows []*active.GroupSupervisor) []application.LifecycleSupervisor {
	result := make([]application.LifecycleSupervisor, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, application.LifecycleSupervisor{StaffID: row.StaffID, Role: row.Role, Ended: row.EndDate != nil})
		}
	}
	return result
}

func (s lifecycleStore) Start(ctx context.Context, deviceID int64, command devicescan.StartSessionCommand) (application.LifecycleSession, error) {
	var group *active.Group
	var err error
	if command.Force {
		group, err = s.active.ForceStartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	} else {
		group, err = s.active.StartActivitySessionWithSupervisors(ctx, command.ActivityID, deviceID, command.SupervisorIDs, command.RoomID)
	}
	if errors.Is(err, activeSvc.ErrSessionConflict) || errors.Is(err, activeSvc.ErrDeviceAlreadyActive) {
		return application.LifecycleSession{}, &application.SessionStartConflict{Cause: classifyPresence(err)}
	}
	return lifecycleSession(group), classifyPresence(err)
}

func (s lifecycleStore) Current(ctx context.Context, deviceID int64) (*application.LifecycleSession, error) {
	group, err := s.active.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		classified := classifyPresence(err)
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
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
	rows, err := s.active.FindSupervisorsByActiveGroupID(ctx, sessionID)
	return lifecycleSupervisors(rows), err
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
		return devicescan.SessionTimeoutResponse{}, classifyPresence(err)
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

type lifecyclePeople struct{ people usersSvc.PersonService }

func (p lifecyclePeople) StaffNames(ctx context.Context, ids []int64) (map[int64]ports.Person, error) {
	staff, err := p.people.GetStaffWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]ports.Person, len(staff))
	for id, row := range staff {
		if row != nil && row.Person != nil {
			result[id] = ports.Person{ID: row.Person.ID, FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		}
	}
	return result, nil
}
