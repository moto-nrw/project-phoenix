package owners

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// NewCarePlanRows binds the Timetable composition's retained schedule and
// care-request rows to the Care Plan capability (#3351): the records in that
// composition's vocabulary, the retained People Directory error shape and
// Care Plan's today.
func NewCarePlanRows(capability careplan.Capability) timetablePorts.CarePlanRows {
	return timetablePorts.CarePlanRows{
		Records: carePlanRecords{capability: capability},
		Errors:  transactionruntime.RecordErrors{},
		Today:   func() timetablePorts.CareDate { return timetablePorts.CareDate(careplan.Today()) },
	}
}

// carePlanRecords translates between the Timetable composition's vocabulary
// and Care Plan's.
type carePlanRecords struct{ capability careplan.Capability }

// carePlanScheduleError marks Care Plan's missing schedule row for the rows.
func carePlanScheduleError(err error) error {
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return timetablePorts.MarkRecordError(timetablePorts.ErrCareScheduleRecordNotFound, err)
	}
	return err
}

// carePlanRequestError marks Care Plan's request answers for the rows.
func carePlanRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrCareScheduleRequestNotFound):
		return timetablePorts.MarkRecordError(timetablePorts.ErrCareRequestRecordNotFound, err)
	case errors.Is(err, careplan.ErrCareScheduleRequestNotPending):
		return timetablePorts.MarkRecordError(timetablePorts.ErrCareRequestRecordNotPending, err)
	case errors.Is(err, careplan.ErrCareScheduleRequestNotDecided):
		return timetablePorts.MarkRecordError(timetablePorts.ErrCareRequestRecordNotDecided, err)
	default:
		return err
	}
}

func carePlanScheduleFilter(filter timetablePorts.CareScheduleFilter) careplan.StudentScheduleFilter {
	return careplan.StudentScheduleFilter{
		StudentIDs: filter.StudentIDs, Weekday: filter.Weekday,
		Date: careplan.Date(filter.Date), From: careplan.Date(filter.From), To: careplan.Date(filter.To),
		UpcomingFrom: careplan.Date(filter.UpcomingFrom),
		Options:      carePlanSchedulePage(filter),
	}
}

// carePlanSchedulePage pages and orders a retained listing; a filter without
// either keeps the owner's order and every row.
func carePlanSchedulePage(filter timetablePorts.CareScheduleFilter) *careplan.StudentScheduleQueryOptions {
	if filter.Limit == 0 && filter.Offset == 0 && len(filter.Sort) == 0 {
		return nil
	}
	options := &careplan.StudentScheduleQueryOptions{Limit: filter.Limit, Offset: filter.Offset}
	for _, field := range filter.Sort {
		options.Sorting = append(options.Sorting, careplan.StudentScheduleSortField(field))
	}
	return options
}

func carePlanSlice[T any, R any](values []T, convert func(T) R) []R {
	result := make([]R, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func careArrivalScheduleToRows(value careplan.ArrivalSchedule) timetablePorts.CareArrivalSchedule {
	return timetablePorts.CareArrivalSchedule(value)
}

func careArrivalScheduleFromRows(value timetablePorts.CareArrivalSchedule) careplan.ArrivalSchedule {
	return careplan.ArrivalSchedule(value)
}

func CarePickupScheduleToRows(value careplan.PickupSchedule) timetablePorts.CarePickupSchedule {
	return timetablePorts.CarePickupSchedule(value)
}

func CarePickupScheduleFromRows(value timetablePorts.CarePickupSchedule) careplan.PickupSchedule {
	return careplan.PickupSchedule(value)
}

func CareArrivalExceptionToRows(value careplan.ArrivalException) timetablePorts.CareArrivalException {
	return timetablePorts.CareArrivalException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, ExpectedArrival: value.ExpectedArrival, Reason: value.Reason, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: timetablePorts.CareDate(value.ExceptionDate)}
}

func CareArrivalExceptionFromRows(value timetablePorts.CareArrivalException) careplan.ArrivalException {
	return careplan.ArrivalException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, ExpectedArrival: value.ExpectedArrival, Reason: value.Reason, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: careplan.Date(value.ExceptionDate)}
}

func CarePickupExceptionToRows(value careplan.PickupException) timetablePorts.CarePickupException {
	return timetablePorts.CarePickupException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, PickupTime: value.PickupTime, Reason: value.Reason, ExcusedFrom: value.ExcusedFrom, ExcusedReason: value.ExcusedReason, ExcusedCreatedBy: value.ExcusedCreatedBy, ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime, ExcusedAuto: value.ExcusedAuto, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: timetablePorts.CareDate(value.ExceptionDate)}
}

func CarePickupExceptionFromRows(value timetablePorts.CarePickupException) careplan.PickupException {
	return careplan.PickupException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, PickupTime: value.PickupTime, Reason: value.Reason, ExcusedFrom: value.ExcusedFrom, ExcusedReason: value.ExcusedReason, ExcusedCreatedBy: value.ExcusedCreatedBy, ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime, ExcusedAuto: value.ExcusedAuto, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: careplan.Date(value.ExceptionDate)}
}

func careArrivalNoteToRows(value careplan.ArrivalNote) timetablePorts.CareArrivalNote {
	return timetablePorts.CareArrivalNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: timetablePorts.CareDate(value.NoteDate)}
}

func careArrivalNoteFromRows(value timetablePorts.CareArrivalNote) careplan.ArrivalNote {
	return careplan.ArrivalNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: careplan.Date(value.NoteDate)}
}

func carePickupNoteToRows(value careplan.PickupNote) timetablePorts.CarePickupNote {
	return timetablePorts.CarePickupNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Weekday: value.Weekday, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: timetablePorts.CareDate(value.NoteDate)}
}

func carePickupNoteFromRows(value timetablePorts.CarePickupNote) careplan.PickupNote {
	return careplan.PickupNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Weekday: value.Weekday, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: careplan.Date(value.NoteDate)}
}

func (r carePlanRecords) CreateArrivalSchedule(ctx context.Context, value timetablePorts.CareArrivalSchedule) (timetablePorts.CareArrivalSchedule, error) {
	created, err := r.capability.CreateArrivalSchedule(ctx, careArrivalScheduleFromRows(value))
	return careArrivalScheduleToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindArrivalSchedule(ctx context.Context, id int64) (timetablePorts.CareArrivalSchedule, error) {
	value, err := r.capability.FindArrivalSchedule(ctx, id)
	return careArrivalScheduleToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdateArrivalSchedule(ctx context.Context, value timetablePorts.CareArrivalSchedule) error {
	return carePlanScheduleError(r.capability.UpdateArrivalSchedule(ctx, careArrivalScheduleFromRows(value)))
}

func (r carePlanRecords) DeleteArrivalSchedulesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalSchedulesByStudent(ctx, studentID))
}

func (r carePlanRecords) ListArrivalSchedules(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CareArrivalSchedule, error) {
	values, err := r.capability.ListArrivalSchedules(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, careArrivalScheduleToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) UpsertArrivalSchedule(ctx context.Context, value timetablePorts.CareArrivalSchedule) (timetablePorts.CareArrivalSchedule, error) {
	upserted, err := r.capability.UpsertArrivalSchedule(ctx, careArrivalScheduleFromRows(value))
	return careArrivalScheduleToRows(upserted), carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupSchedule(ctx context.Context, value timetablePorts.CarePickupSchedule) (timetablePorts.CarePickupSchedule, error) {
	created, err := r.capability.CreatePickupSchedule(ctx, CarePickupScheduleFromRows(value))
	return CarePickupScheduleToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindPickupSchedule(ctx context.Context, id int64) (timetablePorts.CarePickupSchedule, error) {
	value, err := r.capability.FindPickupSchedule(ctx, id)
	return CarePickupScheduleToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdatePickupSchedule(ctx context.Context, value timetablePorts.CarePickupSchedule) error {
	return carePlanScheduleError(r.capability.UpdatePickupSchedule(ctx, CarePickupScheduleFromRows(value)))
}

func (r carePlanRecords) DeletePickupSchedulesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupSchedulesByStudent(ctx, studentID))
}

func (r carePlanRecords) ListPickupSchedules(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CarePickupSchedule, error) {
	values, err := r.capability.ListPickupSchedules(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, CarePickupScheduleToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) UpsertPickupSchedule(ctx context.Context, value timetablePorts.CarePickupSchedule) (timetablePorts.CarePickupSchedule, error) {
	upserted, err := r.capability.UpsertPickupSchedule(ctx, CarePickupScheduleFromRows(value))
	return CarePickupScheduleToRows(upserted), carePlanScheduleError(err)
}

func (r carePlanRecords) CreateArrivalException(ctx context.Context, value timetablePorts.CareArrivalException) (timetablePorts.CareArrivalException, error) {
	created, err := r.capability.CreateArrivalException(ctx, CareArrivalExceptionFromRows(value))
	return CareArrivalExceptionToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindArrivalException(ctx context.Context, id int64) (timetablePorts.CareArrivalException, error) {
	value, err := r.capability.FindArrivalException(ctx, id, false)
	return CareArrivalExceptionToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdateArrivalException(ctx context.Context, value timetablePorts.CareArrivalException) error {
	return carePlanScheduleError(r.capability.UpdateArrivalException(ctx, CareArrivalExceptionFromRows(value)))
}

func (r carePlanRecords) DeleteArrivalExceptionsByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalExceptionsByStudent(ctx, studentID))
}

func (r carePlanRecords) ListArrivalExceptions(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CareArrivalException, error) {
	values, err := r.capability.ListArrivalExceptions(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, CareArrivalExceptionToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) DeleteArrivalExceptionsBefore(ctx context.Context, before timetablePorts.CareDate) (int64, error) {
	rows, err := r.capability.DeleteArrivalExceptionsBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupException(ctx context.Context, value timetablePorts.CarePickupException) (timetablePorts.CarePickupException, error) {
	created, err := r.capability.CreatePickupException(ctx, CarePickupExceptionFromRows(value))
	return CarePickupExceptionToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindPickupException(ctx context.Context, id int64) (timetablePorts.CarePickupException, error) {
	value, err := r.capability.FindPickupException(ctx, id, false)
	return CarePickupExceptionToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdatePickupException(ctx context.Context, value timetablePorts.CarePickupException) error {
	return carePlanScheduleError(r.capability.UpdatePickupException(ctx, CarePickupExceptionFromRows(value)))
}

func (r carePlanRecords) DeletePickupExceptionsByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupExceptionsByStudent(ctx, studentID))
}

func (r carePlanRecords) ListPickupExceptions(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CarePickupException, error) {
	values, err := r.capability.ListPickupExceptions(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, CarePickupExceptionToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) DeletePickupExceptionsBefore(ctx context.Context, before timetablePorts.CareDate) (int64, error) {
	rows, err := r.capability.DeletePickupExceptionsBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreateArrivalNote(ctx context.Context, value timetablePorts.CareArrivalNote) (timetablePorts.CareArrivalNote, error) {
	created, err := r.capability.CreateArrivalNote(ctx, careArrivalNoteFromRows(value))
	return careArrivalNoteToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindArrivalNote(ctx context.Context, id int64) (timetablePorts.CareArrivalNote, error) {
	value, err := r.capability.FindArrivalNote(ctx, id)
	return careArrivalNoteToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdateArrivalNote(ctx context.Context, value timetablePorts.CareArrivalNote) error {
	return carePlanScheduleError(r.capability.UpdateArrivalNote(ctx, careArrivalNoteFromRows(value)))
}

func (r carePlanRecords) DeleteArrivalNotesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalNotesByStudent(ctx, studentID))
}

func (r carePlanRecords) ListArrivalNotes(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CareArrivalNote, error) {
	values, err := r.capability.ListArrivalNotes(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, careArrivalNoteToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) DeleteArrivalNotesBefore(ctx context.Context, before timetablePorts.CareDate) (int64, error) {
	rows, err := r.capability.DeleteArrivalNotesBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupNote(ctx context.Context, value timetablePorts.CarePickupNote) (timetablePorts.CarePickupNote, error) {
	created, err := r.capability.CreatePickupNote(ctx, carePickupNoteFromRows(value))
	return carePickupNoteToRows(created), carePlanScheduleError(err)
}

func (r carePlanRecords) FindPickupNote(ctx context.Context, id int64) (timetablePorts.CarePickupNote, error) {
	value, err := r.capability.FindPickupNote(ctx, id)
	return carePickupNoteToRows(value), carePlanScheduleError(err)
}

func (r carePlanRecords) UpdatePickupNote(ctx context.Context, value timetablePorts.CarePickupNote) error {
	return carePlanScheduleError(r.capability.UpdatePickupNote(ctx, carePickupNoteFromRows(value)))
}

func (r carePlanRecords) DeletePickupNotesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupNotesByStudent(ctx, studentID))
}

func (r carePlanRecords) ListPickupNotes(ctx context.Context, filter timetablePorts.CareScheduleFilter) ([]timetablePorts.CarePickupNote, error) {
	values, err := r.capability.ListPickupNotes(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, carePickupNoteToRows), carePlanScheduleError(err)
}

func (r carePlanRecords) DeletePickupNotesBefore(ctx context.Context, before timetablePorts.CareDate) (int64, error) {
	rows, err := r.capability.DeletePickupNotesBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreateCareScheduleRequest(ctx context.Context, value timetablePorts.CareScheduleRequest) (timetablePorts.CareScheduleRequest, error) {
	created, err := r.capability.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest(value))
	return timetablePorts.CareScheduleRequest(created), carePlanRequestError(err)
}

func (r carePlanRecords) FindCareScheduleRequest(ctx context.Context, id int64) (timetablePorts.CareScheduleRequest, error) {
	value, err := r.capability.FindCareScheduleRequest(ctx, id, false)
	return timetablePorts.CareScheduleRequest(value), carePlanRequestError(err)
}

func (r carePlanRecords) ListCareScheduleRequests(ctx context.Context, filter timetablePorts.CareScheduleRequestFilter) ([]timetablePorts.CareScheduleRequest, error) {
	ownerFilter := careplan.CareScheduleRequestFilter{StudentID: filter.StudentID, RequestKinds: filter.RequestKinds, Statuses: filter.Statuses}
	values, err := r.capability.ListCareScheduleRequests(ctx, ownerFilter)
	return carePlanSlice(values, func(value careplan.CareScheduleChangeRequest) timetablePorts.CareScheduleRequest {
		return timetablePorts.CareScheduleRequest(value)
	}), carePlanRequestError(err)
}

func (r carePlanRecords) DecideCareScheduleRequest(ctx context.Context, decision timetablePorts.CareScheduleRequestDecision) error {
	return carePlanRequestError(r.capability.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision(decision)))
}

func LegacyScheduleID(raw any) (int64, error) {
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

// legacyScheduleQueryOptions translates the retained repository query shape
// into the owner's schedule query options.
func legacyScheduleQueryOptions(options *peoplePorts.QueryOptions) *careplan.StudentScheduleQueryOptions {
	if options == nil {
		return nil
	}
	result := &careplan.StudentScheduleQueryOptions{}
	if options.Filter != nil {
		result.Filter = legacyScheduleQueryFilter(*options.Filter)
	}
	if options.Pagination != nil {
		result.Limit = options.Pagination.PageSize
		result.Offset = options.Pagination.Offset()
	}
	if options.Sorting != nil {
		result.Sorting = make([]careplan.StudentScheduleSortField, 0, len(options.Sorting.Fields))
		for _, field := range options.Sorting.Fields {
			result.Sorting = append(result.Sorting, careplan.StudentScheduleSortField{Field: field.Field, Descending: field.Direction == peoplePorts.SortDesc})
		}
	}
	return result
}

func legacyScheduleQueryFilter(filter peoplePorts.QueryFilter) *careplan.StudentScheduleQueryFilter {
	result := &careplan.StudentScheduleQueryFilter{}
	for _, condition := range filter.Conditions() {
		result.Conditions = append(result.Conditions, careplan.StudentScheduleQueryCondition{Field: condition.Field, Operator: string(condition.Operator), Value: condition.Value})
	}
	for _, child := range filter.OrFilters() {
		result.Or = append(result.Or, *legacyScheduleQueryFilter(child))
	}
	for _, child := range filter.AndFilters() {
		result.And = append(result.And, *legacyScheduleQueryFilter(child))
	}
	return result
}

// noRowsError is the bare not-found sentinel pair without an operation.
func noRowsError() error { return errors.Unwrap(RetainedNotFound("")) }

// retainedNotFound is the retained repository's not-found error for op.
func RetainedNotFound(op string) error {
	return transactionruntime.RecordErrors{}.MissingRecordError(op)
}

// WrapRetainedError preserves the retained repository error shape.
func WrapRetainedError(op string, err error) error {
	return transactionruntime.RecordErrors{}.WrapRecordError(op, err)
}
