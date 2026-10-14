package api

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services"
)

// NewAbsenceTypeTestCapability builds the Workforce capability for the
// absence-type routes; an optional clock pins its calendar day.
func NewAbsenceTypeTestCapability(db transactionruntime.Database, clocks ...func() time.Time) workforce.Capability {
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		panic(err)
	}
	workTime, err := NewWorkforceWithClock(db, membership, services.OptionalClock(clocks))
	if err != nil {
		panic(err)
	}
	return workTime
}

type ShiftTypeTestRepositories struct {
	Timetable timetable.Capability
	Types     *timetableCompose.WorkforceShiftTypeRows
	// Rows are the retained Timetable rows, the activity categories among them.
	Rows timetableCompose.Repositories
}

func NewShiftTypeTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) ShiftTypeTestRepositories {
	repos, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(err)
	}
	return ShiftTypeTestRepositories{Timetable: repos.Timetable, Types: repos.ShiftType, Rows: repos.Repositories}
}
