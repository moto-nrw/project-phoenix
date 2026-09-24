package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
		activityGroups: &helperTemplates{byID: map[int64]*activitiesModels.Group{}},
		settings:       &helperSettings{},
		announcer:      &helperAnnouncer{},
	}
	deps.service = &operations{deps: OperationDependencies{
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
		Date:      scheduleModels.NewDate(start.Year(), start.Month(), start.Day()),
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

func (r *helperInstances) FindByID(_ context.Context, id any) (*scheduleModels.ActivityInstance, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byID[id.(int64)], nil
}

func (r *helperInstances) FindByTenantAndDate(context.Context, scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

func (r *helperInstances) FindByActiveGroupID(context.Context, int64) (*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

type helperPeople struct {
	accountPerson *usersModels.Person
	staffErr      error
}

func (p *helperPeople) FindByAccountID(context.Context, int64) (*usersModels.Person, error) {
	return p.accountPerson, nil
}

func (p *helperPeople) GetByIDs(context.Context, []int64) (map[int64]*usersModels.Person, error) {
	return nil, nil
}

func (p *helperPeople) GetStaffByPersonID(context.Context, int64) (*usersModels.Staff, error) {
	return nil, p.staffErr
}

func (p *helperPeople) GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*usersModels.Staff, error) {
	return nil, nil
}

type helperTemplates struct {
	byID map[int64]*activitiesModels.Group
	err  error
}

func (r *helperTemplates) FindByID(_ context.Context, id any) (*activitiesModels.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	group := r.byID[id.(int64)]
	if group == nil {
		return nil, helperNotFound{}
	}
	return group, nil
}

func (r *helperTemplates) FindByIDs(context.Context, []int64) ([]*activitiesModels.Group, error) {
	return nil, r.err
}

func (r *helperTemplates) FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activitiesModels.GroupTarget, error) {
	return map[int64][]*activitiesModels.GroupTarget{}, r.err
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

func (s *helperSettings) ActionScopeAllStaff(context.Context, ScopedAction) (bool, error) {
	return false, s.stringErr
}

func (s *helperSettings) StudentAbsenceEditAllStaff(context.Context) (bool, error) {
	return false, s.stringErr
}

func (s *helperSettings) OperationalOverviewAllStaff(context.Context) (bool, error) {
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

func (careRuleDays) ResolveForDate(context.Context, []int64, timezone.Date) (map[int64]timetable.CareDayStatus, error) {
	return map[int64]timetable.CareDayStatus{}, nil
}

func (careRuleDays) ResolveForRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64]map[timezone.Date]timetable.CareDayStatus, error) {
	return map[int64]map[timezone.Date]timetable.CareDayStatus{}, nil
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
