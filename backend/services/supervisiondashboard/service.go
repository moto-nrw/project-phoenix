// Package supervisiondashboard builds the complete live projection for the
// "Aktuelle Aufsicht" (active supervision) page in one request (#2096). It
// replaces the Next.js BFF fan-out of ~11 backend calls and owns the
// projection's authorization and strict error contract: mandatory sub-loads
// fail the whole request instead of degrading to silently-empty sections.
package supervisiondashboard

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
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// ErrForbiddenGroup indicates that the requested active group is outside the
// caller's operational scope (supervised sessions, or all sessions under
// admin overview / open care, plus a supervised Schulhof session).
var ErrForbiddenGroup = errors.New("caller does not supervise the requested active group")

const studentPhotoStoredURLPrefix = "/uploads/student-photos/"

type Getter interface {
	Get(ctx context.Context, requestedGroupID int64) (*Projection, error)
}

type Dependencies struct {
	Active      activeService.Service
	UserContext userContextService.UserContextService
	Education   educationService.Service
	Schulhof    facilitiesService.SchulhofService
	Operations  scheduleService.TimetableOperationsService
	Settings    configService.SettingsService
	Pickups     scheduleService.PickupScheduleService
	Arrivals    scheduleService.ArrivalScheduleService
	Now         func() time.Time
}

type service struct{ deps Dependencies }

func NewService(deps Dependencies) Getter {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &service{deps: deps}
}

// Group is one supervised (or, under admin overview / open care, any active)
// session tab. The wire shape carries only what the page renders: the session
// label, room label, color, and ids for selection.
type Group struct {
	ID        int64   `json:"id,string"`
	Name      string  `json:"name"`
	RoomID    *int64  `json:"room_id,string,omitempty"`
	RoomName  string  `json:"room_name,omitempty"`
	RoomColor *string `json:"room_color,omitempty"`
}

type UnclaimedGroup struct {
	ID       int64  `json:"id,string"`
	RoomName string `json:"room_name,omitempty"`
}

type EducationalGroup struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	RoomName string `json:"room_name,omitempty"`
}

// Visit mirrors the fields the page renders from the former per-group
// /active/groups/{id}/visits/display endpoint, restricted to open visits.
type Visit struct {
	StudentID         int64      `json:"student_id,string"`
	StudentName       string     `json:"student_name"`
	SchoolClass       string     `json:"school_class"`
	GroupName         string     `json:"group_name"`
	ActiveGroupID     int64      `json:"active_group_id,string"`
	CheckInTime       time.Time  `json:"check_in_time"`
	ActualArrivalTime *string    `json:"actual_arrival_time,omitempty"`
	ActualPickupTime  *string    `json:"actual_pickup_time,omitempty"`
	Sick              bool       `json:"sick"`
	SickSince         *time.Time `json:"sick_since,omitempty"`
	Excused           bool       `json:"excused"`
	ExcusedSince      *time.Time `json:"excused_since,omitempty"`
	PhotoURL          string     `json:"photo_url,omitempty"`
}

type TrackingIndicators struct {
	Labels  []string         `json:"labels"`
	Results map[int64][]bool `json:"results"`
}

type DayNote struct {
	ID      int64  `json:"id,string"`
	Content string `json:"content"`
}

// PickupTime / ArrivalTime keep the field names of the bulk endpoints they
// replace so the frontend mapping stays identical.
type PickupTime struct {
	StudentID   int64     `json:"student_id,string"`
	Date        string    `json:"date"`
	WeekdayName string    `json:"weekday_name"`
	PickupTime  *string   `json:"pickup_time,omitempty"`
	IsException bool      `json:"is_exception"`
	Notes       string    `json:"notes,omitempty"`
	DayNotes    []DayNote `json:"day_notes,omitempty"`
}

type ArrivalTime struct {
	StudentID       int64     `json:"student_id,string"`
	Date            string    `json:"date"`
	WeekdayName     string    `json:"weekday_name"`
	ExpectedArrival *string   `json:"expected_arrival,omitempty"`
	IsException     bool      `json:"is_exception"`
	Notes           string    `json:"notes,omitempty"`
	DayNotes        []DayNote `json:"day_notes,omitempty"`
}

type Capabilities struct {
	WebSpontaneousActivitiesEnabled bool `json:"web_spontaneous_activities_enabled"`
}

type Projection struct {
	Groups            []Group            `json:"groups"`
	SelectedGroupID   *int64             `json:"selected_group_id,string,omitempty"`
	UnclaimedGroups   []UnclaimedGroup   `json:"unclaimed_groups"`
	CurrentStaffID    *int64             `json:"current_staff_id,string,omitempty"`
	EducationalGroups []EducationalGroup `json:"educational_groups"`
	// Schulhof is nil only when the caller has no staff identity (and thus no
	// Schulhof workflow) — never because a sub-load failed.
	Schulhof           *facilitiesService.SchulhofStatus          `json:"schulhof_status"`
	Capabilities       Capabilities                               `json:"capabilities"`
	ActiveSessions     []scheduleService.OperationActiveSession   `json:"active_sessions"`
	PlannedNow         []scheduleService.OperationPlannedInstance `json:"planned_now"`
	Visits             []Visit                                    `json:"visits"`
	TrackingIndicators TrackingIndicators                         `json:"tracking_indicators"`
	PickupTimes        []PickupTime                               `json:"pickup_times"`
	ArrivalTimes       []ArrivalTime                              `json:"arrival_times"`
}

func emptyProjection() *Projection {
	return &Projection{
		Groups:             []Group{},
		UnclaimedGroups:    []UnclaimedGroup{},
		EducationalGroups:  []EducationalGroup{},
		ActiveSessions:     []scheduleService.OperationActiveSession{},
		PlannedNow:         []scheduleService.OperationPlannedInstance{},
		Visits:             []Visit{},
		TrackingIndicators: TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}},
		PickupTimes:        []PickupTime{},
		ArrivalTimes:       []ArrivalTime{},
	}
}

// plannedNow parameters are the page contract the former BFF hardcoded.
const (
	plannedNowHorizonMinutes = 480
	plannedNowLimit          = 5
)

func (s *service) Get(ctx context.Context, requestedGroupID int64) (*Projection, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}
	ctx, err := s.prefetchSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve dashboard settings: %w", err)
	}

	projection := emptyProjection()

	staffID, err := s.loadCurrentStaffID(ctx)
	if err != nil {
		return nil, err
	}
	projection.CurrentStaffID = staffID

	groups, err := s.resolveGroups(ctx)
	if err != nil {
		return nil, err
	}
	projection.Groups = groups

	if staffID != nil {
		schulhof, err := s.deps.Schulhof.GetSchulhofStatus(ctx, *staffID)
		if err != nil {
			return nil, fmt.Errorf("load schulhof status: %w", err)
		}
		projection.Schulhof = schulhof
	}

	selected, err := selectGroup(groups, projection.Schulhof, requestedGroupID)
	if err != nil {
		return nil, err
	}
	projection.SelectedGroupID = selected

	if err := s.loadStaticSections(ctx, projection); err != nil {
		return nil, err
	}
	if err := s.loadScheduleSections(ctx, projection); err != nil {
		return nil, err
	}
	if selected != nil {
		if err := s.loadSelectedGroupSections(ctx, projection, *selected); err != nil {
			return nil, err
		}
	}
	return projection, nil
}

func (s *service) validateDependencies() error {
	if s.deps.Active == nil || s.deps.UserContext == nil || s.deps.Education == nil ||
		s.deps.Schulhof == nil || s.deps.Operations == nil || s.deps.Settings == nil ||
		s.deps.Pickups == nil || s.deps.Arrivals == nil {
		return errors.New("supervision dashboard service is not fully configured")
	}
	return nil
}

func (s *service) prefetchSettings(ctx context.Context) (context.Context, error) {
	batch, ok := s.deps.Settings.(configService.BatchSettingsService)
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

func (s *service) loadCurrentStaffID(ctx context.Context) (*int64, error) {
	staff, err := s.deps.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) ||
			errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return nil, nil
		}
		return nil, fmt.Errorf("load current staff: %w", err)
	}
	if staff == nil {
		return nil, nil
	}
	id := staff.ID
	return &id, nil
}

// resolveGroups mirrors the scope split of the former fan-out: callers covered
// by the school-wide overview scope (#2380) see all running sessions,
// otherwise their own supervised sessions. Errors propagate — the former BFF's
// silent 403→fallback chain is replaced by one deterministic decision here.
func (s *service) resolveGroups(ctx context.Context) ([]Group, error) {
	broad, err := s.hasOperationalOverview(ctx)
	if err != nil {
		return nil, err
	}

	var groups []*activeModels.Group
	if broad {
		all, err := s.deps.Active.ListActiveGroups(ctx, base.NewQueryOptions())
		if err != nil {
			return nil, fmt.Errorf("load active groups: %w", err)
		}
		for _, group := range all {
			if group.IsActive() {
				groups = append(groups, group)
			}
		}
		groups, err = s.loadGroupsWithRelations(ctx, groups)
		if err != nil {
			return nil, err
		}
	} else {
		supervised, err := s.deps.UserContext.GetMySupervisedGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("load supervised groups: %w", err)
		}
		groups = supervised
	}

	rooms, err := s.loadRooms(ctx, groups)
	if err != nil {
		return nil, err
	}

	result := make([]Group, 0, len(groups))
	for _, group := range groups {
		item := Group{ID: group.ID}
		if group.ActualGroup != nil {
			item.Name = group.ActualGroup.Name
		}
		if group.RoomID > 0 {
			roomID := group.RoomID
			item.RoomID = &roomID
			if room := rooms[group.RoomID]; room != nil {
				item.RoomName = room.Name
				item.RoomColor = room.Color
				if item.Name == "" {
					item.Name = room.Name
				}
			}
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return collation.CompareGerman(result[i].RoomName, result[j].RoomName) < 0
	})
	return result, nil
}

// loadGroupsWithRelations fills the template activity relation for the broad
// overview path, whose list query intentionally returns only active-group
// columns. The supervised path already uses the same bulk lookup internally.
func (s *service) loadGroupsWithRelations(ctx context.Context, groups []*activeModels.Group) ([]*activeModels.Group, error) {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	loaded, err := s.deps.Active.GetActiveGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load active group relations: %w", err)
	}

	result := make([]*activeModels.Group, 0, len(groups))
	for _, group := range groups {
		if loadedGroup := loaded[group.ID]; loadedGroup != nil {
			result = append(result, loadedGroup)
		}
	}
	return result, nil
}

// hasOperationalOverview asks the one school-wide rule (#2380). The
// organisational group mode is deliberately not consulted here: it describes
// how the school organises children, not who may open a running module.
func (s *service) hasOperationalOverview(ctx context.Context) (bool, error) {
	allowed, err := authorize.HasOperationalOverview(ctx, s.deps.Settings, s.deps.UserContext)
	if err != nil {
		return false, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	return allowed, nil
}

// loadRooms bulk-resolves rooms for groups whose relation is not preloaded —
// this replaces the former per-group GET /api/rooms/{id} N+1.
func (s *service) loadRooms(ctx context.Context, groups []*activeModels.Group) (map[int64]*facilitiesModels.Room, error) {
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
	rooms, err := s.deps.Active.GetRoomsByIDs(ctx, missing)
	if err != nil {
		return nil, fmt.Errorf("bulk load rooms: %w", err)
	}
	for _, room := range rooms {
		result[room.ID] = room
	}
	return result, nil
}

// selectGroup resolves the caller-requested session against the operational
// scope. An unknown id is a hard 403 (ErrForbiddenGroup) — the caller retries
// without group_id when its selection went stale.
func selectGroup(groups []Group, schulhof *facilitiesService.SchulhofStatus, requestedGroupID int64) (*int64, error) {
	if requestedGroupID <= 0 {
		if len(groups) == 0 {
			return nil, nil
		}
		id := groups[0].ID
		return &id, nil
	}
	for _, group := range groups {
		if group.ID == requestedGroupID {
			id := group.ID
			return &id, nil
		}
	}
	if schulhof != nil && schulhof.IsUserSupervising &&
		schulhof.ActiveGroupID != nil && *schulhof.ActiveGroupID == requestedGroupID {
		id := requestedGroupID
		return &id, nil
	}
	return nil, ErrForbiddenGroup
}

func (s *service) loadStaticSections(ctx context.Context, projection *Projection) error {
	unclaimed, err := s.deps.Active.GetUnclaimedActiveGroups(ctx)
	if err != nil {
		return fmt.Errorf("load unclaimed groups: %w", err)
	}
	for _, group := range unclaimed {
		item := UnclaimedGroup{ID: group.ID}
		if group.Room != nil {
			item.RoomName = group.Room.Name
		}
		projection.UnclaimedGroups = append(projection.UnclaimedGroups, item)
	}

	myGroups, err := s.deps.UserContext.GetMyGroups(ctx)
	if err != nil {
		return fmt.Errorf("load educational groups: %w", err)
	}
	roomGroupIDs := make([]int64, 0, len(myGroups))
	for _, group := range myGroups {
		if group.RoomID != nil {
			roomGroupIDs = append(roomGroupIDs, group.ID)
		}
	}
	loaded, err := s.loadEducationRooms(ctx, roomGroupIDs)
	if err != nil {
		return err
	}
	for _, group := range myGroups {
		item := EducationalGroup{ID: group.ID, Name: group.Name}
		if withRoom := loaded[group.ID]; withRoom != "" {
			item.RoomName = withRoom
		}
		projection.EducationalGroups = append(projection.EducationalGroups, item)
	}
	return nil
}

func (s *service) loadEducationRooms(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	result := map[int64]string{}
	if len(groupIDs) == 0 {
		return result, nil
	}
	loaded, err := s.deps.Education.GetGroupsWithRoomsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("load education group rooms: %w", err)
	}
	for id, group := range loaded {
		if group != nil && group.Room != nil {
			result[id] = group.Room.Name
		}
	}
	return result, nil
}

// loadScheduleSections fills planned-now, active sessions, and capabilities.
// These mirror the former /timetable/operations/* endpoints, which were gated
// on schedules:read — a caller without it gets them deterministically empty
// (permission-based redaction), never a swallowed error.
func (s *service) loadScheduleSections(ctx context.Context, projection *Projection) error {
	if !authorize.HasPermission(permissions.SchedulesRead, jwt.PermissionsFromCtx(ctx)) {
		return nil
	}
	claims := jwt.ClaimsFromCtx(ctx)
	now := s.deps.Now()
	today := timezone.DateFromTime(now)

	planned, err := s.deps.Operations.PlannedNow(ctx, int64(claims.ID), claims.IsAdmin, today, now, scheduleService.PlannedNowOptions{
		HorizonMinutes: plannedNowHorizonMinutes,
		Limit:          plannedNowLimit,
		IncludeRoster:  true,
	})
	if err != nil {
		return fmt.Errorf("load planned instances: %w", err)
	}
	if planned != nil {
		projection.PlannedNow = planned
	}

	sessions, err := s.deps.Operations.ActiveSessions(ctx, today)
	if err != nil {
		return fmt.Errorf("load active sessions: %w", err)
	}
	if sessions != nil {
		projection.ActiveSessions = sessions
	}

	capabilities, err := s.resolveCapabilities(ctx)
	if err != nil {
		return err
	}
	projection.Capabilities = capabilities
	return nil
}

func (s *service) resolveCapabilities(ctx context.Context) (Capabilities, error) {
	careConcept, err := s.deps.Settings.ResolveString(ctx, configModel.KeyCareConcept)
	if err != nil {
		return Capabilities{}, fmt.Errorf("resolve care concept setting: %w", err)
	}
	if careConcept != configModel.CareConceptOpenRooms {
		return Capabilities{}, nil
	}
	enabled, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyWebSpontaneousActivities)
	if err != nil {
		return Capabilities{}, fmt.Errorf("resolve spontaneous activities setting: %w", err)
	}
	return Capabilities{WebSpontaneousActivitiesEnabled: enabled}, nil
}

// loadSelectedGroupSections fills visits, tracking indicators, and pickup /
// arrival times for the selected session's students.
func (s *service) loadSelectedGroupSections(ctx context.Context, projection *Projection, selectedGroupID int64) error {
	rows, err := s.deps.Active.GetActiveGroupVisitsWithDisplay(ctx, selectedGroupID)
	if err != nil {
		return fmt.Errorf("load group visits: %w", err)
	}

	access := userContextService.ResolveStudentAccess(ctx, s.deps.UserContext)
	fullAccess := access.HasFullAccess()

	openVisits := make([]*activeModels.VisitWithStudentDisplay, 0, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	seen := map[int64]struct{}{}
	for _, row := range rows {
		if row.ExitTime != nil {
			continue
		}
		openVisits = append(openVisits, row)
		if _, ok := seen[row.StudentID]; !ok {
			seen[row.StudentID] = struct{}{}
			studentIDs = append(studentIDs, row.StudentID)
		}
	}

	attendance := map[int64]*activeService.AttendanceStatus{}
	if fullAccess && len(studentIDs) > 0 {
		loaded, err := s.deps.Active.GetStudentsAttendanceStatuses(ctx, studentIDs)
		if err != nil {
			return fmt.Errorf("load attendance statuses: %w", err)
		}
		if loaded != nil {
			attendance = loaded
		}
	}

	photosEnabled, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
	if err != nil {
		return fmt.Errorf("resolve student photos setting: %w", err)
	}
	projection.Visits = buildVisits(openVisits, attendance, fullAccess, photosEnabled)

	tracking, err := s.loadTracking(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("load tracking indicators: %w", err)
	}
	projection.TrackingIndicators = tracking

	return s.loadPlanningTimes(ctx, projection, studentIDs, fullAccess)
}

func buildVisits(rows []*activeModels.VisitWithStudentDisplay, attendance map[int64]*activeService.AttendanceStatus, fullAccess, photosEnabled bool) []Visit {
	result := make([]Visit, 0, len(rows))
	for _, row := range rows {
		visit := Visit{
			StudentID:     row.StudentID,
			StudentName:   strings.TrimSpace(row.FirstName + " " + row.LastName),
			SchoolClass:   row.SchoolClass,
			GroupName:     row.OGSGroupName,
			ActiveGroupID: row.ActiveGroupID,
			CheckInTime:   row.EntryTime,
		}
		if row.Sick != nil {
			visit.Sick = *row.Sick
		}
		visit.SickSince = row.SickSince
		if row.Excused != nil {
			visit.Excused = *row.Excused
		}
		visit.ExcusedSince = row.ExcusedSince
		if fullAccess {
			if status := attendance[row.StudentID]; status != nil {
				visit.ActualArrivalTime = timezone.FormatBerlinClock(status.CheckInTime)
				visit.ActualPickupTime = timezone.FormatBerlinClock(status.CheckOutTime)
			}
		}
		// Same full-access gate the single endpoint used: without it, avatar
		// requests for out-of-scope students would 403 in the byte-serve path.
		if photosEnabled && fullAccess && row.PhotoPath != nil {
			visit.PhotoURL = buildPhotoURL(row.StudentID, *row.PhotoPath)
		}
		result = append(result, visit)
	}
	return result
}

func buildPhotoURL(studentID int64, storedURL string) string {
	if storedURL == "" || !strings.HasPrefix(storedURL, studentPhotoStoredURLPrefix) {
		return storedURL
	}
	return fmt.Sprintf("/api/students/%d/photo/%s", studentID, strings.TrimPrefix(storedURL, studentPhotoStoredURLPrefix))
}

func (s *service) loadTracking(ctx context.Context, studentIDs []int64) (TrackingIndicators, error) {
	empty := TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}}
	if len(studentIDs) == 0 {
		return empty, nil
	}
	enabled, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyTrackingIndicatorsEnabled)
	if err != nil || !enabled {
		return empty, err
	}
	labels := make([]string, 0, 3)
	for _, key := range []string{configModel.KeyTrackingIndicator1, configModel.KeyTrackingIndicator2, configModel.KeyTrackingIndicator3} {
		value, err := s.deps.Settings.ResolveString(ctx, key)
		if err != nil {
			return empty, err
		}
		if value = strings.TrimSpace(value); value != "" {
			labels = append(labels, value)
		}
	}
	if len(labels) == 0 {
		return empty, nil
	}
	results, err := s.deps.Active.GetTrackingIndicators(ctx, studentIDs, labels)
	if err != nil {
		return empty, err
	}
	return TrackingIndicators{Labels: labels, Results: results}, nil
}

// loadPlanningTimes mirrors the former bulk pickup/arrival endpoints, which
// were gated on users:read; without it the sections stay deterministically
// empty. Times are additionally scoped to full-access callers, matching the
// actual arrival/pickup gate on the visit rows.
func (s *service) loadPlanningTimes(ctx context.Context, projection *Projection, studentIDs []int64, fullAccess bool) error {
	if len(studentIDs) == 0 || !fullAccess ||
		!authorize.HasPermission(permissions.UsersRead, jwt.PermissionsFromCtx(ctx)) {
		return nil
	}
	today := timezone.DateFromTime(s.deps.Now())

	pickups, err := s.deps.Pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, today)
	if err != nil {
		return fmt.Errorf("load pickup times: %w", err)
	}
	arrivals, err := s.deps.Arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, today)
	if err != nil {
		return fmt.Errorf("load arrival times: %w", err)
	}

	for _, id := range studentIDs {
		if effective := pickups[id]; effective != nil {
			projection.PickupTimes = append(projection.PickupTimes, pickupTimeFromEffective(id, effective))
		}
		if effective := arrivals[id]; effective != nil {
			projection.ArrivalTimes = append(projection.ArrivalTimes, arrivalTimeFromEffective(id, effective))
		}
	}
	return nil
}

func pickupTimeFromEffective(studentID int64, effective *scheduleService.EffectivePickupTime) PickupTime {
	item := PickupTime{
		StudentID: studentID, Date: effective.Date.String(), WeekdayName: effective.WeekdayName,
		IsException: effective.IsException, Notes: effective.Notes,
	}
	if effective.PickupTime != nil {
		formatted := effective.PickupTime.Format("15:04")
		item.PickupTime = &formatted
	}
	for _, note := range effective.DayNotes {
		item.DayNotes = append(item.DayNotes, DayNote{ID: note.ID, Content: note.Content})
	}
	return item
}

func arrivalTimeFromEffective(studentID int64, effective *scheduleService.EffectiveArrivalTime) ArrivalTime {
	item := ArrivalTime{
		StudentID: studentID, Date: effective.Date.String(), WeekdayName: effective.WeekdayName,
		IsException: effective.IsException, Notes: effective.Notes,
	}
	if effective.ArrivalTime != nil {
		formatted := effective.ArrivalTime.Format("15:04")
		item.ExpectedArrival = &formatted
	}
	for _, note := range effective.DayNotes {
		item.DayNotes = append(item.DayNotes, DayNote{ID: note.ID, Content: note.Content})
	}
	return item
}
