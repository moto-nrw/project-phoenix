package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/statistics"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StatisticsTestModule struct {
	Statistics  statistics.Service
	ClosingDays schedule.ClosingDayService
	ListExport  *listexport.RendererService
}

func NewStatisticsTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (StatisticsTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return StatisticsTestModule{}, err
	}
	r, err := repositories.NewStatisticsTestRepositories(db, command, clocks...)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	calendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return StatisticsTestModule{}, err
	}
	students := overlappingRosterGroupNames{StudentRepository: r.Timetable.Student, groups: groups}
	closing := schedule.NewClosingDayService(r.Timetable.ClosingDay)
	service := statistics.NewService(statistics.Config{
		Statistics: statisticsRoomUtilization{newStudentPresence(db, slog.Default())}, Attendance: statisticsAttendance{newStudentPresence(db, slog.Default())}, StatusDays: statisticsStatusDays{r.CarePlan}, Courses: statisticsReportCourses{r.Timetable.Timetable},
		Holidays:    schedule.NewHolidayService(settings.Settings, schoolCalendarHolidayAdapter{query: calendar}, slog.Default()),
		ClosingDays: closing, Periods: statisticsReportPeriods{calendar}, Students: statisticsReportStudents{students}, Rooms: statisticsReportRooms{rooms},
		AccessLog: statisticsAuditLog{r.AccessLog}, Retention: statisticsRetention{settings.Settings}, PrivacyConsents: statisticsRetentionSettings{newStudentPresence(db, slog.Default())}, Logger: slog.Default(), Now: optionalClock(clocks),
	})
	return StatisticsTestModule{Statistics: service, ClosingDays: closing, ListExport: listexport.NewService()}, nil
}
