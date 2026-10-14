package api

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/services"
)

type WorkSessionTestRepositories struct {
	TimetableTestRepositories
	// Workforce is the capability the time-tracking rows are served from.
	Workforce workforce.Capability
	// WorkSessionEdit is filled by NewWorkSessionTestRepositories.
	WorkSessionEdit   auditModels.WorkSessionEditRepository
	StaffAbsence      workforceCompose.StaffAbsenceRepository
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository
}

func NewWorkSessionTestRepositoriesWithMembers(db transactionruntime.Database, members services.MembershipTestRepositories, clocks ...func() time.Time) (WorkSessionTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	people := members.People
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	workTime, err := NewWorkforceWithClock(db, membership, now)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	return WorkSessionTestRepositories{TimetableTestRepositories: tt,
		Workforce:         workTime,
		StaffAbsence:      NewStaffAbsenceRepository(workTime, membership, people),
		StaffWorkSchedule: workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime),
		WorkTimeModel:     workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)}, nil
}
