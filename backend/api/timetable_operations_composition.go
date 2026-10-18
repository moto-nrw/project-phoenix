package api

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableOperations composes the Timetable owner's operational day and
// binds the collaborators it may not name itself.
func NewTimetableOperations(in TimetableOperationInputs) (timetable.OperationCapability, error) {
	templates, err := in.Rows.OperationTemplates()
	if err != nil {
		return nil, err
	}
	return timetableCompose.NewOperations(timetablePorts.OperationDependencies{
		Instances:            in.Rows.Instances,
		InstanceStaff:        in.Rows.InstanceStaff,
		Participants:         in.Rows.Participants,
		Templates:            templates,
		PlanningTracks:       in.PlanningTracks,
		Sessions:             in.Sessions,
		Supervisions:         in.Supervisions,
		Visits:               in.Visits,
		Attendance:           timetableOperationAttendance{presence: in.Presence},
		Students:             NewTimetableOperationStudents(in.Rows.Students),
		People:               in.People,
		EducationGroups:      in.Rows.EducationGroupNames(),
		Rooms:                in.Rows.RoomNames(),
		Settings:             timetableOperationSettings{settings: in.Settings},
		CareDays:             newTimetableCareDays(in.CareDays),
		Arrivals:             timetableEffectiveArrivals{times: in.Arrivals},
		Pickups:              timetableEffectivePickups{times: in.Pickups},
		Lifecycle:            in.Lifecycle,
		Locks:                in.Rows.AttendanceLocks(),
		Announcer:            newTimetableAttendanceAnnouncer(in.Broadcaster),
		NormalizeSchoolClass: schoolStructure.NormalizeClass,
		Logger:               in.Logger,
		Now:                  in.Now,
	})
}

// newTimetableAttendanceAnnouncer wakes the tenant's clients after an
// attendance patch; nil without a broadcaster.
func newTimetableAttendanceAnnouncer(broadcaster realtime.Broadcaster) timetablePorts.AttendanceAnnouncer {
	if broadcaster == nil {
		return nil
	}
	return timetableAttendanceAnnouncer{broadcaster: broadcaster}
}

// timetableOperationAttendance attributes the visit writes of a check-in or
// check-out to the acting staff member, the way a device-authenticated
// staff write is attributed.
type timetableOperationAttendance struct {
	presence TimetableOperationPresence
}

func (a timetableOperationAttendance) CreateVisitAs(ctx context.Context, staffID int64, visit *studentpresence.Visit) error {
	return a.presence.CreateVisit(devicescanCompose.WithAttendanceStaff(ctx, staffID, visit.TenantID), visit)
}

func (a timetableOperationAttendance) EndVisitAs(ctx context.Context, staffID, tenantID, visitID int64) error {
	return a.presence.EndVisit(devicescanCompose.WithAttendanceStaff(ctx, staffID, tenantID), visitID)
}

func (a timetableOperationAttendance) MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error) {
	return a.presence.MoveStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, auth)
}

// timetableOperationSettings resolves the operational policies of the
// tenant's settings.
type timetableOperationSettings struct {
	settings TimetableSettings
}

func (s timetableOperationSettings) ResolveString(ctx context.Context, key string) (string, error) {
	return s.settings.ResolveString(ctx, key)
}

func (s timetableOperationSettings) StartLeadMinutes(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyTimetableStartLeadMinutes)
}

func (s timetableOperationSettings) EnforcePlannedEnd(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyTimetableEnforcePlannedEnd)
}

func (s timetableOperationSettings) CompleteLeadMinutes(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyTimetableCompleteLeadMinutes)
}

// timetableScopedActionKeys names the scope setting of each scoped action.
var timetableScopedActionKeys = map[timetablePorts.ScopedAction]string{
	timetablePorts.ScopedAttendance:    configModels.KeyAttendanceEditScope,
	timetablePorts.ScopedBlockStart:    configModels.KeyBlockStartScope,
	timetablePorts.ScopedBlockComplete: configModels.KeyBlockCompleteScope,
}

func (timetableOperationSettings) ActionScopeKey(action timetablePorts.ScopedAction) (string, error) {
	key, ok := timetableScopedActionKeys[action]
	if !ok {
		return "", fmt.Errorf("timetable operations: unknown scoped action %d", action)
	}
	return key, nil
}

func (s timetableOperationSettings) StudentAbsenceEditAllStaff(ctx context.Context) (bool, error) {
	scope, err := s.settings.ResolveString(ctx, configModels.KeyStudentAbsenceEditScope)
	return configModels.StudentAbsenceEditOpenToAllStaff(scope), err
}

// timetableEffectiveArrivals serves Care Plan's effective arrivals.
type timetableEffectiveArrivals struct {
	times EffectiveArrivalTimes
}

func (a timetableEffectiveArrivals) EffectiveArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*timetablePorts.ExpectedArrival, error) {
	arrivals, err := a.times.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*timetablePorts.ExpectedArrival, len(arrivals))
	for studentID, arrival := range arrivals {
		if arrival == nil {
			result[studentID] = nil
			continue
		}
		expected := &timetablePorts.ExpectedArrival{ArrivalTime: arrival.ArrivalTime, IsException: arrival.IsException}
		if arrival.ClassException != nil {
			expected.ClassException = &timetablePorts.ClassArrivalNotice{
				ArrivalTime: arrival.ClassException.ArrivalTime, Label: arrival.ClassException.Label,
			}
		}
		result[studentID] = expected
	}
	return result, nil
}

// timetableEffectivePickups serves Care Plan's effective pickup times.
type timetableEffectivePickups struct {
	times EffectivePickupTimes
}

func (p timetableEffectivePickups) EffectivePickups(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error) {
	pickups, err := p.times.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*time.Time, len(pickups))
	for studentID, pickup := range pickups {
		if pickup != nil {
			result[studentID] = pickup.PickupTime
		}
	}
	return result, nil
}

// timetableAttendanceAnnouncer turns an attendance patch into the id-less
// active-supervision wake-up of the tenant's clients (#2085).
type timetableAttendanceAnnouncer struct {
	broadcaster realtime.Broadcaster
}

func (a timetableAttendanceAnnouncer) AnnounceAttendanceChanged(tenantID, activeGroupID, instanceID int64) error {
	instance := strconv.FormatInt(instanceID, 10)
	reason := "timetable_attendance_updated"
	event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, strconv.FormatInt(activeGroupID, 10), realtime.EventData{
		InstanceID: &instance,
		Reason:     &reason,
	})
	return a.broadcaster.BroadcastToTenant(tenantID, event)
}
