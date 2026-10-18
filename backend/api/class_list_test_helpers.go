//go:build testhooks

package api

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// ClassListTestModule is the School Membership owner with its class-list
// administration composed the same way production composes it: the People Directory
// behind the duplicate guard and the match hint, the fail-closed Audit
// command behind the change trail.
type ClassListTestModule struct {
	Membership schoolmembership.Capability
}

func NewClassListTestModule(db transactionruntime.Database) (ClassListTestModule, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return ClassListTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return ClassListTestModule{}, err
	}
	membership, err := composeClassListEntries(db, persons, command)
	if err != nil {
		return ClassListTestModule{}, err
	}
	return ClassListTestModule{Membership: membership}, nil
}
