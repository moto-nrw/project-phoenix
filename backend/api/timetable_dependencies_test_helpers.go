//go:build testhooks

package api

import (
	"fmt"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// The unobserved owners the retained repositories read through, one
// constructor each, for test graphs. A composition failure is a broken test
// setup and panics. Each call composes its owner afresh over db; the owners
// hold no state of their own beyond the database.

// MustNewSchoolStructure composes the School Structure owner.
func MustNewSchoolStructure(db transactionruntime.Database) schoolstructure.Capability {
	groups, err := owners.NewSchoolStructure(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable groups: %v", err))
	}
	return groups
}

// MustNewSchoolCalendar composes the School Calendar owner.
func MustNewSchoolCalendar(db transactionruntime.Database) *schoolcalendar.Module {
	calendar, err := owners.NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable calendar: %v", err))
	}
	return calendar
}

// NewUnobservedTimetableRepositories builds the retained Timetable
// repositories over the unobserved owners of db.
func NewUnobservedTimetableRepositories(db transactionruntime.Database) owners.TimetableRepositories {
	return owners.NewTimetableRepositories(db, owners.MustNewUnobservedTimetable(db), owners.MustNewPeopleDirectory(db), MustNewSchoolStructure(db),
		owners.MustNewUnobservedRooms(db), MustNewSchoolCalendar(db), owners.MustNewSchoolMembership(db), owners.MustNewWorkforce(db))
}
