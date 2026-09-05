package repositories

import (
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	displayRepo "github.com/moto-nrw/project-phoenix/database/repositories/display"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

type DisplayTestRepositories struct {
	TimetableTestRepositories
	SettingsTestRepositories
	Display           displayModels.Repository
	School            platformModels.SchoolRepository
	Attendance        activeModels.AttendanceRepository
	StudentPickupNote scheduleModels.StudentPickupNoteRepository
}

func NewDisplayTestRepositories(db *bun.DB, runtime configRepo.Runtime) (DisplayTestRepositories, error) {
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	care, err := NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	return DisplayTestRepositories{
		TimetableTestRepositories: r, SettingsTestRepositories: NewSettingsTestRepositories(db, runtime),
		Display: displayRepo.NewDisplayRepository(db), School: NewSchoolCapabilityAdapter(organizations, nil),
		Attendance: activeRepo.NewAttendanceRepository(db), StudentPickupNote: NewPickupNoteRepository(care),
	}, nil
}
