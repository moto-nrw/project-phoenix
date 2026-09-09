package repositories

import (
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
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
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	workTime, err := NewWorkforceWithClock(db, membership, now)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	r := &Factory{db: db,
		StaffAbsenceType:                workforceLegacy.NewStaffAbsenceTypeRepository(workTime),
		StaffAbsenceTypeAllowance:       workforce.NewStaffAbsenceTypeAllowanceRepository(db),
		StaffAbsenceTypeAllowanceChange: workforce.NewStaffAbsenceTypeAllowanceChangeRepository(db),
		StaffVacationQuota:              workforceLegacy.NewStaffVacationQuotaRepository(workTime), StaffVacationOpening: workforceLegacy.NewStaffVacationOpeningRepository(workTime),
		StaffBalanceAdjust: workforceLegacy.NewStaffBalanceAdjustmentRepository(workTime), StaffMonthSnapshot: active.NewStaffMonthBalanceSnapshotRepository(db),
		StaffAbsenceAudit: workforceLegacy.NewStaffAbsenceAuditRepository(workTime), TimeTrackingDeletion: audit.NewTimeTrackingDeletionRepository(newTestAuditRuntime(db)),
		TimeTrackingAuditLog: audit.NewTimeTrackingAuditLogRepository(newTestAuditRuntime(db)),
		StaffMasterData:      workforceLegacy.NewStaffMasterDataRepository(workTime), StaffQualification: workforceLegacy.NewStaffQualificationRepository(workTime),
		StaffFinancialData: workforceLegacy.NewStaffFinancialDataRepository(workTime), PersonnelNumberChange: audit.NewPersonnelNumberChangeRepository(newTestAuditRuntime(db)),
		StaffMasterDataChange: audit.NewStaffMasterDataChangeRepository(newTestAuditRuntime(db)), DataAccessLog: audit.NewDataAccessLogRepository(newTestAuditRuntime(db)),
		DataDeletion: audit.NewDataDeletionRepository(newTestAuditRuntime(db)),
	}
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}, workTime)
	r.BindPeopleDirectory(people)
	r.RouteAuditWrites(command)
	return WorkforceTestRepositories{WorkSessionTestRepositories: sessions,
		StaffDocument:    staffDocumentMembershipRepository{StaffDocumentRepository: workforceLegacy.NewStaffDocumentRepository(workTime), membership: func() schoolmembership.Capability { return membership }},
		StaffAbsenceType: r.StaffAbsenceType, StaffAbsenceTypeAllowance: r.StaffAbsenceTypeAllowance, StaffAbsenceTypeAllowanceChange: r.StaffAbsenceTypeAllowanceChange,
		StaffVacationQuota: r.StaffVacationQuota, StaffVacationOpening: r.StaffVacationOpening, StaffBalanceAdjust: r.StaffBalanceAdjust, StaffMonthSnapshot: r.StaffMonthSnapshot,
		StaffAbsenceAudit: r.StaffAbsenceAudit, TimeTrackingDeletion: r.TimeTrackingDeletion, TimeTrackingAuditLog: r.TimeTrackingAuditLog,
		StaffMasterData: r.StaffMasterData, StaffQualification: r.StaffQualification, StaffFinancialData: r.StaffFinancialData,
		PersonnelNumberChange: r.PersonnelNumberChange, StaffMasterDataChange: r.StaffMasterDataChange, DataAccessLog: r.DataAccessLog, DataDeletion: r.DataDeletion}, nil
}
