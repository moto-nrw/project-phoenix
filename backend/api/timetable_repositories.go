package api

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
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
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
	ObserveIdentityAccess services.IdentityAccessObserver
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
func NewTimetableRepositories(db transactionruntime.Database, deps TimetableDependencies) TimetableRepositories {
	return NewTimetableRepositoriesWithPresence(db, deps, services.NewUnloggedStudentPresence(db.Pool()))
}

func NewTimetableRepositoriesWithPresence(db transactionruntime.Database, deps TimetableDependencies, presence *studentpresence.Module) timetableCompose.Repositories {
	return timetableCompose.NewRepositories(timetableCompose.RepositoryDependencies{
		DB: db.Pool(), Timetable: deps.Capability, Presence: presence, PartialAbsences: services.NewPartialAbsencePreview(db.Pool()),
		Calendar: deps.Calendar, Directories: timetableDirectories(deps),
	})
}

// NewUnobservedTimetable composes the unobserved Timetable owner for test
// graphs and CLI roots. The session facts come from Student Presence over
// the same database (#2762).
func NewUnobservedTimetable(db transactionruntime.Database, students peopledirectory.StudentQuery, rooms facilities.Query) (timetable.Capability, error) {
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return nil, err
	}
	return newTimetable(db, students, rooms, membership, func(timetableCompose.Observation) {})
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
