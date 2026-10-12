package repositories

import (
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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

func NewStatisticsTestRepositories(db *bun.DB, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
	timetable, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	carePlan, err := NewCarePlan(db, persons, timetable.InstanceStudent)
	if err != nil {
		return StatisticsTestRepositories{}, err
	}
	return StatisticsTestRepositories{
		Timetable: timetable, CarePlan: carePlan,
	}, nil
}
