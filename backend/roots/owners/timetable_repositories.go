package owners

import (
	"context"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// TimetableRepositories are the Timetable owner's retained activities and
// schedule repositories, named for the suites that may not import the
// owner's compose package.
type TimetableRepositories = timetableCompose.Repositories

// NewTimetableRepositories builds the Timetable owner's retained activities
// and schedule repositories (#2743) over the Timetable capability. The owner
// names of People Directory, School Structure, Facilities, School Membership
// and Workforce reach the adapters through the directories bound here; the
// instance and participant adapters compose an unobserved Student Presence
// owner.
func NewTimetableRepositories(
	db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability, groups schoolstructure.Query,
	rooms facilities.Query, calendar schoolcalendar.Capability, membership schoolmembership.Capability, workTime workforce.Capability,
) TimetableRepositories {
	return NewTimetableRepositoriesWithPresence(db, capability, students, groups, rooms, calendar, membership, workTime, NewUnloggedStudentPresence(db))
}

// NewTimetableRepositoriesWithPresence is NewTimetableRepositories with the
// instance and participant adapters over the given Student Presence owner.
func NewTimetableRepositoriesWithPresence(
	db transactionruntime.Database, capability timetable.Capability, students peopledirectory.Capability, groups schoolstructure.Query,
	rooms facilities.Query, calendar schoolcalendar.Capability, membership schoolmembership.Capability, workTime workforce.Capability,
	presence *studentpresence.Module,
) timetableCompose.Repositories {
	return timetableCompose.NewRepositories(timetableCompose.RepositoryDependencies{
		DB: db.Pool(), Timetable: capability, Presence: presence, PartialAbsences: NewPartialAbsencePreview(db),
		Calendar: calendar, Directories: timetableDirectories(students, groups, rooms, membership, workTime),
	}, MustPresenceReads(db))
}

// NewUnobservedTimetable composes the unobserved Timetable owner for test
// graphs and CLI roots. The session facts come from Student Presence over
// the same database (#2762).
func NewUnobservedTimetable(db transactionruntime.Database, students peopledirectory.StudentQuery, rooms facilities.Query) (timetable.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return newTimetable(db, students, rooms, membership, func(timetableCompose.Observation) {})
}

// timetableDirectories binds the Timetable adapters' directory ports to the
// owners' public capabilities. An absent capability leaves its port unset,
// so the adapter reports it instead of reading a foreign table.
func timetableDirectories(
	students peopledirectory.Capability, groups schoolstructure.Query, rooms facilities.Query,
	membership schoolmembership.Capability, workTime workforce.Capability,
) timetablePorts.RepositoryDirectories {
	var directories timetablePorts.RepositoryDirectories
	if groups != nil {
		directories.GroupNames = func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return schoolStructureCompose.GroupNamesByID(ctx, groups, ids)
		}
	}
	if rooms != nil {
		directories.RoomNames = timetableRoomDirectory(rooms)
	}
	if workTime != nil {
		directories.ShiftTypes = timetableShiftTypes(workTime)
	}
	if membership != nil {
		directories.StaffPersons = timetableStaffPersons(membership)
	}
	if students != nil {
		directories.PersonNames = timetablePersonNames(students)
		directories.StudentAlumni, directories.StudentNames = timetableStudentFacts(students)
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

func timetableShiftTypes(shifts workforce.ShiftQuery) func(context.Context) ([]timetablePorts.ShiftType, error) {
	return func(ctx context.Context) ([]timetablePorts.ShiftType, error) {
		values, err := shifts.ListShiftTypes(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetablePorts.ShiftType, 0, len(values))
		for _, shift := range values {
			result = append(result, timetablePorts.ShiftType{ID: shift.ID, Name: shift.Name, Color: shift.Color})
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

func timetablePersonNames(persons peopledirectory.Query) func(context.Context, []int64) (map[int64]timetablePorts.PersonName, error) {
	return func(ctx context.Context, ids []int64) (map[int64]timetablePorts.PersonName, error) {
		values, err := peopleCompose.PersonsByID(ctx, persons, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]timetablePorts.PersonName, len(values))
		for id, person := range values {
			result[id] = timetablePorts.PersonName{FirstName: person.FirstName, LastName: person.LastName}
		}
		return result, nil
	}
}

func timetableStudentFacts(students peopledirectory.StudentQuery) (
	func(context.Context, []int64) (map[int64]bool, error),
	func(context.Context, []int64) (map[int64]timetablePorts.PersonName, error),
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
	names := func(ctx context.Context, ids []int64) (map[int64]timetablePorts.PersonName, error) {
		values, err := students.ListStudentNamesByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]timetablePorts.PersonName, len(values))
		for _, name := range values {
			result[name.StudentID] = timetablePorts.PersonName{FirstName: name.FirstName, LastName: name.LastName}
		}
		return result, nil
	}
	return alumni, names
}
