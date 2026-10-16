//go:build testhooks

package api

import (
	"fmt"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewUnobservedTimetableDependencies composes the owner capabilities the
// retained repositories read through, unobserved, for test graphs and CLI
// roots. The People Directory is composed here since its construction left
// the repository composition (#2743).
func NewUnobservedTimetableDependencies(db transactionruntime.Database) TimetableDependencies {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable students: %v", err))
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("compose timetable rooms: %v", err))
	}
	capability, err := NewUnobservedTimetable(db, students, rooms)
	if err != nil {
		panic(fmt.Sprintf("compose timetable: %v", err))
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable groups: %v", err))
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable calendar: %v", err))
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable membership: %v", err))
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		panic(fmt.Sprintf("compose workforce work-time: %v", err))
	}
	return TimetableDependencies{Capability: capability, Students: students, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime}
}
