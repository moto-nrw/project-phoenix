package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/statistics"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// The Statistik report reads these facts from other capabilities; the
// composition root implements the ports over their owners.
type (
	StatisticsAccessEvent = ports.StatisticsAccessEvent
	StatisticsRoom        = ports.StatisticsRoom
	StatisticsStudent     = ports.StatisticsStudent
	HolidayPeriod         = ports.HolidayPeriod
	CourseInstance        = ports.CourseInstance
	CourseParticipation   = ports.CourseParticipation
	StatusDay             = ports.StatusDay
)

// StatisticsDependencies binds the report to its foreign inputs. Attendance
// days, room utilization and visit-retention consents are read from Student
// Presence itself over DB.
type StatisticsDependencies struct {
	DB      *bun.DB
	Observe func(Observation)

	StatusDays  ports.StatusDays
	Courses     ports.CourseStatistics
	Holidays    ports.HolidayDates
	ClosingDays ports.ClosingDayDates
	Periods     ports.HolidayPeriods
	Students    ports.StatisticsStudents
	Rooms       ports.StatisticsRooms
	AccessLog   ports.StatisticsAccessLog
	Retention   ports.StatisticsRetention
	Logger      *slog.Logger
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

// NewStatistics builds the Statistik report use case.
func NewStatistics(deps StatisticsDependencies) (studentpresence.StatisticsReports, error) {
	if deps.DB == nil || deps.Observe == nil {
		return nil, errors.New("student presence statistics compose: database and observer are required")
	}
	presence := application.New(postgres.New(databaseRuntime(deps.DB)), transaction{}, deps.Observe)
	return statistics.NewService(statistics.Config{
		Statistics:      presence,
		Attendance:      presence,
		StatusDays:      deps.StatusDays,
		Courses:         deps.Courses,
		Holidays:        deps.Holidays,
		ClosingDays:     deps.ClosingDays,
		Periods:         deps.Periods,
		Students:        deps.Students,
		Rooms:           deps.Rooms,
		AccessLog:       deps.AccessLog,
		Retention:       deps.Retention,
		PrivacyConsents: presence,
		Logger:          deps.Logger,
		Now:             deps.Now,
	}), nil
}
