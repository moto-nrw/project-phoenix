//go:build testhooks

package api

import (
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCapability "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// WorkforceTestRepositories is the Workforce suites' repository graph of the
// Workforce owner. The audit trails come from the Audit persistence, the
// Timetable rows and the people from the timetable fixtures beside it.
type WorkforceTestRepositories struct {
	WorkSessionTestRepositories
	StaffAbsenceType                workforceCapability.Capability
	StaffAbsenceTypeAllowance       *workforceCompose.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChange *workforceCompose.StaffAbsenceTypeAllowanceChangeRepository
	StaffMonthSnapshot              workforceCapability.MonthSnapshots
	StaffAbsenceAudit               workforceCompose.StaffAbsenceAuditRepository
}

func NewWorkforceTestRepositoriesWithMembers(db transactionruntime.Database, members owners.MembershipTestRepositories, clocks ...func() time.Time) (WorkforceTestRepositories, error) {
	sessions, err := NewWorkSessionTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	workTime, err := owners.NewWorkforceWithClock(db, membership, now)
	if err != nil {
		return WorkforceTestRepositories{}, err
	}
	return WorkforceTestRepositories{WorkSessionTestRepositories: sessions,
		StaffAbsenceType:                workTime,
		StaffAbsenceTypeAllowance:       workforceCompose.NewStaffAbsenceTypeAllowanceRepository(db.Pool()),
		StaffAbsenceTypeAllowanceChange: workforceCompose.NewStaffAbsenceTypeAllowanceChangeRepository(db.Pool()),
		StaffMonthSnapshot:              workTime,
		StaffAbsenceAudit:               workforceCompose.NewStaffAbsenceAuditRepository(workTime)}, nil
}
