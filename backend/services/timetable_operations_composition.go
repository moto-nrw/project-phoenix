package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
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
	Lifecycle      timetableplanning.InstanceService
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
		Lifecycle:            timetableOperationLifecycle{instances: in.Lifecycle},
		Locks:                in.Rows.AttendanceLocks(),
		Announcer:            newTimetableAttendanceAnnouncer(in.Broadcaster),
		NormalizeSchoolClass: timetableplanning.NormalizeSchoolClass,
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

func (s timetableOperationSettings) AttendanceEditScope(ctx context.Context) (timetableCompose.AttendanceEditScope, error) {
	scope, err := s.settings.ResolveString(ctx, configModels.KeyAttendanceEditScope)
	if err != nil {
		return timetableCompose.AttendanceEditUnset, err
	}
	switch scope {
	case configModels.AttendanceEditScopeOwn:
		return timetableCompose.AttendanceEditOwn, nil
	case configModels.AttendanceEditScopeAllStaff:
		return timetableCompose.AttendanceEditAllStaff, nil
	default:
		return timetableCompose.AttendanceEditUnset, nil
	}
}

func (s timetableOperationSettings) StudentAbsenceEditAllStaff(ctx context.Context) (bool, error) {
	scope, err := s.settings.ResolveString(ctx, configModels.KeyStudentAbsenceEditScope)
	return scope == configModels.StudentAbsenceEditScopeAllStaff, err
}

func (s timetableOperationSettings) OperationalOverviewAllStaff(ctx context.Context) (bool, error) {
	scope, err := s.settings.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
	return scope == configModels.OverviewScopeAllStaff, err
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

// timetableOperationLifecycle drives the retained instance lifecycle for
// the operational commands until #3424 slice S1 moves it.
type timetableOperationLifecycle struct {
	instances timetableplanning.InstanceService
}

func (l timetableOperationLifecycle) CreateSpontaneous(ctx context.Context, in timetable.SpontaneousStart) (int64, error) {
	spontaneous := true
	instance, err := l.instances.Create(ctx, timetableplanning.CreateInstanceInput{
		Date:             in.Date,
		StartTime:        in.StartTime,
		EndTime:          in.EndTime,
		Title:            in.Title,
		Description:      in.Description,
		Notes:            in.Notes,
		RoomID:           in.RoomID,
		ActivityGroupID:  in.ActivityGroupID,
		IsSpontaneous:    &spontaneous,
		StaffIDs:         in.StaffIDs,
		CreatedByStaffID: in.CreatedByStaffID,
	})
	if err != nil {
		return 0, err
	}
	if instance == nil {
		return 0, errors.New("create spontaneous instance: no instance returned")
	}
	return instance.ID, nil
}

func (l timetableOperationLifecycle) Start(ctx context.Context, instanceID, staffID int64, spontaneous bool) (*timetable.StartedOperation, error) {
	if spontaneous {
		ctx = timetableplanning.WithSpontaneousStartWorkdayGuard(ctx)
	}
	result, err := l.instances.Start(ctx, instanceID, staffID)
	if err != nil {
		return nil, err
	}
	return startedOperation(result), nil
}

func (l timetableOperationLifecycle) Complete(ctx context.Context, instanceID, accountID int64) (*timetable.ScheduledInstance, error) {
	instance, err := l.instances.Complete(timetableplanning.WithLifecycleActor(ctx, accountID), instanceID)
	if err != nil || instance == nil {
		return nil, err
	}
	scheduled := timetableCompose.ScheduledInstanceOf(instance)
	return &scheduled, nil
}

func (l timetableOperationLifecycle) Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*timetable.StartedOperation, error) {
	result, err := l.instances.Reopen(ctx, instanceID, accountID, isAdmin)
	if err != nil {
		return nil, err
	}
	return startedOperation(result), nil
}

func startedOperation(result *timetableplanning.StartInstanceResult) *timetable.StartedOperation {
	if result == nil {
		return nil
	}
	started := &timetable.StartedOperation{ActiveGroupID: result.ActiveGroupID, Warnings: result.Warnings}
	if result.Instance != nil {
		started.InstanceID, started.Status = result.Instance.ID, result.Instance.Status
	}
	return started
}
