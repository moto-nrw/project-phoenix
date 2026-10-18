//go:build testhooks

package api

import (
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// DisplayReadRepositories bundles the readers the info-point dashboard
// aggregates. The screens themselves live behind the Device Fleet owner
// (#2676), so no display repository is part of this bundle; the settings
// stores come from the Settings composition (NewDisplayTestRepositories).
type DisplayReadRepositories struct {
	TimetableTestRepositories
	// The other owners' rows beside the Timetable rows (#4080).
	MembershipTestRepositories
	TimetablePresenceRows
	TimetableCarePlanRows
	TimetableRoomRows
	TimetableDeviationAudit
	TimetableSchoolCalendar
	StudentPickupNote timetablePorts.CarePickupNoteRows
}

// NewDisplayTestRepositoriesWithMembers composes the dashboard's read dependencies.
func NewDisplayTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (DisplayReadRepositories, error) {
	r, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	presence, carePlan, rooms, calendar, err := newTimetableTestCompanions(db, r, members)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	people := members.People
	care, err := NewCarePlan(db, people, r.InstanceStudent)
	if err != nil {
		return DisplayReadRepositories{}, err
	}
	return DisplayReadRepositories{
		TimetableTestRepositories: r, MembershipTestRepositories: members, TimetablePresenceRows: presence, TimetableCarePlanRows: carePlan, TimetableRoomRows: rooms, TimetableSchoolCalendar: calendar,
		StudentPickupNote: NewPickupNoteRepository(care),
	}, nil
}
