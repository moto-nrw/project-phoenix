package repositories

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// DisplayTestRepositories bundles the readers the info-point dashboard
// aggregates. The screens themselves live behind the Device Fleet owner
// (#2676), so no display repository is part of this bundle; the settings
// stores come from the Settings composition (services.NewDisplayTestRepositories).
type DisplayTestRepositories struct {
	TimetableTestRepositories
	StudentPickupNote scheduleModels.StudentPickupNoteRepository
}

// NewDisplayTestRepositories composes the dashboard's read dependencies.
func NewDisplayTestRepositories(db *bun.DB, members MembershipTestRepositories) (DisplayTestRepositories, error) {
	r, err := NewTimetableTestRepositories(db, members)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	people := members.People
	care, err := NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	return DisplayTestRepositories{
		TimetableTestRepositories: r,
		StudentPickupNote:         NewPickupNoteRepository(care),
	}, nil
}
