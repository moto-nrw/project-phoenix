// Package ogsgrouplive builds the complete live projection for the supervised
// OGS group page. It owns the projection's authorization and strict error
// contract so HTTP consumers cannot accidentally expose or suppress a subset.
package ogsgrouplive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/collation"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// ErrForbiddenGroup indicates that the requested group is outside the
// caller's supervised group set.
var ErrForbiddenGroup = errors.New("caller does not supervise the requested group")

const studentPhotoStoredURLPrefix = "/uploads/student-photos/"

type Getter interface {
	Get(ctx context.Context, requestedGroupID int64) (*Projection, error)
}

type Dependencies struct {
	People            userService.PersonService
	Education         educationService.Service
	UserContext       userContextService.UserContextService
	Active            activeService.Service
	Settings          configService.SettingsService
	Pickups           scheduleService.PickupScheduleService
	Arrivals          scheduleService.ArrivalScheduleService
	Instances         scheduleService.InstanceService
	CareDays          scheduleService.CareDayService
	CareParticipation CareParticipationResolver
	ExcusedRequests   absenceService.ExcusedAbsenceRequestService
	StatusDays        *activeService.StudentStatusDayService
	Logger            *slog.Logger
	Now               func() time.Time
}

type CareParticipationResolver interface {
	ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on timezone.Date, actuallyPresent map[int64]bool) (map[int64]bool, error)
}

type service struct{ deps Dependencies }

type boolSettingResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

func NewService(deps Dependencies) Getter {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &service{deps: deps}
}

type Group struct {
	ID              int64  `json:"id,string"`
	Name            string `json:"name"`
	RoomID          *int64 `json:"room_id,string,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	ViaSubstitution bool   `json:"via_substitution"`
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
		TrackingIndicators: TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}},
		Transfers:          []Transfer{},
	}
}

type snapshot struct {
	persons   map[int64]*userModels.Person
	locations *activeService.StudentLocationSnapshot
}

type buildState struct {
	selected   *educationModels.Group
	groups     []Group
	students   []*userModels.Student
	studentIDs []int64
	data       *snapshot
	projected  []Student
	today      timezone.Date
	arrivals   map[int64]*scheduleService.EffectiveArrivalTime
	pickups    map[int64]*scheduleService.EffectivePickupTime
	timetable  map[int64]struct{}
}

func (s *service) Get(ctx context.Context, requestedGroupID int64) (*Projection, error) {
	if err := s.validateDependencies(); err != nil {
		return nil, err
	}
	groups, selected, err := s.resolveGroups(ctx, requestedGroupID)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return EmptyProjection(), nil
	}
	ctx, err = s.prefetchSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve projection settings: %w", err)
	}

	state := &buildState{selected: selected, groups: groups, today: timezone.DateFromTime(s.deps.Now())}
	if err := s.loadStudents(ctx, state); err != nil {
		return nil, err
	}
	if err := s.loadPlanning(ctx, state); err != nil {
		return nil, err
	}
	return s.finishProjection(ctx, state)
}

func (s *service) validateDependencies() error {
	if s.deps.People == nil || s.deps.Education == nil || s.deps.UserContext == nil ||
		s.deps.Active == nil || s.deps.Settings == nil || s.deps.Pickups == nil ||
		s.deps.Arrivals == nil || s.deps.Instances == nil || s.deps.CareDays == nil ||
		s.deps.CareParticipation == nil || s.deps.StatusDays == nil {
		return errors.New("OGS group live service is not fully configured")
	}
	return nil
}

func (s *service) prefetchSettings(ctx context.Context) (context.Context, error) {
	batch, ok := s.deps.Settings.(configService.BatchSettingsService)
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveMany(ctx, []string{
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

func (s *service) resolveGroups(ctx context.Context, requestedGroupID int64) ([]Group, *educationModels.Group, error) {
	myGroups, err := s.deps.UserContext.GetMyGroups(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load supervised groups: %w", err)
	}
	if len(myGroups) == 0 {
		if requestedGroupID > 0 {
			return nil, nil, ErrForbiddenGroup
		}
		return nil, nil, nil
	}

	sortGroups(myGroups)
	selected := selectGroup(myGroups, requestedGroupID)
	if selected == nil {
		return nil, nil, ErrForbiddenGroup
	}

	substituted, err := s.deps.UserContext.GetSubstitutedGroupIDs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load substitution metadata: %w", err)
	}
	groups, err := s.mapGroups(ctx, myGroups, substituted)
	if err != nil {
		return nil, nil, err
	}
	return groups, selected, nil
}

func sortGroups(groups []*educationModels.Group) {
	sort.SliceStable(groups, func(i, j int) bool {
		return collation.CompareGerman(groups[i].Name, groups[j].Name) < 0
	})
}

func selectGroup(groups []*educationModels.Group, requestedGroupID int64) *educationModels.Group {
	if requestedGroupID <= 0 {
		return groups[0]
	}
	for _, group := range groups {
		if group.ID == requestedGroupID {
			return group
		}
	}
	return nil
}

func (s *service) mapGroups(ctx context.Context, groups []*educationModels.Group, substituted map[int64]bool) ([]Group, error) {
	// One bulk query resolves every room name; a per-group FindGroupWithRoom
	// loop would run on every aggregate refresh including SSE revalidations.
	roomGroupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.RoomID != nil {
			roomGroupIDs = append(roomGroupIDs, group.ID)
		}
	}
	withRooms := map[int64]*educationModels.Group{}
	if len(roomGroupIDs) > 0 {
		loaded, err := s.deps.Education.GetGroupsWithRoomsByIDs(ctx, roomGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("load group rooms: %w", err)
		}
		withRooms = loaded
	}

	result := make([]Group, 0, len(groups))
	for _, group := range groups {
		item := Group{ID: group.ID, Name: group.Name, RoomID: group.RoomID, ViaSubstitution: substituted[group.ID]}
		if withRoom := withRooms[group.ID]; withRoom != nil && withRoom.Room != nil {
			item.RoomName = withRoom.Room.Name
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *service) loadStudents(ctx context.Context, state *buildState) error {
	students, err := s.deps.People.GetParticipationCandidatesByGroupIDs(ctx, []int64{state.selected.ID})
	if err != nil {
		return fmt.Errorf("load group students: %w", err)
	}
	state.students = students
	studentIDs, personIDs := collectIDs(students)
	state.studentIDs = studentIDs

	data, err := s.loadSnapshot(ctx, studentIDs, personIDs)
	if err != nil {
		return fmt.Errorf("load student data snapshot: %w", err)
	}
	state.data = data
	if err := s.filterCareParticipation(ctx, state); err != nil {
		return err
	}

	photosEnabled, err := resolvePhotosEnabled(ctx, s.deps.Settings)
	if err != nil {
		return fmt.Errorf("resolve student photos setting: %w", err)
	}
	access := userContextService.ResolveStudentAccess(ctx, s.deps.UserContext)
	state.projected = projectStudents(state.students, data, access, photosEnabled)
	return s.applyStatusDays(ctx, state)
}

func (s *service) filterCareParticipation(ctx context.Context, state *buildState) error {
	participating, err := s.deps.CareParticipation.ParticipatingStudentIDs(
		ctx, state.studentIDs, state.today, nil,
	)
	if err != nil {
		return fmt.Errorf("apply care participation: %w", err)
	}
	students := state.students[:0]
	studentIDs := state.studentIDs[:0]
	for _, student := range state.students {
		if participating[student.ID] {
			students = append(students, student)
			studentIDs = append(studentIDs, student.ID)
		}
	}
	state.students = students
	state.studentIDs = studentIDs
	return nil
}

func resolvePhotosEnabled(ctx context.Context, settings boolSettingResolver) (bool, error) {
	return settings.ResolveBool(ctx, configModel.KeyStudentPhotosEnabled)
}

func collectIDs(students []*userModels.Student) ([]int64, []int64) {
	studentIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
	}
	return studentIDs, personIDs
}

func (s *service) loadSnapshot(ctx context.Context, studentIDs, personIDs []int64) (*snapshot, error) {
	persons := map[int64]*userModels.Person{}
	if len(personIDs) > 0 {
		loaded, err := s.deps.People.GetByIDs(ctx, personIDs)
		if err != nil {
			return nil, fmt.Errorf("bulk load persons: %w", err)
		}
		persons = loaded
	}
	locations, err := s.loadLocations(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load student locations: %w", err)
	}
	return &snapshot{persons: persons, locations: locations}, nil
}

func (s *service) loadLocations(ctx context.Context, studentIDs []int64) (*activeService.StudentLocationSnapshot, error) {
	ids := sliceutil.Unique(studentIDs)
	mode := s.deps.Active.GetPresenceMode(ctx)
	if mode == "" {
		mode = activeService.PresenceModeDetailed
	}
	result := &activeService.StudentLocationSnapshot{
		Mode: mode, Attendances: map[int64]*activeService.AttendanceStatus{},
		Visits: map[int64]*activeModels.Visit{}, Groups: map[int64]*activeModels.Group{},
	}
	if len(ids) == 0 {
		return result, nil
	}
	attendances, err := s.deps.Active.GetStudentsAttendanceStatuses(ctx, ids)
	if err != nil {
		return nil, err
	}
	if attendances != nil {
		result.Attendances = attendances
	}
	if mode == activeService.PresenceModeBinary {
		result.YardRoomColor = activeService.ResolveYardRoomColor(ctx, s.deps.Active)
		return result, nil
	}
	return s.loadDetailedLocations(ctx, result, ids)
}

func (s *service) loadDetailedLocations(
	ctx context.Context, result *activeService.StudentLocationSnapshot, studentIDs []int64,
) (*activeService.StudentLocationSnapshot, error) {
	if len(studentIDs) == 0 {
		return result, nil
	}
	visits, err := s.deps.Active.GetStudentsCurrentVisits(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	if visits != nil {
		result.Visits = visits
	}
	groupSet := make(map[int64]struct{})
	for _, visit := range result.Visits {
		if visit != nil && visit.ActiveGroupID > 0 {
			groupSet[visit.ActiveGroupID] = struct{}{}
		}
	}
	if len(groupSet) == 0 {
		return result, nil
	}
	groups, err := s.deps.Active.GetActiveGroupsByIDs(ctx, slices.Collect(maps.Keys(groupSet)))
	if err != nil {
		return nil, err
	}
	if groups != nil {
		result.Groups = groups
	}
	return result, nil
}

func projectStudents(students []*userModels.Student, data *snapshot, access *userContextService.StudentAccessContext, photosEnabled bool) []Student {
	result := make([]Student, 0, len(students))
	for _, model := range students {
		person := data.persons[model.PersonID]
		if person == nil {
			continue
		}
		fullAccess := access.HasFullAccessToStudent(model)
		location := data.locations.ResolveStudentLocationWithTime(model.ID, fullAccess)
		student := Student{
			ID: model.ID, FirstName: person.FirstName, LastName: person.LastName,
			SchoolClass: model.SchoolClass, Location: location.Location,
			LocationSince: location.Since, RoomColor: location.RoomColor, fullAccess: fullAccess,
		}
		if model.Sick != nil {
			student.Sick = *model.Sick
		}
		student.SickSince = model.SickSince
		if model.Excused != nil {
			student.Excused = *model.Excused
		}
		student.ExcusedSince = model.ExcusedSince
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
	rows, err := s.deps.StatusDays.GetActiveByStudentIDsAndDate(ctx, state.studentIDs, state.today)
	if err != nil {
		return fmt.Errorf("apply student status days: %w", err)
	}
	byStudent := make(map[int64][]*activeModels.StudentStatusDay)
	for _, row := range rows {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}
	for i := range state.projected {
		applyEffectiveStatus(&state.projected[i], activeService.ResolveEffectiveStatus(byStudent[state.projected[i].ID]))
	}
	return nil
}

func applyEffectiveStatus(student *Student, status activeService.EffectiveStatus) {
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
	arrivals, err := s.deps.Arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load arrival times: %w", err)
	}
	pickups, err := s.deps.Pickups.GetBulkEffectivePickupTimesForDate(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load pickup times: %w", err)
	}
	timetable, err := s.loadTimetable(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load timetable planning: %w", err)
	}
	pending, err := s.loadPendingExcused(ctx, state.today)
	if err != nil {
		return fmt.Errorf("load pending excused requests: %w", err)
	}
	state.arrivals, state.pickups, state.timetable = arrivals, pickups, timetable
	applyPlanning(state, pending)
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

func (s *service) loadTimetable(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]struct{}, error) {
	ids, err := s.deps.Instances.GetPlannedStudentIDsByDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	if len(result) == 0 {
		return result, nil
	}
	careDays, err := s.deps.CareDays.ResolveForDate(ctx, ids, date)
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

func (s *service) loadPendingExcused(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error) {
	// Same gate as the student list/detail badge: whoever may decide the
	// request may see its note — users:update, or users:absence under open
	// care (#2232). See authorize.CanReviewExcusedAbsenceRequests.
	if s.deps.ExcusedRequests == nil || !authorize.CanReviewExcusedAbsenceRequests(jwt.PermissionsFromCtx(ctx)) {
		return map[int64]*activeModels.ExcusedAbsenceRequest{}, nil
	}
	return s.deps.ExcusedRequests.PendingByStudentForDate(ctx, date)
}

func applyPlanning(state *buildState, pending map[int64]*activeModels.ExcusedAbsenceRequest) {
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
		_, hasTimetable := state.timetable[student.ID]
		attendance := state.data.locations.Attendances[student.ID]
		decision := scheduleService.ResolveDayPlanning(scheduleService.DayPlanningInputs{
			HasActualAttendance: attendance.IsCurrentlyPresent(),
			Sick:                student.Sick,
			ClassTrip:           student.ClassTrip,
			Excused:             student.Excused,
			Arrival:             state.arrivals[student.ID],
			Pickup:              state.pickups[student.ID],
			HasTimetable:        hasTimetable,
		})
		student.DayPlanningStatus = "not_coming_today"
		if decision.ComesToday {
			student.DayPlanningStatus = "comes_today"
		}
		student.DayPlanningReason = decision.Reason
		student.DayPlanningLabel = planningLabel(decision)
		applyTimes(student, state.arrivals[student.ID], attendance)
	}
}

func planningLabel(decision scheduleService.DayPlanningDecision) string {
	switch decision.Reason {
	case scheduleService.DayPlanningReasonUnplanned:
		return "ungeplant anwesend"
	case scheduleService.DayPlanningReasonSick:
		return "krank gemeldet"
	case scheduleService.DayPlanningReasonClassTrip:
		return "Klassenfahrt"
	case scheduleService.DayPlanningReasonExcused:
		return "entschuldigt"
	case scheduleService.DayPlanningReasonArrivalException:
		if decision.ComesToday {
			return "geplante Ankunft heute"
		}
		return exceptionLabel(decision.ExceptionNotes)
	case scheduleService.DayPlanningReasonArrivalSchedule:
		return "Ankunftsplan heute"
	case scheduleService.DayPlanningReasonPickupException:
		if decision.ComesToday {
			return "geplante Abholung heute"
		}
		return exceptionLabel(decision.ExceptionNotes)
	case scheduleService.DayPlanningReasonPickupSchedule:
		return "Abholplan heute"
	case scheduleService.DayPlanningReasonTimetable:
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

func applyTimes(student *Student, arrival *scheduleService.EffectiveArrivalTime, attendance *activeService.AttendanceStatus) {
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
			if note.Content != "" {
				parts = append(parts, note.Content)
			}
		}
		student.ArrivalNotes = strings.Join(parts, ", ")
	}
	if attendance != nil {
		student.ActualArrivalTime = timezone.FormatBerlinClock(attendance.CheckInTime)
		student.ActualPickupTime = timezone.FormatBerlinClock(attendance.CheckOutTime)
	}
}

func (s *service) finishProjection(ctx context.Context, state *buildState) (*Projection, error) {
	selectedID := state.selected.ID
	projection := &Projection{
		Groups:             state.groups,
		GroupID:            &selectedID,
		Students:           state.projected,
		RoomStatus:         map[string]RoomStatus{},
		PickupTimes:        pickupTimes(state.projected, state.pickups),
		TrackingIndicators: TrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}},
		Transfers:          []Transfer{},
	}

	// The group-derived sections mirror the permission split of the former
	// single endpoints: room status, transfers, and tracking indicators were
	// gated on groups:read, while the roster itself needed only users:read.
	// A caller without groups:read gets these sections deterministically
	// empty (permission-based redaction) instead of a 403 for the whole
	// roster — degradation by permission, never by swallowed errors.
	if !authorize.HasPermission(permissions.GroupsRead, jwt.PermissionsFromCtx(ctx)) {
		return projection, nil
	}

	tracking, err := s.loadTracking(ctx, state.studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load tracking indicators: %w", err)
	}
	transfers, err := s.loadTransfers(ctx, state.selected.ID, state.today)
	if err != nil {
		return nil, fmt.Errorf("load group transfers: %w", err)
	}
	projection.RoomStatus = roomStatuses(state.projected, state.data.locations, state.selected.RoomID)
	projection.TrackingIndicators = tracking
	projection.Transfers = transfers
	return projection, nil
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

func (s *service) loadTransfers(ctx context.Context, groupID int64, date timezone.Date) ([]Transfer, error) {
	substitutions, err := s.deps.Education.GetActiveGroupSubstitutions(ctx, groupID, date)
	if err != nil {
		return nil, err
	}
	result := make([]Transfer, 0, len(substitutions))
	for _, sub := range substitutions {
		if sub.RegularStaffID != nil {
			continue
		}
		item := Transfer{
			ID:                sub.ID,
			GroupID:           sub.GroupID,
			SubstituteStaffID: sub.SubstituteStaffID,
			EndDate:           sub.EndDate.String(),
		}
		if sub.SubstituteStaff != nil && sub.SubstituteStaff.Person != nil {
			item.SubstituteName = strings.TrimSpace(sub.SubstituteStaff.Person.FirstName + " " + sub.SubstituteStaff.Person.LastName)
		}
		result = append(result, item)
	}
	return result, nil
}

func roomStatuses(students []Student, locations *activeService.StudentLocationSnapshot, groupRoomID *int64) map[string]RoomStatus {
	result := make(map[string]RoomStatus, len(students))
	for i := range students {
		key := strconv.FormatInt(students[i].ID, 10)
		status := RoomStatus{}
		if groupRoomID != nil && locations != nil {
			if visit := locations.Visits[students[i].ID]; visit != nil {
				if group := locations.Groups[visit.ActiveGroupID]; group != nil {
					roomID := group.RoomID
					status.CurrentRoomID = &roomID
					status.InGroupRoom = roomID == *groupRoomID
				}
			}
		}
		result[key] = status
	}
	return result
}

func pickupTimes(students []Student, pickups map[int64]*scheduleService.EffectivePickupTime) []PickupTime {
	result := make([]PickupTime, 0, len(students))
	for i := range students {
		if !students[i].fullAccess {
			continue
		}
		effective := pickups[students[i].ID]
		if effective == nil {
			continue
		}
		item := PickupTime{
			StudentID: students[i].ID, Date: effective.Date.String(), WeekdayName: effective.WeekdayName,
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
