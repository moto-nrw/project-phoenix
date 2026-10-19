package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The retained models/schedule student-schedule repositories over the Care
// Plan records (#3351). They perform no persistence of their own: every
// adapter maps the row models onto the records and keeps the error shapes the
// retained callers classify on.

type CareArrivalScheduleRows struct{ rows ports.CarePlanRows }
type CareArrivalExceptionRows struct{ rows ports.CarePlanRows }
type CarePickupScheduleRows struct{ rows ports.CarePlanRows }
type CarePickupExceptionRows struct{ rows ports.CarePlanRows }

func NewCareArrivalScheduleRows(rows ports.CarePlanRows) CareArrivalScheduleRows {
	return CareArrivalScheduleRows{rows: rows}
}
func NewCareArrivalExceptionRows(rows ports.CarePlanRows) CareArrivalExceptionRows {
	return CareArrivalExceptionRows{rows: rows}
}
func NewCarePickupScheduleRows(rows ports.CarePlanRows) CarePickupScheduleRows {
	return CarePickupScheduleRows{rows: rows}
}
func NewCarePickupExceptionRows(rows ports.CarePlanRows) CarePickupExceptionRows {
	return CarePickupExceptionRows{rows: rows}
}

func invalidLegacyEntity(name string) error {
	return fmt.Errorf("%s cannot be nil or zero value", name)
}
func legacyScheduleID(raw any) (int64, error) {
	switch value := raw.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}

// careScheduleListFilter reads the retained listing options: the page, the
// sort order and one student_id equality, the shape the retained callers
// list with. Any other condition is refused rather than dropped.
func careScheduleListFilter(options *domain.ListOptions) (ports.CareScheduleFilter, error) {
	return narrowListOptions(options, "care schedule",
		func(f *ports.CareScheduleFilter, limit, offset int) { f.Limit, f.Offset = limit, offset },
		func(f *ports.CareScheduleFilter, field domain.ListSortField) {
			f.Sort = append(f.Sort, ports.CareScheduleSort{Field: field.Field, Descending: field.Direction == domain.ListSortDesc})
		},
		func(f *ports.CareScheduleFilter, condition domain.ListCondition) error {
			id, ok := condition.Value.(int64)
			if condition.Field != "student_id" || condition.Operator != domain.ListOpEqual || !ok || f.StudentIDs != nil {
				return fmt.Errorf("unsupported care schedule filter %s %s", condition.Field, condition.Operator)
			}
			f.StudentIDs = []int64{id}
			return nil
		})
}

// listWithOptions serves a retained List over one of the owner's listings.
func listWithOptions[T, R any](ctx context.Context, rows ports.CarePlanRows, options *domain.ListOptions, list func(context.Context, ports.CareScheduleFilter) ([]T, error), convert func(T) R) ([]R, error) {
	filter, err := careScheduleListFilter(options)
	var values []T
	if err == nil {
		values, err = list(ctx, filter)
	}
	if err != nil {
		return nil, rows.ScheduleError("list with options", err)
	}
	return mapRows(values, convert), nil
}

func mapRows[T any, R any](values []T, convert func(T) R) []R {
	result := make([]R, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func arrivalScheduleToPublic(v *scheduleModels.StudentArrivalSchedule) ports.CareArrivalSchedule {
	return ports.CareArrivalSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, ExpectedArrival: v.ExpectedArrival, Notes: v.Notes, CreatedBy: v.CreatedBy}
}
func arrivalScheduleToLegacy(v ports.CareArrivalSchedule) *scheduleModels.StudentArrivalSchedule {
	row := new(scheduleModels.StudentArrivalSchedule)
	applyArrivalSchedule(row, v)
	return row
}
func applyArrivalSchedule(r *scheduleModels.StudentArrivalSchedule, v ports.CareArrivalSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.ExpectedArrival, r.Notes, r.CreatedBy = v.StudentID, v.Weekday, v.ExpectedArrival, v.Notes, v.CreatedBy
}
func arrivalExceptionToPublic(v *scheduleModels.StudentArrivalException) ports.CareArrivalException {
	return ports.CareArrivalException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: ports.CareDate(v.ExceptionDate), ExpectedArrival: v.ExpectedArrival, Reason: v.Reason, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func arrivalExceptionToLegacy(v ports.CareArrivalException) *scheduleModels.StudentArrivalException {
	row := new(scheduleModels.StudentArrivalException)
	applyArrivalException(row, v)
	return row
}
func applyArrivalException(r *scheduleModels.StudentArrivalException, v ports.CareArrivalException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.ExpectedArrival, r.Reason, r.Source, r.CreatedBy, r.CreatedByGuardian = v.StudentID, calendar.Date(v.ExceptionDate), v.ExpectedArrival, v.Reason, v.Source, v.CreatedBy, v.CreatedByGuardian
}
func pickupScheduleToPublic(v *scheduleModels.StudentPickupSchedule) ports.CarePickupSchedule {
	return ports.CarePickupSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, PickupTime: v.PickupTime, Notes: v.Notes, CreatedBy: v.CreatedBy, Source: v.Source, CareOfferingID: v.CareOfferingID}
}
func pickupScheduleToLegacy(v ports.CarePickupSchedule) *scheduleModels.StudentPickupSchedule {
	row := new(scheduleModels.StudentPickupSchedule)
	applyPickupSchedule(row, v)
	return row
}
func applyPickupSchedule(r *scheduleModels.StudentPickupSchedule, v ports.CarePickupSchedule) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.Weekday, r.PickupTime, r.Notes, r.CreatedBy, r.Source, r.CareOfferingID = v.StudentID, v.Weekday, v.PickupTime, v.Notes, v.CreatedBy, v.Source, v.CareOfferingID
}
func pickupExceptionToPublic(v *scheduleModels.StudentPickupException) ports.CarePickupException {
	return ports.CarePickupException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: ports.CareDate(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupExceptionToLegacy(v ports.CarePickupException) *scheduleModels.StudentPickupException {
	row := new(scheduleModels.StudentPickupException)
	applyPickupException(row, v)
	return row
}
func applyPickupException(r *scheduleModels.StudentPickupException, v ports.CarePickupException) {
	r.ID, r.TenantID, r.CreatedAt, r.UpdatedAt = v.ID, v.TenantID, v.CreatedAt, v.UpdatedAt
	r.StudentID, r.ExceptionDate, r.PickupTime, r.Reason = v.StudentID, calendar.Date(v.ExceptionDate), v.PickupTime, v.Reason
	r.ExcusedFrom, r.ExcusedReason, r.ExcusedCreatedBy, r.ExcusedOwnsPickupTime, r.ExcusedAuto = v.ExcusedFrom, v.ExcusedReason, v.ExcusedCreatedBy, v.ExcusedOwnsPickupTime, v.ExcusedAuto
	r.Source, r.CreatedBy, r.CreatedByGuardian = v.Source, v.CreatedBy, v.CreatedByGuardian
}

var (
	_ ports.StudentArrivalScheduleRecords  = CareArrivalScheduleRows{}
	_ ports.StudentArrivalExceptionRecords = CareArrivalExceptionRows{}
	_ ports.StudentPickupScheduleRecords   = CarePickupScheduleRows{}
	_ ports.StudentPickupExceptionRecords  = CarePickupExceptionRows{}
)
