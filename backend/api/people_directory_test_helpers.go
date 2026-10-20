//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// TestPersonDirectorySources are the retained repositories a test person
// directory reads. A suite leaves unset what its scenarios never reach; without
// Staff the directory has no staff half.
type TestPersonDirectorySources struct {
	Persons       peoplePorts.PersonRepository
	Students      peoplePorts.StudentRepository
	Teachers      peoplePorts.TeacherRepository
	Staff         peoplePorts.StaffRepository
	RFIDCards     peoplePorts.RFIDCards
	AccountExists func(context.Context, int64) (bool, error)
}

// NewTestPersonDirectory composes the retained person directory
// (modules/peopledirectory/compose, #3753) for the behavior suites outside
// this package that used to build the person service themselves and may not
// import that composition. The dated roster read stays unwired; a suite that
// needs it derives a copy with WithCareParticipation.
func NewTestPersonDirectory(db transactionruntime.Database, sources TestPersonDirectorySources) peoplePorts.PersonDirectory {
	people := owners.MustNewPeopleDirectory(db)
	deps := peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  people.PersonWrites,
		StudentDirectory: people.StudentDirectory,
		PersonRepo:       sources.Persons,
		RFIDRepo:         sources.RFIDCards,
		AccountExists:    sources.AccountExists,
		StudentRepo:      sources.Students,
		TeacherRepo:      sources.Teachers,
	}
	if sources.Staff != nil {
		deps.StaffDirectory = peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{
			Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: sources.Persons, Staff: sources.Staff, Teachers: sources.Teachers,
			MayManageStaff: StaffAdministrationPermitted,
		})
	}
	return peopleCompose.NewPersonDirectory(deps)
}

// NewPeopleDirectorySuiteStudentAudit is the request-actor audit service over
// recorder, the way the root composes it.
func NewPeopleDirectorySuiteStudentAudit(actor peoplePorts.RequestAuditActor, recorder peoplePorts.StudentAuditRecorder) peoplePorts.StudentAuditService {
	return peopleCompose.NewStudentAuditService(actor, recorder)
}

// StaffAdministrationPermitted is the Security Runtime's staff:manage check
// the staff directory asks before a create adopts an existing staff record,
// as the service graph binds it.
func StaffAdministrationPermitted(actorPermissions []string) bool {
	return securityruntime.HasPermission(securityruntime.PermissionStaffManage, actorPermissions)
}
