//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The root binds the School Calendar value for suites that cannot name that owner.
func NewCalendarPeriodFixture() *schoolcalendar.CalendarPeriod {
	return &schoolcalendar.CalendarPeriod{PeriodType: schoolcalendar.PeriodTypeCustom}
}
func CreateCalendarPeriodTestRecord(ctx context.Context, db transactionruntime.Database, row *schoolcalendar.CalendarPeriod) error {
	owner, err := NewSchoolCalendar(db)
	if err != nil {
		return err
	}
	value, err := owner.CreateCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{Name: row.Name, PeriodType: row.PeriodType, StartDate: row.StartDate, EndDate: row.EndDate, WeekCycleLength: row.WeekCycleLength, WeekCycleAnchor: row.WeekCycleAnchor, IsActive: row.IsActive}})
	if err == nil {
		*row = value
	}
	return err
}
func CreateCareOfferingCalendarPeriod(check func(error), db transactionruntime.Database, ctx context.Context, tenantID int64, name string, start, end calendar.Date) *schoolcalendar.CalendarPeriod {
	row := NewCalendarPeriodFixture()
	row.TenantID = tenantID
	row.Name = name
	row.StartDate = start.String()
	row.EndDate = end.String()
	row.WeekCycleLength = 1
	row.IsActive = true
	check(CreateCalendarPeriodTestRecord(ctx, db, row))
	return row
}
func CalendarPeriodReplacementOf(period schoolcalendar.CalendarPeriod) *careplan.CalendarPeriodReplacement {
	return &careplan.CalendarPeriodReplacement{StartDate: calendar.Date(period.StartDate), EndDate: calendar.Date(period.EndDate), IsActive: period.IsActive, WeekCycleLength: period.WeekCycleLength, WeekCycleAnchor: period.WeekCycleAnchor}
}
func UpsertClassArrivalTestRecord(ctx context.Context, db transactionruntime.Database, row *careplan.ClassArrivalException) error {
	owner, err := timetableCompose.NewClassArrivals(db.Pool(), func(timetableCompose.Observation) {})
	if err != nil {
		return err
	}
	value, err := owner.UpsertClassArrivalException(ctx, timetable.ClassArrivalExceptionInput{SchoolClass: row.SchoolClass, Date: row.Date.String(), ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin})
	if err == nil {
		row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	}
	return err
}
