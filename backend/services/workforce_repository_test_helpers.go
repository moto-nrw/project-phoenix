package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceCapability "github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

// WorkforceTestRepositories is the Workforce suites' repository graph. The
// audit repositories are filled by services.NewWorkforceTestRepositories,
// the composition that owns Audit persistence (ADR 0049).
type WorkforceTestRepositories struct {
	WorkSessionTestRepositories
	Membership                      schoolmembership.Capability
	StaffAbsenceType                workforceCapability.Capability
	StaffAbsenceTypeAllowance       timerecords.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChange timerecords.StaffAbsenceTypeAllowanceChangeRepository
	StaffMonthSnapshot              workforceCapability.MonthSnapshots
	StaffAbsenceAudit               timerecords.StaffAbsenceAuditRepository
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

func NewWorkforceTestRepositoriesWithMembers(db *bun.DB, members MembershipTestRepositories, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	sessions, err := NewWorkSessionTestRepositoriesWithMembers(db, members, clocks...)
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
	return WorkforceTestRepositories{WorkSessionTestRepositories: sessions,
		Membership:                      membership,
		StaffAbsenceType:                workTime,
		StaffAbsenceTypeAllowance:       workforce.NewStaffAbsenceTypeAllowanceRepository(db),
		StaffAbsenceTypeAllowanceChange: workforce.NewStaffAbsenceTypeAllowanceChangeRepository(db),
		StaffMonthSnapshot:              workTime,
		StaffAbsenceAudit:               workforceLegacy.NewStaffAbsenceAuditRepository(workTime),
		StaffMasterData:                 workforceLegacy.NewStaffMasterDataRepository(workTime),
		StaffQualification:              workforceLegacy.NewStaffQualificationRepository(workTime),
		StaffFinancialData:              workforceLegacy.NewStaffFinancialDataRepository(workTime)}, nil
}
