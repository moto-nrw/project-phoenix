package api

// The root binds the live-group projection's consumer-owned ports (#2702) to
// the owners' reads and only translates their records: Identity & Access for
// the caller and its groups, the Security Runtime for the overview and
// permission rules, the Settings Platform for the tenant settings, School
// Structure for the groups and handovers, Student Presence for attendance,
// visits and status days, and Care Plan for arrivals, pickups, care days and
// the shared day-planning precedence. The projection names none of these
// owners (#4077); the translation is cross-owner glue and lives here.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	groupliveCompose "github.com/moto-nrw/project-phoenix/modules/grouplive/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	educationService "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// GroupLiveCallerContext is the Identity & Access caller context the
// live-group adapters read.
type GroupLiveCallerContext interface {
	HasCurrentStaff(context.Context) (bool, error)
	HasFullStudentAccess(context.Context) bool
	MyGroupRecords(context.Context) ([]grouplive.GroupRecord, error)
	GetSubstitutedGroupIDs(context.Context) (map[int64]bool, error)
}

// groupLivePresenceRows is the Student Presence read of the school status
// and the visits.
type groupLivePresenceRows interface {
	ListSchoolStatuses(ctx context.Context, ids []int64, date string) ([]studentpresence.SchoolStatus, error)
	ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

// groupLiveSessions is the Student Presence read of the presence mode, the
// running sessions and the tracking indicators.
type groupLiveSessions interface {
	GetPresenceMode(ctx context.Context) (string, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*studentpresence.SessionDetail, error)
	GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)
}

// groupLiveStatusDays is the Student Presence read of the scheduled day
// statuses.
type groupLiveStatusDays interface {
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*absencerecords.StudentStatusDay, error)
}

// groupLiveGroups is the School Structure read of the groups, their rooms
// and their Gruppenleitung.
type groupLiveGroups interface {
	ListGroups(ctx context.Context, query *educationService.GroupListQuery) ([]*educationService.Group, error)
	GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*educationService.Group, error)
	GetTeachersForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*educationService.Teacher, error)
}

// groupLiveSubstitutions is the School Structure read of the handovers.
type groupLiveSubstitutions interface {
	Overview(ctx context.Context, caller educationService.SubstitutionCaller, query educationService.OverviewQuery) (*educationService.OverviewResult, error)
}

// groupLivePickups, groupLiveArrivals, groupLiveCareDays and
// groupLiveCareParticipation are the Care Plan reads of the day plans.
type (
	groupLivePickups interface {
		GetBulkEffectivePickupDaysForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectivePickupTime, error)
	}
	groupLiveArrivals interface {
		GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
	}
	groupLiveCareDays interface {
		ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]careplan.CareDayStatus, error)
	}
	groupLiveCareParticipation interface {
		ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (map[int64]bool, error)
	}
)

// groupLiveSources are the owner reads the root binds to the live-group
// projection. Logger, Now and ExcusedRequests are optional; every other
// source is required.
type groupLiveSources struct {
	Presence          groupLivePresenceRows
	People            GroupRoster
	Education         groupLiveGroups
	Substitutions     groupLiveSubstitutions
	UserContext       GroupLiveCallerContext
	Active            groupLiveSessions
	Settings          grouplive.Resolver
	Pickups           groupLivePickups
	Arrivals          groupLiveArrivals
	PlannedStudentIDs func(context.Context, []int64, calendar.Date) ([]int64, error)
	CareDays          groupLiveCareDays
	CareParticipation groupLiveCareParticipation
	ExcusedRequests   grouplive.PendingExcusedReader
	StatusDays        groupLiveStatusDays
	Logger            *slog.Logger
	Now               func() time.Time
}

// errGroupLiveSources reports a missing owner read. Missing wiring is a
// configuration error and must fail composition, not the first request.
var errGroupLiveSources = errors.New("OGS group live adapters are not fully configured")

// newGroupLive binds the owner reads to the live-group projection's ports.
func newGroupLive(sources groupLiveSources) (grouplive.Query, error) {
	if sources.Presence == nil || sources.People == nil || sources.Education == nil ||
		sources.Substitutions == nil || sources.UserContext == nil || sources.Active == nil ||
		sources.Settings == nil || sources.Pickups == nil || sources.Arrivals == nil ||
		sources.PlannedStudentIDs == nil || sources.CareDays == nil || sources.CareParticipation == nil ||
		sources.StatusDays == nil {
		return nil, errGroupLiveSources
	}
	if sources.Now == nil {
		sources.Now = time.Now
	}
	return groupliveCompose.New(grouplive.Dependencies{
		Access:          groupLiveAccess{settings: sources.Settings, userContext: sources.UserContext},
		Groups:          groupLiveDirectory{education: sources.Education, userContext: sources.UserContext},
		Roster:          groupLiveRoster{GroupRoster: sources.People, careParticipation: sources.CareParticipation},
		Presence:        groupLivePresence{presence: sources.Presence, active: sources.Active, statusDays: sources.StatusDays},
		Planning:        groupLivePlanning{arrivals: sources.Arrivals, pickups: sources.Pickups, planned: sources.PlannedStudentIDs, careDays: sources.CareDays},
		Transfers:       groupLiveTransfers{substitutions: sources.Substitutions},
		Settings:        groupLiveSettings{settings: sources.Settings},
		Calendar:        groupLiveCalendar{now: sources.Now},
		ExcusedRequests: sources.ExcusedRequests,
		Logger:          sources.Logger,
	}), nil
}

type groupLiveAccess struct {
	settings    grouplive.Resolver
	userContext GroupLiveCallerContext
}

func (a groupLiveAccess) Caller(ctx context.Context) (grouplive.Caller, error) {
	assignmentBound := common.IsAssignmentBoundPortal(ctx)
	admin := common.HasEffectiveAdminScope(ctx)
	overview, err := securityruntime.CanViewOperationalOverview(ctx, a.settings, a.userContext, assignmentBound, admin)
	if err != nil {
		return grouplive.Caller{}, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	userPermissions := sessiontoken.PermissionsFromCtx(ctx)
	return grouplive.Caller{
		CanReadStudentPlans:      securityruntime.HasAdminWildcard(userPermissions) || securityruntime.HasPermission(permissions.UsersRead, userPermissions),
		CanReadGroups:            securityruntime.HasPermission(groupLiveHTTP.GroupReadPermission, userPermissions),
		CanReviewExcusedRequests: securityruntime.CanReviewExcusedAbsenceRequests(userPermissions),
		OperationalOverview:      overview,
	}, nil
}

func (a groupLiveAccess) FullStudentAccess(ctx context.Context) (bool, error) {
	return a.userContext.HasFullStudentAccess(ctx), nil
}

// groupLiveDay converts the projection's calendar day into the owners' date
// value.
func groupLiveDay(date grouplive.Date) (calendar.Date, error) {
	return calendar.ParseDate(string(date))
}

type groupLiveDirectory struct {
	education   groupLiveGroups
	userContext GroupLiveCallerContext
}

func (d groupLiveDirectory) SupervisedGroups(ctx context.Context) ([]grouplive.GroupRecord, error) {
	return d.userContext.MyGroupRecords(ctx)
}

func (d groupLiveDirectory) TenantGroups(ctx context.Context) ([]grouplive.GroupRecord, error) {
	groups, err := d.education.ListGroups(ctx, nil)
	if err != nil {
		return nil, err
	}
	records := make([]grouplive.GroupRecord, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			records = append(records, grouplive.GroupRecord{ID: group.ID, Name: group.Name, RoomID: group.RoomID})
		}
	}
	return records, nil
}

func (d groupLiveDirectory) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	return d.userContext.GetSubstitutedGroupIDs(ctx)
}

func (d groupLiveDirectory) GroupRoomNames(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	withRooms, err := d.education.GetGroupsWithRoomsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(withRooms))
	for id, group := range withRooms {
		if group != nil && group.Room != nil {
			names[id] = group.Room.Name
		}
	}
	return names, nil
}

func (d groupLiveDirectory) GroupLeaderNames(ctx context.Context, groupIDs []int64) (map[int64][]string, error) {
	teachers, err := d.education.GetTeachersForGroups(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64][]string, len(teachers))
	for id, groupTeachers := range teachers {
		for _, teacher := range groupTeachers {
			if teacher == nil {
				continue
			}
			if name := strings.TrimSpace(teacher.FullName()); name != "" {
				names[id] = append(names[id], name)
			}
		}
	}
	return names, nil
}

type groupLiveRoster struct {
	GroupRoster
	careParticipation groupLiveCareParticipation
}

func (r groupLiveRoster) CareParticipants(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]bool, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	return r.careParticipation.ParticipatingStudentIDs(ctx, studentIDs, day, nil)
}

type groupLivePresence struct {
	presence   groupLivePresenceRows
	active     groupLiveSessions
	statusDays groupLiveStatusDays
}

func (p groupLivePresence) Snapshot(ctx context.Context, studentIDs []int64, date grouplive.Date) (grouplive.PresenceSnapshot, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	mode, err := p.active.GetPresenceMode(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := studentpresence.NewStudentLocationSnapshot(mode)
	ids := slices.Compact(slices.Sorted(slices.Values(studentIDs)))
	if len(ids) == 0 {
		return groupLiveLocations{snapshot: snapshot}, nil
	}
	if p.presence == nil {
		return nil, errors.New("student presence reader is required")
	}
	attendances, err := p.presence.ListSchoolStatuses(ctx, ids, day.String())
	if err != nil {
		return nil, err
	}
	for _, attendance := range attendances {
		snapshot.Attendances[attendance.StudentID] = &studentpresence.DailyAttendanceStatus{
			StudentID: attendance.StudentID, Date: day, Status: attendance.Status,
			CheckInTime: attendance.CheckInTime, CheckOutTime: attendance.CheckOutTime, YardSince: attendance.YardSince,
		}
	}
	if !snapshot.ReadsRooms() {
		snapshot.YardRoomColor = studentpresence.ResolveYardRoomColor(ctx, p.active)
		return groupLiveLocations{snapshot: snapshot}, nil
	}
	if err := p.loadVisits(ctx, snapshot, ids); err != nil {
		return nil, err
	}
	return groupLiveLocations{snapshot: snapshot}, nil
}

func (p groupLivePresence) loadVisits(ctx context.Context, snapshot *studentpresence.StudentLocationSnapshot, studentIDs []int64) error {
	visits, err := p.presence.ListVisits(ctx, studentpresence.CurrentVisitsFilter(studentIDs))
	if err != nil {
		return err
	}
	snapshot.KeepCurrentVisits(visits)
	groupIDs := snapshot.ActiveGroupIDs()
	if len(groupIDs) == 0 {
		return nil
	}
	groups, err := p.active.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return err
	}
	if groups != nil {
		snapshot.Groups = groups
	}
	return nil
}

type groupLiveLocations struct {
	snapshot *studentpresence.StudentLocationSnapshot
}

func (s groupLiveLocations) Location(studentID int64, fullAccess bool) grouplive.Location {
	info := s.snapshot.ResolveStudentLocationWithTime(studentID, fullAccess)
	return grouplive.Location{Name: info.Location, Since: info.Since, RoomColor: info.RoomColor}
}

func (s groupLiveLocations) Attendance(studentID int64) (grouplive.Attendance, bool) {
	status := s.snapshot.Attendances[studentID]
	if status == nil {
		return grouplive.Attendance{}, false
	}
	return grouplive.Attendance{
		Recorded: true, Present: status.IsCurrentlyPresent(),
		CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime,
	}, true
}

func (s groupLiveLocations) CurrentRoomID(studentID int64) *int64 {
	visit := s.snapshot.Visits[studentID]
	if visit == nil {
		return nil
	}
	group := s.snapshot.Groups[visit.ActiveGroupID]
	if group == nil {
		return nil
	}
	roomID := group.RoomID
	return &roomID
}

func (p groupLivePresence) EffectiveStatuses(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.EffectiveStatus, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	rows, err := p.statusDays.FindActiveByStudentIDsAndDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	byStudent := make(map[int64][]*absencerecords.StudentStatusDay)
	for _, row := range rows {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}
	result := make(map[int64]grouplive.EffectiveStatus, len(byStudent))
	for studentID, statusRows := range byStudent {
		status := studentpresence.ResolveEffectiveStatus(statusRows)
		result[studentID] = grouplive.EffectiveStatus{
			Sick: status.Sick, ClassTrip: status.ClassTrip, Excused: status.Excused,
			SickSince: status.SickSince, ClassTripSince: status.ClassTripSince, ExcusedSince: status.ExcusedSince,
		}
	}
	return result, nil
}

func (p groupLivePresence) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}

type groupLivePlanning struct {
	arrivals groupLiveArrivals
	pickups  groupLivePickups
	planned  func(context.Context, []int64, calendar.Date) ([]int64, error)
	careDays groupLiveCareDays
}

func (p groupLivePlanning) Arrivals(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.Arrival, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	arrivals, err := p.arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]grouplive.Arrival, len(arrivals))
	for studentID, arrival := range arrivals {
		if arrival != nil {
			result[studentID] = groupLiveArrival(arrival)
		}
	}
	return result, nil
}

func groupLiveArrival(arrival *careplan.EffectiveArrivalTime) grouplive.Arrival {
	notes := make([]string, 0, len(arrival.DayNotes))
	for _, note := range arrival.DayNotes {
		notes = append(notes, note.Content)
	}
	return grouplive.Arrival{ArrivalTime: arrival.ArrivalTime, IsException: arrival.IsException, Notes: arrival.Notes, DayNotes: notes}
}

func (p groupLivePlanning) Pickups(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.Pickup, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	pickups, err := p.pickups.GetBulkEffectivePickupDaysForDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]grouplive.Pickup, len(pickups))
	for studentID, pickup := range pickups {
		if pickup != nil {
			result[studentID] = groupLivePickup(pickup)
		}
	}
	return result, nil
}

func groupLivePickup(pickup *careplan.EffectivePickupTime) grouplive.Pickup {
	notes := make([]grouplive.DayNote, 0, len(pickup.DayNotes))
	for _, note := range pickup.DayNotes {
		notes = append(notes, grouplive.DayNote{ID: note.ID, Content: note.Content})
	}
	return grouplive.Pickup{
		Date: grouplive.Date(pickup.Date.String()), WeekdayName: pickup.WeekdayName, PickupTime: pickup.PickupTime,
		IsException: pickup.IsException, Notes: pickup.Notes, DayNotes: notes, DayDepartureMode: pickup.DayDepartureMode,
	}
}

func (p groupLivePlanning) TimetablePlannedStudentIDs(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]bool, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	planned, err := p.planned(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(planned))
	for _, id := range planned {
		result[id] = true
	}
	if len(result) == 0 {
		return result, nil
	}
	careDays, err := p.careDays.ResolveForDate(ctx, planned, day)
	if err != nil {
		return nil, err
	}
	for id := range result {
		if !careDays[id].Expected() {
			delete(result, id)
		}
	}
	return result, nil
}

// DecideDay routes the projection through the shared day-planning
// precedence so it never disagrees with the student search or the
// timetable's care-day derivation.
func (p groupLivePlanning) DecideDay(inputs grouplive.DayInputs) grouplive.DayDecision {
	decisionInputs := careplan.DayPlanningInputs{
		HasActualAttendance: inputs.Present, Sick: inputs.Sick, ClassTrip: inputs.ClassTrip,
		Excused: inputs.Excused, HasTimetable: inputs.HasTimetable,
	}
	if inputs.Arrival != nil {
		decisionInputs.Arrival = &careplan.EffectiveArrivalTime{
			ArrivalTime: inputs.Arrival.ArrivalTime, IsException: inputs.Arrival.IsException, Notes: inputs.Arrival.Notes,
		}
	}
	if inputs.Pickup != nil {
		decisionInputs.Pickup = &careplan.EffectivePickupTime{
			PickupTime: inputs.Pickup.PickupTime, IsException: inputs.Pickup.IsException, Notes: inputs.Pickup.Notes,
		}
	}
	decision := careplan.ResolveDayPlanning(decisionInputs)
	return grouplive.DayDecision{ComesToday: decision.ComesToday, Reason: decision.Reason, ExceptionNotes: decision.ExceptionNotes}
}

// AtSchoolBeforeCheckIn delegates to the day-planning owner's "Schule" rule.
func (p groupLivePlanning) AtSchoolBeforeCheckIn(inputs grouplive.AtSchoolInputs) bool {
	return careplan.IsAtSchoolBeforeCheckIn(careplan.AtSchoolInputs{
		Decision: careplan.DayPlanningDecision{
			ComesToday: inputs.Decision.ComesToday, Reason: inputs.Decision.Reason,
		},
		CheckedInToday: inputs.CheckedIn,
		PickupTime:     inputs.PickupTime,
		CareDayEnd:     inputs.CareDayEnd,
		Now:            inputs.Now,
	})
}

type groupLiveTransfers struct {
	substitutions groupLiveSubstitutions
}

func (t groupLiveTransfers) GroupHandovers(ctx context.Context, groupID int64, date grouplive.Date) ([]grouplive.Transfer, error) {
	day, err := groupLiveDay(date)
	if err != nil {
		return nil, err
	}
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	caller := educationService.SubstitutionCaller{
		AccountID: principal.AccountID(), TenantID: principal.TenantID(), Scope: string(principal.Scope()),
		Roles: principal.Roles(), Admin: principal.HasAdminScope(), HasPermission: principal.HasPermission,
	}
	overview, err := t.substitutions.Overview(ctx, caller, educationService.OverviewQuery{GroupID: groupID, On: &day})
	if err != nil {
		return nil, err
	}
	result := make([]grouplive.Transfer, 0, len(overview.GroupHandovers))
	for _, handover := range overview.GroupHandovers {
		result = append(result, grouplive.Transfer{
			ID:                handover.ID,
			GroupID:           handover.Group.ID,
			SubstituteStaffID: handover.Target.ID,
			SubstituteName:    handover.Target.FullName,
			EndDate:           handover.Period.EndDate,
		})
	}
	return result, nil
}

type groupLiveSettings struct{ settings grouplive.Resolver }

func (s groupLiveSettings) Prepare(ctx context.Context) (context.Context, error) {
	return s.settings.Prefetch(ctx, []string{
		settingsplatform.KeyOperationalOverviewScope,
		settingsplatform.KeyEnrollmentBookingsAuthoritative,
		settingsplatform.KeyPresenceMode,
		settingsplatform.KeySessionEndTime,
		settingsplatform.KeyStudentPhotosEnabled,
		settingsplatform.KeyTrackingIndicatorsEnabled,
		settingsplatform.KeyTrackingIndicator1,
		settingsplatform.KeyTrackingIndicator2,
		settingsplatform.KeyTrackingIndicator3,
	})
}

func (s groupLiveSettings) CareDayEnd(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, settingsplatform.KeySessionEndTime)
}

func (s groupLiveSettings) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, settingsplatform.KeyStudentPhotosEnabled)
}

func (s groupLiveSettings) TrackingIndicatorLabels(ctx context.Context) ([]string, error) {
	return settingsplatform.PrefetchedTrackingIndicatorLabels(ctx, s.settings)
}

type groupLiveCalendar struct{ now func() time.Time }

func (c groupLiveCalendar) Today() grouplive.Date {
	return grouplive.Date(calendar.DateFromTime(c.now()).String())
}

func (c groupLiveCalendar) Clock(at time.Time) string { return at.In(calendar.Berlin).Format("15:04") }

func (c groupLiveCalendar) Now() time.Time { return c.now() }
