//go:build testhooks

package api

import (
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewAbsenceTypeTestCapability builds the Workforce capability for the
// absence-type routes; an optional clock pins its calendar day.
func NewAbsenceTypeTestCapability(db transactionruntime.Database, clocks ...func() time.Time) workforceCompose.WorkTime {
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		panic(err)
	}
	workTime, err := owners.NewWorkforceWithClock(db, membership, OptionalClock(clocks))
	if err != nil {
		panic(err)
	}
	return workTime
}

type ShiftTypeTestRepositories struct {
	Timetable timetable.Capability
	Types     timetablePorts.WorkforceShiftTypeRows
	// Rows are the retained Timetable rows, the activity categories among them.
	Rows timetableCompose.Repositories
}

func NewShiftTypeTestRepositories(db transactionruntime.Database, members owners.MembershipTestRepositories) ShiftTypeTestRepositories {
	repos, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(err)
	}
	return ShiftTypeTestRepositories{Timetable: repos.Timetable, Types: repos.ShiftType, Rows: repos.Repositories}
}
