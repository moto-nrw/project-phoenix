package services

import (
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// GuardianTestRepositories are the guardian suites' repositories; the audit
// fields are filled by services.NewGuardianTestRepositories.
type GuardianTestRepositories struct {
	Phone          usersModels.GuardianPhoneNumberRepository
	Financial      usersModels.GuardianFinancialDataRepository
	FinancialAudit auditModels.GuardianFinancialChangeCreator
	AccessLog      auditModels.DataAccessLogRepository
}

func NewGuardianTestRepositoriesWithoutAudit(db *bun.DB) GuardianTestRepositories {
	return GuardianTestRepositories{Phone: peopleCompose.NewGuardianPhoneNumberRepository(db), Financial: peopleCompose.NewGuardianFinancialDataRepository(db)}
}
