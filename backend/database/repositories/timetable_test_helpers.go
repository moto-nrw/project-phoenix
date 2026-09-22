package repositories

import (
	"time"

	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

type TimetableTestRepositories struct {
	enrollment                *enrollmentCapability.Module
	schoolCalendar            schoolcalendar.Calendar
	calendarPeriodUsage       *timetableCompose.CalendarPeriodUsageRepository
	Timetable                 timetable.Capability
	ActivityGroup             activitiesModels.GroupRepository
	ActivityCategory          activitiesModels.CategoryRepository
	ActivitySchedule          activitiesModels.ScheduleRepository
	ActivitySupervisor        activitiesModels.SupervisorPlannedRepository
	StudentEnrollment         activitiesModels.StudentEnrollmentRepository
	StaffShift                *workforceCompose.ShiftRows
	StaffShiftSeries          *workforceCompose.ShiftSeriesRows
	StaffShiftSeriesException *workforceCompose.ShiftSeriesExceptionRows
	ShiftType                 *workforceCompose.ShiftTypeRows
	PlanningTrack             scheduleModels.PlanningTrackRepository
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
	ActiveGroup               studentpresence.SessionRecords
	GroupSupervisor           studentpresence.SupervisionRecords
	StudentArrivalSchedule    scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException   scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote        scheduleModels.StudentArrivalNoteRepository
	StudentPickupSchedule     scheduleModels.StudentPickupScheduleRepository
	StudentPickupException    scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote         scheduleModels.StudentPickupNoteRepository
	StudentStatusDay          *StudentStatusDayRepository
	// CarePlan is the owner capability the schedule adapters above delegate to.
	CarePlan              careplan.Capability
	Room                  facilitiesModels.RoomRepository
	DeviationEvent        auditModels.DeviationEventRepository
	ClassArrivalTime      educationModels.ClassArrivalTimeRepository
	ClassArrivalException scheduleModels.ClassArrivalExceptionRepository
}

func NewTimetableTestRepositories(db *bun.DB, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	bookings := NewUnobservedTimetableDependencies(db).Capability
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
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	sessions := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Now: now, Rooms: &activeRoomDirectory{},
		Activities: NewSessionActivities(timetableActivityGroupRepository{timetable: bookings}),
		Staff:      &presenceSupervisionStaff{},
	})
	repos := &Factory{
		db: db, Person: members.Person, Staff: members.Staff, Teacher: members.Teacher,
		Group: members.Group, GroupTeacher: members.GroupTeacher, ClassTeacher: members.ClassTeacher,
		Student:               NewStudentRepository(db),
		InstanceStudent:       timetableInstanceStudentRepository{timetable: bookings},
		ActiveGroup:           sessions,
		GroupSupervisor:       sessions,
		Room:                  facilitiesAdapter.New(),
		DeviationEvent:        auditRepo.NewDeviationEventRepository(newTestAuditRuntime(db)),
		ClassArrivalTime:      educationRepo.NewClassArrivalTimeRepository(db),
		ClassArrivalException: timetableCompose.NewClassArrivalExceptionRepository(db),
		SubmissionRateLimit:   enrollmentCompose.New(),
	}
	repos.bindDefaultFacilities(db)
	repos.bindSchoolCalendarAdapters(calendar, NewCalendarPeriodUsage(enrollmentCompose.New(), bookings))
	repos.bindStudentDirectories(persons, persons)
	carePlan, err := NewCarePlan(db, persons, repos.InstanceStudent)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	repos.students = persons
	repos.bindCarePlanAdapters(carePlan)
	repos.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}, workTime)
	repos.BindPeopleDirectory(persons)
	repos.BindSchoolStructure(groups)
	rooms, err := NewFacilities(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	adapters := newTimetableRepositories(bookings, persons, groups, rooms, calendar, membership, workTime)
	repos.ActivityCategory, repos.ActivityGroup = adapters.ActivityCategory, adapters.ActivityGroup
	repos.ActivitySchedule, repos.ActivitySupervisor = adapters.ActivitySchedule, adapters.ActivitySupervisor
	repos.StudentEnrollment, repos.Timeframe = adapters.StudentEnrollment, adapters.Timeframe
	repos.PlanningTrack, repos.RecurrenceRule = adapters.PlanningTrack, adapters.RecurrenceRule
	repos.ActivityException, repos.ActivityInstance = adapters.ActivityException, adapters.ActivityInstance
	repos.InstanceIdempotency, repos.InstanceStaff = adapters.InstanceIdempotency, adapters.InstanceStaff
	result := timetableTestRepositories(repos)
	result.Timetable = bookings
	// The Dienstplan rows belong to Workforce (#2689); suites that still
	// speak the retained rows reach them through the compose adapters
	// over the one facade (#3418).
	result.StaffShift = workforceCompose.NewShiftRows(workTime)
	result.StaffShiftSeries = workforceCompose.NewShiftSeriesRows(workTime)
	result.StaffShiftSeriesException = workforceCompose.NewShiftSeriesExceptionRows(workTime)
	result.ShiftType = workforceCompose.NewShiftTypeRows(workTime)
	return result, nil
}

func timetableTestRepositories(r *Factory) TimetableTestRepositories {
	return TimetableTestRepositories{
		ActivityGroup: r.ActivityGroup, ActivityCategory: r.ActivityCategory, ActivitySchedule: r.ActivitySchedule,
		ActivitySupervisor: r.ActivitySupervisor, StudentEnrollment: r.StudentEnrollment,
		PlanningTrack:    r.PlanningTrack,
		ActivityInstance: r.ActivityInstance, InstanceIdempotency: r.InstanceIdempotency,
		InstanceStaff: r.InstanceStaff, InstanceStudent: r.InstanceStudent, ActivityException: r.ActivityException,
		Timeframe: r.Timeframe, RecurrenceRule: r.RecurrenceRule, CalendarPeriod: r.CalendarPeriod,
		ClosingDay: r.ClosingDay, Dateframe: r.Dateframe,
		Staff: r.Staff, Teacher: r.Teacher, ClassTeacher: r.ClassTeacher, GroupTeacher: r.GroupTeacher,
		Person: r.Person, Student: r.Student, Group: r.Group,
		ActiveGroup: r.ActiveGroup, GroupSupervisor: r.GroupSupervisor,
		StudentArrivalSchedule: r.StudentArrivalSchedule, StudentArrivalException: r.StudentArrivalException,
		StudentArrivalNote: r.StudentArrivalNote, StudentPickupSchedule: r.StudentPickupSchedule,
		StudentPickupException: r.StudentPickupException, StudentPickupNote: r.StudentPickupNote,
		StudentStatusDay: r.StudentStatusDay, CarePlan: r.carePlan,
		Room: r.Room, DeviationEvent: r.DeviationEvent,
		ClassArrivalTime: r.ClassArrivalTime, ClassArrivalException: r.ClassArrivalException,
		enrollment: r.SubmissionRateLimit, schoolCalendar: r.SchoolCalendar(), calendarPeriodUsage: r.CalendarPeriodUsage(),
	}
}

func (r TimetableTestRepositories) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(r.enrollment)
}

// SchoolCalendar returns the calendar owner the period, closing-day and
// dateframe adapters above delegate to.
func (r TimetableTestRepositories) SchoolCalendar() schoolcalendar.Calendar { return r.schoolCalendar }

// CalendarPeriodUsage returns the planning owners' per-period reference
// counts (#3124).
func (r TimetableTestRepositories) CalendarPeriodUsage() *timetableCompose.CalendarPeriodUsageRepository {
	return r.calendarPeriodUsage
}
