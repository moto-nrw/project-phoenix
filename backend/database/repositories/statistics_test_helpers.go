package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type StatisticsTestRepositories struct {
	Timetable  TimetableTestRepositories
	Statistics activeModels.StatisticsRepository
	Courses    scheduleModels.CourseStatisticsRepository
	AccessLog  auditModels.DataAccessLogRepository
	Privacy    usersModels.PrivacyConsentRepository
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
	stats := activeRepo.NewStatisticsRepository(db)
	stats.(*activeRepo.StatisticsRepository).BindCarePlan(statisticsCarePlanDirectory{query: carePlan})
	return StatisticsTestRepositories{
		Timetable: timetable, Statistics: stats, Courses: scheduleRepo.NewCourseStatisticsRepository(db),
		AccessLog: dataAccessLogCommand{auditRepo.NewDataAccessLogRepository(newTestAuditRuntime(db)), command},
		Privacy:   activeRepo.NewPrivacyConsentRepository(db),
	}, nil
}
