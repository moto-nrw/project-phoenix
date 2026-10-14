package api

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// ClassListTestModule is the School Membership owner with its class-list
// administration bound the same way production binds it: the People Directory
// behind the duplicate guard and the match hint, the fail-closed Audit
// command behind the change trail.
type ClassListTestModule struct {
	Membership schoolmembership.Capability
}

func NewClassListTestModule(db transactionruntime.Database) (ClassListTestModule, error) {
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return ClassListTestModule{}, err
	}
	persons, err := services.NewPeopleDirectory(db.Pool())
	if err != nil {
		return ClassListTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return ClassListTestModule{}, err
	}
	if err := services.BindClassListEntryAdministration(membership, persons, command); err != nil {
		return ClassListTestModule{}, err
	}
	return ClassListTestModule{Membership: membership}, nil
}
