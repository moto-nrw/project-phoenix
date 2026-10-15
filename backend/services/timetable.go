package services

import (
	"context"
	"fmt"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type TimetableDependencies struct {
	Capability timetable.Capability
	Students   peopledirectory.Capability
	Groups     schoolstructure.Query
	Rooms      facilities.Capability
	Calendar   schoolcalendar.Capability
	Membership schoolmembership.Capability
	Workforce  workforce.Capability
	// ObserveIdentityAccess records the Identity & Access operations behind
	// the retained operator repositories and account lookups (#2720). The
	// serving root passes its metrics sink; nil composes the module
	// unobserved, which repository tests and CLI roots use.
	ObserveIdentityAccess IdentityAccessObserver
}

// NewTimetable composes the owner behind legacy repository adapters for test
// and CLI graphs. The production root replaces it with the observed module.
// The session facts come from Student Presence over the same database
// (#2762).
func NewTimetable(db *bun.DB, students peopledirectory.StudentQuery, rooms facilities.Query) (timetable.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB:       db,
		Students: repositoryTimetableStudents{students: students},
		Rooms:    repositoryTimetableRooms{rooms: rooms},
		Sessions: newPresenceFacts(db),
		Observe:  func(timetableCompose.Observation) {},
	})
}

type repositoryTimetableStudents struct{ students peopledirectory.StudentQuery }

func (d repositoryTimetableStudents) ListEnrolledStudents(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
	values, err := d.students.ListEnrolledStudents(ctx)
	result := make([]timetableCompose.TargetStudent, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.TargetStudent{
			ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
			EnrolledUntil: value.EnrolledUntil,
		})
	}
	return result, err
}

type repositoryTimetableRooms struct{ rooms facilities.Query }

func (d repositoryTimetableRooms) LockRoomsByID(ctx context.Context, ids []int64) ([]timetable.RoomRef, error) {
	values, err := d.rooms.LockRoomsByID(ctx, ids)
	result := make([]timetable.RoomRef, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.RoomRef{ID: value.ID, TenantID: value.TenantID})
	}
	return result, err
}

// TimetableRepositories are the Timetable owner's retained activities and
// schedule repositories, named for the suites that may not import the
// owner's compose package.
type TimetableRepositories = timetableCompose.Repositories

// NewTimetableRepositories builds the Timetable owner's retained activities
// and schedule repositories (#2743) over deps. The owner names of School
// Structure, Facilities, Workforce, School Membership and People Directory
// reach the adapters through the directories bound here; the instance and
// participant adapters compose an unobserved Student Presence owner.
func NewTimetableRepositories(db *bun.DB, deps TimetableDependencies) TimetableRepositories {
	return NewTimetableRepositoriesWithPresence(db, deps, newStudentPresence(db, presenceCompose.FailureLogger()))
}

func NewTimetableRepositoriesWithPresence(db *bun.DB, deps TimetableDependencies, presence *studentpresence.Module) timetableCompose.Repositories {
	return timetableCompose.NewRepositories(timetableCompose.RepositoryDependencies{
		DB: db, Timetable: deps.Capability, Presence: presence, PartialAbsences: newPartialAbsencePreview(db),
		Calendar: deps.Calendar, Directories: timetableDirectories(deps),
	})
}

// timetableDirectories binds the Timetable adapters' directory ports to the
// owners' public capabilities. An absent capability leaves its port unset,
// so the adapter reports it instead of reading a foreign table.
func timetableDirectories(deps TimetableDependencies) timetableCompose.RepositoryDirectories {
	var directories timetableCompose.RepositoryDirectories
	if deps.Groups != nil {
		directories.GroupNames = func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return schoolStructureCompose.GroupNamesByID(ctx, deps.Groups, ids)
		}
	}
	if deps.Rooms != nil {
		directories.RoomNames = timetableRoomDirectory(deps.Rooms)
	}
	if deps.Workforce != nil {
		directories.ShiftTypes = timetableShiftTypes(deps.Workforce)
	}
	if deps.Membership != nil {
		directories.StaffPersons = timetableStaffPersons(deps.Membership)
	}
	if deps.Students != nil {
		directories.PersonNames = timetablePersonNames(deps.Students)
		directories.StudentAlumni, directories.StudentNames = timetableStudentFacts(deps.Students)
	}
	return directories
}

func timetableRoomDirectory(rooms facilities.Query) func(context.Context, []int64) (map[int64]string, error) {
	return func(ctx context.Context, ids []int64) (map[int64]string, error) {
		values, err := rooms.ListRoomsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		names := make(map[int64]string, len(values))
		for _, room := range values {
			names[room.ID] = room.Name
		}
		return names, nil
	}
}

func timetableShiftTypes(shifts workforce.ShiftQuery) func(context.Context) ([]timetableCompose.ShiftType, error) {
	return func(ctx context.Context) ([]timetableCompose.ShiftType, error) {
		values, err := shifts.ListShiftTypes(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.ShiftType, 0, len(values))
		for _, shift := range values {
			result = append(result, timetableCompose.ShiftType{ID: shift.ID, Name: shift.Name, Color: shift.Color})
		}
		return result, nil
	}
}

func timetableStaffPersons(membership schoolMembershipCompose.StaffLookup) func(context.Context, []int64, bool) (map[int64]int64, error) {
	return func(ctx context.Context, ids []int64, includeDeleted bool) (map[int64]int64, error) {
		members, err := schoolMembershipCompose.StaffByID(ctx, membership, ids, includeDeleted)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]int64, len(members))
		for id, member := range members {
			result[id] = member.PersonID
		}
		return result, nil
	}
}

func timetablePersonNames(persons peopledirectory.Query) func(context.Context, []int64) (map[int64]timetableCompose.PersonName, error) {
	return func(ctx context.Context, ids []int64) (map[int64]timetableCompose.PersonName, error) {
		values, err := peopleCompose.PersonsByID(ctx, persons, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]timetableCompose.PersonName, len(values))
		for id, person := range values {
			result[id] = timetableCompose.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
		return result, nil
	}
}

func timetableStudentFacts(students peopledirectory.StudentQuery) (
	func(context.Context, []int64) (map[int64]bool, error),
	func(context.Context, []int64) (map[int64]timetableCompose.PersonName, error),
) {
	alumni := func(ctx context.Context, ids []int64) (map[int64]bool, error) {
		values, err := students.ListStudentsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]bool, len(values))
		for _, student := range values {
			result[student.ID] = student.IsAlumnus()
		}
		return result, nil
	}
	names := func(ctx context.Context, ids []int64) (map[int64]timetableCompose.PersonName, error) {
		values, err := students.ListStudentNamesByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]timetableCompose.PersonName, len(values))
		for _, name := range values {
			result[name.StudentID] = timetableCompose.PersonName{FirstName: name.FirstName, LastName: name.LastName}
		}
		return result, nil
	}
	return alumni, names
}

// The Student Presence roots the retained Timetable builders, the session
// cleanup and the retention cleanup compose over one database. The typed root
// owns them (api/student_presence_roots.go, #4010); these private copies stay
// with their callers here, in command_roots.go and in cleanup_composition.go
// until #4009 and #4013 move those callers.

// newStudentPresence composes Student Presence with its attendance rules bound
// to an unobserved Timetable owner over the same database.
func newStudentPresence(db *bun.DB, observe func(presenceCompose.Observation)) *studentpresence.Module {
	students := peopleCompose.MustNewReads(db)
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		panic(err)
	}
	capability, err := NewTimetable(db, students, rooms)
	if err != nil {
		panic(err)
	}
	queries, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(err)
	}
	locks, err := carePlanCompose.NewDayLocks(db, students.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		panic(err)
	}
	module, err := presenceCompose.New(presenceCompose.Dependencies{
		DB: db, Observe: observe, Roster: timetablePlannedRoster{timetable: capability},
		CarePlan: presenceCarePlanDirectory{query: queries}, CareDays: locks,
	})
	if err != nil {
		panic(err)
	}
	return module
}

// newPresenceFacts composes the Student Presence queries Timetable's session
// facts read; they need no roster.
func newPresenceFacts(db *bun.DB) timetable.SessionFacts {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: presenceCompose.FailureLogger()})
	if err != nil {
		panic(err)
	}
	return timetableCompose.NewPresenceSessionFacts(module)
}

// newPartialAbsencePreview composes the Timetable partial-absence preview over
// the People Directory, the Care Plan pickup exceptions and the joined plan
// reads of the database.
func newPartialAbsencePreview(db *bun.DB) timetableCompose.PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	reads, err := timetableCompose.NewPresenceReads(db)
	if err != nil {
		panic(err)
	}
	students := peopleCompose.MustNewReads(db)
	enrolled := func(ctx context.Context, studentID int64) (bool, error) {
		values, err := students.ListStudentsByID(ctx, []int64{studentID})
		if err != nil {
			return false, err
		}
		for _, student := range values {
			if student.ID == studentID && !student.IsAlumnus() {
				return true, nil
			}
		}
		return false, nil
	}
	return timetableCompose.NewPartialAbsencePreview(reads, enrolled, pickupExceptionDirectory{query: exceptions})
}

// timetablePlannedRoster serves Student Presence's PlannedRoster port from the
// Timetable owner.
type timetablePlannedRoster struct {
	timetable timetable.InstanceStudentQuery
}

func (r timetablePlannedRoster) ListPlannedParticipants(ctx context.Context, filter studentpresence.PlannedRosterFilter) ([]studentpresence.PlannedParticipant, error) {
	query := timetable.InstanceStudentFilter{IDs: filter.IDs, InstanceIDs: filter.InstanceIDs, StudentIDs: filter.StudentIDs, ExcludeCancelled: filter.ExcludeCancelled}
	if filter.Date != "" {
		date := filter.Date
		query.Date = &date
	}
	if filter.FromClock != "" {
		clock := filter.FromClock
		query.FromClock = &clock
	}
	rows, err := r.timetable.ListPlannedInstanceStudents(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.PlannedParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.PlannedParticipant(row))
	}
	return result, nil
}

// presenceCarePlanDirectory serves Student Presence's care-plan port from the
// Care Plan exception queries.
type presenceCarePlanDirectory struct {
	query carePlanCompose.ExceptionQueries
}

func (d presenceCarePlanDirectory) FindPickupException(ctx context.Context, id int64) (*studentpresence.SessionPickupException, error) {
	value, err := pickupExceptionDirectory(d).FindPickupException(ctx, id)
	if value == nil || err != nil {
		return nil, err
	}
	result := studentpresence.SessionPickupException(*value)
	return &result, nil
}

func (d presenceCarePlanDirectory) ListPickupExceptions(ctx context.Context, filter studentpresence.SessionPickupExceptionFilter) ([]studentpresence.SessionPickupException, error) {
	values, err := pickupExceptionDirectory(d).ListPickupExceptions(ctx, timetableCompose.PickupExceptionFilter(filter))
	result := make([]studentpresence.SessionPickupException, 0, len(values))
	for _, value := range values {
		result = append(result, studentpresence.SessionPickupException(value))
	}
	return result, err
}

func (d presenceCarePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*studentpresence.SessionStudentStatusDay, error) {
	value, err := pickupExceptionDirectory(d).FindStudentStatusDay(ctx, id, activeOnly)
	if value == nil || err != nil {
		return nil, err
	}
	result := studentpresence.SessionStudentStatusDay(*value)
	return &result, nil
}

func (d presenceCarePlanDirectory) ListStudentStatusDays(ctx context.Context, filter studentpresence.SessionStudentStatusDayFilter) ([]studentpresence.SessionStudentStatusDay, error) {
	values, err := pickupExceptionDirectory(d).ListStudentStatusDays(ctx, timetableCompose.StudentStatusDayFilter(filter))
	result := make([]studentpresence.SessionStudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, studentpresence.SessionStudentStatusDay(value))
	}
	return result, err
}
