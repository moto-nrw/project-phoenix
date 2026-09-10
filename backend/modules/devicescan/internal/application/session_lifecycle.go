package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// LifecycleSession is the session projection needed by the manual kiosk controls.
type LifecycleSession struct {
	ID, RoomID             int64
	ActivityID             *int64
	StartTime              time.Time
	ActivityName, RoomName *string
	Supervisors            []LifecycleSupervisor
}

type LifecycleSupervisor struct {
	StaffID int64
	Role    string
	Ended   bool
}

// SessionStartConflict preserves the start outcomes that require a second
// conflict lookup. Its cause retains the ordinary fallback classification.
type SessionStartConflict struct{ Cause error }

func (e *SessionStartConflict) Error() string { return e.Cause.Error() }
func (e *SessionStartConflict) Unwrap() error { return e.Cause }

// ErrLifecycleSessionAbsent distinguishes the normal no-session state.
var ErrLifecycleSessionAbsent = errors.New("kiosk session absent")

// LifecycleStore is the consumer-owned session seam. Current wraps
// ErrLifecycleSessionAbsent when absent and preserves the service wire text.
type LifecycleStore interface {
	Start(context.Context, int64, devicescan.StartSessionCommand) (LifecycleSession, error)
	Current(context.Context, int64) (*LifecycleSession, error)
	Conflict(context.Context, int64, int64) (devicescan.ConflictInfoResponse, error)
	Supervisors(context.Context, int64) ([]LifecycleSupervisor, error)
	ReplaceSupervisors(context.Context, int64, []int64) (LifecycleSession, error)
	CountStudents(context.Context, int64) (int, error)
	Touch(context.Context, int64) error
	Timeout(context.Context, int64) (devicescan.SessionTimeoutResponse, error)
	ValidateTimeout(context.Context, int64, int) error
	TimeoutInfo(context.Context, int64) (devicescan.SessionTimeoutInfoResponse, error)
}

type LifecyclePeople interface {
	StaffNames(context.Context, []int64) (map[int64]ports.Person, error)
}

// SessionMirror owns the optional best-effort timetable mirror and its
// post-commit notification. A nil mirror leaves the session command unchanged.
type SessionMirror interface {
	MirrorSession(context.Context, LifecycleSession, []int64)
}

type SessionHeartbeat interface {
	PingDevice(context.Context, string) error
}

type sessionLifecycle struct {
	store      LifecycleStore
	people     LifecyclePeople
	principals ports.Principals
	heartbeat  SessionHeartbeat
	mirror     SessionMirror
	clock      ports.Clock
	logger     *slog.Logger
}

func NewSessionLifecycle(store LifecycleStore, people LifecyclePeople, principals ports.Principals, heartbeat SessionHeartbeat, mirror SessionMirror, clock ports.Clock, logger *slog.Logger) devicescan.SessionLifecycle {
	if logger == nil {
		logger = slog.Default()
	}
	return &sessionLifecycle{store: store, people: people, principals: principals, heartbeat: heartbeat, mirror: mirror, clock: clock, logger: logger}
}

func (s *sessionLifecycle) device(ctx context.Context) (*ports.Device, error) {
	device, ok := s.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	return device, nil
}

func (s *sessionLifecycle) StartSession(ctx context.Context, command devicescan.StartSessionCommand) (devicescan.SessionStartResponse, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.SessionStartResponse{}, err
	}
	if len(command.SupervisorIDs) == 0 {
		return devicescan.SessionStartResponse{}, devicescan.Internal("at least one supervisor ID is required", nil)
	}
	group, err := s.store.Start(ctx, device.ID, command)
	if err != nil {
		var conflict *SessionStartConflict
		if errors.As(err, &conflict) {
			info, lookupErr := s.store.Conflict(ctx, command.ActivityID, device.ID)
			if lookupErr == nil && info.HasConflict {
				return devicescan.SessionStartResponse{Status: "conflict", Message: info.ConflictMessage, ConflictInfo: &info}, nil
			}
		}
		return devicescan.SessionStartResponse{}, classifyPresenceFailure(err)
	}
	if s.mirror != nil {
		s.mirror.MirrorSession(ctx, group, command.SupervisorIDs)
	}
	response := devicescan.SessionStartResponse{ActiveGroupID: group.ID, ActivityID: group.ActivityID, DeviceID: device.ID, StartTime: group.StartTime, Status: "started", Message: "Activity session started successfully"}
	if supervisors, lookupErr := s.store.Supervisors(ctx, group.ID); lookupErr == nil && len(supervisors) > 0 {
		response.Supervisors = s.supervisorInfos(ctx, supervisors)
	}
	return response, nil
}

func (s *sessionLifecycle) supervisorInfos(ctx context.Context, supervisors []LifecycleSupervisor) []devicescan.SupervisorInfo {
	result := make([]devicescan.SupervisorInfo, 0, len(supervisors))
	if len(supervisors) == 0 {
		return result
	}
	ids := make([]int64, 0, len(supervisors))
	seen := make(map[int64]bool, len(supervisors))
	for _, supervisor := range supervisors {
		if !seen[supervisor.StaffID] {
			ids = append(ids, supervisor.StaffID)
			seen[supervisor.StaffID] = true
		}
	}
	people, err := s.people.StaffNames(ctx, ids)
	if err != nil {
		s.logger.WarnContext(ctx, "batch staff lookup failed, falling back to empty list", slog.String("error", err.Error()))
		return result
	}
	for _, supervisor := range supervisors {
		if person, ok := people[supervisor.StaffID]; ok {
			result = append(result, devicescan.SupervisorInfo{StaffID: supervisor.StaffID, FirstName: person.FirstName, LastName: person.LastName, DisplayName: person.FirstName + " " + person.LastName, Role: supervisor.Role})
		}
	}
	return result
}

func (s *sessionLifecycle) CurrentSession(ctx context.Context) (devicescan.SessionCurrentResponse, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.SessionCurrentResponse{}, err
	}
	if err := s.heartbeat.PingDevice(ctx, device.DeviceID); err != nil {
		s.logger.WarnContext(ctx, "failed to update device last seen",
			slog.String("device_id", device.DeviceID),
			slog.String("error", err.Error()),
		)
	}
	response := devicescan.SessionCurrentResponse{DeviceID: device.ID}
	group, err := s.store.Current(ctx, device.ID)
	if err != nil && !errors.Is(err, ErrLifecycleSessionAbsent) {
		return response, classifyPresenceFailure(err)
	}
	if group == nil {
		return response, nil
	}
	if err := s.store.Touch(ctx, group.ID); err != nil {
		s.logger.WarnContext(ctx, "failed to update session activity",
			slog.Int64("session_id", group.ID),
			slog.String("error", err.Error()),
		)
	}
	duration := s.clock.Now().Sub(group.StartTime).String()
	response.IsActive = true
	response.ActiveGroupID, response.ActivityID, response.RoomID = &group.ID, group.ActivityID, &group.RoomID
	response.StartTime, response.Duration = &group.StartTime, &duration
	response.ActivityName, response.RoomName = group.ActivityName, group.RoomName
	if count, countErr := s.store.CountStudents(ctx, group.ID); countErr == nil {
		response.ActiveStudents = &count
	} else {
		s.logger.WarnContext(ctx, "failed to get active student count",
			slog.Int64("session_id", group.ID),
			slog.String("error", countErr.Error()),
		)
	}
	if supervisors, lookupErr := s.store.Supervisors(ctx, group.ID); lookupErr == nil {
		if len(supervisors) > 0 {
			response.Supervisors = s.supervisorInfos(ctx, supervisors)
		}
	} else {
		s.logger.WarnContext(ctx, "failed to get supervisors",
			slog.Int64("session_id", group.ID),
			slog.String("error", lookupErr.Error()),
		)
	}
	return response, nil
}

func (s *sessionLifecycle) SessionToEnd(ctx context.Context) (devicescan.SessionToEnd, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.SessionToEnd{}, err
	}
	group, err := s.store.Current(ctx, device.ID)
	if err != nil && !errors.Is(err, ErrLifecycleSessionAbsent) {
		return devicescan.SessionToEnd{}, classifyPresenceFailure(err)
	}
	if group == nil {
		return devicescan.SessionToEnd{}, devicescan.InvalidRequest("no active session to end")
	}
	return devicescan.SessionToEnd{ID: group.ID, ActivityID: group.ActivityID, DeviceID: device.ID, StartTime: group.StartTime}, nil
}

func (s *sessionLifecycle) ReplaceSessionSupervisors(ctx context.Context, sessionID int64, ids []int64) (devicescan.UpdateSupervisorsResponse, error) {
	if _, err := s.device(ctx); err != nil {
		return devicescan.UpdateSupervisorsResponse{}, err
	}
	group, err := s.store.ReplaceSupervisors(ctx, sessionID, ids)
	if err != nil {
		return devicescan.UpdateSupervisorsResponse{}, classifyPresenceFailure(err)
	}
	active := make([]LifecycleSupervisor, 0, len(group.Supervisors))
	for _, supervisor := range group.Supervisors {
		if !supervisor.Ended && supervisor.StaffID > 0 {
			active = append(active, supervisor)
		}
	}
	return devicescan.UpdateSupervisorsResponse{ActiveGroupID: group.ID, Supervisors: s.supervisorInfos(ctx, active), Status: "success", Message: "Supervisors updated successfully"}, nil
}

func (s *sessionLifecycle) CheckSessionConflict(ctx context.Context, activityID int64) (devicescan.ConflictInfoResponse, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.ConflictInfoResponse{}, err
	}
	info, err := s.store.Conflict(ctx, activityID, device.ID)
	if err != nil {
		return info, classifyPresenceFailure(err)
	}
	return info, nil
}

func (s *sessionLifecycle) TimeoutSession(ctx context.Context) (devicescan.SessionTimeoutResponse, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.SessionTimeoutResponse{}, err
	}
	result, err := s.store.Timeout(ctx, device.ID)
	if err != nil {
		return result, classifyPresenceFailure(err)
	}
	result.Status = "completed"
	result.Message = fmt.Sprintf("Session ended due to timeout. %d students checked out.", result.StudentsCheckedOut)
	return result, nil
}

func (s *sessionLifecycle) TouchSession(ctx context.Context) (int64, error) {
	device, err := s.device(ctx)
	if err != nil {
		return 0, err
	}
	group, err := s.store.Current(ctx, device.ID)
	if err != nil {
		return 0, classifyPresenceFailure(err)
	}
	if group == nil {
		return 0, devicescan.NotFound("no active session found for device")
	}
	if err := s.store.Touch(ctx, group.ID); err != nil {
		return 0, classifyPresenceFailure(err)
	}
	return group.ID, nil
}

func (s *sessionLifecycle) ValidateTimeout(ctx context.Context, minutes int) error {
	device, err := s.device(ctx)
	if err != nil {
		return err
	}
	if err := s.store.ValidateTimeout(ctx, device.ID, minutes); err != nil {
		return classifyPresenceFailure(err)
	}
	return nil
}

func (s *sessionLifecycle) SessionTimeoutInfo(ctx context.Context) (devicescan.SessionTimeoutInfoResponse, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.SessionTimeoutInfoResponse{}, err
	}
	info, err := s.store.TimeoutInfo(ctx, device.ID)
	if err != nil {
		return info, classifyPresenceFailure(err)
	}
	return info, nil
}
