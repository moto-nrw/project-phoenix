// Package legacy adapts the retained owner services to the supervision
// projection's consumer-owned ports (#2703). It exists only because those
// owners (identity, settings, presence, the Schulhof workflow, timetable
// operations, day planning) still live in legacy service packages; the
// adapters translate rows into plain records and delegate every rule to its
// owner, deciding nothing themselves. Delete this package with the last
// legacy source once each owner exposes the fact through its public
// capability.
package legacy

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// Sources are the retained owner services the projection's ports adapt.
type Sources struct {
	Active       activeService.Service
	ActiveGroups activeModels.GroupRepository
	OpenVisits   activeService.VisitDisplayBatchReader
	Rooms        supervisiondashboard.RoomDirectory
	UserContext  userContextService.UserContextService
	Education    educationService.Service
	Schulhof     facilitiesService.SchulhofService
	Operations   scheduleService.TimetableOperationsService
	Settings     configService.SettingsService
	Pickups      scheduleService.PickupScheduleService
	Arrivals     scheduleService.ArrivalScheduleService
	Now          func() time.Time
}

// ErrIncompleteSources reports a missing retained service. Missing wiring is
// a configuration error and must fail composition, not the first request.
var ErrIncompleteSources = errors.New("supervision dashboard adapters are not fully configured")

// New composes the supervision projection over the retained sources. Now is
// optional; every other source is required.
func New(sources Sources) (supervisiondashboard.Query, error) {
	if sources.Active == nil || sources.UserContext == nil || sources.Education == nil ||
		sources.Schulhof == nil || sources.Operations == nil || sources.Settings == nil ||
		sources.Pickups == nil || sources.Arrivals == nil {
		return nil, ErrIncompleteSources
	}
	if sources.Now == nil {
		sources.Now = time.Now
	}
	return supervisiondashboard.New(supervisiondashboard.Dependencies{
		Access:   access{settings: sources.Settings, userContext: sources.UserContext},
		Sessions: sessions{active: sources.Active, groups: sources.ActiveGroups, userContext: sources.UserContext},
		Rooms:    sources.Rooms,
		Yard:     yard{schulhof: sources.Schulhof},
		Groups:   groups{education: sources.Education, userContext: sources.UserContext},
		Schedule: schedule{operations: sources.Operations},
		Presence: presence{active: sources.Active, openVisits: sources.OpenVisits},
		Planning: planning{pickups: sources.Pickups, arrivals: sources.Arrivals},
		Settings: settings{settings: sources.Settings},
		Calendar: calendar{},
		Now:      sources.Now,
	}), nil
}

type access struct {
	settings    configService.SettingsService
	userContext userContextService.UserContextService
}

func (a access) CurrentStaffID(ctx context.Context) (*int64, error) {
	staff, err := a.userContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) ||
			errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return nil, nil
		}
		return nil, err
	}
	if staff == nil {
		return nil, nil
	}
	id := staff.ID
	return &id, nil
}

// Caller asks the one school-wide overview rule (#2380). The organisational
// group mode is deliberately not consulted here: it describes how the school
// organises children, not who may open a running module.
func (a access) Caller(ctx context.Context) (supervisiondashboard.Caller, error) {
	principal, principalErr := permissions.PrincipalFromContext(ctx)
	assignmentBound := principalErr == nil && principal.Scope() == permissions.ScopeSchool
	admin := principalErr == nil && principal.HasAdminScope()
	overview, err := authorize.HasOperationalOverview(ctx, a.settings, a.userContext, assignmentBound, admin)
	if err != nil {
		return supervisiondashboard.Caller{}, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	claims := jwt.ClaimsFromCtx(ctx)
	userPermissions := jwt.PermissionsFromCtx(ctx)
	return supervisiondashboard.Caller{
		AccountID:           int64(claims.ID),
		TokenAdmin:          claims.IsAdmin,
		AdminScope:          admin,
		OperationalOverview: overview,
		CanReadSchedules:    authorize.HasPermission(permissions.SchedulesRead, userPermissions),
		CanReadStudents:     authorize.HasPermission(permissions.UsersRead, userPermissions),
	}, nil
}

func (a access) FullStudentAccess(ctx context.Context) (bool, error) {
	return userContextService.ResolveStudentAccess(ctx, a.userContext).HasFullAccess(), nil
}

type sessions struct {
	active      activeService.Service
	groups      activeModels.GroupRepository
	userContext userContextService.UserContextService
}

// Running lists every running session with the template activity relation
// filled in: the list query intentionally returns only active-group columns,
// so the rows are re-read in bulk by id.
func (s sessions) Running(ctx context.Context) ([]supervisiondashboard.Session, error) {
	all, err := s.active.ListActiveGroups(ctx, base.NewQueryOptions())
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(all))
	for _, group := range all {
		if group.IsActive() {
			ids = append(ids, group.ID)
		}
	}
	loaded, err := s.active.GetActiveGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load active group relations: %w", err)
	}
	groups := make([]*activeModels.Group, 0, len(ids))
	for _, id := range ids {
		if group := loaded[id]; group != nil {
			groups = append(groups, group)
		}
	}
	return s.records(ctx, groups)
}

func (s sessions) Supervised(ctx context.Context) ([]supervisiondashboard.Session, error) {
	groups, err := s.userContext.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, err
	}
	return s.records(ctx, groups)
}

func (s sessions) SupervisedByStaff(ctx context.Context, staffID int64) (map[int64]struct{}, error) {
	supervisions, err := s.active.GetStaffActiveSupervisions(ctx, staffID)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]struct{}, len(supervisions))
	for _, supervision := range supervisions {
		result[supervision.GroupID] = struct{}{}
	}
	return result, nil
}

func (s sessions) Unclaimed(ctx context.Context) ([]supervisiondashboard.UnclaimedGroup, error) {
	unclaimed, err := s.active.GetUnclaimedActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.UnclaimedGroup, 0, len(unclaimed))
	for _, group := range unclaimed {
		item := supervisiondashboard.UnclaimedGroup{ID: group.ID}
		if group.Room != nil {
			item.RoomName = group.Room.Name
		}
		result = append(result, item)
	}
	return result, nil
}

func (s sessions) InRooms(ctx context.Context, roomIDs []int64) ([]supervisiondashboard.RunningSession, error) {
	if s.groups == nil {
		return nil, errors.New("open-room session repository is not configured")
	}
	groups, err := s.groups.FindOpenSessionsInRooms(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.RunningSession, 0, len(groups))
	for _, group := range groups {
		result = append(result, supervisiondashboard.RunningSession{
			ActiveGroupID:      group.ActiveGroupID,
			RoomID:             group.RoomID,
			ActivityName:       group.ActivityName,
			StartTime:          group.StartTime,
			SupervisorStaffIDs: group.SupervisorStaffIDs,
		})
	}
	return result, nil
}

// records maps the rows to session records with their rooms resolved and
// sorts them in German dictionary order of the room name.
func (s sessions) records(ctx context.Context, groups []*activeModels.Group) ([]supervisiondashboard.Session, error) {
	rooms, err := s.rooms(ctx, groups)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.Session, 0, len(groups))
	for _, group := range groups {
		item := supervisiondashboard.Session{ID: group.ID}
		if group.ActualGroup != nil {
			item.Name = group.ActualGroup.Name
		}
		if group.RoomID > 0 {
			roomID := group.RoomID
			item.RoomID = &roomID
			if room := rooms[group.RoomID]; room != nil {
				item.RoomName = room.Name
				item.RoomColor = room.Color
			}
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return collation.CompareGerman(result[i].RoomName, result[j].RoomName) < 0
	})
	return result, nil
}

// rooms bulk-resolves rooms for groups whose relation is not preloaded —
// this replaces the former per-group GET /api/rooms/{id} N+1.
func (s sessions) rooms(ctx context.Context, groups []*activeModels.Group) (map[int64]*facilitiesModels.Room, error) {
	missing := make([]int64, 0, len(groups))
	seen := map[int64]struct{}{}
	result := map[int64]*facilitiesModels.Room{}
	for _, group := range groups {
		if group.RoomID <= 0 {
			continue
		}
		if group.Room != nil {
			result[group.RoomID] = group.Room
			continue
		}
		if _, ok := seen[group.RoomID]; ok {
			continue
		}
		seen[group.RoomID] = struct{}{}
		missing = append(missing, group.RoomID)
	}
	if len(missing) == 0 {
		return result, nil
	}
	rooms, err := s.active.GetRoomsByIDs(ctx, missing)
	if err != nil {
		return nil, fmt.Errorf("bulk load rooms: %w", err)
	}
	for _, room := range rooms {
		result[room.ID] = room
	}
	return result, nil
}

type yard struct {
	schulhof facilitiesService.SchulhofService
}

func (y yard) Status(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
	status, err := y.schulhof.GetSchulhofStatus(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return schulhofStatus(status), nil
}

func schulhofStatus(status *facilitiesService.SchulhofStatus) *supervisiondashboard.SchulhofStatus {
	if status == nil {
		return nil
	}
	return &supervisiondashboard.SchulhofStatus{
		Exists:            status.Exists,
		RoomID:            status.RoomID,
		RoomName:          status.RoomName,
		ActivityGroupID:   status.ActivityGroupID,
		ActiveGroupID:     status.ActiveGroupID,
		IsUserSupervising: status.IsUserSupervising,
		SupervisionID:     status.SupervisionID,
		SupervisorCount:   status.SupervisorCount,
		StudentCount:      status.StudentCount,
		Supervisors: mapSlice(status.Supervisors, func(supervisor facilitiesService.SupervisorInfo) supervisiondashboard.Supervisor {
			return supervisiondashboard.Supervisor{ID: supervisor.ID, StaffID: supervisor.StaffID, Name: supervisor.Name, IsCurrentUser: supervisor.IsCurrentUser}
		}),
	}
}

type groups struct {
	education   educationService.Service
	userContext userContextService.UserContextService
}

func (g groups) MyGroups(ctx context.Context) ([]supervisiondashboard.EducationalGroup, error) {
	myGroups, err := g.userContext.GetMyGroups(ctx)
	if err != nil {
		return nil, err
	}
	roomGroupIDs := make([]int64, 0, len(myGroups))
	for _, group := range myGroups {
		if group.RoomID != nil {
			roomGroupIDs = append(roomGroupIDs, group.ID)
		}
	}
	roomNames := map[int64]string{}
	if len(roomGroupIDs) > 0 {
		loaded, err := g.education.GetGroupsWithRoomsByIDs(ctx, roomGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("load education group rooms: %w", err)
		}
		for id, group := range loaded {
			if group != nil && group.Room != nil {
				roomNames[id] = group.Room.Name
			}
		}
	}
	result := make([]supervisiondashboard.EducationalGroup, 0, len(myGroups))
	for _, group := range myGroups {
		result = append(result, supervisiondashboard.EducationalGroup{ID: group.ID, Name: group.Name, RoomName: roomNames[group.ID]})
	}
	return result, nil
}

type schedule struct {
	operations scheduleService.TimetableOperationsService
}

func (s schedule) PlannedNow(ctx context.Context, query supervisiondashboard.PlannedNowQuery) ([]supervisiondashboard.PlannedInstance, error) {
	day, err := parseDay(query.Date)
	if err != nil {
		return nil, err
	}
	planned, err := s.operations.PlannedNow(ctx, query.AccountID, query.TokenAdmin, day, query.Now, scheduleService.PlannedNowOptions{
		HorizonMinutes: query.HorizonMinutes,
		Limit:          query.Limit,
		IncludeRoster:  query.IncludeRoster,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(planned, plannedInstance), nil
}

func (s schedule) ActiveSessions(ctx context.Context, date supervisiondashboard.Date) ([]supervisiondashboard.ActiveSession, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	sessions, err := s.operations.ActiveSessions(ctx, day)
	if err != nil {
		return nil, err
	}
	return mapSlice(sessions, func(session scheduleService.OperationActiveSession) supervisiondashboard.ActiveSession {
		return supervisiondashboard.ActiveSession{
			ActiveGroupID: session.ActiveGroupID,
			InstanceID:    session.InstanceID,
			Title:         session.Title,
			StartTime:     session.StartTime,
			EndTime:       session.EndTime,
		}
	}), nil
}

func plannedInstance(instance scheduleService.OperationPlannedInstance) supervisiondashboard.PlannedInstance {
	return supervisiondashboard.PlannedInstance{
		ID:                    instance.ID,
		Title:                 instance.Title,
		Date:                  instance.Date,
		StartTime:             instance.StartTime,
		EndTime:               instance.EndTime,
		RoomID:                instance.RoomID,
		RoomName:              instance.RoomName,
		Status:                instance.Status,
		IsOverdue:             instance.IsOverdue,
		MinutesUntilStart:     instance.MinutesUntilStart,
		ExpectedStudentsCount: instance.ExpectedStudentsCount,
		PresentStudentsCount:  instance.PresentStudentsCount,
		NotScheduledCount:     instance.NotScheduledCount,
		AssignedStaffIDs:      instance.AssignedStaffIDs,
		IsAssigned:            instance.IsAssigned,
		IsPrimary:             instance.IsPrimary,
		IsSubstitute:          instance.IsSubstitute,
		IsAbsent:              instance.IsAbsent,
		RosterPreview:         mapSlice(instance.RosterPreview, rosterRow),
		PickupTimesLoaded:     instance.PickupTimesLoaded,
		PickupTimesRedacted:   instance.PickupTimesRedacted,
		Warnings:              mapSlice(instance.Warnings, conflictWarning),
		CanStart:              instance.CanStart,
		StartAvailableAt:      instance.StartAvailableAt,
		StartExpiresAt:        instance.StartExpiresAt,
		ActiveGroupID:         instance.ActiveGroupID,
		CancelReason:          instance.CancelReason,
		PlanningTrackName:     instance.PlanningTrackName,
		PlanningTrackColor:    instance.PlanningTrackColor,
		GroupName:             instance.GroupName,
		StaffNames: mapSlice(instance.StaffNames, func(name scheduleService.OperationStaffName) supervisiondashboard.StaffName {
			return supervisiondashboard.StaffName{StaffID: name.StaffID, DisplayName: name.DisplayName, IsSubstitute: name.IsSubstitute}
		}),
	}
}

func rosterRow(row scheduleService.OperationRosterRow) supervisiondashboard.RosterRow {
	var parallel *supervisiondashboard.ParallelPresence
	if row.ParallelPresentIn != nil {
		parallel = &supervisiondashboard.ParallelPresence{
			InstanceID: row.ParallelPresentIn.InstanceID,
			Title:      row.ParallelPresentIn.Title,
			StartTime:  row.ParallelPresentIn.StartTime,
			EndTime:    row.ParallelPresentIn.EndTime,
		}
	}
	return supervisiondashboard.RosterRow{
		StudentID:        row.StudentID,
		StudentName:      row.StudentName,
		SchoolClass:      row.SchoolClass,
		GroupName:        row.GroupName,
		Planned:          row.Planned,
		IsUnplanned:      row.IsUnplanned,
		CurrentlyPresent: row.CurrentlyPresent,
		VisitID:          row.VisitID,
		Status:           row.Status,
		Substatus:        row.Substatus,
		Note:             row.Note,
		CheckedInAt:      row.CheckedInAt,
		CheckedOutAt:     row.CheckedOutAt,
		VisitEntryTime:   row.VisitEntryTime,
		PickupTime:       row.PickupTime,
		Warnings: mapSlice(row.Warnings, func(warning scheduleService.OperationRosterWarning) supervisiondashboard.RosterWarning {
			return supervisiondashboard.RosterWarning{
				Kind:                  warning.Kind,
				Message:               warning.Message,
				ExpectedArrival:       warning.ExpectedArrival,
				SlotStart:             warning.SlotStart,
				ExpectedGroupID:       warning.ExpectedGroupID,
				ExpectedGroupName:     warning.ExpectedGroupName,
				CurrentEducationGroup: warning.CurrentEducationGroup,
			}
		}),
		ParallelPresentIn: parallel,
		CareDayStatus:     string(row.CareDayStatus),
	}
}

func conflictWarning(warning scheduleService.InstanceConflictWarning) supervisiondashboard.ConflictWarning {
	return supervisiondashboard.ConflictWarning{
		Kind:                  warning.Kind,
		ResourceID:            warning.ResourceID,
		Message:               warning.Message,
		CanOverride:           warning.CanOverride,
		Fingerprint:           warning.Fingerprint,
		ConflictingInstanceID: warning.ConflictingInstanceID,
		ConflictingTitle:      warning.ConflictingTitle,
		OverlapStart:          warning.OverlapStart,
		OverlapEnd:            warning.OverlapEnd,
	}
}

type presence struct {
	active     activeService.Service
	openVisits activeService.VisitDisplayBatchReader
}

func (p presence) GroupVisits(ctx context.Context, activeGroupID int64) ([]supervisiondashboard.VisitRecord, error) {
	rows, err := p.active.GetActiveGroupVisitsWithDisplay(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	return visitRecords(rows), nil
}

func visitRecords(rows []*activeService.VisitWithStudentDisplay) []supervisiondashboard.VisitRecord {
	result := make([]supervisiondashboard.VisitRecord, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		record := supervisiondashboard.VisitRecord{
			StudentID:     row.StudentID,
			ActiveGroupID: row.ActiveGroupID,
			EntryTime:     row.EntryTime,
			ExitTime:      row.ExitTime,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			SchoolClass:   row.SchoolClass,
			GroupName:     row.OGSGroupName,
			SickSince:     row.SickSince,
			ExcusedSince:  row.ExcusedSince,
			PhotoPath:     row.PhotoPath,
		}
		if row.Sick != nil {
			record.Sick = *row.Sick
		}
		if row.Excused != nil {
			record.Excused = *row.Excused
		}
		result = append(result, record)
	}
	return result
}

func (p presence) OpenVisitsOfSessions(ctx context.Context, activeGroupIDs []int64) ([]supervisiondashboard.VisitRecord, error) {
	if p.openVisits == nil {
		return nil, errors.New("open-room visit reader is not configured")
	}
	rows, err := p.openVisits.GetActiveGroupVisitsWithDisplayForGroups(ctx, activeGroupIDs)
	if err != nil {
		return nil, err
	}
	return visitRecords(rows), nil
}

func (p presence) AttendanceTimes(ctx context.Context, studentIDs []int64) (map[int64]supervisiondashboard.Attendance, error) {
	statuses, err := p.active.GetStudentsAttendanceStatuses(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]supervisiondashboard.Attendance, len(statuses))
	for id, status := range statuses {
		if status == nil {
			continue
		}
		result[id] = supervisiondashboard.Attendance{CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime}
	}
	return result, nil
}

func (p presence) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}

type planning struct {
	pickups  scheduleService.PickupScheduleService
	arrivals scheduleService.ArrivalScheduleService
}

func (p planning) Pickups(ctx context.Context, studentIDs []int64, date supervisiondashboard.Date) (map[int64]supervisiondashboard.Pickup, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	effective, err := p.pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]supervisiondashboard.Pickup, len(effective))
	for id, pickup := range effective {
		if pickup == nil {
			continue
		}
		result[id] = supervisiondashboard.Pickup{
			Date:        supervisiondashboard.Date(pickup.Date.String()),
			WeekdayName: pickup.WeekdayName,
			PickupTime:  wallClock(pickup.PickupTime),
			IsException: pickup.IsException,
			Notes:       pickup.Notes,
			DayNotes: mapSlice(pickup.DayNotes, func(note scheduleService.NoteData) supervisiondashboard.DayNote {
				return supervisiondashboard.DayNote{ID: note.ID, Content: note.Content}
			}),
		}
	}
	return result, nil
}

func (p planning) Arrivals(ctx context.Context, studentIDs []int64, date supervisiondashboard.Date) (map[int64]supervisiondashboard.Arrival, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	effective, err := p.arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]supervisiondashboard.Arrival, len(effective))
	for id, arrival := range effective {
		if arrival == nil {
			continue
		}
		result[id] = supervisiondashboard.Arrival{
			Date:        supervisiondashboard.Date(arrival.Date.String()),
			WeekdayName: arrival.WeekdayName,
			ArrivalTime: wallClock(arrival.ArrivalTime),
			IsException: arrival.IsException,
			Notes:       arrival.Notes,
			DayNotes: mapSlice(arrival.DayNotes, func(note scheduleService.ArrivalNoteData) supervisiondashboard.DayNote {
				return supervisiondashboard.DayNote{ID: note.ID, Content: note.Content}
			}),
		}
	}
	return result, nil
}

// wallClock renders a normalized wall-clock value as HH:MM. The retained
// plans map PostgreSQL TIME columns: the clock lives in the value itself
// (timezone.NormalizeWallClock), so converting it to Berlin would be wrong.
func wallClock(at *time.Time) *string {
	if at == nil {
		return nil
	}
	formatted := at.Format("15:04")
	return &formatted
}

type settings struct{ settings configService.SettingsService }

func (s settings) Prepare(ctx context.Context) (context.Context, error) {
	batch, ok := s.settings.(configService.BatchSettingsService)
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveMany(ctx, []string{
		configModel.KeyOperationalOverviewScope,
		configModel.KeyGroupMode,
		configModel.KeyStudentPhotosEnabled,
		configModel.KeyCareConcept,
		configModel.KeyWebSpontaneousActivities,
		configModel.KeyTrackingIndicatorsEnabled,
		configModel.KeyTrackingIndicator1,
		configModel.KeyTrackingIndicator2,
		configModel.KeyTrackingIndicator3,
	})
	if err != nil {
		return ctx, err
	}
	return configService.WithSettingsSnapshot(ctx, snapshot), nil
}

func (s settings) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
}

func (s settings) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyTrackingIndicatorsEnabled)
	if err != nil || !enabled {
		return nil, err
	}
	labels := make([]string, 0, 3)
	for _, key := range []string{configModel.KeyTrackingIndicator1, configModel.KeyTrackingIndicator2, configModel.KeyTrackingIndicator3} {
		value, err := s.settings.ResolveString(ctx, key)
		if err != nil {
			return nil, err
		}
		if value = strings.TrimSpace(value); value != "" {
			labels = append(labels, value)
		}
	}
	return labels, nil
}

func (s settings) SpontaneousActivitiesEnabled(ctx context.Context) (bool, error) {
	careConcept, err := s.settings.ResolveString(ctx, configModel.KeyCareConcept)
	if err != nil {
		return false, fmt.Errorf("resolve care concept setting: %w", err)
	}
	if careConcept != configModel.CareConceptOpenRooms {
		return false, nil
	}
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyWebSpontaneousActivities)
	if err != nil {
		return false, fmt.Errorf("resolve spontaneous activities setting: %w", err)
	}
	return enabled, nil
}

type calendar struct{}

func (calendar) DayOf(at time.Time) supervisiondashboard.Date {
	return supervisiondashboard.Date(timezone.DateFromTime(at).String())
}

func (calendar) Weekday(date supervisiondashboard.Date) time.Weekday {
	return timezone.Date(date).Weekday()
}

func (calendar) Clock(at time.Time) string {
	return *timezone.FormatBerlinClock(&at)
}

// parseDay converts the projection's calendar day into the retained
// signatures' date value.
func parseDay(date supervisiondashboard.Date) (timezone.Date, error) {
	return timezone.ParseDate(string(date))
}

// mapSlice converts every element and keeps nil as nil, so a wire field
// without omitempty serialises exactly like the retained row.
func mapSlice[From, To any](from []From, convert func(From) To) []To {
	if from == nil {
		return nil
	}
	result := make([]To, 0, len(from))
	for _, item := range from {
		result = append(result, convert(item))
	}
	return result
}
