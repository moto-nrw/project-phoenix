//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableOwnerRows are the retained rows the Timetable owner's reads run
// over (#3551), named for the suites of other owners that bind them.
type TimetableOwnerRows = timetableCompose.NativeMirrorRows

type TimetableTestRepositories struct {
	enrollment          *enrollmentCapability.Module
	schoolCalendar      schoolcalendar.Calendar
	calendarPeriodUsage timetablePorts.CalendarPeriodUsageRepository
	Timetable           timetable.Capability
	// Repositories are the retained Timetable adapters (#2743).
	timetableCompose.Repositories
	StaffShift timetablePorts.WorkforceShiftRows
	ShiftType  timetablePorts.WorkforceShiftTypeRows
	// CalendarRows are the School Calendar rows and the class arrival
	// exceptions.
	timetableCompose.CalendarRows
	Staff           peoplePorts.StaffRepository
	Teacher         peoplePorts.TeacherRepository
	ClassTeacher    ClassTeacherRepository
	GroupTeacher    GroupTeacherRepository
	Person          peoplePorts.PersonRepository
	Student         peoplePorts.StudentRepository
	Group           EducationGroupRepository
	ActiveGroup     studentpresence.SessionRecords
	GroupSupervisor studentpresence.SupervisionRecords
	// CareScheduleRepositories are the Care Plan schedule rows.
	timetableCompose.CareScheduleRepositories
	StudentStatusDay *StudentStatusDayRepository
	// CarePlan is the owner capability the schedule adapters above delegate to.
	CarePlan careplan.Capability
	Room     *facilitiesAdapter.Repository
	// Facilities is the owner capability the Room adapter delegates to.
	Facilities facilitiesModule.Capability
	// DeviationEvent is filled by NewTimetableTestRepositories, the
	// composition that owns Audit persistence (ADR 0049).
	DeviationEvent auditModels.DeviationEventRepository
}

// NewTimetableTestRepositoriesWithMembers composes the timetable suites' repository
// graph over the People Directory and School Membership repositories in
// members, which services composes (#2743).
func NewTimetableTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (TimetableTestRepositories, error) {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	persons := members.People
	timetableRooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	bookings, err := NewUnobservedTimetable(db, persons, timetableRooms)
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
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	adapters := NewTimetableRepositoriesWithPresence(db, TimetableDependencies{
		Capability: bookings, Students: persons, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime,
	}, NewUnloggedStudentPresence(db))
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
		Repositories: adapters,
		CalendarRows: timetableCompose.CalendarRows{
			CalendarPeriod: timetableCompose.NewCalendarPeriodRepository(calendar, usage),
			ClosingDay:     timetableCompose.NewClosingDayRepository(calendar), Dateframe: timetableCompose.NewDateframeRepository(calendar),
			ClassArrivalException: timetableCompose.NewClassArrivalExceptionRepository(db.Pool()),
		},
		Staff: members.Staff, Teacher: members.Teacher, ClassTeacher: members.ClassTeacher, GroupTeacher: members.GroupTeacher,
		Person: members.Person, Student: members.Student, Group: members.Group,
		ActiveGroup: sessions, GroupSupervisor: NewGroupSupervisionRecords(sessions, groups),
		CareScheduleRepositories: NewCareScheduleRepositories(carePlan),
		StudentStatusDay:         NewStudentStatusDayRepository(carePlan), CarePlan: carePlan,
		enrollment: enrollment, schoolCalendar: calendar, calendarPeriodUsage: usage,
	}
	result.Room = room
	result.Facilities = rooms
	result.Timetable = bookings
	// The Dienstplan rows belong to Workforce (#2689); suites that still
	// speak the retained rows reach them through the legacy row adapters
	// over the one facade (#3418).
	result.StaffShift = newWorkforceShiftRows(workTime)
	result.ShiftType = newWorkforceShiftTypeRows(workTime)
	return result, nil
}

func (r TimetableTestRepositories) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(newEnrollmentBookings(r.enrollment))
}

// DeviationEventsBetween lists the Änderungsprotokoll entries with an
// occurrence in [from, to], for suites that read them back without naming the
// Audit Platform's date.
func (r TimetableTestRepositories) DeviationEventsBetween(ctx context.Context, from, to timezone.Date) ([]*auditModels.DeviationEvent, error) {
	return r.DeviationEvent.ListByRange(ctx, auditModels.Date(from), auditModels.Date(to), nil, nil)
}

// SchoolCalendar returns the calendar owner the period, closing-day and
// dateframe adapters above delegate to.
func (r TimetableTestRepositories) SchoolCalendar() schoolcalendar.Calendar { return r.schoolCalendar }

// CalendarPeriodUsage returns the planning owners' per-period reference
// counts (#3124).
func (r TimetableTestRepositories) CalendarPeriodUsage() timetablePorts.CalendarPeriodUsageRepository {
	return r.calendarPeriodUsage
}

// OwnerRows are the retained rows the Timetable owner's reads run over
// (#3551), filled from the test repository set.
func (r TimetableTestRepositories) OwnerRows() timetableCompose.OwnerRows {
	rows := timetableCompose.OwnerRows{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, Participants: r.InstanceStudent,
		Templates: r.ActivityGroup, Categories: r.ActivityCategory, Students: NewTimetableRosterStudents(r.Student),
		EducationGroups: TimetableEducationGroupNames(r.Group), Rooms: facilitiesCompose.NewRoomNameDirectory(r.Room),
		PickupExceptions: r.StudentPickupException, ArrivalExceptions: r.StudentArrivalException,
	}
	rows.DeviationEvents, rows.Deviations = TimetableDeviationPorts(r.DeviationEvent)
	return rows
}

// RosterMaintenance is the Timetable owner's roster maintenance over the test
// repository set (#3424 slice S2), for suites that compose a grade
// transition or the Enrollment resync.
func (r TimetableTestRepositories) RosterMaintenance(logger *slog.Logger, now func() time.Time) timetable.RosterMaintenance {
	return timetableCompose.NewRosterReconciler(r.ActivityInstance, r.InstanceStudent, r.StudentEnrollment, logger, now)
}

// newTimetableTestRecurrenceLock composes the Timetable owner's tenant
// recurrence gate over the test database and School Structure's
// grade-transition key, as the composition roots do.
func newTimetableTestRecurrenceLock(db transactionruntime.Database) (timetable.RecurrenceWriteLock, error) {
	return timetableCompose.NewRecurrenceWriteLock(timetableCompose.RecurrenceLockDependencies{
		DB:                     db.Pool(),
		GradeTransitionLockKey: schoolstructure.TransitionsLockKey,
	})
}

// MustNewTimetableRecurrenceLock is newTimetableTestRecurrenceLock for test
// fixtures; it panics on a nil database.
func MustNewTimetableRecurrenceLock(db transactionruntime.Database) timetable.RecurrenceWriteLock {
	lock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		panic(err)
	}
	return lock
}

// TimetableTemplateRows fills the template rows from the test repository set.
func (r TimetableTestRepositories) TimetableTemplateRows() timetableCompose.TemplateRows {
	return timetableCompose.TemplateRows{
		Groups: r.ActivityGroup, Categories: r.ActivityCategory, Schedules: r.ActivitySchedule,
		Enrollments: r.StudentEnrollment, Supervisors: r.ActivitySupervisor, Instances: r.ActivityInstance,
		InstanceStaff: r.InstanceStaff, Participants: r.InstanceStudent, Timeframes: r.Timeframe,
		CalendarPeriods: r.CalendarPeriod, Exceptions: r.ActivityException, EducationGroups: r.Group,
	}
}
