package api

// The root binds the supervision dashboard's ports (#2703) to their owners:
// the Security Runtime for the overview and permission rules, the Settings
// Platform for the tenant settings, Facilities for the Schulhof status,
// Student Presence for the sessions and visits, School Structure for the
// group rooms, Timetable for the schedule and Care Plan for the planning
// times. The dashboard names none of these owners (#4077); the translation
// is cross-owner glue and lives here. Each adapter reads one owner and
// decides nothing itself.

import (
	"context"
	"errors"
	"fmt"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	supervisionDashboardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	educationService "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardCompose "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	calendarpkg "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SupervisionCallerContext is the Identity & Access caller context the
// supervision dashboard reads: the access facts the root binds and the
// groups the dashboard's adapters list.
type SupervisionCallerContext interface {
	CurrentStaffID(context.Context) (*int64, error)
	FullStudentAccess(context.Context) (bool, error)
	HasCurrentStaff(context.Context) (bool, error)
	GetMySupervisedGroups(context.Context) ([]*studentpresence.SessionDetail, error)
	MyGroups(context.Context) ([]SupervisionCallerGroup, error)
}

// SupervisionCallerGroup is one of the caller's educational groups, as the
// root serves the dashboard's caller port (#4006).
type SupervisionCallerGroup struct {
	ID     int64
	Name   string
	RoomID *int64
}

// supervisionAccessCaller is the slice of the caller context the access
// port reads.
type supervisionAccessCaller interface {
	CurrentStaffID(context.Context) (*int64, error)
	FullStudentAccess(context.Context) (bool, error)
	HasCurrentStaff(context.Context) (bool, error)
}

// newSupervisionAccess binds the dashboard's access port; without one of its
// sources it binds none, so the composition fails on the missing wiring.
func newSupervisionAccess(settings supervisiondashboard.Resolver, caller supervisionAccessCaller) supervisiondashboard.Access {
	if settings == nil || caller == nil {
		return nil
	}
	return supervisionAccess{settings: settings, caller: caller}
}

type supervisionAccess struct {
	settings supervisiondashboard.Resolver
	caller   supervisionAccessCaller
}

func (a supervisionAccess) CurrentStaffID(ctx context.Context) (*int64, error) {
	return a.caller.CurrentStaffID(ctx)
}

func (a supervisionAccess) FullStudentAccess(ctx context.Context) (bool, error) {
	return a.caller.FullStudentAccess(ctx)
}

// Caller asks the one school-wide overview rule (#2380). The organisational
// group mode is deliberately not consulted here: it describes how the school
// organises children, not who may open a running module.
func (a supervisionAccess) Caller(ctx context.Context) (supervisiondashboard.Caller, error) {
	assignmentBound := common.IsAssignmentBoundPortal(ctx)
	admin := common.HasEffectiveAdminScope(ctx)
	overview, err := securityruntime.CanViewOperationalOverview(ctx, a.settings, a.caller, assignmentBound, admin)
	if err != nil {
		return supervisiondashboard.Caller{}, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	claims := sessiontoken.ClaimsFromCtx(ctx)
	userPermissions := sessiontoken.PermissionsFromCtx(ctx)
	return supervisiondashboard.Caller{
		AccountID:           int64(claims.ID),
		TokenAdmin:          claims.IsAdmin,
		AdminScope:          admin,
		OperationalOverview: overview,
		CanReadSchedules:    securityruntime.HasPermission(supervisionDashboardHTTP.ScheduleReadPermission, userPermissions),
		CanReadStudents:     securityruntime.HasPermission(supervisionDashboardHTTP.StudentReadPermission, userPermissions),
	}, nil
}

// newSupervisionSettings binds the dashboard's settings port; without the
// resolver it binds none.
func newSupervisionSettings(settings supervisiondashboard.Resolver) supervisiondashboard.Settings {
	if settings == nil {
		return nil
	}
	return supervisionSettings{settings: settings}
}

type supervisionSettings struct{ settings supervisiondashboard.Resolver }

func (s supervisionSettings) Prepare(ctx context.Context) (context.Context, error) {
	return s.settings.Prefetch(ctx, []string{
		settingsplatform.KeyOperationalOverviewScope,
		settingsplatform.KeyGroupMode,
		settingsplatform.KeyStudentPhotosEnabled,
		settingsplatform.KeyWebSpontaneousActivities, settingsplatform.KeyWeekendFollowsFriday,
		settingsplatform.KeyTrackingIndicatorsEnabled,
		settingsplatform.KeyTrackingIndicator1,
		settingsplatform.KeyTrackingIndicator2,
		settingsplatform.KeyTrackingIndicator3,
	})
}

func (s supervisionSettings) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, settingsplatform.KeyStudentPhotosEnabled)
}

func (s supervisionSettings) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	return settingsplatform.PrefetchedTrackingIndicatorLabels(ctx, s.settings)
}

func (s supervisionSettings) SpontaneousActivitiesEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.settings.ResolveBool(ctx, settingsplatform.KeyWebSpontaneousActivities)
	if err != nil {
		return false, fmt.Errorf("resolve spontaneous activities setting: %w", err)
	}
	return enabled, nil
}

// newSupervisionYard adapts the Facilities Schulhof service to the
// dashboard's yard port, also for callers that only need the courtyard
// status; without the service it binds none.
func newSupervisionYard(schulhof facilitiesCompose.SchulhofService) supervisiondashboard.Yard {
	if schulhof == nil {
		return nil
	}
	return supervisionYard{schulhof: schulhof}
}

type supervisionYard struct {
	schulhof facilitiesCompose.SchulhofService
}

// Status carries the Facilities status onto the dashboard field for field;
// the two shapes are the same wire contract.
func (y supervisionYard) Status(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
	status, err := y.schulhof.GetSchulhofStatus(ctx, staffID)
	if err != nil || status == nil {
		return nil, err
	}
	result := &supervisiondashboard.SchulhofStatus{
		Exists:            status.Exists,
		RoomID:            status.RoomID,
		RoomName:          status.RoomName,
		ActivityGroupID:   status.ActivityGroupID,
		ActiveGroupID:     status.ActiveGroupID,
		IsUserSupervising: status.IsUserSupervising,
		SupervisionID:     status.SupervisionID,
		SupervisorCount:   status.SupervisorCount,
		StudentCount:      status.StudentCount,
	}
	// A nil list stays nil, so the wire field serialises exactly like the
	// Facilities status.
	if status.Supervisors != nil {
		result.Supervisors = make([]supervisiondashboard.Supervisor, 0, len(status.Supervisors))
		for _, supervisor := range status.Supervisors {
			result.Supervisors = append(result.Supervisors, supervisiondashboard.Supervisor{
				ID: supervisor.ID, StaffID: supervisor.StaffID, Name: supervisor.Name, IsCurrentUser: supervisor.IsCurrentUser,
			})
		}
	}
	return result, nil
}

// errSupervisionIncompleteSources reports a missing owner source. Missing
// wiring is a configuration error and must fail composition, not the first
// request.
var errSupervisionIncompleteSources = errors.New("supervision dashboard adapters are not fully configured")

// newSupervisionDashboard composes the supervision projection over the ports
// the root bound. Rooms and Now are optional; every other port is required,
// and a root constructor binds none when one of its sources is missing.
func newSupervisionDashboard(deps supervisiondashboard.Dependencies) (supervisiondashboard.Query, error) {
	if deps.Access == nil || deps.Sessions == nil || deps.Yard == nil || deps.Groups == nil || deps.Schedule == nil ||
		deps.Presence == nil || deps.Planning == nil || deps.Settings == nil || deps.Staff == nil || deps.Calendar == nil {
		return nil, errSupervisionIncompleteSources
	}
	return supervisiondashboard.New(deps), nil
}

// SupervisionOpenRoomSessions are the running sessions the dashboard shows
// (#4010).
type SupervisionOpenRoomSessions interface {
	GetStaffActiveGroupIDs(context.Context, int64) ([]int64, error)
	ListRunningSessionIDs(context.Context) ([]int64, error)
	FindOpenSessionsInRooms(context.Context, []int64) ([]supervisiondashboard.RunningSession, error)
	SessionSupervisorStaffIDs(context.Context, []int64) (map[int64][]int64, error)
}

// supervisionSessionCaller is the slice of the caller context the session
// port reads.
type supervisionSessionCaller interface {
	GetMySupervisedGroups(context.Context) ([]*studentpresence.SessionDetail, error)
}

// newSupervisionSessions binds the dashboard's session port to Student
// Presence; without the presence service or the caller it binds none. The
// open-room sessions are optional and checked when InRooms is asked.
func newSupervisionSessions(active studentpresence.Presence, groups SupervisionOpenRoomSessions, caller supervisionSessionCaller) supervisiondashboard.SessionDirectory {
	if active == nil || caller == nil {
		return nil
	}
	return supervisionSessions{active: active, groups: groups, userContext: caller, collate: supervisiondashboardCompose.CompareRoomNames}
}

type supervisionSessions struct {
	active      studentpresence.Presence
	groups      SupervisionOpenRoomSessions
	userContext supervisionSessionCaller
	collate     func(a, b string) int
}

// Running lists every running session with the template activity relation
// filled in: the list query intentionally returns only active-group columns,
// so the rows are re-read in bulk by id.
func (s supervisionSessions) Running(ctx context.Context) ([]supervisiondashboard.Session, error) {
	ids, err := s.groups.ListRunningSessionIDs(ctx)
	if err != nil {
		return nil, err
	}
	loaded, err := s.active.GetActiveGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load active group relations: %w", err)
	}
	groups := make([]*studentpresence.SessionDetail, 0, len(ids))
	for _, id := range ids {
		if group := loaded[id]; group != nil {
			groups = append(groups, group)
		}
	}
	return s.records(ctx, groups)
}

func (s supervisionSessions) Supervised(ctx context.Context) ([]supervisiondashboard.Session, error) {
	groups, err := s.userContext.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, err
	}
	return s.records(ctx, groups)
}

func (s supervisionSessions) SupervisedByStaff(ctx context.Context, staffID int64) (map[int64]struct{}, error) {
	supervisions, err := s.groups.GetStaffActiveGroupIDs(ctx, staffID)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]struct{}, len(supervisions))
	for _, groupID := range supervisions {
		result[groupID] = struct{}{}
	}
	return result, nil
}

func (s supervisionSessions) Unclaimed(ctx context.Context) ([]supervisiondashboard.UnclaimedGroup, error) {
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

func (s supervisionSessions) InRooms(ctx context.Context, roomIDs []int64) ([]supervisiondashboard.RunningSession, error) {
	if s.groups == nil {
		return nil, errors.New("open-room session repository is not configured")
	}
	return s.groups.FindOpenSessionsInRooms(ctx, roomIDs)
}

func (s supervisionSessions) SupervisorStaffIDs(ctx context.Context, sessionIDs []int64) (map[int64][]int64, error) {
	if s.groups == nil {
		return nil, errors.New("open-room session repository is not configured")
	}
	return s.groups.SessionSupervisorStaffIDs(ctx, sessionIDs)
}

// records maps the rows to session records with their rooms resolved and
// sorts them in German dictionary order of the room name.
func (s supervisionSessions) records(ctx context.Context, groups []*studentpresence.SessionDetail) ([]supervisiondashboard.Session, error) {
	rooms, err := s.rooms(ctx, groups)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.Session, 0, len(groups))
	for _, group := range groups {
		item := supervisiondashboard.Session{ID: group.ID}
		if activity := group.Activity; activity != nil {
			item.Name, item.ParticipantLimit = activity.Name, timetable.ParticipantLimitPtr(activity.MaxParticipants)
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
	supervisiondashboard.SortSessionsByRoom(result, s.collate)
	return result, nil
}

// rooms bulk-resolves rooms for groups whose relation is not preloaded —
// this replaces the former per-group GET /api/rooms/{id} N+1.
func (s supervisionSessions) rooms(ctx context.Context, groups []*studentpresence.SessionDetail) (map[int64]*studentpresence.SessionRoomSummary, error) {
	missing := make([]int64, 0, len(groups))
	seen := map[int64]struct{}{}
	result := map[int64]*studentpresence.SessionRoomSummary{}
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

// supervisionGroupsCaller is the slice of the caller context the education
// group port reads.
type supervisionGroupsCaller interface {
	MyGroups(context.Context) ([]SupervisionCallerGroup, error)
}

// newSupervisionGroups binds the dashboard's education group port to School
// Structure; without one of its sources it binds none.
func newSupervisionGroups(education educationService.GroupRoomsQuery, caller supervisionGroupsCaller) supervisiondashboard.EducationGroups {
	if education == nil || caller == nil {
		return nil
	}
	return supervisionGroups{education: education, userContext: caller}
}

type supervisionGroups struct {
	education   educationService.GroupRoomsQuery
	userContext supervisionGroupsCaller
}

func (g supervisionGroups) MyGroups(ctx context.Context) ([]supervisiondashboard.EducationalGroup, error) {
	myGroups, err := g.userContext.MyGroups(ctx)
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

// newSupervisionSchedule binds the dashboard's schedule port to the
// Timetable owner's operational reads; without them it binds none.
func newSupervisionSchedule(operations timetable.OperationQuery) supervisiondashboard.Schedule {
	if operations == nil {
		return nil
	}
	return supervisionSchedule{operations: operations}
}

type supervisionSchedule struct {
	operations timetable.OperationQuery
}

func (s supervisionSchedule) PlannedNow(ctx context.Context, query supervisiondashboard.PlannedNowQuery) ([]supervisiondashboard.PlannedInstance, error) {
	day, err := supervisionDay(query.Date)
	if err != nil {
		return nil, err
	}
	planned, err := s.operations.PlannedNow(ctx, query.AccountID, query.TokenAdmin, day, query.Now, timetable.PlannedNowOptions{
		HorizonMinutes: query.HorizonMinutes,
		Limit:          query.Limit,
		IncludeRoster:  query.IncludeRoster,
	})
	if err != nil {
		return nil, err
	}
	return owners.CalendarMapSlice(planned, supervisionPlannedInstance), nil
}

func (s supervisionSchedule) ActiveSessions(ctx context.Context, date supervisiondashboard.Date) ([]supervisiondashboard.ActiveSession, error) {
	day, err := supervisionDay(date)
	if err != nil {
		return nil, err
	}
	sessions, err := s.operations.ActiveSessions(ctx, day)
	if err != nil {
		return nil, err
	}
	return owners.CalendarMapSlice(sessions, func(session timetable.OperationActiveSession) supervisiondashboard.ActiveSession {
		return supervisiondashboard.ActiveSession{
			ActiveGroupID: session.ActiveGroupID,
			InstanceID:    session.InstanceID,
			Title:         session.Title,
			StartTime:     session.StartTime,
			EndTime:       session.EndTime,
		}
	}), nil
}

func (s supervisionSchedule) SessionBlocks(ctx context.Context, query supervisiondashboard.SessionBlocksQuery) ([]supervisiondashboard.SessionBlock, error) {
	day, err := supervisionDay(query.Date)
	if err != nil {
		return nil, err
	}
	blocks, err := s.operations.SessionBlocks(ctx, query.AccountID, query.TokenAdmin, day, query.Supervisors)
	if err != nil {
		return nil, err
	}
	return owners.CalendarMapSlice(blocks, func(block timetable.OperationSessionBlock) supervisiondashboard.SessionBlock {
		return supervisiondashboard.SessionBlock{
			ActiveGroupID: block.ActiveGroupID,
			InstanceID:    block.InstanceID,
			Title:         block.Title,
			StartTime:     block.StartTime,
			EndTime:       block.EndTime,
			IsSpontaneous: block.IsSpontaneous,
			IsAssigned:    block.IsAssigned,
			CanOperate:    block.CanOperate,
		}
	}), nil
}

func supervisionPlannedInstance(instance timetable.OperationPlannedInstance) supervisiondashboard.PlannedInstance {
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
		CurrentStudentsCount:  instance.CurrentStudentsCount,
		PlannedStudentsCount:  instance.PlannedStudentsCount,
		IsSpontaneous:         instance.IsSpontaneous,
		AssignedStaffIDs:      instance.AssignedStaffIDs,
		IsAssigned:            instance.IsAssigned,
		IsPrimary:             instance.IsPrimary,
		IsSubstitute:          instance.IsSubstitute,
		IsAbsent:              instance.IsAbsent,
		RosterPreview:         owners.CalendarMapSlice(instance.RosterPreview, supervisionRosterRow),
		PickupTimesLoaded:     instance.PickupTimesLoaded,
		PickupTimesRedacted:   instance.PickupTimesRedacted,
		Warnings:              owners.CalendarMapSlice(instance.Warnings, supervisionConflictWarning),
		CanStart:              instance.CanStart,
		StartAvailableAt:      instance.StartAvailableAt,
		StartExpiresAt:        instance.StartExpiresAt,
		ActiveGroupID:         instance.ActiveGroupID,
		CancelReason:          instance.CancelReason,
		PlanningTrackName:     instance.PlanningTrackName,
		PlanningTrackColor:    instance.PlanningTrackColor,
		GroupName:             instance.GroupName,
		StaffNames: owners.CalendarMapSlice(instance.StaffNames, func(name timetable.OperationStaffName) supervisiondashboard.StaffName {
			return supervisiondashboard.StaffName{StaffID: name.StaffID, DisplayName: name.DisplayName, IsSubstitute: name.IsSubstitute}
		}),
	}
}

func supervisionRosterRow(row timetable.OperationRosterRow) supervisiondashboard.RosterRow {
	var parallel *supervisiondashboard.ParallelPresence
	if in := row.ParallelPresentIn; in != nil {
		parallel = &supervisiondashboard.ParallelPresence{InstanceID: in.InstanceID, Title: in.Title, StartTime: in.StartTime, EndTime: in.EndTime}
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
		Warnings: owners.CalendarMapSlice(row.Warnings, func(warning timetable.OperationRosterWarning) supervisiondashboard.RosterWarning {
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

// supervisionConflictWarning carries the Timetable owner's warning onto the
// page; the two shapes are field for field the same wire contract.
func supervisionConflictWarning(warning timetable.InstanceConflictWarning) supervisiondashboard.ConflictWarning {
	return supervisiondashboard.ConflictWarning(warning)
}

// newSupervisionPresence binds the dashboard's presence port to Student
// Presence; without the presence service it binds none. The open-visit
// reader is optional and checked when OpenVisitsOfSessions is asked.
func newSupervisionPresence(active studentpresence.Presence, openVisits studentpresence.SessionReads) supervisiondashboard.Presence {
	if active == nil {
		return nil
	}
	return supervisionPresence{active: active, openVisits: openVisits}
}

type supervisionPresence struct {
	active     studentpresence.Presence
	openVisits studentpresence.SessionReads
}

func (p supervisionPresence) GroupVisits(ctx context.Context, activeGroupID int64) ([]supervisiondashboard.VisitRecord, error) {
	rows, err := p.active.GetActiveGroupVisitsWithDisplay(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	return supervisionVisitRecords(rows), nil
}

func supervisionVisitRecords(rows []*studentpresence.VisitDisplay) []supervisiondashboard.VisitRecord {
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

func (p supervisionPresence) OpenVisitsOfSessions(ctx context.Context, activeGroupIDs []int64) ([]supervisiondashboard.VisitRecord, error) {
	if p.openVisits == nil {
		return nil, errors.New("open-room visit reader is not configured")
	}
	rows, err := p.openVisits.GetActiveGroupVisitsWithDisplayForGroups(ctx, activeGroupIDs)
	if err != nil {
		return nil, err
	}
	return supervisionVisitRecords(rows), nil
}

func (p supervisionPresence) AttendanceTimes(ctx context.Context, studentIDs []int64) (map[int64]supervisiondashboard.Attendance, error) {
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

func (p supervisionPresence) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}

// newSupervisionPlanning binds the dashboard's planning port to Care Plan;
// without one of its sources it binds none.
func newSupervisionPlanning(pickups careplan.BulkPickupDays, arrivals careplan.BulkArrivalTimes) supervisiondashboard.Planning {
	if pickups == nil || arrivals == nil {
		return nil
	}
	return supervisionPlanning{pickups: pickups, arrivals: arrivals}
}

type supervisionPlanning struct {
	pickups  careplan.BulkPickupDays
	arrivals careplan.BulkArrivalTimes
}

func (p supervisionPlanning) Pickups(ctx context.Context, studentIDs []int64, date supervisiondashboard.Date) (map[int64]supervisiondashboard.Pickup, error) {
	day, err := supervisionDay(date)
	if err != nil {
		return nil, err
	}
	effective, err := p.pickups.GetBulkEffectivePickupDaysForDate(ctx, studentIDs, day)
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
			PickupTime:  supervisionWallClock(pickup.PickupTime),
			IsException: pickup.IsException, DayDepartureMode: pickup.DayDepartureMode,
			Notes: pickup.Notes,
			DayNotes: owners.CalendarMapSlice(pickup.DayNotes, func(note careplan.NoteData) supervisiondashboard.DayNote {
				return supervisiondashboard.DayNote{ID: note.ID, Content: note.Content}
			}),
		}
	}
	return result, nil
}

func (p supervisionPlanning) Arrivals(ctx context.Context, studentIDs []int64, date supervisiondashboard.Date) (map[int64]supervisiondashboard.Arrival, error) {
	day, err := supervisionDay(date)
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
			ArrivalTime: supervisionWallClock(arrival.ArrivalTime),
			IsException: arrival.IsException,
			Notes:       arrival.Notes,
			DayNotes: owners.CalendarMapSlice(arrival.DayNotes, func(note careplan.ArrivalNoteData) supervisiondashboard.DayNote {
				return supervisiondashboard.DayNote{ID: note.ID, Content: note.Content}
			}),
		}
	}
	return result, nil
}

// supervisionWallClock renders a normalized wall-clock value as HH:MM. The
// retained plans map PostgreSQL TIME columns: the clock lives in the value
// itself (timezone.NormalizeWallClock), so converting it to Berlin would be
// wrong.
func supervisionWallClock(at *time.Time) *string {
	if at == nil {
		return nil
	}
	formatted := at.Format("15:04")
	return &formatted
}

// supervisionCalendar binds the dashboard's calendar port to the shared
// kernel's Berlin calendar.
type supervisionCalendar struct{}

func (supervisionCalendar) DayOf(at time.Time) supervisiondashboard.Date {
	return supervisiondashboard.Date(calendarpkg.DateFromTime(at).String())
}

func (supervisionCalendar) Weekday(date supervisiondashboard.Date) time.Weekday {
	return calendarpkg.Date(date).Weekday()
}

func (supervisionCalendar) Clock(at time.Time) string {
	return *calendarpkg.FormatBerlinClock(&at)
}

// supervisionDay converts the projection's calendar day into the owners'
// date value.
func supervisionDay(date supervisiondashboard.Date) (calendarpkg.Date, error) {
	return calendarpkg.ParseDate(string(date))
}

// newSupervisionTeacherHints binds People Directory's day hints of the
// Lehrkraft (#4795); without the directory no hint is attached.
func newSupervisionTeacherHints(hints peopledirectory.SchoolDayHintQuery) supervisiondashboard.TeacherHintReader {
	if hints == nil {
		return nil
	}
	return supervisionTeacherHints{hints: hints}
}

type supervisionTeacherHints struct {
	hints peopledirectory.SchoolDayHintQuery
}

func (b supervisionTeacherHints) TeacherHints(ctx context.Context, studentIDs []int64, date supervisiondashboard.Date) ([]supervisiondashboard.TeacherHint, error) {
	day, err := calendarpkg.ParseDate(string(date))
	if err != nil {
		return nil, err
	}
	rows, err := teacherHintsByStudent(ctx, b.hints, day, studentIDs, func(hint peopledirectory.SchoolDayHint) supervisiondashboard.TeacherHint {
		return supervisiondashboard.TeacherHint{
			StudentID: hint.StudentID, Body: hint.Body, AuthorName: hint.AuthorName, SchoolClass: hint.SchoolClass,
		}
	})
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.TeacherHint, 0, len(rows))
	for _, id := range studentIDs {
		result = append(result, rows[id]...)
	}
	return result, nil
}
