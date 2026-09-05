package repositories

import (
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type WorkforceTestRepositories struct {
	WorkSessionTestRepositories
	StaffDocument                   userModels.StaffDocumentRepository
	StaffAbsenceType                activeModels.StaffAbsenceTypeRepository
	StaffAbsenceTypeAllowance       activeModels.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChange activeModels.StaffAbsenceTypeAllowanceChangeRepository
	StaffVacationQuota              activeModels.StaffVacationQuotaRepository
	StaffVacationOpening            activeModels.StaffVacationOpeningRepository
	StaffBalanceAdjust              activeModels.StaffBalanceAdjustmentRepository
	StaffMonthSnapshot              activeModels.StaffMonthBalanceSnapshotRepository
	StaffAbsenceAudit               activeModels.StaffAbsenceAuditRepository
	TimeTrackingDeletion            auditModels.TimeTrackingDeletionRepository
	TimeTrackingAuditLog            auditModels.TimeTrackingAuditLogRepository
	StaffMasterData                 userModels.StaffMasterDataRepository
	StaffQualification              userModels.StaffQualificationRepository
	StaffFinancialData              userModels.StaffFinancialDataRepository
	PersonnelNumberChange           auditModels.PersonnelNumberChangeCreator
	StaffMasterDataChange           auditModels.StaffMasterDataChangeCreator
	DataAccessLog                   auditModels.DataAccessLogRepository
	DataDeletion                    auditModels.DataDeletionRepository
}

func NewWorkforceTestRepositories(db *bun.DB, command auditModels.Command, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	sessions, err := NewWorkSessionTestRepositories(db, clocks...)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	r := &Factory{db: db,
		StaffAbsenceType:                active.NewStaffAbsenceTypeRepository(db),
		StaffAbsenceTypeAllowance:       workforce.NewStaffAbsenceTypeAllowanceRepository(db),
		StaffAbsenceTypeAllowanceChange: workforce.NewStaffAbsenceTypeAllowanceChangeRepository(db),
		StaffVacationQuota:              active.NewStaffVacationQuotaRepository(db), StaffVacationOpening: active.NewStaffVacationOpeningRepository(db),
		StaffBalanceAdjust: active.NewStaffBalanceAdjustmentRepository(db), StaffMonthSnapshot: active.NewStaffMonthBalanceSnapshotRepository(db),
		StaffAbsenceAudit: active.NewStaffAbsenceAuditRepository(db), TimeTrackingDeletion: audit.NewTimeTrackingDeletionRepository(newTestAuditRuntime(db)),
		TimeTrackingAuditLog: audit.NewTimeTrackingAuditLogRepository(newTestAuditRuntime(db)),
		StaffMasterData:      users.NewStaffMasterDataRepository(db), StaffQualification: users.NewStaffQualificationRepository(db),
		StaffFinancialData: users.NewStaffFinancialDataRepository(db), PersonnelNumberChange: audit.NewPersonnelNumberChangeRepository(newTestAuditRuntime(db)),
		StaffMasterDataChange: audit.NewStaffMasterDataChangeRepository(newTestAuditRuntime(db)), DataAccessLog: audit.NewDataAccessLogRepository(newTestAuditRuntime(db)),
		DataDeletion: audit.NewDataDeletionRepository(newTestAuditRuntime(db)),
	}
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }})
	r.BindPeopleDirectory(people)
	r.RouteAuditWrites(command)
	return WorkforceTestRepositories{WorkSessionTestRepositories: sessions,
		StaffDocument:    staffDocumentMembershipRepository{StaffDocumentRepository: users.NewStaffDocumentRepository(db), membership: func() schoolmembership.Capability { return membership }},
		StaffAbsenceType: r.StaffAbsenceType, StaffAbsenceTypeAllowance: r.StaffAbsenceTypeAllowance, StaffAbsenceTypeAllowanceChange: r.StaffAbsenceTypeAllowanceChange,
		StaffVacationQuota: r.StaffVacationQuota, StaffVacationOpening: r.StaffVacationOpening, StaffBalanceAdjust: r.StaffBalanceAdjust, StaffMonthSnapshot: r.StaffMonthSnapshot,
		StaffAbsenceAudit: r.StaffAbsenceAudit, TimeTrackingDeletion: r.TimeTrackingDeletion, TimeTrackingAuditLog: r.TimeTrackingAuditLog,
		StaffMasterData: r.StaffMasterData, StaffQualification: r.StaffQualification, StaffFinancialData: r.StaffFinancialData,
		PersonnelNumberChange: r.PersonnelNumberChange, StaffMasterDataChange: r.StaffMasterDataChange, DataAccessLog: r.DataAccessLog, DataDeletion: r.DataDeletion}, nil
}
