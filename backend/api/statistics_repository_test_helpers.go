package api

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// StatisticsTestRepositories are the statistics suites' repositories;
// AccessLog is filled by NewStatisticsTestRepositories.
type StatisticsTestRepositories struct {
	Timetable TimetableTestRepositories
	CarePlan  careplan.StudentStatusDaysQuery
	AccessLog auditModels.DataAccessLogRepository
}

func NewStatisticsTestRepositoriesWithMembers(db transactionruntime.Database, members services.MembershipTestRepositories, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
	timetable, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	persons := members.People
	carePlan, err := NewCarePlan(db, persons, timetable.InstanceStudent)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	return StatisticsTestRepositories{
		Timetable: timetable, CarePlan: carePlan,
	}, nil
}
