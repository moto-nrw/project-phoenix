package repositories

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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

func NewGuardianTestRepositories(db *bun.DB) GuardianTestRepositories {
	return GuardianTestRepositories{Phone: usersRepo.NewGuardianPhoneNumberRepository(peopleRuntime(db)), Financial: usersRepo.NewGuardianFinancialDataRepository(peopleRuntime(db))}
}
