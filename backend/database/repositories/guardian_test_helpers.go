package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type GuardianTestRepositories struct {
	Phone          usersModels.GuardianPhoneNumberRepository
	Financial      usersModels.GuardianFinancialDataRepository
	FinancialAudit auditModels.GuardianFinancialChangeCreator
	AccessLog      auditModels.DataAccessLogRepository
}

func NewGuardianTestRepositories(db *bun.DB, command auditModels.Command) GuardianTestRepositories {
	return GuardianTestRepositories{Phone: usersRepo.NewGuardianPhoneNumberRepository(db), Financial: usersRepo.NewGuardianFinancialDataRepository(db),
		FinancialAudit: guardianFinancialChangeCommand{command}, AccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command}}
}

func NewImportAuditTestRepository(db *bun.DB, command auditModels.Command) auditModels.DataImportRepository {
	return dataImportCommand{auditRepo.NewDataImportRepository(newTestAuditRuntime(db)), command}
}

func NewDataDeletionTestRepository(db *bun.DB, command auditModels.Command) auditModels.DataDeletionRepository {
	return dataDeletionCommand{auditRepo.NewDataDeletionRepository(newTestAuditRuntime(db)), command}
}
