package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/uptrace/bun"
)

// ClassListTestModule is the School Membership owner with its class-list
// administration bound the same way production binds it: the People Directory
// behind the duplicate guard and the match hint, the fail-closed Audit
// command behind the change trail.
type ClassListTestModule struct {
	Membership schoolmembership.Capability
}

func NewClassListTestModule(db *bun.DB) (ClassListTestModule, error) {
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return ClassListTestModule{}, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return ClassListTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return ClassListTestModule{}, err
	}
	if err := bindClassListEntryAdministration(membership, persons, command); err != nil {
		return ClassListTestModule{}, err
	}
	return ClassListTestModule{Membership: membership}, nil
}
