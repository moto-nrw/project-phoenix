package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
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

func (r WorkSessionTestRepositories) WithConfigRuntime(runtime configRepo.Runtime) WorkSessionTestRepositories {
	return WorkSessionTestRepositories{
		TimetableTestRepositories: r.TimetableTestRepositories,
		WorkSession:               r.WorkSession, WorkSessionBreak: r.WorkSessionBreak,
		WorkSessionEdit: r.WorkSessionEdit, StaffAbsence: r.StaffAbsence,
		StaffWorkSchedule: configRepo.NewStaffWorkScheduleRepository(runtime),
		WorkTimeModel:     configRepo.NewWorkTimeModelRepository(runtime),
	}
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
	r := &Factory{db: db,
		WorkSession:       activeRepo.NewWorkSessionRepository(db, clocks...),
		WorkSessionBreak:  activeRepo.NewWorkSessionBreakRepository(db),
		WorkSessionEdit:   auditRepo.NewWorkSessionEditRepository(newTestAuditRuntime(db)),
		StaffAbsence:      activeRepo.NewStaffAbsenceRepository(db),
		StaffWorkSchedule: configRepo.NewStaffWorkScheduleRepository(configRepo.NewRuntime(db)),
		WorkTimeModel:     configRepo.NewWorkTimeModelRepository(configRepo.NewRuntime(db)),
	}
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }})
	r.BindPeopleDirectory(people)
	return WorkSessionTestRepositories{TimetableTestRepositories: tt, WorkSession: r.WorkSession, WorkSessionBreak: r.WorkSessionBreak,
		WorkSessionEdit: r.WorkSessionEdit, StaffAbsence: r.StaffAbsence, StaffWorkSchedule: r.StaffWorkSchedule, WorkTimeModel: r.WorkTimeModel}, nil
}
