//go:build testhooks

package api

import (
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type ActivitiesTestModule struct {
	Activities  timetableCompose.ActivityService
	Timetable   timetable.Capability
	Users       peoplePorts.PersonDirectory
	UserContext *CallerRows
}

func NewActivitiesTestModule(db transactionruntime.Database) (ActivitiesTestModule, error) {
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	identity, members, err := newStaffIdentityForTests(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	presence, err := NewTimetablePresenceRows(db, r, members)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	activityService, err := timetableCompose.NewService(r.Timetable, r.ActivityGroup, r.ActivitySchedule,
		r.ActivitySupervisor, r.StudentEnrollment, presence.ActiveGroup, NewTimetableStaff(members.Staff), members.Student)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	// Activities consumes the owner's timeframe reads and staff-directory reads only.
	return ActivitiesTestModule{
		Activities: activityService, Timetable: r.Timetable, UserContext: identity,
		Users: peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
			PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
			StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
			PersonRepo:       members.Person, TeacherRepo: members.Teacher,
			StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, MayManageStaff: StaffAdministrationPermitted}),
		}),
	}, nil
}
