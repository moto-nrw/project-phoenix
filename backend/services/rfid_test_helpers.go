package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

type RFIDTestModule struct {
	FeedbackStudents devicescanCompose.FeedbackStudents
	Users            *peopleCompose.PersonDirectory
	TagAssignments   devicescanCompose.TagAssignments
}

// NewRFIDTestModule provides identity lookup and tag assignment, without
// constructing attendance, timetable or enrollment services.
func NewRFIDTestModule(db *bun.DB) (RFIDTestModule, error) {
	r, err := repositories.NewRFIDTestRepositories(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	accounts, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	people := peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  repositories.NewPersonDirectory(repositories.MustNewPeopleDirectory(db)),
		StudentDirectory: repositories.NewStudentDirectory(repositories.MustNewPeopleDirectory(db)),
		PersonRepo:       r.Membership.Person, TeacherRepo: r.Membership.Teacher,
		StaffDirectory: NewStaffDirectory(StaffDirectoryDependencies{DB: db, Persons: r.Membership.Person, Staff: r.Membership.Staff, Teachers: r.Membership.Teacher}),
		AccountExists:  repositories.AccountExists(accounts), StudentRepo: r.Student, RFIDRepo: r.RFID,
	})
	return RFIDTestModule{Users: people, FeedbackStudents: devicescanCompose.NewFeedbackStudents(people), TagAssignments: devicescanCompose.NewTagAssignments(people, nil)}, nil
}
