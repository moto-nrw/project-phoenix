//go:build testhooks

package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// GuardianTestRepositories are the guardian suites' repositories; the audit
// fields are filled by NewGuardianTestRepositories.
type GuardianTestRepositories struct {
	Phone          peoplePorts.GuardianPhoneNumberRepository
	Financial      usersModels.GuardianFinancialDataRepository
	FinancialAudit auditModels.GuardianFinancialChangeCreator
	AccessLog      auditModels.DataAccessLogRepository
}

func NewGuardianTestRepositoriesWithoutAudit(db transactionruntime.Database) GuardianTestRepositories {
	return GuardianTestRepositories{Phone: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()), Financial: peopleCompose.NewGuardianFinancialDataRepository(db.Pool())}
}
