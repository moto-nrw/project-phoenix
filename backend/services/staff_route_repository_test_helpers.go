package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

// NewAbsenceTypeTestCapability builds the Workforce capability for the
// absence-type routes; an optional clock pins its calendar day.
func NewAbsenceTypeTestCapability(db *bun.DB, clocks ...func() time.Time) workforce.Capability {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(err)
	}
	workTime, err := NewWorkforceWithClock(db, membership, optionalClock(clocks))
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

func NewShiftTypeTestRepositories(db *bun.DB, members MembershipTestRepositories) ShiftTypeTestRepositories {
	repos, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(err)
	}
	return ShiftTypeTestRepositories{Timetable: repos.Timetable, Types: repos.ShiftType, Rows: repos.Repositories}
}
