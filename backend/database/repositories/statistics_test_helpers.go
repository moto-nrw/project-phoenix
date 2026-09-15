package repositories

import (
	"time"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

type StatisticsTestRepositories struct {
	Timetable TimetableTestRepositories
	CarePlan  careplan.StudentStatusDaysQuery
	AccessLog auditModels.DataAccessLogRepository
}

func NewStatisticsTestRepositories(db *bun.DB, command auditModels.Command, clocks ...func() time.Time) (StatisticsTestRepositories, error) {
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
		AccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command},
	}, nil
}
