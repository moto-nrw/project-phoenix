package repositories

import (
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// DisplayTestRepositories bundles the readers the info-point dashboard
// aggregates. The screens themselves live behind the Device Fleet owner
// (#2676), so no display repository is part of this bundle.
type DisplayTestRepositories struct {
	TimetableTestRepositories
	SettingsTestRepositories
	School            platformModels.SchoolRepository
	StudentPickupNote scheduleModels.StudentPickupNoteRepository
}

// NewDisplayTestRepositories composes the dashboard's read dependencies.
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
		School:            NewSchoolCapabilityAdapter(organizations, nil),
		StudentPickupNote: NewPickupNoteRepository(care),
	}, nil
}
