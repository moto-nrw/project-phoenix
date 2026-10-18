//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// RFIDTestModule is the Device Scan side of the RFID graph: the feedback
// student lookup and the tag assignments over the People Directory's person
// directory (NewRFIDTestPeople).
type RFIDTestModule struct {
	FeedbackStudents devicescan.FeedbackStudents
	TagAssignments   devicescanCompose.TagAssignments
}

// NewRFIDTestModule provides identity lookup and tag assignment, without
// constructing attendance, timetable or enrollment services.
func NewRFIDTestModule(db transactionruntime.Database) (RFIDTestModule, error) {
	people, err := NewRFIDTestPeople(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	return RFIDTestModule{FeedbackStudents: devicescanCompose.NewFeedbackStudents(NewDeviceScanPeople(people)), TagAssignments: devicescanCompose.NewTagAssignments(NewDeviceScanPeople(people), nil)}, nil
}

// NewRFIDTestPeople composes the People Directory's person directory with the
// RFID card reads the RFID graph looks identities up through.
func NewRFIDTestPeople(db transactionruntime.Database) (peoplePorts.PersonDirectory, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return nil, err
	}
	accounts, err := NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(MustNewPeopleDirectory(db)),
		StudentDirectory: peopleCompose.NewStudentDirectory(MustNewPeopleDirectory(db)),
		PersonRepo:       members.Person, TeacherRepo: members.Teacher,
		StaffDirectory: peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: members.Person, Staff: members.Staff, Teachers: members.Teacher, MayManageStaff: StaffAdministrationPermitted}),
		AccountExists:  identityaccessCompose.AccountExists(accounts), StudentRepo: NewUnboundStudentRepository(db), RFIDRepo: NewPeopleIdentityAccess(db, nil),
	}), nil
}
