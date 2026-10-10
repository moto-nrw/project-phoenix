package services

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
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
func NewTestPersonDirectory(db *bun.DB, sources TestPersonDirectorySources) *peopleCompose.PersonDirectory {
	people := repositories.MustNewPeopleDirectory(db)
	deps := peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  repositories.NewPersonDirectory(people),
		StudentDirectory: repositories.NewStudentDirectory(people),
		PersonRepo:       sources.Persons,
		RFIDRepo:         sources.RFIDCards,
		AccountExists:    sources.AccountExists,
		StudentRepo:      sources.Students,
		TeacherRepo:      sources.Teachers,
	}
	if sources.Staff != nil {
		deps.StaffDirectory = NewStaffDirectory(StaffDirectoryDependencies{
			DB: db, Persons: sources.Persons, Staff: sources.Staff, Teachers: sources.Teachers,
		})
	}
	return peopleCompose.NewPersonDirectory(deps)
}

// The People Directory composition contracts the behavior suites outside this
// package name (#3753), restated here because their test scopes may not import
// modules/peopledirectory/compose. Every entry goes with the last suite that
// uses it.
type (
	PeopleDirectorySuitePhotoUnlinker  = peopleCompose.PhotoUnlinker
	PeopleDirectorySuiteStudentPhotos  = peopleCompose.StudentPhotoService
	PeopleDirectorySuiteCommitUpload   = peopleCompose.CommitUploadRequest
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
