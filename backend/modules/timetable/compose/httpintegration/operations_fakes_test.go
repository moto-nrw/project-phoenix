package httpintegration_test

import (
	"context"
	"strings"
	"time"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The operational day's behaviour tests run the owner's public capability
// against in-memory fakes of its ports. The fakes keep the answers of the
// retained repositories and services the composition root binds: the
// settings map the configured strings the way the root's settings binding
// does, and the care-day port applies Care Plan's row rule, so the tests pin
// what the owner hands over and what it makes of the answers.

// Setting values as the Settings Platform stores them.
const (
	overviewScopeKey                = "operations.operational_overview_scope"
	overviewScopeOwn                = "own"
	overviewScopeAdmins             = "admins"
	overviewScopeAllStaff           = "all_staff"
	attendanceEditScopeOwn          = "own"
	attendanceEditScopeAllStaff     = "all_staff"
	studentAbsenceEditScopeAllStaff = "all_staff"
	groupModeFixedGroups            = "fixed_groups"
	groupModeOpenCare               = "open_care"
)

// opsNow is the fixed wall clock of blocks whose times do not matter.
var opsNow = time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC)

// notFoundError is a repository miss as the retained repositories report it.
type notFoundError struct{}

func (notFoundError) Error() string       { return "not found" }
func (notFoundError) RepositoryNotFound() {}

func wireAssignedStaff(deps *timetableOpsTestDeps, accountID, personID, staffID, instanceID int64) {
	deps.personService.accountPerson = &usersModels.Person{}
	deps.personService.accountPerson.ID = personID
	deps.personService.staffByPersonID[personID] = &usersModels.Staff{}
	deps.personService.staffByPersonID[personID].ID = staffID
	deps.staffRepo.assign(instanceID, &scheduleModels.InstanceStaff{StaffID: staffID})
}

func instanceWithTimes(id int64, status string, start, end time.Time) *scheduleModels.ActivityInstance {
	return instanceWithRoomAndTimes(id, 810, status, start, end)
}

func instanceWithRoomAndTimes(id, roomID int64, status string, start, end time.Time) *scheduleModels.ActivityInstance {
	inst := &scheduleModels.ActivityInstance{
		Date:      scheduleModels.NewDate(start.Year(), start.Month(), start.Day()),
		Title:     "Lernzeit",
		StartTime: start,
		EndTime:   end,
		RoomID:    roomID,
		Status:    status,
	}
	inst.ID = id
	return inst
}

func activeInstance(id, activeGroupID int64) *scheduleModels.ActivityInstance {
	inst := instanceWithTimes(id, scheduleModels.InstanceStatusActive, time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC), time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	inst.ID = id
	inst.ActiveGroupID = &activeGroupID
	return inst
}

// normalizeSchoolClass is the LOWER(BTRIM(...)) class match every
// school_class join uses; the root binds schoolclass.Normalize.
func normalizeSchoolClass(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

type timetableOpsTestDeps struct {
	service         timetable.OperationCapability
	now             func() time.Time
	instanceRepo    *fakeOpsInstanceRepo
	staffRepo       *fakeOpsStaffRepo
	studentRepo     *fakeOpsInstanceStudentRepo
	instanceService *fakeOpsLifecycle
	activeGroups    *fakeOpsSessions
	activityGroups  *fakeOpsTemplates
	activeService   *fakeOpsAttendance
	arrivalService  *fakeOpsArrivals
	pickupService   *fakeOpsPickups
	supervisors     *fakeOpsSupervisions
	visitRepo       *fakeOpsVisitRepo
	students        *fakeOpsStudentRepo
	groups          *fakeOpsEducationGroups
	rooms           *fakeOpsRooms
	personService   *fakeOpsPersonService
	tracks          *fakeOpsPlanningTracks
	settings        *fakeOpsSettings
	announcer       *fakeOpsAnnouncer
	careDayService  *fakeOpsCareDays
}

func newTimetableOpsDeps() *timetableOpsTestDeps {
	deps := &timetableOpsTestDeps{
		instanceRepo:    &fakeOpsInstanceRepo{byID: map[int64]*scheduleModels.ActivityInstance{}},
		staffRepo:       &fakeOpsStaffRepo{byInstance: map[int64][]*scheduleModels.InstanceStaff{}},
		studentRepo:     &fakeOpsInstanceStudentRepo{byInstance: map[int64][]*scheduleModels.InstanceStudent{}, byInstanceStudent: map[instanceStudentKey]*scheduleModels.InstanceStudent{}},
		instanceService: &fakeOpsLifecycle{},
		activeGroups:    &fakeOpsSessions{lastActivity: map[int64]time.Time{}},
		activityGroups:  &fakeOpsTemplates{byID: map[int64]*activitiesModels.Group{}, targetsByGroup: map[int64][]*activitiesModels.GroupTarget{}},
		activeService:   &fakeOpsAttendance{},
		arrivalService:  &fakeOpsArrivals{byStudent: map[int64]*compose.ExpectedArrival{}},
		pickupService:   &fakeOpsPickups{byStudent: map[int64]*time.Time{}},
		careDayService:  &fakeOpsCareDays{byStudent: map[int64]timetable.CareDayStatus{}},
		supervisors:     &fakeOpsSupervisions{byActiveGroup: map[int64][]*studentpresence.StaffedSupervision{}},
		visitRepo: &fakeOpsVisitRepo{
			byActiveGroup:            map[int64][]*studentpresence.Visit{},
			currentByStudent:         map[int64]*studentpresence.Visit{},
			currentByStudentSequence: map[int64][]*studentpresence.Visit{},
		},
		students:      &fakeOpsStudentRepo{byID: map[int64]*usersModels.Student{}},
		groups:        &fakeOpsEducationGroups{names: map[int64]string{}},
		rooms:         &fakeOpsRooms{names: map[int64]string{810: "Lernraum"}},
		personService: &fakeOpsPersonService{people: map[int64]*usersModels.Person{}, staffByPersonID: map[int64]*usersModels.Staff{}, staffWithPerson: map[int64]*usersModels.Staff{}},
		tracks:        &fakeOpsPlanningTracks{byID: map[int64]timetable.PlanningTrack{}},
		settings:      &fakeOpsSettings{attendanceScope: attendanceEditScopeOwn, absenceScope: studentAbsenceEditScopeAllStaff},
		announcer:     &fakeOpsAnnouncer{},
	}
	service, err := compose.NewOperations(deps.dependencies())
	if err != nil {
		panic(err)
	}
	deps.service = service
	return deps
}

// dependencies wires every fake; the clock follows deps.now, else the wall
// clock as in production.
func (deps *timetableOpsTestDeps) dependencies() compose.OperationDependencies {
	return compose.OperationDependencies{
		Instances:            deps.instanceRepo,
		InstanceStaff:        deps.staffRepo,
		Participants:         deps.studentRepo,
		Templates:            deps.activityGroups,
		PlanningTracks:       deps.tracks,
		Sessions:             deps.activeGroups,
		Supervisions:         deps.supervisors,
		Visits:               deps.visitRepo,
		Attendance:           deps.activeService,
		Students:             deps.students,
		People:               deps.personService,
		EducationGroups:      deps.groups,
		Rooms:                deps.rooms,
		Settings:             deps.settings,
		CareDays:             deps.careDayService,
		Arrivals:             deps.arrivalService,
		Pickups:              deps.pickupService,
		Lifecycle:            deps.instanceService,
		Announcer:            deps.announcer,
		NormalizeSchoolClass: normalizeSchoolClass,
		Now: func() time.Time {
			if deps.now != nil {
				return deps.now()
			}
			return time.Now()
		},
	}
}

type fakeOpsInstanceRepo struct {
	byID          map[int64]*scheduleModels.ActivityInstance
	byDate        []*scheduleModels.ActivityInstance
	err           error
	findByDateErr error
}

func (r *fakeOpsInstanceRepo) FindByID(_ context.Context, id any) (*scheduleModels.ActivityInstance, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byID[id.(int64)], nil
}

func (r *fakeOpsInstanceRepo) FindByTenantAndDate(_ context.Context, _ scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error) {
	if r.findByDateErr != nil {
		return nil, r.findByDateErr
	}
	return r.byDate, nil
}

func (r *fakeOpsInstanceRepo) FindByActiveGroupID(_ context.Context, activeGroupID int64) (*scheduleModels.ActivityInstance, error) {
	for _, inst := range r.byID {
		if inst != nil && inst.ActiveGroupID != nil && *inst.ActiveGroupID == activeGroupID {
			return inst, nil
		}
	}
	return nil, nil
}

type fakeOpsStaffRepo struct {
	byInstance map[int64][]*scheduleModels.InstanceStaff
	err        error
}

func (r *fakeOpsStaffRepo) FindByInstanceID(_ context.Context, instanceID int64) ([]*scheduleModels.InstanceStaff, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstance[instanceID], nil
}

// FindByInstanceIDs answers the batched read with the seeded rows, stamped
// with their instance the way the repository returns them.
func (r *fakeOpsStaffRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStaff, error) {
	if r.err != nil {
		return nil, r.err
	}
	var rows []*scheduleModels.InstanceStaff
	for _, instanceID := range instanceIDs {
		for _, row := range r.byInstance[instanceID] {
			stamped := *row
			stamped.InstanceID = instanceID
			rows = append(rows, &stamped)
		}
	}
	return rows, nil
}

// assign seeds the staff rows FindByInstanceID returns for one instance.
func (r *fakeOpsStaffRepo) assign(instanceID int64, rows ...*scheduleModels.InstanceStaff) {
	r.byInstance[instanceID] = rows
}

type instanceStudentKey struct {
	instanceID int64
	studentID  int64
}

type attendanceUpdate struct {
	rowID int64
	patch scheduleModels.AttendanceFieldPatch
}

type fakeOpsInstanceStudentRepo struct {
	byInstance           map[int64][]*scheduleModels.InstanceStudent
	byInstanceStudent    map[instanceStudentKey]*scheduleModels.InstanceStudent
	err                  error
	updateErr            error
	updates              []attendanceUpdate
	parallelPresence     []scheduleModels.ParallelPresence
	parallelPresenceErr  error
	parallelPresenceCall int
}

func (r *fakeOpsInstanceStudentRepo) FindPresentInOtherActiveInstances(_ context.Context, excludeInstanceID int64, _ scheduleModels.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	r.parallelPresenceCall++
	if r.parallelPresenceErr != nil {
		return nil, r.parallelPresenceErr
	}
	requested := map[int64]bool{}
	for _, id := range studentIDs {
		requested[id] = true
	}
	rows := make([]scheduleModels.ParallelPresence, 0, len(r.parallelPresence))
	for _, row := range r.parallelPresence {
		if row.InstanceID != excludeInstanceID && requested[row.StudentID] {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceID(_ context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstance[instanceID], nil
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	var rows []*scheduleModels.InstanceStudent
	for _, instanceID := range instanceIDs {
		for _, row := range r.byInstance[instanceID] {
			stamped := *row
			stamped.InstanceID = instanceID
			rows = append(rows, &stamped)
		}
	}
	return rows, nil
}

func (r *fakeOpsInstanceStudentRepo) FindByInstanceAndStudent(_ context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byInstanceStudent[instanceStudentKey{instanceID, studentID}], nil
}

func (r *fakeOpsInstanceStudentRepo) UpdateAttendanceFields(_ context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updates = append(r.updates, attendanceUpdate{rowID: id, patch: patch})
	return nil
}

type startCall struct{ instanceID, staffID int64 }

// fakeOpsLifecycle stands in for the retained instance lifecycle.
type fakeOpsLifecycle struct {
	started   []startCall
	completed []int64
}

func (s *fakeOpsLifecycle) CreateSpontaneous(context.Context, timetable.SpontaneousStart) (int64, error) {
	return 0, nil
}

func (s *fakeOpsLifecycle) Start(_ context.Context, instanceID, staffID int64, _ bool) (*timetable.StartedOperation, error) {
	s.started = append(s.started, startCall{instanceID: instanceID, staffID: staffID})
	return &timetable.StartedOperation{InstanceID: instanceID, Status: scheduleModels.InstanceStatusActive, ActiveGroupID: 910}, nil
}

func (s *fakeOpsLifecycle) Complete(_ context.Context, instanceID, _ int64) (*timetable.ScheduledInstance, error) {
	s.completed = append(s.completed, instanceID)
	return &timetable.ScheduledInstance{ID: instanceID, Status: scheduleModels.InstanceStatusCompleted}, nil
}

func (s *fakeOpsLifecycle) Reopen(_ context.Context, instanceID, _ int64, _ bool) (*timetable.StartedOperation, error) {
	return &timetable.StartedOperation{InstanceID: instanceID, Status: scheduleModels.InstanceStatusActive, ActiveGroupID: 911}, nil
}

type fakeOpsSessions struct {
	byID         map[int64]*studentpresence.LiveGroup
	lastActivity map[int64]time.Time
	updateErr    error
}

func (r *fakeOpsSessions) FindSession(_ context.Context, id int64) (*studentpresence.LiveGroup, error) {
	group := r.byID[id]
	if group == nil {
		return nil, notFoundError{}
	}
	return group, nil
}

func (r *fakeOpsSessions) UpdateLastActivity(_ context.Context, id int64, lastActivity time.Time) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.lastActivity[id] = lastActivity
	return nil
}

type fakeOpsTemplates struct {
	byID           map[int64]*activitiesModels.Group
	targetsByGroup map[int64][]*activitiesModels.GroupTarget
	err            error
}

func (r *fakeOpsTemplates) FindTargetsByGroupIDs(_ context.Context, groupIDs []int64) (map[int64][]*activitiesModels.GroupTarget, error) {
	result := make(map[int64][]*activitiesModels.GroupTarget, len(groupIDs))
	for _, groupID := range groupIDs {
		result[groupID] = r.targetsByGroup[groupID]
	}
	return result, nil
}

func (r *fakeOpsTemplates) FindByID(_ context.Context, id any) (*activitiesModels.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	group := r.byID[id.(int64)]
	if group == nil {
		return nil, notFoundError{}
	}
	return group, nil
}

func (r *fakeOpsTemplates) FindByIDs(_ context.Context, ids []int64) ([]*activitiesModels.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	groups := make([]*activitiesModels.Group, 0, len(ids))
	for _, id := range ids {
		if group := r.byID[id]; group != nil {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

type opsMoveCall struct {
	studentIDs    []int64
	activeGroupID int64
	auth          studentpresence.StudentMoveAuthorization
}

// fakeOpsAttendance records the visit writes of the Student Presence port.
type fakeOpsAttendance struct {
	created    []*studentpresence.Visit
	ended      []int64
	createErr  error
	endErr     error
	moveCalls  []opsMoveCall
	moveResult *studentpresence.StudentMoveResult
	moveErr    error
}

func (s *fakeOpsAttendance) MoveStudentsToActiveGroupAuthorized(_ context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error) {
	s.moveCalls = append(s.moveCalls, opsMoveCall{studentIDs: studentIDs, activeGroupID: activeGroupID, auth: auth})
	if s.moveErr != nil {
		return nil, s.moveErr
	}
	if s.moveResult != nil {
		return s.moveResult, nil
	}
	return &studentpresence.StudentMoveResult{Moved: studentIDs, ActiveGroupID: &activeGroupID}, nil
}

func (s *fakeOpsAttendance) CreateVisitAs(_ context.Context, _ int64, visit *studentpresence.Visit) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = append(s.created, visit)
	return nil
}

func (s *fakeOpsAttendance) EndVisitAs(_ context.Context, _, _, visitID int64) error {
	if s.endErr != nil {
		return s.endErr
	}
	s.ended = append(s.ended, visitID)
	return nil
}

// fakeOpsArrivals answers every asked child: a configured arrival, else an
// entry without a time, as Care Plan's bulk read does.
type fakeOpsArrivals struct {
	byStudent map[int64]*compose.ExpectedArrival
	err       error
}

func (s *fakeOpsArrivals) EffectiveArrivals(_ context.Context, studentIDs []int64, _ calendar.Date) (map[int64]*compose.ExpectedArrival, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int64]*compose.ExpectedArrival, len(studentIDs))
	for _, studentID := range studentIDs {
		if arrival := s.byStudent[studentID]; arrival != nil {
			out[studentID] = arrival
			continue
		}
		out[studentID] = &compose.ExpectedArrival{}
	}
	return out, nil
}

// fakeOpsPickups records the one bulk read a roster may make.
type fakeOpsPickups struct {
	byStudent  map[int64]*time.Time
	err        error
	calls      int
	studentIDs []int64
	date       calendar.Date
}

func (s *fakeOpsPickups) EffectivePickups(_ context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error) {
	s.calls++
	s.studentIDs = append([]int64(nil), studentIDs...)
	s.date = date
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int64]*time.Time, len(studentIDs))
	for _, studentID := range studentIDs {
		out[studentID] = s.byStudent[studentID]
	}
	return out, nil
}

type fakeOpsSupervisions struct {
	byActiveGroup map[int64][]*studentpresence.StaffedSupervision
}

func (r *fakeOpsSupervisions) FindByActiveGroupID(_ context.Context, activeGroupID int64, _ bool) ([]*studentpresence.StaffedSupervision, error) {
	return r.byActiveGroup[activeGroupID], nil
}

type fakeOpsVisitRepo struct {
	byActiveGroup            map[int64][]*studentpresence.Visit
	currentByStudent         map[int64]*studentpresence.Visit
	currentByStudentSequence map[int64][]*studentpresence.Visit
	err                      error
}

func (r *fakeOpsVisitRepo) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(filter.ActiveGroupIDs) > 0 {
		var visits []studentpresence.Visit
		for _, visit := range r.byActiveGroup[filter.ActiveGroupIDs[0]] {
			visits = append(visits, *visit)
		}
		return visits, nil
	}
	studentID := filter.StudentIDs[0]
	current := r.currentByStudent[studentID]
	if sequence := r.currentByStudentSequence[studentID]; len(sequence) > 0 {
		current = sequence[0]
		r.currentByStudentSequence[studentID] = sequence[1:]
	}
	if current == nil {
		return nil, nil
	}
	return []studentpresence.Visit{*current}, nil
}

type fakeOpsStudentRepo struct {
	byID map[int64]*usersModels.Student
}

func (r *fakeOpsStudentRepo) FindByIDs(_ context.Context, ids []int64) (map[int64]*usersModels.Student, error) {
	out := map[int64]*usersModels.Student{}
	for _, id := range ids {
		if st := r.byID[id]; st != nil {
			out[id] = st
		}
	}
	return out, nil
}

type fakeOpsEducationGroups struct {
	names map[int64]string
}

func (r *fakeOpsEducationGroups) EducationGroupNames(_ context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	for _, id := range ids {
		if name, ok := r.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

type fakeOpsRooms struct {
	names map[int64]string
	err   error
}

func (r *fakeOpsRooms) AllRoomNames(context.Context) (map[int64]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[int64]string, len(r.names))
	for id, name := range r.names {
		out[id] = name
	}
	return out, nil
}

func (r *fakeOpsRooms) RoomName(_ context.Context, id int64) (string, bool, error) {
	if r.err != nil {
		return "", false, r.err
	}
	name, ok := r.names[id]
	return name, ok, nil
}

type fakeOpsPersonService struct {
	accountPerson   *usersModels.Person
	accountErr      error
	people          map[int64]*usersModels.Person
	staffByPersonID map[int64]*usersModels.Staff
	staffWithPerson map[int64]*usersModels.Staff
}

func (s *fakeOpsPersonService) FindByAccountID(context.Context, int64) (*usersModels.Person, error) {
	if s.accountErr != nil {
		return nil, s.accountErr
	}
	return s.accountPerson, nil
}

func (s *fakeOpsPersonService) GetByIDs(_ context.Context, ids []int64) (map[int64]*usersModels.Person, error) {
	out := map[int64]*usersModels.Person{}
	for _, id := range ids {
		if person := s.people[id]; person != nil {
			out[id] = person
		}
	}
	return out, nil
}

func (s *fakeOpsPersonService) GetStaffByPersonID(_ context.Context, personID int64) (*usersModels.Staff, error) {
	return s.staffByPersonID[personID], nil
}

func (s *fakeOpsPersonService) GetStaffWithPersonByIDs(_ context.Context, ids []int64) (map[int64]*usersModels.Staff, error) {
	out := map[int64]*usersModels.Staff{}
	for _, id := range ids {
		if staff := s.staffWithPerson[id]; staff != nil {
			out[id] = staff
		}
	}
	return out, nil
}

type fakeOpsPlanningTracks struct {
	timetable.PlanningTrackQuery
	byID map[int64]timetable.PlanningTrack
}

func (r *fakeOpsPlanningTracks) ListPlanningTracks(_ context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	tracks := make([]timetable.PlanningTrack, 0, len(filter.IDs))
	for _, id := range filter.IDs {
		if track, ok := r.byID[id]; ok {
			tracks = append(tracks, track)
		}
	}
	return tracks, nil
}

// fakeOpsSettings keeps the configured setting values and maps them the way
// the composition root's settings binding does: an unknown attendance-edit
// scope grants nothing, only "all_staff" opens absence reports and the
// operational overview. mode answers every other key (the group mode).
type fakeOpsSettings struct {
	attendanceScope string
	absenceScope    string
	err             error
	mode            string
	scope           string
	stringErr       error
	leadMinutes     int
}

func (s *fakeOpsSettings) ResolveString(_ context.Context, key string) (string, error) {
	if s.stringErr != nil {
		return "", s.stringErr
	}
	if key == overviewScopeKey {
		return s.scope, nil
	}
	return s.mode, nil
}

func (s *fakeOpsSettings) StartLeadMinutes(context.Context) (int, error) {
	if s.leadMinutes > 0 {
		return s.leadMinutes, s.err
	}
	return 15, s.err
}

func (s *fakeOpsSettings) EnforcePlannedEnd(context.Context) (bool, error) {
	return false, s.err
}

func (s *fakeOpsSettings) AttendanceEditScope(context.Context) (compose.AttendanceEditScope, error) {
	if s.stringErr != nil {
		return compose.AttendanceEditUnset, s.stringErr
	}
	switch s.attendanceScope {
	case attendanceEditScopeOwn:
		return compose.AttendanceEditOwn, nil
	case attendanceEditScopeAllStaff:
		return compose.AttendanceEditAllStaff, nil
	}
	return compose.AttendanceEditUnset, nil
}

func (s *fakeOpsSettings) StudentAbsenceEditAllStaff(context.Context) (bool, error) {
	if s.stringErr != nil {
		return false, s.stringErr
	}
	return s.absenceScope == studentAbsenceEditScopeAllStaff, nil
}

func (s *fakeOpsSettings) OperationalOverviewAllStaff(context.Context) (bool, error) {
	if s.stringErr != nil {
		return false, s.stringErr
	}
	return s.scope == overviewScopeAllStaff, nil
}

// fakeOpsCareDays reports the care-plan verdict per student. Empty by
// default, which reads as "unknown" everywhere — the pre-#1747 behaviour.
// The row rule and the expected/exempt reading are Care Plan's
// (careplan.AttendanceRowCareDay, CareDayStatus.Expected/ExemptFromAbsence).
type fakeOpsCareDays struct {
	byStudent map[int64]timetable.CareDayStatus
}

func (f *fakeOpsCareDays) ResolveForDate(_ context.Context, studentIDs []int64, _ calendar.Date) (map[int64]timetable.CareDayStatus, error) {
	out := make(map[int64]timetable.CareDayStatus, len(studentIDs))
	for _, id := range studentIDs {
		if status, ok := f.byStudent[id]; ok {
			out[id] = status
		}
	}
	return out, nil
}

func (f *fakeOpsCareDays) ResolveForRange(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]timetable.CareDayStatus, error) {
	byDate, err := f.ResolveForDate(ctx, studentIDs, from)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]map[calendar.Date]timetable.CareDayStatus, len(byDate))
	for studentID, status := range byDate {
		out[studentID] = map[calendar.Date]timetable.CareDayStatus{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			out[studentID][date] = status
		}
	}
	return out, nil
}

func (*fakeOpsCareDays) AttendanceRowCareDay(instanceCompleted bool, row timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus {
	switch {
	case row.ManuallyDecided:
		return timetable.CareDayUnknown
	case instanceCompleted:
		if row.NotScheduled && row.Expected {
			return timetable.CareDayNotScheduled
		}
		return timetable.CareDayUnknown
	case !row.Expected:
		if planVerdict == timetable.CareDayNotScheduled && row.PlanOwnedAbsence {
			return timetable.CareDayNotScheduled
		}
		return timetable.CareDayUnknown
	case planVerdict != "":
		return planVerdict
	}
	return timetable.CareDayUnknown
}

func (*fakeOpsCareDays) Expected(status timetable.CareDayStatus) bool {
	return status != timetable.CareDayNotScheduled && status != timetable.CareDayCancelled
}

func (*fakeOpsCareDays) ExemptFromAbsence(status timetable.CareDayStatus) bool {
	return status == timetable.CareDayNotScheduled
}

type announcement struct{ tenantID, activeGroupID, instanceID int64 }

// fakeOpsAnnouncer records the attendance wake-ups the owner hands over.
type fakeOpsAnnouncer struct {
	calls []announcement
	err   error
}

func (a *fakeOpsAnnouncer) AnnounceAttendanceChanged(tenantID, activeGroupID, instanceID int64) error {
	a.calls = append(a.calls, announcement{tenantID: tenantID, activeGroupID: activeGroupID, instanceID: instanceID})
	return a.err
}
