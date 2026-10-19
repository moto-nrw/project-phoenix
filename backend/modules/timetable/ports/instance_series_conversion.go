package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TemplateSeriesSeeding is the slice of the template writes the series
// conversion seeds a series with.
type TemplateSeriesSeeding interface {
	CreateTemplate(ctx context.Context, cmd timetable.CreateTemplateCommand) (*timetable.CreateTemplateResult, error)
	TemplateAssignmentsOn(ctx context.Context, templateID int64, date calendar.Date, calendarPeriodID int64) (timetable.TemplateAssignments, error)
	AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from calendar.Date) error
}

// InstanceSeriesConversionDependencies are the capabilities the conversion
// coordinates; it adds transactionality and ordering, not persistence rules.
type InstanceSeriesConversionDependencies struct {
	Transactions   TenantTransactions
	InstanceRepo   ActivityInstanceRecords
	Lifecycle      timetable.InstancePlanning
	Templates      TemplateSeriesSeeding
	RecurrenceLock timetable.RecurrenceWriteLock
}
