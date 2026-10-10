package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/uptrace/bun"
)

type ActivitiesTestModule struct {
	Activities  activities.ActivityService
	Timetable   timetable.Capability
	Users       *peopleCompose.PersonDirectory
	UserContext *repositories.CallerRows
}

func NewActivitiesTestModule(db *bun.DB) (ActivitiesTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	activityService, err := activities.NewService(r.Timetable, r.ActivityGroup, r.ActivitySchedule,
		r.ActivitySupervisor, r.StudentEnrollment, r.ActiveGroup, r.Staff, r.Student)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	// Activities consumes the owner's timeframe reads and staff-directory reads only.
	return ActivitiesTestModule{
		Activities: activityService, Timetable: r.Timetable, UserContext: identity,
		Users: peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
			PersonDirectory:  repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
			StudentDirectory: repositories.NewStudentDirectory(repositories.MustNewPeopleDirectory(db)),
			PersonRepo:       r.Person, TeacherRepo: r.Teacher,
			StaffDirectory: NewStaffDirectory(StaffDirectoryDependencies{DB: db, Persons: r.Person, Staff: r.Staff, Teachers: r.Teacher}),
		}),
	}, nil
}
