//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ScheduleTestModule bundles what the schedules API binds from the Timetable
// owner: the timeframes and recurrence rules, and the care-offering guard on
// timeframe changes.
type ScheduleTestModule struct {
	Timetable      timetable.Capability
	TimeframeGuard timetableHTTPAdapter.TimeframeChangeGuard
}

// NewScheduleTestModule composes the schedules API's Timetable side and
// returns it with the School Calendar that serves the dateframes.
func NewScheduleTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (ScheduleTestModule, schoolcalendar.Calendar, error) {
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return ScheduleTestModule{}, nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return ScheduleTestModule{}, nil, err
	}
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return ScheduleTestModule{}, nil, err
	}
	lock := recurrenceLock.LockRecurrenceWrites
	offerings, err := newTestCareOfferingCatalog(db, r, settings.Settings, CareOfferingCatalogTestOptions{
		LockRecurrence: lock, Today: timezone.TodayDate,
	})
	if err != nil {
		return ScheduleTestModule{}, nil, err
	}
	guard := TimeframeChangeGuard(lock, offerings.ValidateTimeframeChange)
	calendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		return ScheduleTestModule{}, nil, err
	}
	return ScheduleTestModule{Timetable: r.Timetable, TimeframeGuard: guard}, calendar.SchoolCalendar(), nil
}
