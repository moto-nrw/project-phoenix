package api

import (
	"context"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// TestPersonDirectorySources are the retained repositories a test person
// directory reads. A suite leaves unset what its scenarios never reach; without
// Staff the directory has no staff half.
type TestPersonDirectorySources struct {
	Persons       userModels.PersonRepository
	Students      userModels.StudentRepository
	Teachers      userModels.TeacherRepository
	Staff         userModels.StaffRepository
	RFIDCards     peopleCompose.RFIDCards
	AccountExists func(context.Context, int64) (bool, error)
}

// NewTestPersonDirectory composes the retained person directory
// (modules/peopledirectory/compose, #3753) for the behavior suites outside
// this package that used to build the person service themselves and may not
// import that composition. The dated roster read stays unwired; a suite that
// needs it derives a copy with WithCareParticipation.
func NewTestPersonDirectory(db transactionruntime.Database, sources TestPersonDirectorySources) *peopleCompose.PersonDirectory {
	people := services.MustNewPeopleDirectory(db.Pool())
	deps := peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(people),
		StudentDirectory: peopleCompose.NewStudentDirectory(people),
		PersonRepo:       sources.Persons,
		RFIDRepo:         sources.RFIDCards,
		AccountExists:    sources.AccountExists,
		StudentRepo:      sources.Students,
		TeacherRepo:      sources.Teachers,
	}
	if sources.Staff != nil {
		deps.StaffDirectory = services.NewStaffDirectory(services.StaffDirectoryDependencies{
			Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: sources.Persons, Staff: sources.Staff, Teachers: sources.Teachers,
		})
	}
	return peopleCompose.NewPersonDirectory(deps)
}

// The People Directory composition contracts the behavior suites outside this
// package name (#3753), restated here because their test scopes may not import
// modules/peopledirectory/compose. Every entry goes with the last suite that
// uses it.
type (
	PeopleDirectorySuiteStudentService = peopleCompose.StudentService
)

// NewPeopleDirectorySuiteStudentAudit is the request-actor audit service over
// recorder, the way the root composes it.
func NewPeopleDirectorySuiteStudentAudit(actor peopleCompose.RequestAuditActor, recorder peopleCompose.StudentAuditRecorder) peopleCompose.StudentAuditService {
	return peopleCompose.NewStudentAuditService(actor, recorder)
}

// NewPeopleDirectorySuiteStudentService is the retained student directory
// service over the owner's directory seam.
func NewPeopleDirectorySuiteStudentService(
	directory peopleCompose.StudentDirectoryAccess,
	classes peopleCompose.StudentClassReader,
	students userModels.StudentRepository,
) peopleCompose.StudentService {
	return peopleCompose.NewStudentService(directory, classes, students)
}
