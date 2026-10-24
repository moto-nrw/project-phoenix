package owners

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableOperations composes the Timetable owner's operational day
// (#3551) and binds the collaborators it may not name itself. deps carries
// the planning tracks, the session, supervision and visit reads, the people,
// the lifecycle, the logger and the clock; the retained rows, the presence
// writes, the operational settings, Care Plan's care days and effective
// arrival and pickup times and the broadcaster are bound here. broadcaster
// is optional.
func NewTimetableOperations(
	deps timetablePorts.OperationDependencies, rows timetableCompose.OwnerRows, presence TimetableOperationPresence,
	settings TimetableSettings, careDays careplan.CareDayQuery, arrivals EffectiveArrivalTimes, pickups EffectivePickupTimes,
	broadcaster realtime.Broadcaster, withStaff func(context.Context, int64, int64) context.Context,
) (timetable.OperationCapability, error) {
	deps.Instances, deps.InstanceStaff, deps.Participants, deps.Templates = rows.Instances.Reads, rows.InstanceStaff, rows.Participants, rows.Templates
	deps.Attendance = timetableOperationAttendance{presence: presence, withStaff: withStaff}
	deps.Students = rows.Students
	deps.EducationGroups, deps.Rooms = rows.EducationGroups, rows.Rooms
	deps.Settings = timetableOperationSettings{settings: settings}
	deps.CareDays = newTimetableCareDays(careDays)
	deps.Arrivals, deps.Pickups = timetableEffectiveArrivals{times: arrivals}, timetableEffectivePickups{times: pickups}
	deps.Locks = rows.Locks
	deps.Announcer = newTimetableAttendanceAnnouncer(broadcaster)
	deps.NormalizeSchoolClass = schoolStructure.NormalizeClass
	return timetableCompose.NewOperations(deps, TimetableOperationCaller{RequestSession: deps.Session})
}

// EffectiveArrivalTimes and EffectivePickupTimes are Care Plan's effective
// arrival and pickup times of a day.
type (
	EffectiveArrivalTimes interface {
		GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
	}
	EffectivePickupTimes interface {
		GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectivePickupTime, error)
	}
)

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
	presence  TimetableOperationPresence
	withStaff func(context.Context, int64, int64) context.Context
}

func (a timetableOperationAttendance) CreateVisitAs(ctx context.Context, staffID int64, visit *studentpresence.Visit) error {
	return a.presence.CreateVisit(a.withStaff(ctx, staffID, visit.TenantID), visit)
}

func (a timetableOperationAttendance) EndVisitAs(ctx context.Context, staffID, tenantID, visitID int64) error {
	return a.presence.EndVisit(a.withStaff(ctx, staffID, tenantID), visitID)
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
