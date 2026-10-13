package services

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

func NewAbsenceTypeTestCapability(db *bun.DB) workforce.Capability {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(err)
	}
	workTime, err := NewWorkforce(db, membership)
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
