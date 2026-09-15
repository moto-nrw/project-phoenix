// Package grouplive is the live-group read projection (#2702): the complete
// projection for the supervised OGS group page, built from plain owner facts.
// It owns the projection's authorization and strict error contract so HTTP
// consumers cannot accidentally expose or suppress a subset. Every foreign
// fact enters through a consumer-owned port; the projection persists nothing
// and never writes.
package grouplive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// ErrForbiddenGroup indicates that the requested group is outside the
// caller's visible group set.
var ErrForbiddenGroup = errors.New("caller cannot view the requested group")

const studentPhotoStoredURLPrefix = "/uploads/student-photos/"

// Query is the single public capability of the projection.
type Query interface {
	LiveGroup(ctx context.Context, requestedGroupID int64) (*Projection, error)
	ListGroups(ctx context.Context) ([]Group, error)
}

// Dependencies are the consumer-owned ports the projection reads through.
// ExcusedRequests is optional: without it no pending note is attached.
// Logger defaults to slog.Default; every other field is required.
type Dependencies struct {
	Access          Access
	Groups          GroupDirectory
	Roster          Roster
	Presence        Presence
	Planning        Planning
	Transfers       TransferReader
	Settings        Settings
	Calendar        Calendar
	ExcusedRequests PendingExcusedReader
	Logger          *slog.Logger
}

type service struct{ deps Dependencies }

// New builds the projection. Missing wiring surfaces as an error on the
// first call, never as a partial projection.
func New(deps Dependencies) Query {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &service{deps: deps}
}

type Group struct {
	ID              int64  `json:"id,string"`
	Name            string `json:"name"`
	RoomID          *int64 `json:"room_id,string,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	ViaSubstitution bool   `json:"via_substitution"`
	IsPersonal      bool   `json:"is_personal"`
}

type Student struct {
	ID                 int64      `json:"id,string"`
	FirstName          string     `json:"first_name"`
	LastName           string     `json:"last_name"`
	SchoolClass        string     `json:"school_class"`
	Location           string     `json:"current_location"`
	LocationSince      *time.Time `json:"location_since,omitempty"`
	RoomColor          *string    `json:"current_room_color,omitempty"`
	Sick               bool       `json:"sick"`
	SickSince          *time.Time `json:"sick_since,omitempty"`
	Excused            bool       `json:"excused"`
	ExcusedSince       *time.Time `json:"excused_since,omitempty"`
	ClassTrip          bool       `json:"class_trip"`
	ClassTripSince     *time.Time `json:"class_trip_since,omitempty"`
	DayPlanningStatus  string     `json:"day_planning_status,omitempty"`
	DayPlanningReason  string     `json:"day_planning_reason,omitempty"`
	DayPlanningLabel   string     `json:"day_planning_label,omitempty"`
	PendingExcusedNote *string    `json:"pending_excused_note,omitempty"`
	ArrivalTime        *string    `json:"arrival_time,omitempty"`
	ArrivalIsException bool       `json:"arrival_is_exception,omitempty"`
	ArrivalNotes       string     `json:"arrival_notes,omitempty"`
	ActualArrivalTime  *string    `json:"actual_arrival_time,omitempty"`
	ActualPickupTime   *string    `json:"actual_pickup_time,omitempty"`
	PhotoURL           string     `json:"photo_url,omitempty"`
	fullAccess         bool
}

type RoomStatus struct {
	InGroupRoom   bool   `json:"in_group_room"`
	CurrentRoomID *int64 `json:"current_room_id,string,omitempty"`
}

type DayNote struct {
	ID      int64  `json:"id,string"`
	Content string `json:"content"`
}

type PickupTime struct {
	StudentID   int64     `json:"student_id,string"`
	Date        string    `json:"date"`
	WeekdayName string    `json:"weekday_name"`
	PickupTime  *string   `json:"pickup_time,omitempty"`
	IsException bool      `json:"is_exception"`
	Notes       string    `json:"notes,omitempty"`
	DayNotes    []DayNote `json:"day_notes,omitempty"`
}

type TrackingIndicators struct {
	Labels  []string         `json:"labels"`
	Results map[int64][]bool `json:"results"`
}

type Transfer struct {
	ID                int64  `json:"id,string"`
	GroupID           int64  `json:"group_id,string"`
	SubstituteStaffID int64  `json:"substitute_staff_id,string"`
	SubstituteName    string `json:"substitute_name,omitempty"`
	EndDate           string `json:"end_date"`
}

type Projection struct {
	Groups             []Group               `json:"groups"`
	GroupID            *int64                `json:"group_id,string,omitempty"`
	Students           []Student             `json:"students"`
	RoomStatus         map[string]RoomStatus `json:"room_status"`
	PickupTimes        []PickupTime          `json:"pickup_times"`
	TrackingIndicators TrackingIndicators    `json:"tracking_indicators"`
	Transfers          []Transfer            `json:"transfers"`
}

func EmptyProjection() *Projection {
	return &Projection{
		Groups:             []Group{},
		Students:           []Student{},
		RoomStatus:         map[string]RoomStatus{},
		PickupTimes:        []PickupTime{},
		TrackingIndicators: emptyTracking(),
		Transfers:          []Transfer{},
	}
}

func emptyTracking() TrackingIndicators {
	return TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}}
}

type buildState struct {
	caller     Caller
	selected   GroupRecord
	groups     []Group
	students   []RosterStudent
	studentIDs []int64
	presence   PresenceSnapshot
	projected  []Student
	today      Date
	arrivals   map[int64]Arrival
	pickups    map[int64]Pickup
	timetable  map[int64]bool
}

func (s *service) LiveGroup(ctx context.Context, requestedGroupID int64) (*Projection, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}
	ctx, err := s.deps.Settings.Prepare(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve projection settings: %w", err)
	}
	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve caller access: %w", err)
	}
	groups, selected, err := s.resolveGroups(ctx, caller, requestedGroupID, false)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return EmptyProjection(), nil
	}

	today := s.deps.Calendar.Today()
	students, presence, err := s.loadRoster(ctx, selected.ID, today)
	if err != nil {
		return nil, err
	}
	state := &buildState{
		caller: caller, selected: *selected, groups: groups, today: today,
		students: students, studentIDs: studentIDs(students), presence: presence,
	}
	if err := s.projectRoster(ctx, state); err != nil {
		return nil, err
	}
	if err := s.loadPlanning(ctx, state); err != nil {
		return nil, err
	}
	return s.finishProjection(ctx, state)
}

func (s *service) ListGroups(ctx context.Context) ([]Group, error) {
	if err := s.validateGroupDependencies(); err != nil {
		return nil, err
	}
	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve caller access: %w", err)
	}
	groups, _, err := s.resolveGroups(ctx, caller, 0, true)
	return groups, err
}

func (s *service) validateDependencies() error {
	if err := s.validateGroupDependencies(); err != nil {
		return err
	}
	if s.deps.Roster == nil || s.deps.Presence == nil || s.deps.Planning == nil ||
		s.deps.Transfers == nil || s.deps.Settings == nil || s.deps.Calendar == nil {
		return errors.New("OGS group live service is not fully configured")
	}
	return nil
}

func (s *service) validateGroupDependencies() error {
	if s.deps.Access == nil || s.deps.Groups == nil {
		return errors.New("OGS group live service is not fully configured")
	}
	return nil
}

func (s *service) resolveGroups(ctx context.Context, caller Caller, requestedGroupID int64, allowMissingRoomNames bool) ([]Group, *GroupRecord, error) {
	myGroups, err := s.deps.Groups.SupervisedGroups(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load supervised groups: %w", err)
	}
	personalIDs := make(map[int64]bool, len(myGroups))
	for _, group := range myGroups {
		personalIDs[group.ID] = true
	}

	visibleGroups := myGroups
	if caller.OperationalOverview && caller.CanReadGroups {
		visibleGroups, err = s.deps.Groups.TenantGroups(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("load tenant groups: %w", err)
		}
	}

	if len(visibleGroups) == 0 {
		if requestedGroupID > 0 {
			return nil, nil, ErrForbiddenGroup
		}
		return nil, nil, nil
	}

	selected := selectGroup(visibleGroups, personalIDs, requestedGroupID)
	if selected == nil {
		return nil, nil, ErrForbiddenGroup
	}

	substituted, err := s.deps.Groups.SubstitutedGroupIDs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load substitution metadata: %w", err)
	}
	groups, err := s.mapGroups(ctx, visibleGroups, personalIDs, substituted, allowMissingRoomNames)
	if err != nil {
		return nil, nil, err
	}
	return groups, selected, nil
}

// selectGroup picks the requested group from the directory order, or the
// first personal group (falling back to the first visible one) when no group
// was requested. The directory delivers groups in German dictionary order.
func selectGroup(groups []GroupRecord, personalIDs map[int64]bool, requestedGroupID int64) *GroupRecord {
	if requestedGroupID <= 0 {
		for i := range groups {
			if personalIDs[groups[i].ID] {
				return &groups[i]
			}
		}
		return &groups[0]
	}
	for i := range groups {
		if groups[i].ID == requestedGroupID {
			return &groups[i]
		}
	}
	return nil
}

func (s *service) mapGroups(ctx context.Context, groups []GroupRecord, personalIDs, substituted map[int64]bool, allowMissingRoomNames bool) ([]Group, error) {
	// One bulk lookup resolves every room name; a per-group lookup loop would
	// run on every aggregate refresh including SSE revalidations.
	roomGroupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.RoomID != nil {
			roomGroupIDs = append(roomGroupIDs, group.ID)
		}
	}
	roomNames := map[int64]string{}
	if len(roomGroupIDs) > 0 {
		loaded, err := s.deps.Groups.GroupRoomNames(ctx, roomGroupIDs)
		if err != nil {
			if !allowMissingRoomNames {
				return nil, fmt.Errorf("load group rooms: %w", err)
			}
			s.deps.Logger.Warn("load group rooms failed",
				"error", err,
			)
		} else {
			roomNames = loaded
		}
	}

	result := make([]Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, Group{
			ID: group.ID, Name: group.Name, RoomID: group.RoomID, RoomName: roomNames[group.ID],
			ViaSubstitution: substituted[group.ID], IsPersonal: personalIDs[group.ID],
		})
	}
	return result, nil
}

// loadRoster returns the group's care participants of the day together with
// their presence snapshot. Every sub-load is all-or-nothing: a failing owner
// query fails the projection instead of silently thinning the roster.
func (s *service) loadRoster(ctx context.Context, groupID int64, today Date) ([]RosterStudent, PresenceSnapshot, error) {
	members, err := s.deps.Roster.GroupMembers(ctx, groupID)
	if err != nil {
		return nil, nil, fmt.Errorf("load group students: %w", err)
	}
	participating, err := s.deps.Roster.CareParticipants(ctx, studentIDs(members), today)
	if err != nil {
		return nil, nil, fmt.Errorf("apply care participation: %w", err)
	}
	students := make([]RosterStudent, 0, len(members))
	for _, member := range members {
		if participating[member.ID] {
			students = append(students, member)
		}
	}
	snapshot, err := s.deps.Presence.Snapshot(ctx, studentIDs(students), today)
	if err != nil {
		return nil, nil, fmt.Errorf("load student locations: %w", err)
	}
	return students, snapshot, nil
}

func studentIDs(students []RosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	return ids
}

func (s *service) projectRoster(ctx context.Context, state *buildState) error {
	photosEnabled, err := s.deps.Settings.StudentPhotosEnabled(ctx)
	if err != nil {
		return fmt.Errorf("resolve student photos setting: %w", err)
	}
	fullAccess, err := s.deps.Access.FullStudentAccess(ctx)
	if err != nil {
		return fmt.Errorf("resolve student access: %w", err)
	}
	state.projected = projectStudents(state.students, state.presence, fullAccess, photosEnabled)
	return s.applyStatusDays(ctx, state)
}

func projectStudents(students []RosterStudent, presence PresenceSnapshot, fullAccess, photosEnabled bool) []Student {
	result := make([]Student, 0, len(students))
	for _, model := range students {
		location := presence.Location(model.ID, fullAccess)
		student := Student{
			ID: model.ID, FirstName: model.FirstName, LastName: model.LastName,
			SchoolClass: model.SchoolClass, Location: location.Name,
			LocationSince: location.Since, RoomColor: location.RoomColor, fullAccess: fullAccess,
			Sick: model.Sick, SickSince: model.SickSince,
			Excused: model.Excused, ExcusedSince: model.ExcusedSince,
		}
		if photosEnabled && fullAccess && model.PhotoPath != nil {
			student.PhotoURL = buildPhotoURL(model.ID, *model.PhotoPath)
		}
		result = append(result, student)
	}
	return result
}

func buildPhotoURL(studentID int64, storedURL string) string {
	if storedURL == "" || !strings.HasPrefix(storedURL, studentPhotoStoredURLPrefix) {
		return storedURL
	}
	return fmt.Sprintf("/api/students/%d/photo/%s", studentID, strings.TrimPrefix(storedURL, studentPhotoStoredURLPrefix))
}

func (s *service) applyStatusDays(ctx context.Context, state *buildState) error {
	if len(state.projected) == 0 {
		return nil
	}
	statuses, err := s.deps.Presence.EffectiveStatuses(ctx, state.studentIDs, state.today)
	if err != nil {
		return fmt.Errorf("apply student status days: %w", err)
	}
	for i := range state.projected {
		applyEffectiveStatus(&state.projected[i], statuses[state.projected[i].ID])
	}
	return nil
}

func applyEffectiveStatus(student *Student, status EffectiveStatus) {
	if status.Sick {
		student.Sick, student.SickSince = true, status.SickSince
		student.ClassTrip, student.ClassTripSince = false, nil
		student.Excused, student.ExcusedSince = false, nil
		return
	}
	if status.ClassTrip {
		student.ClassTrip, student.ClassTripSince = true, status.ClassTripSince
		student.Sick, student.SickSince = false, nil
		student.Excused, student.ExcusedSince = false, nil
		return
	}
	if status.Excused && !student.Sick {
		student.Excused, student.ExcusedSince = true, status.ExcusedSince
	}
}

func (s *service) loadPlanning(ctx context.Context, state *buildState) error {
	ids := fullAccessIDs(state.projected)
	arrivals, err := s.deps.Planning.Arrivals(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load arrival times: %w", err)
	}
	pickups, err := s.deps.Planning.Pickups(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load pickup times: %w", err)
	}
	timetable, err := s.deps.Planning.TimetablePlannedStudentIDs(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load timetable planning: %w", err)
	}
	pending, err := s.loadPendingExcused(ctx, state)
	if err != nil {
		return fmt.Errorf("load pending excused requests: %w", err)
	}
	state.arrivals, state.pickups, state.timetable = arrivals, pickups, timetable
	s.applyPlanning(state, pending)
	return nil
}

func fullAccessIDs(students []Student) []int64 {
	ids := make([]int64, 0, len(students))
	for i := range students {
		if students[i].fullAccess {
			ids = append(ids, students[i].ID)
		}
	}
	return ids
}

// pendingExcusedRequest is the Care Plan request row the planning badge reads
// its note from; the alias keeps the projection tests free of that import.
type pendingExcusedRequest = excusedrequests.Request

func (s *service) loadPendingExcused(ctx context.Context, state *buildState) (map[int64]*pendingExcusedRequest, error) {
	// Same gate as the student list/detail badge: whoever may decide the
	// request may see its note — users:update, or users:absence under open
	// care (#2232). The access port answers with the shared authorize rule.
	if s.deps.ExcusedRequests == nil || !state.caller.CanReviewExcusedRequests {
		return map[int64]*pendingExcusedRequest{}, nil
	}
	return s.deps.ExcusedRequests.PendingByStudentForDate(ctx, excusedrequests.Date(state.today))
}

func (s *service) applyPlanning(state *buildState, pending map[int64]*pendingExcusedRequest) {
	for i := range state.projected {
		student := &state.projected[i]
		// The parent's pending note runs FIRST and outside the full-access
		// branch below — same split as the student list (#2232): the note is
		// the review queue's signal, gated by the permission to decide the
		// request, not by read access to the child's record. Under open care
		// the deciding staffer supervises no group, so every child reaches
		// them restricted; tying the note to full access would hide exactly
		// the request they own. loadPendingExcused already returns an empty
		// map for anyone who may not review.
		if request := pending[student.ID]; request != nil {
			note := request.Note
			student.PendingExcusedNote = &note
		}
		if !student.fullAccess {
			continue
		}
		attendance, _ := state.presence.Attendance(student.ID)
		arrival, hasArrival := state.arrivals[student.ID]
		pickup, hasPickup := state.pickups[student.ID]
		inputs := DayInputs{
			Present:      attendance.Present,
			Sick:         student.Sick,
			ClassTrip:    student.ClassTrip,
			Excused:      student.Excused,
			HasTimetable: state.timetable[student.ID],
		}
		if hasArrival {
			inputs.Arrival = &arrival
		}
		if hasPickup {
			inputs.Pickup = &pickup
		}
		decision := s.deps.Planning.DecideDay(inputs)
		student.DayPlanningStatus = "not_coming_today"
		if decision.ComesToday {
			student.DayPlanningStatus = "comes_today"
		}
		student.DayPlanningReason = decision.Reason
		student.DayPlanningLabel = planningLabel(decision)
		applyTimes(student, inputs.Arrival, attendance, s.deps.Calendar)
	}
}

func planningLabel(decision DayDecision) string {
	switch decision.Reason {
	case DayReasonUnplanned:
		return "ungeplant anwesend"
	case DayReasonSick:
		return "krank gemeldet"
	case DayReasonClassTrip:
		return "Klassenfahrt"
	case DayReasonExcused:
		return "entschuldigt"
	case DayReasonArrivalException:
		if decision.ComesToday {
			return "geplante Ankunft heute"
		}
		return exceptionLabel(decision.ExceptionNotes)
	case DayReasonArrivalSchedule:
		return "Ankunftsplan heute"
	case DayReasonPickupException:
		if decision.ComesToday {
			return "geplante Abholung heute"
		}
		return exceptionLabel(decision.ExceptionNotes)
	case DayReasonPickupSchedule:
		return "Abholplan heute"
	case DayReasonTimetable:
		return "Betreuungsplan heute"
	default:
		return "kein Plan für heute"
	}
}

func exceptionLabel(notes string) string {
	if notes != "" {
		return notes
	}
	return "Tagesausnahme"
}

func applyTimes(student *Student, arrival *Arrival, attendance Attendance, calendar Calendar) {
	if arrival != nil {
		if arrival.ArrivalTime != nil {
			formatted := arrival.ArrivalTime.Format("15:04")
			student.ArrivalTime = &formatted
		}
		student.ArrivalIsException = arrival.IsException
		parts := make([]string, 0, len(arrival.DayNotes)+1)
		if arrival.Notes != "" {
			parts = append(parts, arrival.Notes)
		}
		for _, note := range arrival.DayNotes {
			if note != "" {
				parts = append(parts, note)
			}
		}
		student.ArrivalNotes = strings.Join(parts, ", ")
	}
	if attendance.Recorded {
		student.ActualArrivalTime = clock(calendar, attendance.CheckInTime)
		student.ActualPickupTime = clock(calendar, attendance.CheckOutTime)
	}
}

func clock(calendar Calendar, at *time.Time) *string {
	if at == nil {
		return nil
	}
	formatted := calendar.Clock(*at)
	return &formatted
}

func (s *service) finishProjection(ctx context.Context, state *buildState) (*Projection, error) {
	selectedID := state.selected.ID
	projection := &Projection{
		Groups:             state.groups,
		GroupID:            &selectedID,
		Students:           state.projected,
		RoomStatus:         map[string]RoomStatus{},
		PickupTimes:        pickupTimes(state.projected, state.pickups),
		TrackingIndicators: emptyTracking(),
		Transfers:          []Transfer{},
	}

	// The group-derived sections mirror the permission split of the former
	// single endpoints: room status, transfers, and tracking indicators were
	// gated on groups:read, while the roster itself needed only users:read.
	// A caller without groups:read gets these sections deterministically
	// empty (permission-based redaction) instead of a 403 for the whole
	// roster — degradation by permission, never by swallowed errors.
	if !state.caller.CanReadGroups {
		return projection, nil
	}

	tracking, err := s.loadTracking(ctx, state.studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load tracking indicators: %w", err)
	}
	transfers, err := s.deps.Transfers.GroupHandovers(ctx, state.selected.ID, state.today)
	if err != nil {
		return nil, fmt.Errorf("load group transfers: %w", err)
	}
	if transfers == nil {
		transfers = []Transfer{}
	}
	projection.RoomStatus = roomStatuses(state.projected, state.presence, state.selected.RoomID)
	projection.TrackingIndicators = tracking
	projection.Transfers = transfers
	return projection, nil
}

func (s *service) loadTracking(ctx context.Context, studentIDs []int64) (TrackingIndicators, error) {
	empty := emptyTracking()
	if len(studentIDs) == 0 {
		return empty, nil
	}
	labels, err := s.deps.Settings.TrackingIndicatorLabels(ctx)
	if err != nil {
		return empty, err
	}
	trimmed := make([]string, 0, len(labels))
	for _, label := range labels {
		if label = strings.TrimSpace(label); label != "" {
			trimmed = append(trimmed, label)
		}
	}
	if len(trimmed) == 0 {
		return empty, nil
	}
	results, err := s.deps.Presence.TrackingIndicators(ctx, studentIDs, trimmed)
	if err != nil {
		return empty, err
	}
	return TrackingIndicators{Labels: trimmed, Results: results}, nil
}

func roomStatuses(students []Student, presence PresenceSnapshot, groupRoomID *int64) map[string]RoomStatus {
	result := make(map[string]RoomStatus, len(students))
	for i := range students {
		key := strconv.FormatInt(students[i].ID, 10)
		status := RoomStatus{}
		if groupRoomID != nil && presence != nil {
			if roomID := presence.CurrentRoomID(students[i].ID); roomID != nil {
				current := *roomID
				status.CurrentRoomID = &current
				status.InGroupRoom = current == *groupRoomID
			}
		}
		result[key] = status
	}
	return result
}

func pickupTimes(students []Student, pickups map[int64]Pickup) []PickupTime {
	result := make([]PickupTime, 0, len(students))
	for i := range students {
		if !students[i].fullAccess {
			continue
		}
		effective, ok := pickups[students[i].ID]
		if !ok {
			continue
		}
		item := PickupTime{
			StudentID: students[i].ID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
			IsException: effective.IsException, Notes: effective.Notes,
		}
		if effective.PickupTime != nil {
			formatted := effective.PickupTime.Format("15:04")
			item.PickupTime = &formatted
		}
		for _, note := range effective.DayNotes {
			item.DayNotes = append(item.DayNotes, DayNote{ID: note.ID, Content: note.Content})
		}
		result = append(result, item)
	}
	return result
}
