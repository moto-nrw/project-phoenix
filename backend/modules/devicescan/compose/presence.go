package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// classifyPresence maps a retained presence service failure to the port
// sentinels the kiosk contract renders, keeping the service's wording. The
// groups mirror the historic IoT error mapping: a departed child, then the
// conflict, not-found and validation sentinels of the active service.
func classifyPresence(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, activeSvc.ErrRoomCapacityExceeded) {
		return mapError(err, ports.ErrConflict)
	}
	if errors.Is(err, activeSvc.ErrNoRoomAvailable) {
		return mapError(err, ports.ErrInvalid)
	}
	activeErr, ok := err.(*activeSvc.ActiveError) //nolint:errorlint // the historic mapping classified the outer wrapper only
	if !ok {
		return err
	}
	inner := activeErr.Err
	switch {
	case errors.Is(inner, activeSvc.ErrStudentGraduated), errors.Is(inner, activeSvc.ErrStudentCareEnded):
		return mapError(err, ports.ErrStudentNotInCare)
	case errors.Is(inner, activeSvc.ErrRoomConflict), errors.Is(inner, activeSvc.ErrSessionConflict),
		errors.Is(inner, activeSvc.ErrStudentAlreadyInGroup), errors.Is(inner, activeSvc.ErrGroupAlreadyInCombination),
		errors.Is(inner, activeSvc.ErrStudentAlreadyActive), errors.Is(inner, activeSvc.ErrStaffAlreadySupervising),
		errors.Is(inner, activeSvc.ErrDeviceAlreadyActive):
		return mapError(err, ports.ErrConflict)
	case errors.Is(inner, activeSvc.ErrActiveGroupNotFound), errors.Is(inner, activeSvc.ErrVisitNotFound),
		errors.Is(inner, activeSvc.ErrGroupSupervisorNotFound), errors.Is(inner, activeSvc.ErrCombinedGroupNotFound),
		errors.Is(inner, activeSvc.ErrGroupMappingNotFound), errors.Is(inner, activeSvc.ErrNoActiveSession),
		errors.Is(inner, activeSvc.ErrStaffNotFound):
		return mapError(err, ports.ErrNotFound)
	case errors.Is(inner, activeSvc.ErrActiveGroupAlreadyEnded), errors.Is(inner, activeSvc.ErrVisitAlreadyEnded),
		errors.Is(inner, activeSvc.ErrSupervisionAlreadyEnded), errors.Is(inner, activeSvc.ErrCombinedGroupAlreadyEnded),
		errors.Is(inner, activeSvc.ErrInvalidTimeRange), errors.Is(inner, activeSvc.ErrCannotDeleteActiveGroup),
		errors.Is(inner, activeSvc.ErrInvalidData), errors.Is(inner, activeSvc.ErrInvalidActivitySession),
		errors.Is(inner, activeSvc.ErrNoRoomAvailable):
		return mapError(err, ports.ErrInvalid)
	}
	return err
}

// visits binds the room stay transitions to the retained presence service.
type visits struct {
	active activeSvc.Service
	clock  clock
}

func (v visits) Current(ctx context.Context, studentID int64) (*ports.CurrentVisit, error) {
	visit, err := v.active.GetStudentCurrentVisitWithRoom(ctx, studentID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrVisitNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if visit == nil || visit.ExitTime != nil {
		return nil, nil
	}
	current := &ports.CurrentVisit{ID: visit.ID, EntryTime: visit.EntryTime}
	if group := visit.ActiveGroup; group != nil {
		current.Session = &ports.SessionRef{ID: group.ID, RoomID: group.RoomID, StartTime: group.StartTime, DeviceID: group.DeviceID}
		if group.Room != nil {
			current.Session.Room = &ports.Room{ID: group.Room.ID, Name: group.Room.Name}
		}
	}
	return current, nil
}

func (v visits) End(ctx context.Context, visitID int64) error { return v.active.EndVisit(ctx, visitID) }

func (v visits) Record(ctx context.Context, studentID, sessionID int64) (int64, error) {
	visit := &studentpresence.Visit{StudentID: studentID, ActiveGroupID: sessionID, EntryTime: v.clock.Now()}
	if err := v.active.CreateVisit(ctx, visit); err != nil {
		var roomCapacity *activeSvc.RoomCapacityError
		if errors.As(err, &roomCapacity) {
			return 0, &ports.RoomCapacityExceeded{
				RoomID: roomCapacity.RoomID, RoomName: roomCapacity.RoomName,
				CurrentOccupancy: roomCapacity.CurrentOccupancy, MaxCapacity: roomCapacity.MaxCapacity,
			}
		}
		if errors.Is(err, activeSvc.ErrStudentAlreadyActive) {
			return 0, mapError(err, ports.ErrStudentAlreadyActive)
		}
		if errors.Is(err, activeSvc.ErrStudentGraduated) || errors.Is(err, activeSvc.ErrStudentCareEnded) {
			return 0, mapError(err, ports.ErrStudentNotInCare)
		}
		return 0, err
	}
	return visit.ID, nil
}

// sessions binds the room session transitions to the retained presence
// service.
type sessions struct {
	active activeSvc.Service
	clock  clock
}

func sessionFromGroup(group *active.Group) ports.Session {
	session := ports.Session{
		ID: group.ID, RoomID: group.RoomID, StartTime: group.StartTime, EndTime: group.EndTime,
		DeviceID: group.DeviceID, TemplateID: group.GroupID,
	}
	if group.ActualGroup != nil {
		session.Activity = activityFromGroup(group.ActualGroup)
		session.ActivityName = group.ActualGroup.Name
	}
	if group.Room != nil {
		session.RoomName = group.Room.Name
	}
	return session
}

func (s sessions) ListOpenInRoom(ctx context.Context, roomID int64) ([]ports.Session, error) {
	groups, err := s.active.FindActiveGroupsByRoomID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	result := make([]ports.Session, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		result = append(result, sessionFromGroup(group))
	}
	return result, nil
}

func (s sessions) FindDeviceSessionInRoom(ctx context.Context, roomID, deviceID int64) (*ports.Session, error) {
	group, err := s.active.FindDeviceActiveGroupInRoom(ctx, roomID, deviceID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	session := sessionFromGroup(group)
	return &session, nil
}

func (s sessions) Current(ctx context.Context, deviceID int64) (*ports.Session, error) {
	group, err := s.active.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
			return nil, nil
		}
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	session := sessionFromGroup(group)
	return &session, nil
}

func (s sessions) Start(ctx context.Context, input ports.NewSession) (ports.Session, error) {
	now := s.clock.Now()
	activityID := input.ActivityID
	group := &active.Group{GroupID: &activityID, RoomID: input.RoomID, StartTime: now, LastActivity: now}
	if err := s.active.CreateActiveGroup(ctx, group); err != nil {
		return ports.Session{}, err
	}
	return sessionFromGroup(group), nil
}

func (s sessions) Delete(ctx context.Context, sessionID int64) error {
	return s.active.DeleteActiveGroup(ctx, sessionID)
}

func (s sessions) End(ctx context.Context, sessionID int64) error {
	err := s.active.EndActiveGroupSession(ctx, sessionID)
	if err != nil && !errors.Is(err, activeSvc.ErrActiveGroupAlreadyEnded) {
		return err
	}
	return nil
}

func (s sessions) Touch(ctx context.Context, sessionID int64) error {
	return s.active.UpdateSessionActivity(ctx, sessionID)
}

func (s sessions) Supervisors(ctx context.Context, sessionID int64) ([]ports.Supervisor, error) {
	group, err := s.active.GetActiveGroupWithSupervisors(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]ports.Supervisor, 0, len(group.Supervisors))
	for _, supervisor := range group.Supervisors {
		if supervisor == nil {
			continue
		}
		result = append(result, ports.Supervisor{StaffID: supervisor.StaffID, Ended: supervisor.EndDate != nil})
	}
	return result, nil
}

func (s sessions) ReplaceSupervisors(ctx context.Context, sessionID int64, staffIDs []int64) error {
	_, err := s.active.UpdateActiveGroupSupervisors(ctx, sessionID, staffIDs)
	return err
}

// attendance binds the daily attendance transitions to the retained
// presence service.
type attendance struct{ active activeSvc.Service }

func (a attendance) Status(ctx context.Context, studentID int64) (*ports.AttendanceState, error) {
	status, err := a.active.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &ports.AttendanceState{
		Status: status.Status, Date: status.Date, CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime,
		CheckedInBy: status.CheckedInBy, CheckedOutBy: status.CheckedOutBy,
	}, nil
}

func (a attendance) Toggle(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (string, error) {
	result, err := a.active.ToggleStudentAttendance(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if err != nil {
		return "", classifyPresence(err)
	}
	return result.Action, nil
}

func (a attendance) ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (string, error) {
	result, err := a.active.ConfirmDailyCheckout(ctx, studentID, deviceID, destination)
	if err != nil {
		if errors.Is(err, activeSvc.ErrNoAttendanceRecordForCheckout) {
			return "", mapError(err, ports.ErrNoAttendanceRecord)
		}
		return "", classifyPresence(err)
	}
	return result.Action, nil
}
