// Package supervisiondashboard is the supervision read projection (#2703):
// the complete live projection for the "Aktuelle Aufsicht" (active
// supervision) page in one request (#2096), built from plain owner facts. It
// owns the projection's authorization and strict error contract: mandatory
// sub-loads fail the whole request instead of degrading to silently-empty
// sections. Every foreign fact enters through a consumer-owned port; the
// projection persists nothing and never writes.
package supervisiondashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrForbiddenGroup indicates that the requested active group is outside the
// caller's operational scope (supervised sessions, or all sessions under
// admin overview / open care, plus a supervised Schulhof session).
var ErrForbiddenGroup = errors.New("caller does not supervise the requested active group")

// ErrIncompleteDependencies reports missing port wiring. Composition must
// fail, not the first request; the projection still refuses to run half
// wired.
var ErrIncompleteDependencies = errors.New("supervision dashboard projection is not fully configured")

const studentPhotoStoredURLPrefix = "/uploads/student-photos/"

// SpontaneousStartBlockedWeekend is the reason code for a weekend business
// day on which no spontaneous session may start.
const SpontaneousStartBlockedWeekend = "weekend"

// plannedNow parameters are the page contract the former BFF hardcoded.
const (
	plannedNowHorizonMinutes = 480
	plannedNowLimit          = 5
)

// Query is the single public capability of the projection.
type Query interface {
	Dashboard(ctx context.Context, requestedGroupID int64) (*Projection, error)
}

// Dependencies are the consumer-owned ports the projection reads through.
// Now defaults to time.Now; every other field is required.
type Dependencies struct {
	Access   Access
	Sessions SessionDirectory
	Rooms    RoomDirectory
	Yard     Yard
	Groups   EducationGroups
	Schedule Schedule
	Presence Presence
	Planning Planning
	Settings Settings
	Calendar Calendar
	Now      func() time.Time
}

func (d Dependencies) complete() bool {
	return d.Access != nil && d.Sessions != nil && d.Yard != nil && d.Groups != nil &&
		d.Schedule != nil && d.Presence != nil && d.Planning != nil && d.Settings != nil && d.Calendar != nil
}

type service struct{ deps Dependencies }

// New builds the projection. Missing wiring surfaces as
// ErrIncompleteDependencies on every call, never as a partial projection.
func New(deps Dependencies) Query {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &service{deps: deps}
}

type SpontaneousStartAvailability struct {
	Available     bool   `json:"available"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

// Group is one supervised (or, under admin overview / open care, any active)
// session tab. The wire shape carries only what the page renders: the session
// label, room label, color, and ids for selection.
type Group struct {
	ID                       int64   `json:"id,string"`
	Name                     string  `json:"name"`
	RoomID                   *int64  `json:"room_id,string,omitempty"`
	RoomName                 string  `json:"room_name,omitempty"`
	RoomColor                *string `json:"room_color,omitempty"`
	IsCurrentUserSupervising bool    `json:"is_current_user_supervising"`
	CanAssign                bool    `json:"can_assign"`
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

// SchulhofStatus mirrors the Facilities owner's Schulhof workflow state on
// the wire; the yard port maps it field by field.
type SchulhofStatus struct {
	Exists            bool         `json:"exists"`
	RoomID            *int64       `json:"room_id,omitempty"`
	RoomName          string       `json:"room_name"`
	ActivityGroupID   *int64       `json:"activity_group_id,omitempty"`
	ActiveGroupID     *int64       `json:"active_group_id,omitempty"`
	IsUserSupervising bool         `json:"is_user_supervising"`
	SupervisionID     *int64       `json:"supervision_id,omitempty"`
	SupervisorCount   int          `json:"supervisor_count"`
	StudentCount      int          `json:"student_count"`
	Supervisors       []Supervisor `json:"supervisors"`
}

type Supervisor struct {
	ID            int64  `json:"id"`
	StaffID       int64  `json:"staff_id"`
	Name          string `json:"name"`
	IsCurrentUser bool   `json:"is_current_user"`
}

// ActiveSession is one running instance seen from its live session (#2265).
// StartTime/EndTime are the plan window (HH:MM), not the actual start.
type ActiveSession struct {
	ActiveGroupID int64  `json:"active_group_id"`
	InstanceID    int64  `json:"instance_id"`
	Title         string `json:"title"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

// PlannedInstance mirrors the Timetable owner's planned-now row on the wire.
type PlannedInstance struct {
	ID                    int64             `json:"id"`
	Title                 string            `json:"title"`
	Date                  string            `json:"date"`
	StartTime             string            `json:"start_time"`
	EndTime               string            `json:"end_time"`
	RoomID                int64             `json:"room_id"`
	RoomName              *string           `json:"room_name,omitempty"`
	Status                string            `json:"status"`
	IsOverdue             bool              `json:"is_overdue"`
	MinutesUntilStart     int               `json:"minutes_until_start"`
	ExpectedStudentsCount int               `json:"expected_students_count"`
	PresentStudentsCount  int               `json:"present_students_count"`
	NotScheduledCount     int               `json:"not_scheduled_students_count"`
	AssignedStaffIDs      []int64           `json:"assigned_staff_ids"`
	IsAssigned            bool              `json:"is_assigned"`
	IsPrimary             bool              `json:"is_primary"`
	IsSubstitute          bool              `json:"is_substitute"`
	IsAbsent              bool              `json:"is_absent"`
	RosterPreview         []RosterRow       `json:"roster_preview,omitempty"`
	PickupTimesLoaded     bool              `json:"pickup_times_loaded"`
	PickupTimesRedacted   bool              `json:"pickup_times_redacted,omitempty"`
	Warnings              []ConflictWarning `json:"warnings"`
	CanStart              bool              `json:"can_start"`
	StartAvailableAt      string            `json:"start_available_at"`
	StartExpiresAt        string            `json:"start_expires_at"`
	ActiveGroupID         *int64            `json:"active_group_id,omitempty"`
	CancelReason          *string           `json:"cancel_reason,omitempty"`
	PlanningTrackName     *string           `json:"planning_track_name,omitempty"`
	PlanningTrackColor    *string           `json:"planning_track_color,omitempty"`
	GroupName             *string           `json:"group_name,omitempty"`
	StaffNames            []StaffName       `json:"staff_names,omitempty"`
}

// StaffName is one assigned staff member on a planned block.
type StaffName struct {
	StaffID      int64  `json:"staff_id"`
	DisplayName  string `json:"display_name"`
	IsSubstitute bool   `json:"is_substitute"`
}

// RosterRow is one child on a planned block's roster preview.
type RosterRow struct {
	StudentID         int64             `json:"student_id"`
	StudentName       string            `json:"student_name"`
	SchoolClass       string            `json:"school_class"`
	GroupName         string            `json:"group_name"`
	Planned           bool              `json:"planned"`
	IsUnplanned       bool              `json:"is_unplanned"`
	CurrentlyPresent  bool              `json:"currently_present"`
	VisitID           *int64            `json:"visit_id,omitempty"`
	Status            string            `json:"status"`
	Substatus         *string           `json:"substatus,omitempty"`
	Note              *string           `json:"note,omitempty"`
	CheckedInAt       *string           `json:"checked_in_at,omitempty"`
	CheckedOutAt      *string           `json:"checked_out_at,omitempty"`
	VisitEntryTime    *string           `json:"visit_entry_time,omitempty"`
	PickupTime        *string           `json:"pickup_time"`
	Warnings          []RosterWarning   `json:"warnings,omitempty"`
	ParallelPresentIn *ParallelPresence `json:"parallel_present_in,omitempty"`
	CareDayStatus     string            `json:"care_day_status"`
}

// RosterWarning is one arrival or group warning on a roster row.
type RosterWarning struct {
	Kind                  string  `json:"kind"`
	Message               string  `json:"message"`
	ExpectedArrival       *string `json:"expected_arrival,omitempty"`
	SlotStart             *string `json:"slot_start,omitempty"`
	ExpectedGroupID       *int64  `json:"expected_group_id,omitempty"`
	ExpectedGroupName     *string `json:"expected_group_name,omitempty"`
	CurrentEducationGroup *int64  `json:"current_education_group_id,omitempty"`
}

// ParallelPresence identifies the other running instance a roster row's
// parallel-presence hint points at (#2265).
type ParallelPresence struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

// ConflictWarning is one staff or student conflict on a planned block.
type ConflictWarning struct {
	Kind                  string `json:"kind"`
	ResourceID            int64  `json:"resource_id"`
	Message               string `json:"message"`
	CanOverride           bool   `json:"can_override"`
	Fingerprint           string `json:"fingerprint,omitempty"`
	ConflictingInstanceID int64  `json:"conflicting_instance_id,omitempty"`
	ConflictingTitle      string `json:"conflicting_title,omitempty"`
	OverlapStart          string `json:"overlap_start,omitempty"`
	OverlapEnd            string `json:"overlap_end,omitempty"`
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

// OpenRoom is one released room and every child currently recorded there.
type OpenRoom struct {
	RoomID            int64             `json:"room_id,string"`
	Name              string            `json:"name"`
	IsUserSupervising bool              `json:"is_user_supervising"`
	ActiveGroupIDs    []string          `json:"active_group_ids"`
	StudentCount      int               `json:"student_count"`
	Students          []OpenRoomStudent `json:"students"`
}

// OpenRoomStudent reuses the selected session's visit projection.
type OpenRoomStudent struct {
	Visit
	ActivityName string `json:"activity_name,omitempty"`
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
	BusinessDay                  Date                         `json:"business_day"`
	SpontaneousStartAvailability SpontaneousStartAvailability `json:"spontaneous_start_availability"`
	Groups                       []Group                      `json:"groups"`
	SelectedGroupID              *int64                       `json:"selected_group_id,string,omitempty"`
	UnclaimedGroups              []UnclaimedGroup             `json:"unclaimed_groups"`
	CurrentStaffID               *int64                       `json:"current_staff_id,string,omitempty"`
	EducationalGroups            []EducationalGroup           `json:"educational_groups"`
	// Schulhof is nil only when the caller has no staff identity (and thus no
	// Schulhof workflow) — never because a sub-load failed.
	Schulhof           *SchulhofStatus    `json:"schulhof_status"`
	OpenRooms          []OpenRoom         `json:"open_rooms"`
	Capabilities       Capabilities       `json:"capabilities"`
	ActiveSessions     []ActiveSession    `json:"active_sessions"`
	PlannedNow         []PlannedInstance  `json:"planned_now"`
	Visits             []Visit            `json:"visits"`
	TrackingIndicators TrackingIndicators `json:"tracking_indicators"`
	PickupTimes        []PickupTime       `json:"pickup_times"`
	ArrivalTimes       []ArrivalTime      `json:"arrival_times"`
}

func emptyProjection() *Projection {
	return &Projection{
		Groups:             []Group{},
		OpenRooms:          []OpenRoom{},
		UnclaimedGroups:    []UnclaimedGroup{},
		EducationalGroups:  []EducationalGroup{},
		ActiveSessions:     []ActiveSession{},
		PlannedNow:         []PlannedInstance{},
		Visits:             []Visit{},
		TrackingIndicators: TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}},
		PickupTimes:        []PickupTime{},
		ArrivalTimes:       []ArrivalTime{},
	}
}

type daySnapshot struct {
	instant          time.Time
	businessDay      Date
	spontaneousStart SpontaneousStartAvailability
}

// newDaySnapshot captures the server clock once per request so every
// section reads the same business day, even across midnight.
func newDaySnapshot(now time.Time, calendar Calendar) daySnapshot {
	businessDay := calendar.DayOf(now)
	availability := SpontaneousStartAvailability{Available: true}
	if weekday := calendar.Weekday(businessDay); weekday == time.Saturday || weekday == time.Sunday {
		availability = SpontaneousStartAvailability{BlockedReason: SpontaneousStartBlockedWeekend}
	}
	return daySnapshot{
		instant:          now,
		businessDay:      businessDay,
		spontaneousStart: availability,
	}
}

func (s *service) Dashboard(ctx context.Context, requestedGroupID int64) (*Projection, error) {
	if !s.deps.complete() {
		return nil, ErrIncompleteDependencies
	}
	snapshot := newDaySnapshot(s.deps.Now(), s.deps.Calendar)
	ctx, err := s.deps.Settings.Prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve dashboard settings: %w", err)
	}

	projection := emptyProjection()
	projection.BusinessDay = snapshot.businessDay
	projection.SpontaneousStartAvailability = snapshot.spontaneousStart

	staffID, err := s.deps.Access.CurrentStaffID(ctx)
	if err != nil {
		return nil, fmt.Errorf("load current staff: %w", err)
	}
	projection.CurrentStaffID = staffID

	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return nil, err
	}

	groups, err := s.resolveGroups(ctx, caller, staffID)
	if err != nil {
		return nil, err
	}
	projection.Groups = groups

	if staffID != nil {
		schulhof, err := s.deps.Yard.Status(ctx, *staffID)
		if err != nil {
			return nil, fmt.Errorf("load schulhof status: %w", err)
		}
		projection.Schulhof = schulhof
	}
	openRooms, err := s.loadOpenRoomInputs(ctx)
	if err != nil {
		return nil, err
	}

	selected, err := selectGroup(groups, projection.Schulhof, requestedGroupID)
	if err != nil {
		return nil, err
	}
	projection.SelectedGroupID = selected

	if err := s.loadStaticSections(ctx, projection); err != nil {
		return nil, err
	}
	if err := s.loadScheduleSections(ctx, projection, caller, snapshot); err != nil {
		return nil, err
	}
	if err := s.loadPresenceSections(ctx, projection, caller, selected, openRooms, staffID, snapshot.businessDay); err != nil {
		return nil, err
	}
	return projection, nil
}

// resolveGroups mirrors the scope split of the former fan-out: callers covered
// by the school-wide overview scope (#2380) see all running sessions,
// otherwise their own supervised sessions. Errors propagate — the former BFF's
// silent 403→fallback chain is replaced by one deterministic decision here.
func (s *service) resolveGroups(ctx context.Context, caller Caller, staffID *int64) ([]Group, error) {
	var (
		sessions []Session
		err      error
	)
	if caller.OperationalOverview {
		sessions, err = s.deps.Sessions.Running(ctx)
		if err != nil {
			return nil, fmt.Errorf("load active groups: %w", err)
		}
	} else {
		sessions, err = s.deps.Sessions.Supervised(ctx)
		if err != nil {
			return nil, fmt.Errorf("load supervised groups: %w", err)
		}
	}
	ownedGroupIDs, err := s.ownedGroupIDs(ctx, caller.OperationalOverview, staffID)
	if err != nil {
		return nil, err
	}

	result := make([]Group, 0, len(sessions))
	for _, session := range sessions {
		item := Group{
			ID:                       session.ID,
			Name:                     session.Name,
			RoomID:                   session.RoomID,
			RoomName:                 session.RoomName,
			RoomColor:                session.RoomColor,
			IsCurrentUserSupervising: !caller.OperationalOverview,
		}
		if caller.OperationalOverview {
			_, item.IsCurrentUserSupervising = ownedGroupIDs[session.ID]
		}
		item.CanAssign = caller.AdminScope || item.IsCurrentUserSupervising
		if item.Name == "" {
			item.Name = session.RoomName
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *service) ownedGroupIDs(ctx context.Context, broad bool, staffID *int64) (map[int64]struct{}, error) {
	if !broad || staffID == nil {
		return map[int64]struct{}{}, nil
	}
	owned, err := s.deps.Sessions.SupervisedByStaff(ctx, *staffID)
	if err != nil {
		return nil, fmt.Errorf("load current staff supervisions: %w", err)
	}
	if owned == nil {
		owned = map[int64]struct{}{}
	}
	return owned, nil
}

// selectGroup resolves the caller-requested session against the operational
// scope. An unknown id is a hard 403 (ErrForbiddenGroup) — the caller retries
// without group_id when its selection went stale.
func selectGroup(groups []Group, schulhof *SchulhofStatus, requestedGroupID int64) (*int64, error) {
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
	unclaimed, err := s.deps.Sessions.Unclaimed(ctx)
	if err != nil {
		return fmt.Errorf("load unclaimed groups: %w", err)
	}
	projection.UnclaimedGroups = append(projection.UnclaimedGroups, unclaimed...)

	myGroups, err := s.deps.Groups.MyGroups(ctx)
	if err != nil {
		return fmt.Errorf("load educational groups: %w", err)
	}
	projection.EducationalGroups = append(projection.EducationalGroups, myGroups...)
	return nil
}

// loadScheduleSections fills planned-now, active sessions, and capabilities.
// These mirror the former /timetable/operations/* endpoints, which were gated
// on schedules:read — a caller without it gets them deterministically empty
// (permission-based redaction), never a swallowed error.
func (s *service) loadScheduleSections(ctx context.Context, projection *Projection, caller Caller, snapshot daySnapshot) error {
	if !caller.CanReadSchedules {
		return nil
	}
	planned, err := s.deps.Schedule.PlannedNow(ctx, PlannedNowQuery{
		AccountID:      caller.AccountID,
		TokenAdmin:     caller.TokenAdmin,
		Date:           snapshot.businessDay,
		Now:            snapshot.instant,
		HorizonMinutes: plannedNowHorizonMinutes,
		Limit:          plannedNowLimit,
		IncludeRoster:  true,
	})
	if err != nil {
		return fmt.Errorf("load planned instances: %w", err)
	}
	if !caller.CanReadStudents {
		redactPlannedPickupTimes(planned)
	}
	if planned != nil {
		projection.PlannedNow = planned
	}

	sessions, err := s.deps.Schedule.ActiveSessions(ctx, snapshot.businessDay)
	if err != nil {
		return fmt.Errorf("load active sessions: %w", err)
	}
	if sessions != nil {
		projection.ActiveSessions = sessions
	}

	enabled, err := s.deps.Settings.SpontaneousActivitiesEnabled(ctx)
	if err != nil {
		return err
	}
	projection.Capabilities = Capabilities{WebSpontaneousActivitiesEnabled: enabled}
	return nil
}

func redactPlannedPickupTimes(instances []PlannedInstance) {
	for i := range instances {
		instances[i].PickupTimesLoaded = false
		instances[i].PickupTimesRedacted = true
		for j := range instances[i].RosterPreview {
			instances[i].RosterPreview[j].PickupTime = nil
		}
	}
}

// loadPresenceSections projects the selected session and every released room
// together. Student-specific reads therefore run once over their union.
func (s *service) loadPresenceSections(ctx context.Context, projection *Projection, caller Caller, selectedGroupID *int64, openRooms openRoomInputs, staffID *int64, businessDay Date) error {
	selectedRows := make([]VisitRecord, 0)
	if selectedGroupID != nil {
		rows, err := s.deps.Presence.GroupVisits(ctx, *selectedGroupID)
		if err != nil {
			return fmt.Errorf("load group visits: %w", err)
		}
		for _, row := range rows {
			if row.ExitTime == nil {
				selectedRows = append(selectedRows, row)
			}
		}
	}
	studentIDs := unionStudentIDs(selectedRows, openRooms.studentIDs())

	fullAccess, err := s.deps.Access.FullStudentAccess(ctx)
	if err != nil {
		return fmt.Errorf("resolve student access: %w", err)
	}

	attendance := map[int64]Attendance{}
	if fullAccess && len(studentIDs) > 0 {
		loaded, err := s.deps.Presence.AttendanceTimes(ctx, studentIDs)
		if err != nil {
			return fmt.Errorf("load attendance statuses: %w", err)
		}
		if loaded != nil {
			attendance = loaded
		}
	}

	photosEnabled, err := s.deps.Settings.StudentPhotosEnabled(ctx)
	if err != nil {
		return fmt.Errorf("resolve student photos setting: %w", err)
	}
	projection.Visits = s.buildVisits(selectedRows, attendance, fullAccess, photosEnabled)
	projection.OpenRooms = s.assembleOpenRooms(openRooms, attendance, fullAccess, photosEnabled, staffID)

	tracking, err := s.loadTracking(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("load tracking indicators: %w", err)
	}
	projection.TrackingIndicators = tracking

	return s.loadPlanningTimes(ctx, projection, caller, studentIDs, fullAccess, businessDay)
}

func unionStudentIDs(selected []VisitRecord, openRoomStudents []int64) []int64 {
	seen := make(map[int64]struct{}, len(selected)+len(openRoomStudents))
	ids := make([]int64, 0, len(selected)+len(openRoomStudents))
	add := func(id int64) {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	for _, row := range selected {
		add(row.StudentID)
	}
	for _, id := range openRoomStudents {
		add(id)
	}
	return ids
}

func (s *service) buildVisits(rows []VisitRecord, attendance map[int64]Attendance, fullAccess, photosEnabled bool) []Visit {
	result := make([]Visit, 0, len(rows))
	for _, row := range rows {
		result = append(result, s.buildVisit(row, attendance, fullAccess, photosEnabled))
	}
	return result
}

func (s *service) buildVisit(row VisitRecord, attendance map[int64]Attendance, fullAccess, photosEnabled bool) Visit {
	visit := Visit{
		StudentID:     row.StudentID,
		StudentName:   strings.TrimSpace(row.FirstName + " " + row.LastName),
		SchoolClass:   row.SchoolClass,
		GroupName:     row.GroupName,
		ActiveGroupID: row.ActiveGroupID,
		CheckInTime:   row.EntryTime,
		Sick:          row.Sick,
		SickSince:     row.SickSince,
		Excused:       row.Excused,
		ExcusedSince:  row.ExcusedSince,
	}
	if fullAccess {
		if status, ok := attendance[row.StudentID]; ok {
			visit.ActualArrivalTime = s.clock(status.CheckInTime)
			visit.ActualPickupTime = s.clock(status.CheckOutTime)
		}
	}
	// Same full-access gate the single endpoint used: without it, avatar
	// requests for out-of-scope students would 403 in the byte-serve path.
	if photosEnabled && fullAccess && row.PhotoPath != nil {
		visit.PhotoURL = buildPhotoURL(row.StudentID, *row.PhotoPath)
	}
	return visit
}

func (s *service) clock(at *time.Time) *string {
	if at == nil {
		return nil
	}
	formatted := s.deps.Calendar.Clock(*at)
	return &formatted
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
	labels, err := s.deps.Settings.TrackingIndicatorLabels(ctx)
	if err != nil {
		return empty, err
	}
	if len(labels) == 0 {
		return empty, nil
	}
	results, err := s.deps.Presence.TrackingIndicators(ctx, studentIDs, labels)
	if err != nil {
		return empty, err
	}
	return TrackingIndicators{Labels: labels, Results: results}, nil
}

// loadPlanningTimes mirrors the former bulk pickup/arrival endpoints, which
// were gated on users:read; without it the sections stay deterministically
// empty. Times are additionally scoped to full-access callers, matching the
// actual arrival/pickup gate on the visit rows.
func (s *service) loadPlanningTimes(ctx context.Context, projection *Projection, caller Caller, studentIDs []int64, fullAccess bool, businessDay Date) error {
	if len(studentIDs) == 0 || !fullAccess || !caller.CanReadStudents {
		return nil
	}
	pickups, err := s.deps.Planning.Pickups(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load pickup times: %w", err)
	}
	arrivals, err := s.deps.Planning.Arrivals(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load arrival times: %w", err)
	}

	for _, id := range studentIDs {
		if effective, ok := pickups[id]; ok {
			projection.PickupTimes = append(projection.PickupTimes, pickupTimeFromEffective(id, effective))
		}
		if effective, ok := arrivals[id]; ok {
			projection.ArrivalTimes = append(projection.ArrivalTimes, arrivalTimeFromEffective(id, effective))
		}
	}
	return nil
}

func pickupTimeFromEffective(studentID int64, effective Pickup) PickupTime {
	return PickupTime{
		StudentID: studentID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
		PickupTime: effective.PickupTime, IsException: effective.IsException, Notes: effective.Notes,
		DayNotes: effective.DayNotes,
	}
}

func arrivalTimeFromEffective(studentID int64, effective Arrival) ArrivalTime {
	return ArrivalTime{
		StudentID: studentID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
		ExpectedArrival: effective.ArrivalTime, IsException: effective.IsException, Notes: effective.Notes,
		DayNotes: effective.DayNotes,
	}
}
