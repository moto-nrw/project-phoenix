//go:build testhooks

package api

import (
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// GuardianTestRepositories are the guardian suites' People Directory
// repositories; their financial-change and access trails come from
// NewTestAuditPersistence.
type GuardianTestRepositories struct {
	Phone     peoplePorts.GuardianPhoneNumberRepository
	Financial usersModels.GuardianFinancialDataRepository
}

func NewGuardianTestRepositoriesWithoutAudit(db transactionruntime.Database) GuardianTestRepositories {
	return GuardianTestRepositories{Phone: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()), Financial: peopleCompose.NewGuardianFinancialDataRepository(db.Pool())}
}
