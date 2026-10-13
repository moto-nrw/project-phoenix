package services

import (
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// DisplayReadRepositories bundles the readers the info-point dashboard
// aggregates. The screens themselves live behind the Device Fleet owner
// (#2676), so no display repository is part of this bundle; the settings
// stores come from the Settings composition (services.NewDisplayTestRepositories).
type DisplayReadRepositories struct {
	TimetableTestRepositories
	StudentPickupNote timetableCompose.CarePickupNoteRows
}

// NewDisplayTestRepositoriesWithMembers composes the dashboard's read dependencies.
func NewDisplayTestRepositoriesWithMembers(db *bun.DB, members MembershipTestRepositories) (DisplayReadRepositories, error) {
	r, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	people := members.People
	care, err := NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	return DisplayReadRepositories{
		TimetableTestRepositories: r,
		StudentPickupNote:         NewPickupNoteRepository(care),
	}, nil
}
