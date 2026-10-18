//go:build testhooks

package api

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// StatisticsTestRepositories are the statistics suites' Care Plan status
// days over the Timetable owner's participants.
type StatisticsTestRepositories struct {
	CarePlan careplan.StudentStatusDaysQuery
}

func NewStatisticsTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
	timetable, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	persons := members.People
	carePlan, err := NewCarePlan(db, persons, timetable.InstanceStudent)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	return StatisticsTestRepositories{CarePlan: carePlan}, nil
}
