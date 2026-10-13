package repositories

import (
	"log/slog"
	"time"

	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type TimetableTestRepositories struct {
	enrollment          *enrollmentCapability.Module
	schoolCalendar      schoolcalendar.Calendar
	calendarPeriodUsage *timetableCompose.CalendarPeriodUsageRepository
	Timetable           timetable.Capability
	// Repositories are the retained Timetable adapters (#2743).
	timetableCompose.Repositories
	StaffShift              *WorkforceShiftRows
	ShiftType               *WorkforceShiftTypeRows
	CalendarPeriod          scheduleModels.CalendarPeriodRepository
	ClosingDay              scheduleModels.ClosingDayRepository
	Dateframe               scheduleModels.DateframeRepository
	Staff                   usersModels.StaffRepository
	Teacher                 usersModels.TeacherRepository
	ClassTeacher            ClassTeacherRepository
	GroupTeacher            GroupTeacherRepository
	Person                  usersModels.PersonRepository
	Student                 usersModels.StudentRepository
	Group                   EducationGroupRepository
	ActiveGroup             studentpresence.SessionRecords
	GroupSupervisor         studentpresence.SupervisionRecords
	StudentArrivalSchedule  scheduleModels.StudentArrivalScheduleRepository
	StudentArrivalException scheduleModels.StudentArrivalExceptionRepository
	StudentArrivalNote      scheduleModels.StudentArrivalNoteRepository
	StudentPickupSchedule   scheduleModels.StudentPickupScheduleRepository
	StudentPickupException  scheduleModels.StudentPickupExceptionRepository
	StudentPickupNote       scheduleModels.StudentPickupNoteRepository
	StudentStatusDay        *StudentStatusDayRepository
	// CarePlan is the owner capability the schedule adapters above delegate to.
	CarePlan careplan.Capability
	Room     facilitiesModels.RoomRepository
	// DeviationEvent is filled by services.NewTimetableTestRepositories, the
	// composition that owns Audit persistence (ADR 0049).
	DeviationEvent        auditModels.DeviationEventRepository
	ClassArrivalException scheduleModels.ClassArrivalExceptionRepository
}

// NewTimetableTestRepositories composes the timetable suites' repository
// graph over the People Directory and School Membership repositories in
// members, which services composes (#2743).
func NewTimetableTestRepositories(db *bun.DB, members MembershipTestRepositories, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	persons := members.People
	timetableRooms, err := NewFacilities(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	bookings, err := NewTimetable(db, persons, timetableRooms)
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
	rooms, err := NewFacilities(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	adapters := newTimetableRepositories(db, TimetableDependencies{
		Capability: bookings, Students: persons, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime,
	}, newStudentPresence(db))
	sessions := NewSessionRecords(db, SessionRecordOwners{
		Now: now, Rooms: rooms, Timetable: bookings, Membership: membership, Persons: persons,
	})
	carePlan, err := NewCarePlan(db, persons, adapters.InstanceStudent)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	enrollment := enrollmentCompose.New()
	usage := timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), bookings)
	room := facilitiesAdapter.New()
	room.Bind(rooms)
	result := TimetableTestRepositories{
		Repositories:   adapters,
		CalendarPeriod: timetableCompose.NewCalendarPeriodRepository(calendar, usage),
		ClosingDay:     timetableCompose.NewClosingDayRepository(calendar), Dateframe: timetableCompose.NewDateframeRepository(calendar),
		Staff: members.Staff, Teacher: members.Teacher, ClassTeacher: members.ClassTeacher, GroupTeacher: members.GroupTeacher,
		Person: members.Person, Student: members.Student, Group: members.Group,
		ActiveGroup: sessions, GroupSupervisor: NewGroupSupervisionRecords(sessions, groups),
		StudentArrivalSchedule: NewArrivalScheduleRepository(carePlan), StudentArrivalException: NewArrivalExceptionRepository(carePlan),
		StudentArrivalNote: NewArrivalNoteRepository(carePlan), StudentPickupSchedule: NewPickupScheduleRepository(carePlan),
		StudentPickupException: NewPickupExceptionRepository(carePlan), StudentPickupNote: NewPickupNoteRepository(carePlan),
		StudentStatusDay: NewStudentStatusDayRepository(carePlan), CarePlan: carePlan,
		enrollment: enrollment, schoolCalendar: calendar, calendarPeriodUsage: usage,
	}
	result.ClassArrivalException = timetableCompose.NewClassArrivalExceptionRepository(db)
	result.Room = room
	result.Timetable = bookings
	// The Dienstplan rows belong to Workforce (#2689); suites that still
	// speak the retained rows reach them through the legacy row adapters
	// over the one facade (#3418).
	result.StaffShift = NewWorkforceShiftRows(workTime)
	result.ShiftType = NewWorkforceShiftTypeRows(workTime)
	return result, nil
}

// EnrollmentPickupSchedules binds Enrollment's weekly pickup port to the
// pickup schedules.
func (r TimetableTestRepositories) EnrollmentPickupSchedules() timetableCompose.EnrollmentPickupSchedules {
	return timetableCompose.NewEnrollmentPickupSchedules(r.StudentPickupSchedule)
}

// EnrollmentArrivalSchedules binds Enrollment's weekly arrival port to the
// arrival schedules.
func (r TimetableTestRepositories) EnrollmentArrivalSchedules() timetableCompose.EnrollmentArrivalSchedules {
	return timetableCompose.NewEnrollmentArrivalSchedules(r.StudentArrivalSchedule)
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

// OwnerRows are the retained rows the Timetable owner's reads run over
// (#3551), filled from the test repository set.
func (r TimetableTestRepositories) OwnerRows() TimetableOwnerRows {
	return TimetableOwnerRows{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, Participants: r.InstanceStudent,
		Templates: r.ActivityGroup, Categories: r.ActivityCategory, Students: r.Student,
		EducationGroups: r.Group, Rooms: r.Room, PickupExceptions: r.StudentPickupException,
		ArrivalExceptions: r.StudentArrivalException, DeviationEvents: r.DeviationEvent,
	}
}

// RosterMaintenance is the Timetable owner's roster maintenance over the test
// repository set (#3424 slice S2), for suites that compose a grade
// transition or the Enrollment resync.
func (r TimetableTestRepositories) RosterMaintenance(logger *slog.Logger, now func() time.Time) timetable.RosterMaintenance {
	return NewTimetableRosterMaintenance(r.ActivityInstance, r.InstanceStudent, r.StudentEnrollment, logger, now)
}

// MustNewTimetableRecurrenceLock is NewTimetableRecurrenceLock for test
// fixtures; it panics on a nil database.
func MustNewTimetableRecurrenceLock(db *bun.DB) timetable.RecurrenceWriteLock {
	lock, err := NewTimetableRecurrenceLock(db)
	if err != nil {
		panic(err)
	}
	return lock
}

// NewTimetableRosterMaintenance is the Timetable owner's roster maintenance
// (#3424 slice S2) over the retained instance, participant and enrollment
// rows. logger may be nil; now defaults to the Berlin clock.
func NewTimetableRosterMaintenance(
	instances scheduleModels.ActivityInstanceRepository,
	participants scheduleModels.InstanceStudentRepository,
	enrollments activitiesModels.StudentEnrollmentRepository,
	logger *slog.Logger,
	now ...func() time.Time,
) timetable.RosterMaintenance {
	return timetableCompose.NewRosterReconciler(instances, participants, enrollments, logger, now...)
}

// TimetableTemplateRows fills the template rows from the test repository set.
func (r TimetableTestRepositories) TimetableTemplateRows() TimetableTemplateRows {
	return TimetableTemplateRows{
		Groups: r.ActivityGroup, Categories: r.ActivityCategory, Schedules: r.ActivitySchedule,
		Enrollments: r.StudentEnrollment, Supervisors: r.ActivitySupervisor, Instances: r.ActivityInstance,
		InstanceStaff: r.InstanceStaff, Participants: r.InstanceStudent, Timeframes: r.Timeframe,
		CalendarPeriods: r.CalendarPeriod, Exceptions: r.ActivityException, EducationGroups: r.Group,
	}
}
