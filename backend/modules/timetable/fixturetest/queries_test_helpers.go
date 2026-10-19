//go:build testhooks

package fixturetest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// These fixture-only queries translate the old suites' query construction.
// Production persistence accepts only each owner's bounded native filter.
func queryError(op string, err error) error { return &domain.StoreError{Op: op, Err: err} }
func (adapter fromTimeframeRecords) List(ctx context.Context, options *QueryOptions) ([]*Timeframe, error) {
	description, limit, offset, err := TimeframeListOptions(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListTimeframes(context.Context, timetable.TimeframeFilter) ([]*records.Timeframe, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture Timeframe query is not configured for %T", adapter.value)
	}
	values, err := source.ListTimeframes(ctx, timetable.TimeframeFilter{DescriptionContains: strings.TrimSuffix(strings.TrimPrefix(description, "%"), "%"), Limit: limit, Offset: offset})
	return fromNative44(values), err
}
func (adapter fromRecurrenceRuleRecords) List(ctx context.Context, options *QueryOptions) ([]*RecurrenceRule, error) {
	frequency, frequencies, sortBy, descending, limit, offset, err := RecurrenceRuleListOptions(options)
	if err != nil {
		return nil, queryError("list", err)
	}
	source, ok := adapter.value.(interface {
		ListRecurrences(context.Context, timetable.RecurrenceRuleFilter) ([]*records.RecurrenceRule, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture RecurrenceRule query is not configured for %T", adapter.value)
	}
	values, err := source.ListRecurrences(ctx, timetable.RecurrenceRuleFilter{Frequency: frequency, Frequencies: frequencies, SortBy: sortBy, SortDescending: descending, Limit: limit, Offset: offset})
	return fromNative37(values), err
}
func (adapter fromActivityExceptionRecords) List(ctx context.Context, options *QueryOptions) ([]*ActivityException, error) {
	groupID, limit, offset, err := ActivityExceptionListOptions(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListExceptions(context.Context, timetable.ActivityExceptionFilter) ([]*records.ActivityException, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture ActivityException query is not configured for %T", adapter.value)
	}
	values, err := source.ListExceptions(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: groupID, Limit: limit, Offset: offset})
	return fromNative1(values), err
}
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
func (adapter fromInstanceStaffRecords) List(ctx context.Context, options *QueryOptions) ([]*InstanceStaff, error) {
	filter, err := InstanceStaffListOptions(options)
	if err != nil {
		return nil, queryError("list with options", err)
	}
	source, ok := adapter.value.(interface {
		ListStaff(context.Context, timetable.InstanceStaffFilter) ([]*records.InstanceStaff, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture InstanceStaff query is not configured for %T", adapter.value)
	}
	values, err := source.ListStaff(ctx, filter)
	return fromNative6(values), err
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

// Listing remains optional: embedding an unconfigured base fake must not
// silently gain a capability that dereferences its nil embedded interface.
type fromListedActivityInstances struct {
	fromActivityInstanceRecords
	listing ports.InstanceListing
}

func (adapter fromListedActivityInstances) ListInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*ActivityInstance, error) {
	values, err := adapter.listing.ListInstances(ctx, filter)
	return fromNative3(values), err
}

type intoListedActivityInstances struct {
	intoActivityInstanceRecords
	listing InstanceListing
}

func (adapter intoListedActivityInstances) ListInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*records.ActivityInstance, error) {
	values, err := adapter.listing.ListInstances(ctx, filter)
	if values == nil {
		return nil, err
	}
	result := make([]*records.ActivityInstance, len(values))
	for i, value := range values {
		if value != nil {
			result[i] = &value.ActivityInstance
		}
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
