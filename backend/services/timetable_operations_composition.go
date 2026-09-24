package services

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// timetableOperationPresence is the Student Presence slice the operational
// day writes visits through.
type timetableOperationPresence interface {
	CreateVisit(ctx context.Context, visit *studentpresence.Visit) error
	EndVisit(ctx context.Context, id int64) error
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error)
}

// timetableOperationSettingsSource is the slice of the settings service the
// operational policies resolve through.
type timetableOperationSettingsSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

type timetableArrivalTimes interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
}

type timetablePickupTimes interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*careplan.EffectivePickupTime, error)
}

// timetableOperationInputs are the retained rows and the owners the
// Timetable operational day reads through (#3551). Broadcaster is optional.
type timetableOperationInputs struct {
	Rows           repositories.TimetableOwnerRows
	PlanningTracks timetable.PlanningTrackQuery
	Sessions       studentpresence.SessionRecords
	Supervisions   studentpresence.SupervisionRecords
	Visits         timetableCompose.OperationVisits
	Presence       timetableOperationPresence
	People         timetableCompose.OperationPeople
	Settings       timetableOperationSettingsSource
	CareDays       careplan.CareDayQuery
	Arrivals       timetableArrivalTimes
	Pickups        timetablePickupTimes
	Lifecycle      timetableCompose.OperationLifecycle
	Broadcaster    realtime.Broadcaster
	Logger         *slog.Logger
	Now            func() time.Time
}

// newTimetableOperations composes the Timetable owner's operational day and
// binds the collaborators it may not name itself.
func newTimetableOperations(in timetableOperationInputs) (timetable.OperationCapability, error) {
	templates, err := in.Rows.OperationTemplates()
	if err != nil {
		return nil, err
	}
	return timetableCompose.NewOperations(timetableCompose.OperationDependencies{
		Instances:            in.Rows.Instances,
		InstanceStaff:        in.Rows.InstanceStaff,
		Participants:         in.Rows.Participants,
		Templates:            templates,
		PlanningTracks:       in.PlanningTracks,
		Sessions:             in.Sessions,
		Supervisions:         in.Supervisions,
		Visits:               in.Visits,
		Attendance:           timetableOperationAttendance{presence: in.Presence},
		Students:             in.Rows.Students,
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
func newTimetableAttendanceAnnouncer(broadcaster realtime.Broadcaster) timetableCompose.AttendanceAnnouncer {
	if broadcaster == nil {
		return nil
	}
	return timetableAttendanceAnnouncer{broadcaster: broadcaster}
}

// timetableOperationAttendance attributes the visit writes of a check-in or
// check-out to the acting staff member, the way a device-authenticated
// staff write is attributed.
type timetableOperationAttendance struct {
	presence timetableOperationPresence
}

func (a timetableOperationAttendance) CreateVisitAs(ctx context.Context, staffID int64, visit *studentpresence.Visit) error {
	return a.presence.CreateVisit(WithAttendanceStaff(ctx, staffID, visit.TenantID), visit)
}

func (a timetableOperationAttendance) EndVisitAs(ctx context.Context, staffID, tenantID, visitID int64) error {
	return a.presence.EndVisit(WithAttendanceStaff(ctx, staffID, tenantID), visitID)
}

func (a timetableOperationAttendance) MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error) {
	return a.presence.MoveStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, auth)
}

// timetableOperationSettings resolves the operational policies of the
// tenant's settings.
type timetableOperationSettings struct {
	settings timetableOperationSettingsSource
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

// timetableScopedActionKeys names the scope setting of each scoped action.
var timetableScopedActionKeys = map[timetableCompose.ScopedAction]string{
	timetableCompose.ScopedAttendance:    configModels.KeyAttendanceEditScope,
	timetableCompose.ScopedBlockStart:    configModels.KeyBlockStartScope,
	timetableCompose.ScopedBlockComplete: configModels.KeyBlockCompleteScope,
}

func (timetableOperationSettings) ActionScopeKey(action timetableCompose.ScopedAction) (string, error) {
	key, ok := timetableScopedActionKeys[action]
	if !ok {
		return "", fmt.Errorf("timetable operations: unknown scoped action %d", action)
	}
	return key, nil
}

func (s timetableOperationSettings) StudentAbsenceEditAllStaff(ctx context.Context) (bool, error) {
	scope, err := s.settings.ResolveString(ctx, configModels.KeyStudentAbsenceEditScope)
	return scope == configModels.StudentAbsenceEditScopeAllStaff, err
}

// timetableEffectiveArrivals serves Care Plan's effective arrivals.
type timetableEffectiveArrivals struct {
	times timetableArrivalTimes
}

func (a timetableEffectiveArrivals) EffectiveArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*timetableCompose.ExpectedArrival, error) {
	arrivals, err := a.times.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*timetableCompose.ExpectedArrival, len(arrivals))
	for studentID, arrival := range arrivals {
		if arrival == nil {
			result[studentID] = nil
			continue
		}
		expected := &timetableCompose.ExpectedArrival{ArrivalTime: arrival.ArrivalTime, IsException: arrival.IsException}
		if arrival.ClassException != nil {
			expected.ClassException = &timetableCompose.ClassArrivalNotice{
				ArrivalTime: arrival.ClassException.ArrivalTime, Label: arrival.ClassException.Label,
			}
		}
		result[studentID] = expected
	}
	return result, nil
}

// timetableEffectivePickups serves Care Plan's effective pickup times.
type timetableEffectivePickups struct {
	times timetablePickupTimes
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
