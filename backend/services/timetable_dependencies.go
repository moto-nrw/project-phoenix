package services

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/uptrace/bun"
)

// NewUnobservedTimetableDependencies composes the owner capabilities the
// retained repositories read through, unobserved, for test graphs and CLI
// roots. The People Directory is composed here since its construction left
// the repository composition (#2743).
func NewUnobservedTimetableDependencies(db *bun.DB) repositories.TimetableDependencies {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable students: %v", err))
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable rooms: %v", err))
	}
	capability, err := repositories.NewTimetable(db, students, rooms)
	if err != nil {
		panic(fmt.Sprintf("compose timetable: %v", err))
	}
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable groups: %v", err))
	}
	calendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable calendar: %v", err))
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable membership: %v", err))
	}
	workTime, err := repositories.NewWorkforce(db, membership)
	if err != nil {
		panic(fmt.Sprintf("compose workforce work-time: %v", err))
	}
	return repositories.TimetableDependencies{Capability: capability, Students: students, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime}
}
