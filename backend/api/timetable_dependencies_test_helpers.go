//go:build testhooks

package api

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The unobserved owners the retained repositories read through, one
// constructor each, for test graphs. A composition failure is a broken test
// setup and panics. Each call composes its owner afresh over db; the owners
// hold no state of their own beyond the database.

// MustNewUnobservedTimetable composes the unobserved Timetable owner over
// the People Directory and Facilities owners of db.
func MustNewUnobservedTimetable(db transactionruntime.Database) timetable.Capability {
	capability, err := NewUnobservedTimetable(db, MustNewPeopleDirectory(db), MustNewUnobservedRooms(db))
	if err != nil {
		panic(fmt.Sprintf("compose timetable: %v", err))
	}
	return capability
}

// MustNewUnobservedRooms composes the unobserved Facilities owner.
func MustNewUnobservedRooms(db transactionruntime.Database) facilities.Capability {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("compose timetable rooms: %v", err))
	}
	return rooms
}

// MustNewSchoolStructure composes the School Structure owner.
func MustNewSchoolStructure(db transactionruntime.Database) schoolstructure.Capability {
	groups, err := NewSchoolStructure(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable groups: %v", err))
	}
	return groups
}

// MustNewSchoolCalendar composes the School Calendar owner.
func MustNewSchoolCalendar(db transactionruntime.Database) *schoolcalendar.Module {
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable calendar: %v", err))
	}
	return calendar
}

// MustNewSchoolMembership composes the School Membership owner.
func MustNewSchoolMembership(db transactionruntime.Database) schoolmembership.Capability {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable membership: %v", err))
	}
	return membership
}

// MustNewWorkforce composes the unobserved Workforce owner over the School
// Membership owner of db.
func MustNewWorkforce(db transactionruntime.Database) workforce.Capability {
	workTime, err := NewWorkforce(db, MustNewSchoolMembership(db))
	if err != nil {
		panic(fmt.Sprintf("compose workforce work-time: %v", err))
	}
	return workTime
}

// NewUnobservedTimetableRepositories builds the retained Timetable
// repositories over the unobserved owners of db.
func NewUnobservedTimetableRepositories(db transactionruntime.Database) TimetableRepositories {
	return NewTimetableRepositories(db, MustNewUnobservedTimetable(db), MustNewPeopleDirectory(db), MustNewSchoolStructure(db),
		MustNewUnobservedRooms(db), MustNewSchoolCalendar(db), MustNewSchoolMembership(db), MustNewWorkforce(db))
}
