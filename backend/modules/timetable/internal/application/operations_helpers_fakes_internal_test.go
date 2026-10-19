package application

import (
	"context"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The operational day's internal helpers are checked here against the few
// ports they touch; the public capability runs against the full fake set in
// httpintegration (operations_*_test.go). The operations value is built
// directly with only those ports, since NewOperations requires all of them.

type timetableOpsTestDeps struct {
	service        *operations
	instanceRepo   *helperInstances
	personService  *helperPeople
	activityGroups *helperTemplates
	settings       *helperSettings
	announcer      *helperAnnouncer
}

func newTimetableOpsDeps() *timetableOpsTestDeps {
	deps := &timetableOpsTestDeps{
		instanceRepo:   &helperInstances{byID: map[int64]*scheduleModels.ActivityInstance{}},
		personService:  &helperPeople{},
		activityGroups: &helperTemplates{byID: map[int64]*activityContract.Group{}},
		settings:       &helperSettings{},
		announcer:      &helperAnnouncer{},
	}
	deps.service = &operations{caller: sessionlessCaller{}, deps: ports.OperationDependencies{
		Instances: deps.instanceRepo,
		People:    deps.personService,
		Templates: deps.activityGroups,
		Settings:  deps.settings,
		Rooms:     helperRooms{810: "Lernraum"},
		CareDays:  careRuleDays{},
		Announcer: deps.announcer,
	}}
	return deps
}

func instanceWithTimes(id int64, status string, start, end time.Time) *scheduleModels.ActivityInstance {
	inst := &scheduleModels.ActivityInstance{
		Date:      calendar.NewDate(start.Year(), start.Month(), start.Day()),
		Title:     "Lernzeit",
		StartTime: start,
		EndTime:   end,
		RoomID:    810,
		Status:    status,
	}
	inst.ID = id
	return inst
}

func activeInstance(id, activeGroupID int64) *scheduleModels.ActivityInstance {
	inst := instanceWithTimes(id, scheduleModels.InstanceStatusActive, time.Date(2026, time.May, 10, 14, 0, 0, 0, time.UTC), time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC))
	inst.ActiveGroupID = &activeGroupID
	return inst
}

// helperNotFound is a repository miss as the retained repositories report it.
type helperNotFound struct{}

func (helperNotFound) Error() string       { return "not found" }
func (helperNotFound) RepositoryNotFound() {}

type helperInstances struct {
	byID map[int64]*scheduleModels.ActivityInstance
	err  error
}

func (r *helperInstances) FindByID(_ context.Context, nativeRecordID int64) (*scheduleModels.ActivityInstance, error) {
	var id any = nativeRecordID
	if r.err != nil {
		return nil, r.err
	}
	return r.byID[id.(int64)], nil
}

func (r *helperInstances) FindByTenantAndDate(context.Context, calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

func (r *helperInstances) FindByActiveGroupID(context.Context, int64) (*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

type helperPeople struct {
	accountPerson *timetable.Person
	staffErr      error
}

func (p *helperPeople) FindByAccountID(context.Context, int64) (*timetable.Person, error) {
	return p.accountPerson, nil
}

func (p *helperPeople) GetByIDs(context.Context, []int64) (map[int64]*timetable.Person, error) {
	return nil, nil
}

func (p *helperPeople) GetStaffByPersonID(context.Context, int64) (*timetable.StaffMember, error) {
	return nil, p.staffErr
}

func (p *helperPeople) GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*timetable.StaffMember, error) {
	return nil, nil
}

type helperTemplates struct {
	byID map[int64]*activityContract.Group
	err  error
}

func (r *helperTemplates) FindByID(_ context.Context, id int64) (*activityContract.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	group := r.byID[id]
	if group == nil {
		return nil, helperNotFound{}
	}
	return group, nil
}

func (r *helperTemplates) FindByIDs(context.Context, []int64) ([]*activityContract.Group, error) {
	return nil, r.err
}

func (r *helperTemplates) FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activityContract.GroupTarget, error) {
	return map[int64][]*activityContract.GroupTarget{}, r.err
}

// helperSettings answers every policy as unset; stringErr fails them all.
type helperSettings struct {
	stringErr error
}

func (s *helperSettings) ResolveString(context.Context, string) (string, error) {
	return "", s.stringErr
}

func (s *helperSettings) StartLeadMinutes(context.Context) (int, error) { return 15, nil }

func (s *helperSettings) EnforcePlannedEnd(context.Context) (bool, error) { return false, nil }

func (s *helperSettings) CompleteLeadMinutes(context.Context) (int, error) { return 0, nil }

func (s *helperSettings) ActionScopeKey(ports.ScopedAction) (string, error) {
	return "operations.attendance_edit_scope", nil
}

func (s *helperSettings) StudentAbsenceEditAllStaff(context.Context) (bool, error) {
	return false, s.stringErr
}

type helperRooms map[int64]string

func (r helperRooms) AllRoomNames(context.Context) (map[int64]string, error) {
	return r, nil
}

func (r helperRooms) RoomName(_ context.Context, id int64) (string, bool, error) {
	name, ok := r[id]
	return name, ok, nil
}

type announcement struct{ tenantID, activeGroupID, instanceID int64 }

type helperAnnouncer struct {
	calls []announcement
	err   error
}

func (a *helperAnnouncer) AnnounceAttendanceChanged(tenantID, activeGroupID, instanceID int64) error {
	a.calls = append(a.calls, announcement{tenantID: tenantID, activeGroupID: activeGroupID, instanceID: instanceID})
	return a.err
}

// careRuleDays applies Care Plan's row rule and expected/exempt reading
// (careplan.AttendanceRowCareDay, CareDayStatus.Expected/ExemptFromAbsence);
// the plan verdicts come from the caller.
type careRuleDays struct{}

func (careRuleDays) ResolveForDate(context.Context, []int64, calendar.Date) (map[int64]timetable.CareDayStatus, error) {
	return map[int64]timetable.CareDayStatus{}, nil
}

func (careRuleDays) ResolveForRange(context.Context, []int64, calendar.Date, calendar.Date) (map[int64]map[calendar.Date]timetable.CareDayStatus, error) {
	return map[int64]map[calendar.Date]timetable.CareDayStatus{}, nil
}

func (careRuleDays) AttendanceRowCareDay(instanceCompleted bool, row timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus {
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

func (careRuleDays) Expected(status timetable.CareDayStatus) bool {
	return status != timetable.CareDayNotScheduled && status != timetable.CareDayCancelled
}

func (careRuleDays) ExemptFromAbsence(status timetable.CareDayStatus) bool {
	return status == timetable.CareDayNotScheduled
}

// sessionlessCaller is the caller of a request without a session token: no
// portal, no admin, no OGS actor. The two settings-backed gates resolve the
// settings first, so a settings fault fails them the way the shared rules do.
type sessionlessCaller struct{}

func (sessionlessCaller) AssignmentBoundPortal(context.Context) bool { return false }
func (sessionlessCaller) AdministrativeAccess(context.Context) bool  { return false }
func (sessionlessCaller) OGSActorToken(context.Context, int64) bool  { return false }

func (sessionlessCaller) SchoolWideActionScope(ctx context.Context, settings ports.OperationSettings, scopeKey string) (bool, error) {
	if _, err := settings.ResolveString(ctx, scopeKey); err != nil {
		return false, err
	}
	return false, nil
}

func (sessionlessCaller) OperationalOverview(ctx context.Context, settings ports.OperationSettings, assignmentBound, administrative, _ bool) (bool, error) {
	if _, err := settings.ResolveString(ctx, "operations.overview_scope"); err != nil {
		return false, err
	}
	return administrative && !assignmentBound, nil
}

func (sessionlessCaller) IsPersonNotFound(error) bool { return false }
