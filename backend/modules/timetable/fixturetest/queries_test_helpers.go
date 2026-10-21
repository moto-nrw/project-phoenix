//go:build testhooks

package fixturetest

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// These fixture-only queries translate the old suites' query construction.
// Production persistence accepts only each owner's bounded native filter.
func queryError(op string, err error) error { return &domain.StoreError{Op: op, Err: err} }
func (adapter fromActivityInstanceRecords) List(ctx context.Context, options *QueryOptions) ([]*ActivityInstance, error) {
	filter, err := ActivityInstanceListOptions(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListInstances(context.Context, ports.LegacyInstanceFilter) ([]*records.ActivityInstance, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture ActivityInstance query is not configured for %T", adapter.value)
	}
	values, err := source.ListInstances(ctx, filter)
	return fromNative3(values), err
}
func (adapter fromInstanceStudentRecords) List(ctx context.Context, options *QueryOptions) ([]*InstanceStudent, error) {
	filter, err := InstanceStudentListOptions(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListStudents(context.Context, ports.ParticipantListingFilter) ([]*records.InstanceStudent, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture InstanceStudent query is not configured for %T", adapter.value)
	}
	values, err := source.ListStudents(ctx, filter)
	return fromNative26(values), err
}
func (adapter fromStudentArrivalScheduleRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentArrivalSchedule, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentArrivalSchedule, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentArrivalSchedule query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative15(values), err
}
func (adapter fromStudentArrivalExceptionRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentArrivalException, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentArrivalException, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentArrivalException query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative13(values), err
}
func (adapter fromStudentArrivalNoteRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentArrivalNote, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentArrivalNote, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentArrivalNote query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative42(values), err
}
func (adapter fromStudentPickupScheduleRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentPickupSchedule, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentPickupSchedule, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentPickupSchedule query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative21(values), err
}
func (adapter fromStudentPickupExceptionRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentPickupException, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentPickupException, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentPickupException query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative17(values), err
}
func (adapter fromStudentPickupNoteRecords) List(ctx context.Context, options *QueryOptions) ([]*StudentPickupNote, error) {
	filter, err := careScheduleListFilter(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListRows(context.Context, ports.CareScheduleFilter) ([]*records.StudentPickupNote, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture StudentPickupNote query is not configured for %T", adapter.value)
	}
	values, err := source.ListRows(ctx, filter)
	return fromNative19(values), err
}
func (adapter fromCalendarPeriodRecords) List(ctx context.Context, options *QueryOptions) ([]*CalendarPeriod, error) {
	return (UnfilteredListing[*CalendarPeriod]{Source: adapter.FindByTenantID}).List(ctx, options)
}

func (adapter fromDateframeRecords) List(ctx context.Context, options *QueryOptions) ([]*Dateframe, error) {
	listing, err := dateframeListingFromOptions(options)
	if err != nil {
		return nil, queryError("list", err)
	}
	filter := schoolcalendar.DateframeFilter{Name: listing.Name, NamePattern: listing.NamePattern, Limit: listing.Limit, Offset: listing.Offset}
	for _, field := range listing.Sort {
		filter.Sort = append(filter.Sort, schoolcalendar.DateframeSort{Field: field.Field, Descending: field.Descending})
	}
	source, ok := adapter.value.(interface {
		ListDateframes(context.Context, schoolcalendar.DateframeFilter) ([]*records.Dateframe, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture dateframe query is not configured for %T", adapter.value)
	}
	values, err := source.ListDateframes(ctx, filter)
	if values == nil {
		return nil, err
	}
	result := make([]*Dateframe, len(values))
	for i, value := range values {
		result[i] = &Dateframe{*value}
	}
	return result, err
}

// FindPartialAbsenceBlocks exposes the native preview only for its fixture suite.
func (adapter fromInstanceStudentRecords) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date calendar.Date, cutoff time.Time) ([]PartialAbsenceBlock, error) {
	source, ok := adapter.value.(interface {
		FindPartialAbsenceBlocks(context.Context, int64, calendar.Date, time.Time) ([]records.PartialAbsenceBlock, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture partial absence preview is not configured for %T", adapter.value)
	}
	values, err := source.FindPartialAbsenceBlocks(ctx, studentID, date, cutoff)
	if values == nil {
		return nil, err
	}
	result := make([]PartialAbsenceBlock, len(values))
	for i, value := range values {
		result[i] = PartialAbsenceBlock{value}
	}
	return result, err
}

// ListInstances reads the bounded instance selection of a fixture.
func (adapter fromActivityInstanceRecords) ListInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*ActivityInstance, error) {
	values, err := adapter.value.ListInstances(ctx, filter)
	return fromNative3(values), err
}
