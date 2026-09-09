package repositories

import (
	"time"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type WorkSessionTestRepositories struct {
	TimetableTestRepositories
	WorkSession       activeModels.WorkSessionRepository
	WorkSessionBreak  activeModels.WorkSessionBreakRepository
	WorkSessionEdit   auditModels.WorkSessionEditRepository
	StaffAbsence      activeModels.StaffAbsenceRepository
	StaffWorkSchedule configModels.StaffWorkScheduleRepository
	WorkTimeModel     configModels.WorkTimeModelRepository
}

func NewWorkSessionTestRepositories(db *bun.DB, clocks ...func() time.Time) (WorkSessionTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
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
	r := &Factory{db: db,
		WorkSession:       workforceLegacy.NewWorkSessionRepository(workTime),
		WorkSessionBreak:  workforceLegacy.NewWorkSessionBreakRepository(workTime),
		WorkSessionEdit:   auditRepo.NewWorkSessionEditRepository(newTestAuditRuntime(db)),
		StaffAbsence:      workforceLegacy.NewStaffAbsenceRepository(workTime),
		StaffWorkSchedule: workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime),
		WorkTimeModel:     workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime),
	}
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}, workTime)
	r.BindPeopleDirectory(people)
	return WorkSessionTestRepositories{TimetableTestRepositories: tt, WorkSession: r.WorkSession, WorkSessionBreak: r.WorkSessionBreak,
		WorkSessionEdit: r.WorkSessionEdit, StaffAbsence: r.StaffAbsence, StaffWorkSchedule: r.StaffWorkSchedule, WorkTimeModel: r.WorkTimeModel}, nil
}
