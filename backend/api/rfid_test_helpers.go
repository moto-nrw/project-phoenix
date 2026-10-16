//go:build testhooks

package api

import (
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type RFIDTestModule struct {
	FeedbackStudents devicescanCompose.FeedbackStudents
	Users            *peopleCompose.PersonDirectory
	TagAssignments   devicescanCompose.TagAssignments
}

// NewRFIDTestModule provides identity lookup and tag assignment, without
// constructing attendance, timetable or enrollment services.
func NewRFIDTestModule(db transactionruntime.Database) (RFIDTestModule, error) {
	r, err := NewRFIDTestRepositories(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	accounts, err := NewIdentityAccessForTests(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	people := peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       r.Membership.Person, TeacherRepo: r.Membership.Teacher,
		StaffDirectory: peopleCompose.NewStaffDirectory(peopleCompose.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: r.Membership.Person, Staff: r.Membership.Staff, Teachers: r.Membership.Teacher, MayManageStaff: StaffAdministrationPermitted}),
		AccountExists:  identityaccessCompose.AccountExists(accounts), StudentRepo: r.Student, RFIDRepo: r.RFID,
	})
	return RFIDTestModule{Users: people, FeedbackStudents: devicescanCompose.NewFeedbackStudents(people), TagAssignments: devicescanCompose.NewTagAssignments(people, nil)}, nil
}
