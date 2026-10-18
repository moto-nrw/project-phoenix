//go:build testhooks

package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
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

// TimetableTestRepositories are the Timetable owner's retained rows and
// capability over the test database. The rows of the other owners the
// suites arrange through come beside them: the People Directory and School
// Structure rows from the members the graph is composed over, and the
// Student Presence, Care Plan, Facilities, Audit and School Calendar rows
// from NewTimetablePresenceRows, NewTimetableCarePlanRows,
// NewTimetableRoomRows, NewTimetableDeviationAudit and
// NewTimetableSchoolCalendar.
type TimetableTestRepositories struct {
	calendarPeriodUsage timetablePorts.CalendarPeriodUsageRepository
	Timetable           timetable.Capability
	// Repositories are the retained Timetable adapters (#2743).
	timetableCompose.Repositories
	StaffShift timetablePorts.WorkforceShiftRows
	ShiftType  timetablePorts.WorkforceShiftTypeRows
	// CalendarRows are the School Calendar rows and the class arrival
	// exceptions.
	timetableCompose.CalendarRows
	// CareScheduleRepositories are the Care Plan schedule rows.
	timetableCompose.CareScheduleRepositories
	// The owner's ports over the other owners' rows, which OwnerRows and
	// TimetableTemplateRows hand the owner's reads.
	rosterStudents      timetableCompose.RosterStudents
	educationGroups     timetablePorts.EducationGroupDirectory
	educationGroupNames timetablePorts.EducationGroupNames
	ownerRooms          timetableCompose.OwnerRooms
	deviationEvents     timetablePorts.DeviationEventReader
	deviations          timetablePorts.DeviationProtocol
}

// NewTimetableTestRepositoriesWithMembers composes the timetable suites' repository graph over
// the People Directory and School Membership repositories in members, which
// services composes (#2743). The graph has no deviation protocol; the
// Änderungsprotokoll is bound by NewTimetableTestRepositories.
func NewTimetableTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (TimetableTestRepositories, error) {
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
	rooms, err := NewTimetableRoomRows(db)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	adapters := NewTimetableRepositoriesWithPresence(db, bookings, persons, groups, rooms.Facilities, calendar, membership, workTime, NewUnloggedStudentPresence(db))
	carePlan, err := NewCarePlan(db, persons, adapters.InstanceStudent)
	if err != nil {
		return TimetableTestRepositories{}, err
	}
	usage := timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), bookings)
	structure := NewSchoolStructureSuiteRepositories(db)
	result := TimetableTestRepositories{
		Repositories: adapters,
		CalendarRows: timetableCompose.CalendarRows{
			CalendarPeriod: timetableCompose.NewCalendarPeriodRepository(calendar, usage),
			ClosingDay:     timetableCompose.NewClosingDayRepository(calendar), Dateframe: timetableCompose.NewDateframeRepository(calendar),
			ClassArrivalException: timetableCompose.NewClassArrivalExceptionRepository(db.Pool()),
		},
		CareScheduleRepositories: NewCareScheduleRepositories(carePlan),
		calendarPeriodUsage:      usage,
		rosterStudents:           NewTimetableRosterStudents(members.Student), educationGroups: structure.Group,
		educationGroupNames: TimetableEducationGroupNames(structure.Group),
		ownerRooms:          facilitiesCompose.NewRoomNameDirectory(rooms.Room.Records),
	}
	result.deviationEvents, result.deviations = TimetableDeviationPorts(nil)
	result.Timetable = bookings
	// The Dienstplan rows belong to Workforce (#2689); suites that still
	// speak the retained rows reach them through the legacy row adapters
	// over the one facade (#3418).
	result.StaffShift = newWorkforceShiftRows(workTime)
	result.ShiftType = newWorkforceShiftTypeRows(workTime)
	return result, nil
}

// withDeviationAudit binds the owner rows' Änderungsprotokoll to the Audit
// Platform's deviation events.
func (r TimetableTestRepositories) withDeviationAudit(events auditModels.DeviationEventRepository) TimetableTestRepositories {
	reader, protocol := TimetableDeviationPorts(events)
	return TimetableTestRepositories{
		calendarPeriodUsage: r.calendarPeriodUsage, Timetable: r.Timetable, Repositories: r.Repositories,
		StaffShift: r.StaffShift, ShiftType: r.ShiftType, CalendarRows: r.CalendarRows,
		CareScheduleRepositories: r.CareScheduleRepositories,
		rosterStudents:           r.rosterStudents, educationGroups: r.educationGroups,
		educationGroupNames: r.educationGroupNames,
		ownerRooms:          r.ownerRooms,
		deviationEvents:     reader, deviations: protocol,
	}
}

func (r TimetableTestRepositories) Enrollment() EnrollmentBookingProjection {
	return NewEnrollmentBookingProjection(newEnrollmentBookings(enrollmentCompose.New()))
}

// CalendarPeriodUsage returns the planning owners' per-period reference
// counts (#3124).
func (r TimetableTestRepositories) CalendarPeriodUsage() timetablePorts.CalendarPeriodUsageRepository {
	return r.calendarPeriodUsage
}

// OwnerRows are the retained rows the Timetable owner's reads run over
// (#3551), filled from the test repository set.
func (r TimetableTestRepositories) OwnerRows() timetableCompose.OwnerRows {
	return timetableCompose.OwnerRows{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, Participants: r.InstanceStudent,
		Templates: r.ActivityGroup, Categories: r.ActivityCategory, Students: r.rosterStudents,
		EducationGroups: r.educationGroupNames, Rooms: r.ownerRooms,
		PickupExceptions: r.StudentPickupException, ArrivalExceptions: r.StudentArrivalException,
		DeviationEvents: r.deviationEvents, Deviations: r.deviations,
	}
}

// TimetablePresenceRows are Student Presence's live sessions and
// supervisions over the Timetable owner's activities.
type TimetablePresenceRows struct {
	ActiveGroup     studentpresence.SessionRecords
	GroupSupervisor studentpresence.SupervisionRecords
}

// NewTimetablePresenceRows composes the live sessions and supervisions over
// the activities of timetable and the people in members.
func NewTimetablePresenceRows(db transactionruntime.Database, timetable TimetableTestRepositories, members MembershipTestRepositories, clocks ...func() time.Time) (TimetablePresenceRows, error) {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	sessions := NewSessionRecords(db, now, nil, rooms, timetable.Timetable, membership, members.People)
	return TimetablePresenceRows{ActiveGroup: sessions, GroupSupervisor: NewGroupSupervisionRecords(sessions, groups)}, nil
}

// TimetableCarePlanRows are the Care Plan owner and its status-day rows over
// the Timetable owner's participants.
type TimetableCarePlanRows struct {
	StudentStatusDay *StudentStatusDayRepository
	CarePlan         careplan.Capability
}

// NewTimetableCarePlanRows composes the Care Plan owner over the participants
// of timetable and the people in members.
func NewTimetableCarePlanRows(db transactionruntime.Database, timetable TimetableTestRepositories, members MembershipTestRepositories) (TimetableCarePlanRows, error) {
	carePlan, err := NewCarePlan(db, members.People, timetable.InstanceStudent)
	if err != nil {
		return TimetableCarePlanRows{}, err
	}
	return TimetableCarePlanRows{StudentStatusDay: NewStudentStatusDayRepository(carePlan), CarePlan: carePlan}, nil
}

// TimetableRoomRows are the Facilities rooms the timetable suites arrange
// through.
type TimetableRoomRows struct {
	Room *facilitiesAdapter.Repository
	// Facilities is the owner capability the Room adapter delegates to.
	Facilities facilitiesModule.Capability
}

// NewTimetableRoomRows composes the Facilities owner unobserved with the
// retained room adapter over it.
func NewTimetableRoomRows(db transactionruntime.Database) (TimetableRoomRows, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return TimetableRoomRows{}, err
	}
	room := facilitiesAdapter.New(rooms)
	return TimetableRoomRows{Room: room, Facilities: rooms}, nil
}

// TimetableDeviationAudit is the Audit Platform's Änderungsprotokoll the
// timetable suites read back.
type TimetableDeviationAudit struct {
	DeviationEvent auditModels.DeviationEventRepository
}

// NewTimetableDeviationAudit composes the deviation events over the test
// database, the composition that owns Audit persistence (ADR 0049).
func NewTimetableDeviationAudit(db transactionruntime.Database) TimetableDeviationAudit {
	return TimetableDeviationAudit{DeviationEvent: NewTestAuditPersistence(db, nil).DeviationEvent}
}

// DeviationEventsBetween lists the Änderungsprotokoll entries with an
// occurrence in [from, to], for suites that read them back without naming the
// Audit Platform's date.
func (r TimetableDeviationAudit) DeviationEventsBetween(ctx context.Context, from, to timezone.Date) ([]*auditModels.DeviationEvent, error) {
	return r.DeviationEvent.ListByRange(ctx, auditModels.Date(from), auditModels.Date(to), nil, nil)
}

// TimetableSchoolCalendar is the School Calendar owner the timetable's
// period, closing-day and dateframe adapters delegate to.
type TimetableSchoolCalendar struct {
	schoolCalendar schoolcalendar.Calendar
}

// NewTimetableSchoolCalendar composes the School Calendar owner over db.
func NewTimetableSchoolCalendar(db transactionruntime.Database) (TimetableSchoolCalendar, error) {
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return TimetableSchoolCalendar{}, err
	}
	return TimetableSchoolCalendar{schoolCalendar: calendar}, nil
}

// MustNewTimetablePresenceRows are the live sessions and supervisions over
// the unobserved Timetable rows and People Directory rows of db.
func MustNewTimetablePresenceRows(db transactionruntime.Database, clocks ...func() time.Time) TimetablePresenceRows {
	members := MustNewMembershipTestRepositories(db)
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose timetable: %v", err))
	}
	rows, err := NewTimetablePresenceRows(db, tt, members, clocks...)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose presence: %v", err))
	}
	return rows
}

// MustNewTimetableRoomRows is NewTimetableRoomRows for suites that cannot
// handle a composition error.
func MustNewTimetableRoomRows(db transactionruntime.Database) TimetableRoomRows {
	rows, err := NewTimetableRoomRows(db)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose rooms: %v", err))
	}
	return rows
}

// MustNewTimetableSchoolCalendar is NewTimetableSchoolCalendar for suites
// that cannot handle a composition error.
func MustNewTimetableSchoolCalendar(db transactionruntime.Database) TimetableSchoolCalendar {
	calendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose school calendar: %v", err))
	}
	return calendar
}

// SchoolCalendar returns the calendar owner.
func (c TimetableSchoolCalendar) SchoolCalendar() schoolcalendar.Calendar { return c.schoolCalendar }

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
		CalendarPeriods: r.CalendarPeriod, Exceptions: r.ActivityException, EducationGroups: r.educationGroups,
	}
}
