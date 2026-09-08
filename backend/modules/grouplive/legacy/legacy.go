// Package legacy adapts the retained owner services to the live-group
// projection's consumer-owned ports (#2702). It exists only because those
// owners (identity, settings, presence rules, day planning, care
// participation, substitutions) still live in legacy service packages; the
// adapters translate rows into plain records and delegate every rule to its
// owner, deciding nothing themselves. Delete this package with the last
// legacy source once each owner exposes the fact through its public
// capability.
package legacy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Sources are the retained owner services the projection's ports adapt.
type Sources struct {
	Presence          studentpresence.Query
	People            userService.PersonService
	Education         educationService.Service
	Substitutions     educationService.SubstitutionModule
	UserContext       userContextService.UserContextService
	Active            activeService.Service
	Settings          configService.SettingsService
	Pickups           scheduleService.PickupScheduleService
	Arrivals          scheduleService.ArrivalScheduleService
	Instances         scheduleService.InstanceService
	CareDays          scheduleService.CareDayService
	CareParticipation userService.CareLifecycleService
	ExcusedRequests   grouplive.PendingExcusedReader
	StatusDays        *activeService.StudentStatusDayService
	Logger            *slog.Logger
	Now               func() time.Time
}

// ErrIncompleteSources reports a missing retained service. Missing wiring is
// a configuration error and must fail composition, not the first request.
var ErrIncompleteSources = errors.New("OGS group live adapters are not fully configured")

// New composes the live-group projection over the retained sources. Logger,
// Now, and ExcusedRequests are optional; every other source is required.
func New(sources Sources) (grouplive.Query, error) {
	if sources.Presence == nil || sources.People == nil || sources.Education == nil ||
		sources.Substitutions == nil || sources.UserContext == nil || sources.Active == nil ||
		sources.Settings == nil || sources.Pickups == nil || sources.Arrivals == nil ||
		sources.Instances == nil || sources.CareDays == nil || sources.CareParticipation == nil ||
		sources.StatusDays == nil {
		return nil, ErrIncompleteSources
	}
	if sources.Now == nil {
		sources.Now = time.Now
	}
	return grouplive.New(grouplive.Dependencies{
		Access:          access{settings: sources.Settings, userContext: sources.UserContext},
		Groups:          directory{education: sources.Education, userContext: sources.UserContext},
		Roster:          roster{people: sources.People, careParticipation: sources.CareParticipation},
		Presence:        presence{presence: sources.Presence, active: sources.Active, statusDays: sources.StatusDays},
		Planning:        planning{arrivals: sources.Arrivals, pickups: sources.Pickups, instances: sources.Instances, careDays: sources.CareDays},
		Transfers:       transfers{substitutions: sources.Substitutions},
		Settings:        settings{settings: sources.Settings},
		Calendar:        calendar{now: sources.Now},
		ExcusedRequests: sources.ExcusedRequests,
		Logger:          sources.Logger,
	}), nil
}

type access struct {
	settings    configService.SettingsService
	userContext userContextService.UserContextService
}

func (a access) Caller(ctx context.Context) (grouplive.Caller, error) {
	principal, principalErr := permissions.PrincipalFromContext(ctx)
	assignmentBound := principalErr == nil && principal.Scope() == permissions.ScopeSchool
	admin := principalErr == nil && principal.HasAdminScope()
	overview, err := authorize.HasOperationalOverview(ctx, a.settings, a.userContext, assignmentBound, admin)
	if err != nil {
		return grouplive.Caller{}, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	userPermissions := jwt.PermissionsFromCtx(ctx)
	return grouplive.Caller{
		CanReadGroups:            authorize.HasPermission(permissions.GroupsRead, userPermissions),
		CanReviewExcusedRequests: authorize.CanReviewExcusedAbsenceRequests(userPermissions),
		OperationalOverview:      overview,
	}, nil
}

func (a access) FullStudentAccess(ctx context.Context) (bool, error) {
	return userContextService.ResolveStudentAccess(ctx, a.userContext).HasFullAccess(), nil
}

// parseDay converts the projection's calendar day into the retained
// signatures' date value.
func parseDay(date grouplive.Date) (timezone.Date, error) {
	return timezone.ParseDate(string(date))
}

type directory struct {
	education   educationService.Service
	userContext userContextService.UserContextService
}

func (d directory) SupervisedGroups(ctx context.Context) ([]grouplive.GroupRecord, error) {
	groups, err := d.userContext.GetMyGroups(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]grouplive.GroupRecord, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			records = append(records, grouplive.GroupRecord{ID: group.ID, Name: group.Name, RoomID: group.RoomID})
		}
	}
	return sortedRecords(records), nil
}

func (d directory) TenantGroups(ctx context.Context) ([]grouplive.GroupRecord, error) {
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
	return sortedRecords(records), nil
}

// sortedRecords orders groups in German dictionary order (DIN 5007-1), the
// order the projection's group selection and response rely on.
func sortedRecords(records []grouplive.GroupRecord) []grouplive.GroupRecord {
	sort.SliceStable(records, func(i, j int) bool {
		return collation.CompareGerman(records[i].Name, records[j].Name) < 0
	})
	return records
}

func (d directory) SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error) {
	return d.userContext.GetSubstitutedGroupIDs(ctx)
}

func (d directory) GroupRoomNames(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
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

type roster struct {
	people            userService.PersonService
	careParticipation userService.CareLifecycleService
}

func (r roster) GroupMembers(ctx context.Context, groupID int64) ([]grouplive.RosterStudent, error) {
	students, err := r.people.GetParticipationCandidatesByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return []grouplive.RosterStudent{}, nil
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := r.people.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("bulk load persons: %w", err)
	}
	members := make([]grouplive.RosterStudent, 0, len(students))
	for _, student := range students {
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		member := grouplive.RosterStudent{
			ID: student.ID, FirstName: person.FirstName, LastName: person.LastName,
			SchoolClass: student.SchoolClass, SickSince: student.SickSince, ExcusedSince: student.ExcusedSince,
			PhotoPath: student.PhotoPath,
		}
		if student.Sick != nil {
			member.Sick = *student.Sick
		}
		if student.Excused != nil {
			member.Excused = *student.Excused
		}
		members = append(members, member)
	}
	return members, nil
}

func (r roster) CareParticipants(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]bool, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	return r.careParticipation.ParticipatingStudentIDs(ctx, studentIDs, day, nil)
}

type presence struct {
	presence   studentpresence.Query
	active     activeService.Service
	statusDays *activeService.StudentStatusDayService
}

func (p presence) Snapshot(ctx context.Context, studentIDs []int64, date grouplive.Date) (grouplive.PresenceSnapshot, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	mode, err := p.active.GetPresenceMode(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := activeService.NewStudentLocationSnapshot(mode)
	ids := slices.Compact(slices.Sorted(slices.Values(studentIDs)))
	if len(ids) == 0 {
		return locationSnapshot{snapshot: snapshot}, nil
	}
	if p.presence == nil {
		return nil, errors.New("student presence reader is required")
	}
	attendances, err := p.presence.ListSchoolStatuses(ctx, ids, day.String())
	if err != nil {
		return nil, err
	}
	for _, attendance := range attendances {
		snapshot.Attendances[attendance.StudentID] = &activeService.AttendanceStatus{
			StudentID: attendance.StudentID, Date: day, Status: attendance.Status,
			CheckInTime: attendance.CheckInTime, CheckOutTime: attendance.CheckOutTime, YardSince: attendance.YardSince,
		}
	}
	if mode == activeService.PresenceModeBinary {
		snapshot.YardRoomColor = activeService.ResolveYardRoomColor(ctx, p.active)
		return locationSnapshot{snapshot: snapshot}, nil
	}
	if err := p.loadVisits(ctx, snapshot, ids); err != nil {
		return nil, err
	}
	return locationSnapshot{snapshot: snapshot}, nil
}

func (p presence) loadVisits(ctx context.Context, snapshot *activeService.StudentLocationSnapshot, studentIDs []int64) error {
	visits, err := p.presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true, StudentOrder: true, NewestFirst: true})
	if err != nil {
		return err
	}
	for _, visit := range visits {
		if _, found := snapshot.Visits[visit.StudentID]; found {
			continue
		}
		snapshot.Visits[visit.StudentID] = &visit
	}
	groupSet := make(map[int64]struct{})
	for _, visit := range snapshot.Visits {
		if visit != nil && visit.ActiveGroupID > 0 {
			groupSet[visit.ActiveGroupID] = struct{}{}
		}
	}
	if len(groupSet) == 0 {
		return nil
	}
	groups, err := p.active.GetActiveGroupsByIDs(ctx, slices.Collect(maps.Keys(groupSet)))
	if err != nil {
		return err
	}
	if groups != nil {
		snapshot.Groups = groups
	}
	return nil
}

type locationSnapshot struct {
	snapshot *activeService.StudentLocationSnapshot
}

func (s locationSnapshot) Location(studentID int64, fullAccess bool) grouplive.Location {
	info := s.snapshot.ResolveStudentLocationWithTime(studentID, fullAccess)
	return grouplive.Location{Name: info.Location, Since: info.Since, RoomColor: info.RoomColor}
}

func (s locationSnapshot) Attendance(studentID int64) (grouplive.Attendance, bool) {
	status := s.snapshot.Attendances[studentID]
	if status == nil {
		return grouplive.Attendance{}, false
	}
	return grouplive.Attendance{
		Recorded: true, Present: status.IsCurrentlyPresent(),
		CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime,
	}, true
}

func (s locationSnapshot) CurrentRoomID(studentID int64) *int64 {
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

func (p presence) EffectiveStatuses(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.EffectiveStatus, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	rows, err := p.statusDays.GetActiveByStudentIDsAndDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	byStudent := make(map[int64][]*activeModels.StudentStatusDay)
	for _, row := range rows {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}
	result := make(map[int64]grouplive.EffectiveStatus, len(byStudent))
	for studentID, statusRows := range byStudent {
		status := activeService.ResolveEffectiveStatus(statusRows)
		result[studentID] = grouplive.EffectiveStatus{
			Sick: status.Sick, ClassTrip: status.ClassTrip, Excused: status.Excused,
			SickSince: status.SickSince, ClassTripSince: status.ClassTripSince, ExcusedSince: status.ExcusedSince,
		}
	}
	return result, nil
}

func (p presence) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}

type planning struct {
	arrivals  scheduleService.ArrivalScheduleService
	pickups   scheduleService.PickupScheduleService
	instances scheduleService.InstanceService
	careDays  scheduleService.CareDayService
}

func (p planning) Arrivals(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.Arrival, error) {
	day, err := parseDay(date)
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
			result[studentID] = arrivalRecord(arrival)
		}
	}
	return result, nil
}

func arrivalRecord(arrival *scheduleService.EffectiveArrivalTime) grouplive.Arrival {
	notes := make([]string, 0, len(arrival.DayNotes))
	for _, note := range arrival.DayNotes {
		notes = append(notes, note.Content)
	}
	return grouplive.Arrival{ArrivalTime: arrival.ArrivalTime, IsException: arrival.IsException, Notes: arrival.Notes, DayNotes: notes}
}

func (p planning) Pickups(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]grouplive.Pickup, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	pickups, err := p.pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]grouplive.Pickup, len(pickups))
	for studentID, pickup := range pickups {
		if pickup != nil {
			result[studentID] = pickupRecord(pickup)
		}
	}
	return result, nil
}

func pickupRecord(pickup *scheduleService.EffectivePickupTime) grouplive.Pickup {
	notes := make([]grouplive.DayNote, 0, len(pickup.DayNotes))
	for _, note := range pickup.DayNotes {
		notes = append(notes, grouplive.DayNote{ID: note.ID, Content: note.Content})
	}
	return grouplive.Pickup{
		Date: grouplive.Date(pickup.Date.String()), WeekdayName: pickup.WeekdayName, PickupTime: pickup.PickupTime,
		IsException: pickup.IsException, Notes: pickup.Notes, DayNotes: notes,
	}
}

func (p planning) TimetablePlannedStudentIDs(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64]bool, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	planned, err := p.instances.GetPlannedStudentIDsByDate(ctx, studentIDs, day)
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
func (p planning) DecideDay(inputs grouplive.DayInputs) grouplive.DayDecision {
	decisionInputs := scheduleService.DayPlanningInputs{
		HasActualAttendance: inputs.Present, Sick: inputs.Sick, ClassTrip: inputs.ClassTrip,
		Excused: inputs.Excused, HasTimetable: inputs.HasTimetable,
	}
	if inputs.Arrival != nil {
		decisionInputs.Arrival = &scheduleService.EffectiveArrivalTime{
			ArrivalTime: inputs.Arrival.ArrivalTime, IsException: inputs.Arrival.IsException, Notes: inputs.Arrival.Notes,
		}
	}
	if inputs.Pickup != nil {
		decisionInputs.Pickup = &scheduleService.EffectivePickupTime{
			PickupTime: inputs.Pickup.PickupTime, IsException: inputs.Pickup.IsException, Notes: inputs.Pickup.Notes,
		}
	}
	decision := scheduleService.ResolveDayPlanning(decisionInputs)
	return grouplive.DayDecision{ComesToday: decision.ComesToday, Reason: decision.Reason, ExceptionNotes: decision.ExceptionNotes}
}

type transfers struct {
	substitutions educationService.SubstitutionModule
}

func (t transfers) GroupHandovers(ctx context.Context, groupID int64, date grouplive.Date) ([]grouplive.Transfer, error) {
	day, err := parseDay(date)
	if err != nil {
		return nil, err
	}
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	caller := educationService.SubstitutionCaller{
		AccountID: principal.AccountID(), TenantID: principal.TenantID(), Scope: string(principal.Scope()),
		Roles: principal.Roles(), Admin: principal.HasAdminScope(),
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

type settings struct{ settings configService.SettingsService }

func (s settings) Prepare(ctx context.Context) (context.Context, error) {
	batch, ok := s.settings.(configService.BatchSettingsService)
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveMany(ctx, []string{
		configModel.KeyOperationalOverviewScope,
		configModel.KeyEnrollmentBookingsAuthoritative,
		configModel.KeyPresenceMode,
		configModel.KeyStudentPhotosEnabled,
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

type calendar struct{ now func() time.Time }

func (c calendar) Today() grouplive.Date {
	return grouplive.Date(timezone.DateFromTime(c.now()).String())
}

func (c calendar) Clock(at time.Time) string { return at.In(timezone.Berlin).Format("15:04") }
