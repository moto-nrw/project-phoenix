//go:build testhooks

package api

import (
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// ActivitiesTestModule is the Timetable side of the activity routes; the
// person directory and the caller rows come beside it from
// NewActivitiesTestModule (#4080).
type ActivitiesTestModule struct {
	Activities timetableCompose.ActivityService
	Timetable  timetable.Capability
}

func NewActivitiesTestModule(db transactionruntime.Database) (ActivitiesTestModule, peoplePorts.PersonDirectory, *CallerRows, error) {
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return ActivitiesTestModule{}, nil, nil, err
	}
	identity, members, err := newStaffIdentityForTests(db)
	if err != nil {
		return ActivitiesTestModule{}, nil, nil, err
	}
	presence, err := NewTimetablePresenceRows(db, r, members)
	if err != nil {
		return ActivitiesTestModule{}, nil, nil, err
	}
	activityService, err := timetableCompose.NewService(r.Timetable, r.ActivityGroup, r.ActivitySchedule,
		r.ActivitySupervisor, r.StudentEnrollment, presence.ActiveGroup, NewTimetableStaff(members.Staff), members.Student)
	if err != nil {
		return ActivitiesTestModule{}, nil, nil, err
	}
	// Activities consumes the owner's timeframe reads and staff-directory reads only.
	users := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       members.Person, TeacherRepo: members.Teacher,
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, MayManageStaff: StaffAdministrationPermitted}),
	})
	return ActivitiesTestModule{Activities: activityService, Timetable: r.Timetable}, users, identity, nil
}
