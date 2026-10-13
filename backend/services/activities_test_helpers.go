package services

import (
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type ActivitiesTestModule struct {
	Activities  timetableCompose.ActivityService
	Timetable   timetable.Capability
	Users       *peopleCompose.PersonDirectory
	UserContext *CallerRows
}

func NewActivitiesTestModule(db *bun.DB) (ActivitiesTestModule, error) {
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	activityService, err := timetableCompose.NewService(r.Timetable, r.ActivityGroup, r.ActivitySchedule,
		r.ActivitySupervisor, r.StudentEnrollment, r.ActiveGroup, r.Staff, r.Student)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	// Activities consumes the owner's timeframe reads and staff-directory reads only.
	return ActivitiesTestModule{
		Activities: activityService, Timetable: r.Timetable, UserContext: identity,
		Users: peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
			PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
			StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
			PersonRepo:       r.Person, TeacherRepo: r.Teacher,
			StaffDirectory: NewStaffDirectory(StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db), Persons: r.Person, Staff: r.Staff, Teachers: r.Teacher}),
		}),
	}, nil
}
