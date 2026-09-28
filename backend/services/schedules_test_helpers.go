package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ScheduleTestModule bundles what the schedules API binds: the School
// Calendar for dateframes, the Timetable owner for timeframes and
// recurrence rules, and the care-offering guard on timeframe changes.
type ScheduleTestModule struct {
	Calendar       schoolcalendar.Calendar
	Timetable      timetable.Capability
	TimeframeGuard timetableHTTPAdapter.TimeframeChangeGuard
}

func NewScheduleTestModule(db *bun.DB, unit tenant.UnitOfWork) (ScheduleTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return ScheduleTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return ScheduleTestModule{}, err
	}
	recurrenceLock, err := repositories.NewTimetableRecurrenceLock(db)
	if err != nil {
		return ScheduleTestModule{}, err
	}
	lock := recurrenceLock.LockRecurrenceWrites
	offerings, err := newTestCareOfferingCatalog(r, settings.Settings, CareOfferingCatalogTestOptions{
		LockRecurrence: lock, Today: timezone.TodayDate,
	})
	if err != nil {
		return ScheduleTestModule{}, err
	}
	guard := TimeframeChangeGuard(lock, offerings.ValidateTimeframeChange)
	return ScheduleTestModule{Calendar: r.SchoolCalendar(), Timetable: r.Timetable, TimeframeGuard: guard}, nil
}
