package repositories

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type WorkSessionTestRepositories struct {
	TimetableTestRepositories
	WorkSession      timerecords.WorkSessionRepository
	WorkSessionBreak timerecords.WorkSessionBreakRepository
	// WorkSessionEdit is filled by services.NewWorkSessionTestRepositories.
	WorkSessionEdit   auditModels.WorkSessionEditRepository
	StaffAbsence      timerecords.StaffAbsenceRepository
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository
}

func NewWorkSessionTestRepositories(db *bun.DB, members MembershipTestRepositories, clocks ...func() time.Time) (WorkSessionTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db, members, clocks...)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	people := members.People
	membership, err := NewSchoolMembership(db)
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
	r := &Factory{db: db, StaffAbsence: workforceLegacy.NewStaffAbsenceRepository(workTime)}
	r.bindStaffProjections(membership, workTime)
	r.bindPersonProjections(people)
	return WorkSessionTestRepositories{TimetableTestRepositories: tt,
		WorkSession:       workforceLegacy.NewWorkSessionRepository(workTime),
		WorkSessionBreak:  workforceLegacy.NewWorkSessionBreakRepository(workTime),
		StaffAbsence:      r.StaffAbsence,
		StaffWorkSchedule: workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime),
		WorkTimeModel:     workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)}, nil
}
