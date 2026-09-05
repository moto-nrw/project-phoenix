package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type TimetableTestRepositories struct {
	ActivityGroup             activitiesModels.GroupRepository
	ActivityCategory          activitiesModels.CategoryRepository
	ActivitySchedule          activitiesModels.ScheduleRepository
	ActivitySupervisor        activitiesModels.SupervisorPlannedRepository
	StudentEnrollment         activitiesModels.StudentEnrollmentRepository
	StaffShift                scheduleModels.StaffShiftRepository
	StaffShiftSeries          scheduleModels.StaffShiftSeriesRepository
	StaffShiftSeriesException scheduleModels.StaffShiftSeriesExceptionRepository
	ShiftType                 scheduleModels.ShiftTypeRepository
	PlanningTrack             scheduleModels.PlanningTrackRepository
	TimetableConflictAck      scheduleModels.TimetableConflictAckRepository
	ActivityInstance          scheduleModels.ActivityInstanceRepository
	InstanceIdempotency       scheduleModels.InstanceIdempotencyRepository
	InstanceStaff             scheduleModels.InstanceStaffRepository
	InstanceStudent           scheduleModels.InstanceStudentRepository
	ActivityException         scheduleModels.ActivityExceptionRepository
	Timeframe                 scheduleModels.TimeframeRepository
	RecurrenceRule            scheduleModels.RecurrenceRuleRepository
	CalendarPeriod            scheduleModels.CalendarPeriodRepository
	ClosingDay                scheduleModels.ClosingDayRepository
	Dateframe                 scheduleModels.DateframeRepository
	Staff                     usersModels.StaffRepository
	Teacher                   usersModels.TeacherRepository
	ClassTeacher              educationModels.ClassTeacherRepository
	GroupTeacher              educationModels.GroupTeacherRepository
	Person                    usersModels.PersonRepository
	Student                   usersModels.StudentRepository
	Group                     educationModels.GroupRepository
	ActiveGroup               activeModels.GroupRepository
	ActiveVisit               activeModels.VisitRepository
	GroupSupervisor           activeModels.GroupSupervisorRepository
	StudentArrivalSchedule    scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException   scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote        scheduleModels.StudentArrivalNoteRepository
	StudentPickupSchedule     scheduleModels.StudentPickupScheduleRepository
	StudentPickupException    scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote         scheduleModels.StudentPickupNoteRepository
	StudentStatusDay          activeModels.StudentStatusDayOverviewRepository
	CareOffering              enrollmentModels.CareOfferingRepository
	RequestChildOffering      enrollmentModels.RequestChildOfferingRepository
	Room                      facilitiesModels.RoomRepository
	DeviationEvent            auditModels.DeviationEventRepository
	ClassArrivalTime          educationModels.ClassArrivalTimeRepository
	ClassArrivalException     scheduleModels.ClassArrivalExceptionRepository
	Phase                     enrollmentModels.PhaseRepository
}

func NewTimetableTestRepositories(db *bun.DB, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	instances := scheduleRepo.NewActivityInstanceRepository(db, now)
	repos := &Factory{
		db: db, Person: members.Person, Staff: members.Staff, Teacher: members.Teacher,
		Group: members.Group, GroupTeacher: members.GroupTeacher, ClassTeacher: members.ClassTeacher,
		Student:       usersRepo.NewStudentRepository(db),
		ActivityGroup: activitiesRepo.NewGroupRepository(db), ActivityCategory: activitiesRepo.NewCategoryRepository(db),
		ActivitySchedule: activitiesRepo.NewScheduleRepository(db), ActivitySupervisor: activitiesRepo.NewSupervisorPlannedRepository(db),
		StudentEnrollment: activitiesRepo.NewStudentEnrollmentRepository(db),
		StaffShift:        scheduleRepo.NewStaffShiftRepository(db), StaffShiftSeries: scheduleRepo.NewStaffShiftSeriesRepository(db),
		StaffShiftSeriesException: scheduleRepo.NewStaffShiftSeriesExceptionRepository(db),
		ShiftType:                 scheduleRepo.NewShiftTypeRepository(db), PlanningTrack: scheduleRepo.NewPlanningTrackRepository(db),
		TimetableConflictAck: scheduleRepo.NewTimetableConflictAckRepository(db),
		ActivityInstance:     instances, InstanceIdempotency: instances,
		InstanceStaff: scheduleRepo.NewInstanceStaffRepository(db), InstanceStudent: scheduleRepo.NewInstanceStudentRepository(db),
		ActivityException: scheduleRepo.NewActivityExceptionRepository(db), Timeframe: scheduleRepo.NewTimeframeRepository(db),
		RecurrenceRule: scheduleRepo.NewRecurrenceRuleRepository(db),
		ActiveGroup:    activeRepo.NewGroupRepository(db), ActiveVisit: activeRepo.NewVisitRepository(db),
		GroupSupervisor:       activeRepo.NewGroupSupervisorRepository(db, now),
		RequestChildOffering:  enrollmentRepo.NewRequestChildOfferingRepository(db),
		Room:                  facilitiesAdapter.New(),
		DeviationEvent:        auditRepo.NewDeviationEventRepository(newTestAuditRuntime(db)),
		ClassArrivalTime:      educationRepo.NewClassArrivalTimeRepository(db),
		ClassArrivalException: scheduleRepo.NewClassArrivalExceptionRepository(db),
		Phase:                 enrollmentRepo.NewPhaseRepository(db),
	}
	repos.bindActivityTemplateShiftTypes()
	repos.bindDefaultFacilities(db)
	repos.bindSchoolCalendarAdapters(calendar)
	repos.bindStudentDirectories(persons, persons)
	carePlan, err := NewCarePlan(db, persons, repos.InstanceStudent)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	repos.students = persons
	repos.bindCarePlanAdapters(carePlan)
	repos.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }})
	repos.BindPeopleDirectory(persons)
	repos.BindSchoolStructure(groups)
	return timetableTestRepositories(repos), nil
}

func timetableTestRepositories(r *Factory) TimetableTestRepositories {
	return TimetableTestRepositories{
		ActivityGroup: r.ActivityGroup, ActivityCategory: r.ActivityCategory, ActivitySchedule: r.ActivitySchedule,
		ActivitySupervisor: r.ActivitySupervisor, StudentEnrollment: r.StudentEnrollment,
		StaffShift: r.StaffShift, StaffShiftSeries: r.StaffShiftSeries, StaffShiftSeriesException: r.StaffShiftSeriesException,
		ShiftType: r.ShiftType, PlanningTrack: r.PlanningTrack, TimetableConflictAck: r.TimetableConflictAck,
		ActivityInstance: r.ActivityInstance, InstanceIdempotency: r.InstanceIdempotency,
		InstanceStaff: r.InstanceStaff, InstanceStudent: r.InstanceStudent, ActivityException: r.ActivityException,
		Timeframe: r.Timeframe, RecurrenceRule: r.RecurrenceRule, CalendarPeriod: r.CalendarPeriod,
		ClosingDay: r.ClosingDay, Dateframe: r.Dateframe,
		Staff: r.Staff, Teacher: r.Teacher, ClassTeacher: r.ClassTeacher, GroupTeacher: r.GroupTeacher,
		Person: r.Person, Student: r.Student, Group: r.Group,
		ActiveGroup: r.ActiveGroup, ActiveVisit: r.ActiveVisit, GroupSupervisor: r.GroupSupervisor,
		StudentArrivalSchedule: r.StudentArrivalSchedule, StudentArrivalException: r.StudentArrivalException,
		StudentArrivalNote: r.StudentArrivalNote, StudentPickupSchedule: r.StudentPickupSchedule,
		StudentPickupException: r.StudentPickupException, StudentPickupNote: r.StudentPickupNote,
		StudentStatusDay: r.StudentStatusDay, CareOffering: r.CareOffering,
		RequestChildOffering: r.RequestChildOffering, Room: r.Room, DeviationEvent: r.DeviationEvent,
		ClassArrivalTime: r.ClassArrivalTime, ClassArrivalException: r.ClassArrivalException, Phase: r.Phase,
	}
}
