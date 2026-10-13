package services

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

// StatisticsTestRepositories are the statistics suites' repositories;
// AccessLog is filled by services.NewStatisticsTestRepositories.
type StatisticsTestRepositories struct {
	Timetable TimetableTestRepositories
	CarePlan  careplan.StudentStatusDaysQuery
	AccessLog auditModels.DataAccessLogRepository
}

func NewStatisticsTestRepositoriesWithMembers(db *bun.DB, members MembershipTestRepositories, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
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
