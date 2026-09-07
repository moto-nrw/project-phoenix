package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
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
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return WorkSessionTestRepositories{}, err
	}
	r := &Factory{db: db,
		WorkSession:       activeRepo.NewWorkSessionRepository(db, clocks...),
		WorkSessionBreak:  activeRepo.NewWorkSessionBreakRepository(db),
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
