package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// classifyPresence maps a retained presence service failure to the port
// sentinels the kiosk contract renders, keeping the service's wording. The
// groups mirror the historic IoT error mapping: a departed child, then the
// conflict, not-found and validation sentinels of the active service.
func classifyPresence(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, studentpresence.ErrRoomCapacityExceeded) {
		return mapError(err, ports.ErrConflict)
	}
	if errors.Is(err, studentpresence.ErrNoRoomAvailable) {
		return mapError(err, ports.ErrInvalid)
	}
	activeErr, ok := err.(*studentpresence.OperationError) //nolint:errorlint // the historic mapping classified the outer wrapper only
	if !ok {
		return err
	}
	inner := activeErr.Err
	switch {
	case errors.Is(inner, studentpresence.ErrStudentGraduated), errors.Is(inner, studentpresence.ErrStudentCareEnded):
		return mapError(err, ports.ErrStudentNotInCare)
	case errors.Is(inner, studentpresence.ErrRoomConflict), errors.Is(inner, studentpresence.ErrSessionConflict),
		errors.Is(inner, studentpresence.ErrStudentAlreadyInGroup), errors.Is(inner, studentpresence.ErrGroupAlreadyInCombination),
		errors.Is(inner, studentpresence.ErrStudentAlreadyActive), errors.Is(inner, studentpresence.ErrStaffAlreadySupervising),
		errors.Is(inner, studentpresence.ErrDeviceAlreadyActive):
		return mapError(err, ports.ErrConflict)
	case errors.Is(inner, studentpresence.ErrGroupNotFound), errors.Is(inner, studentpresence.ErrVisitNotFound),
		errors.Is(inner, studentpresence.ErrGroupSupervisorNotFound), errors.Is(inner, studentpresence.ErrCombinedGroupNotFound),
		errors.Is(inner, studentpresence.ErrGroupMappingNotFound), errors.Is(inner, studentpresence.ErrNoActiveSession),
		errors.Is(inner, studentpresence.ErrStaffNotFound):
		return mapError(err, ports.ErrNotFound)
	case errors.Is(inner, studentpresence.ErrGroupAlreadyEnded), errors.Is(inner, studentpresence.ErrVisitAlreadyEnded),
		errors.Is(inner, studentpresence.ErrSupervisionAlreadyEnded), errors.Is(inner, studentpresence.ErrCombinedGroupAlreadyEnded),
		errors.Is(inner, studentpresence.ErrInvalidTimeRange), errors.Is(inner, studentpresence.ErrCannotDeleteActiveGroup),
		errors.Is(inner, studentpresence.ErrInvalidData), errors.Is(inner, studentpresence.ErrInvalidActivitySession),
		errors.Is(inner, studentpresence.ErrNoRoomAvailable):
		return mapError(err, ports.ErrInvalid)
	}
	return err
}

// visits binds the room stay transitions to the retained presence service.
type visits struct {
	active studentpresence.Presence
	clock  clock
}

func (v visits) Current(ctx context.Context, studentID int64) (*ports.CurrentVisit, error) {
	visit, err := v.active.GetStudentCurrentVisitWithRoom(ctx, studentID)
	if err != nil {
		if errors.Is(err, studentpresence.ErrVisitNotFound) {
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
		var roomCapacity *studentpresence.RoomCapacityError
		if errors.As(err, &roomCapacity) {
			return 0, &ports.RoomCapacityExceeded{
				RoomID: roomCapacity.RoomID, RoomName: roomCapacity.RoomName,
				CurrentOccupancy: roomCapacity.CurrentOccupancy, MaxCapacity: roomCapacity.MaxCapacity,
			}
		}
		if errors.Is(err, studentpresence.ErrStudentAlreadyActive) {
			return 0, mapError(err, ports.ErrStudentAlreadyActive)
		}
		if errors.Is(err, studentpresence.ErrStudentGraduated) || errors.Is(err, studentpresence.ErrStudentCareEnded) {
			return 0, mapError(err, ports.ErrStudentNotInCare)
		}
		return 0, err
	}
	return visit.ID, nil
}

// sessions binds the room session transitions to the retained presence
// service.
type sessions struct {
	active   studentpresence.Presence
	presence roomSessionQuery
	clock    clock
}

type roomSessionQuery interface {
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func sessionFromGroup(group *studentpresence.SessionDetail) ports.Session {
	session := ports.Session{
		ID: group.ID, RoomID: group.RoomID, StartTime: group.StartTime, EndTime: group.EndTime,
		DeviceID: group.DeviceID, TemplateID: group.ActivityGroupID,
	}
	if group.Activity != nil {
		activity := group.Activity
		session.Activity = &ports.Activity{
			ID: activity.ID, Name: activity.Name, MaxParticipants: activity.MaxParticipants,
			PlannedRoomID: activity.PlannedRoomID, IsSystem: activity.IsSystem, IsOpen: activity.IsOpen,
		}
		session.ActivityName = group.Activity.Name
	}
	if group.Room != nil {
		session.RoomName = group.Room.Name
	}
	return session
}

func (s sessions) ListOpenInRoom(ctx context.Context, roomID int64) ([]ports.Session, error) {
	groups, err := s.presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	result := make([]ports.Session, 0, len(groups))
	for _, group := range groups {
		result = append(result, ports.Session{
			ID: group.ID, RoomID: group.RoomID, StartTime: group.StartTime, EndTime: group.EndTime,
			DeviceID: group.DeviceID, TemplateID: group.ActivityGroupID,
		})
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
		if errors.Is(err, studentpresence.ErrNoActiveSession) {
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
	group := &studentpresence.LiveGroup{ActivityGroupID: &activityID, RoomID: input.RoomID, StartTime: now, LastActivity: now}
	if err := s.active.CreateActiveGroup(ctx, group); err != nil {
		return ports.Session{}, err
	}
	return ports.Session{
		ID: group.ID, RoomID: group.RoomID, StartTime: group.StartTime, EndTime: group.EndTime,
		DeviceID: group.DeviceID, TemplateID: group.ActivityGroupID,
	}, nil
}

func (s sessions) EnsureRoomSession(ctx context.Context, input ports.NewSession) (ports.Session, error) {
	group, err := s.active.EnsureOpenRoomSession(ctx, input.RoomID, input.ActivityID)
	if err != nil {
		return ports.Session{}, err
	}
	return sessionFromGroup(group), nil
}

func (s sessions) Delete(ctx context.Context, sessionID int64) error {
	return s.active.DeleteActiveGroup(ctx, sessionID)
}

func (s sessions) End(ctx context.Context, sessionID int64) error {
	err := s.active.EndActiveGroupSession(ctx, sessionID)
	if err != nil && !errors.Is(err, studentpresence.ErrGroupAlreadyEnded) {
		return err
	}
	return nil
}

func (s sessions) Touch(ctx context.Context, sessionID int64) error {
	return s.active.UpdateSessionActivity(ctx, sessionID)
}

func (s sessions) Supervisors(ctx context.Context, sessionID int64) ([]ports.Supervisor, error) {
	const operation = "GetActiveGroupWithSupervisors"
	groups, err := s.presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: []int64{sessionID}})
	if err != nil || len(groups) == 0 {
		return nil, &studentpresence.OperationError{Op: operation, Err: studentpresence.ErrGroupNotFound}
	}
	today := s.clock.Day(s.clock.Now())
	day := today.String()
	supervisors, err := s.presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{sessionID}, ActiveOn: &day})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	result := make([]ports.Supervisor, 0, len(supervisors))
	for _, supervisor := range supervisors {
		result = append(result, ports.Supervisor{StaffID: supervisor.StaffID, Ended: supervisionEnded(supervisor.EndDate, today.String())})
	}
	return result, nil
}

func (s sessions) ReplaceSupervisors(ctx context.Context, sessionID int64, staffIDs []int64) error {
	_, err := s.active.UpdateActiveGroupSupervisors(ctx, sessionID, staffIDs)
	return err
}

// attendance binds the daily attendance transitions to the retained
// presence service.
type attendance struct{ active studentpresence.Presence }

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
		if errors.Is(err, studentpresence.ErrNoAttendanceRecordForCheckout) {
			return "", mapError(err, ports.ErrNoAttendanceRecord)
		}
		return "", classifyPresence(err)
	}
	return result.Action, nil
}
