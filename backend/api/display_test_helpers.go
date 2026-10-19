//go:build testhooks

package api

import (
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// DisplayReadRepositories bundles the readers the info-point dashboard
// aggregates. The screens themselves live behind the Device Fleet owner
// (#2676), so no display repository is part of this bundle; the settings
// stores come from the Settings composition (NewDisplayTestRepositories).
// The live sessions come from MustNewTimetablePresenceRows (#4080).
type DisplayReadRepositories struct {
	owners.TimetableTestRepositories
	StudentPickupNote timetablePorts.CarePickupNoteRows
}

// NewDisplayTestRepositoriesWithMembers composes the dashboard's read dependencies.
func NewDisplayTestRepositoriesWithMembers(db transactionruntime.Database, members owners.MembershipTestRepositories) (DisplayReadRepositories, error) {
	r, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	people := members.People
	care, err := owners.NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	return DisplayReadRepositories{
		TimetableTestRepositories: r,
		StudentPickupNote:         NewPickupNoteRepository(care),
	}, nil
}
