package services

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewCarePlanRows binds the Timetable composition's retained schedule and
// care-request rows to the Care Plan capability (#3351): the records in that
// composition's vocabulary, the retained People Directory error shape and
// Care Plan's today.
func NewCarePlanRows(capability careplan.Capability) timetableCompose.CarePlanRows {
	return timetableCompose.CarePlanRows{
		Records: carePlanRecords{capability: capability},
		Errors:  peopleCompose.RetainedRecordErrors{},
		Today:   func() timetableCompose.CareDate { return timetableCompose.CareDate(carePlanCompose.Today()) },
	}
}

// carePlanRecords translates between the Timetable composition's vocabulary
// and Care Plan's.
type carePlanRecords struct{ capability careplan.Capability }

// carePlanScheduleError marks Care Plan's missing schedule row for the rows.
func carePlanScheduleError(err error) error {
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return timetableCompose.MarkRecordError(timetableCompose.ErrCareScheduleRecordNotFound, err)
	}
	return err
}

// carePlanRequestError marks Care Plan's request answers for the rows.
func carePlanRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrCareScheduleRequestNotFound):
		return timetableCompose.MarkRecordError(timetableCompose.ErrCareRequestRecordNotFound, err)
	case errors.Is(err, careplan.ErrCareScheduleRequestNotPending):
		return timetableCompose.MarkRecordError(timetableCompose.ErrCareRequestRecordNotPending, err)
	case errors.Is(err, careplan.ErrCareScheduleRequestNotDecided):
		return timetableCompose.MarkRecordError(timetableCompose.ErrCareRequestRecordNotDecided, err)
	default:
		return err
	}
}

func carePlanScheduleFilter(filter timetableCompose.CareScheduleFilter) careplan.StudentScheduleFilter {
	return careplan.StudentScheduleFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Weekday: filter.Weekday,
		Date: careplan.Date(filter.Date), From: careplan.Date(filter.From), To: careplan.Date(filter.To),
		UpcomingFrom: careplan.Date(filter.UpcomingFrom), LockForUpdate: filter.LockForUpdate,
		Options: legacyScheduleQueryOptions(filter.Options),
	}
}

func carePlanSlice[T any, R any](values []T, convert func(T) R) []R {
	result := make([]R, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func careArrivalScheduleToRows(value careplan.ArrivalSchedule) timetableCompose.CareArrivalSchedule {
	return timetableCompose.CareArrivalSchedule(value)
}
func careArrivalScheduleFromRows(value timetableCompose.CareArrivalSchedule) careplan.ArrivalSchedule {
	return careplan.ArrivalSchedule(value)
}

func carePickupScheduleToRows(value careplan.PickupSchedule) timetableCompose.CarePickupSchedule {
	return timetableCompose.CarePickupSchedule(value)
}
func carePickupScheduleFromRows(value timetableCompose.CarePickupSchedule) careplan.PickupSchedule {
	return careplan.PickupSchedule(value)
}

func careArrivalExceptionToRows(value careplan.ArrivalException) timetableCompose.CareArrivalException {
	return timetableCompose.CareArrivalException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, ExpectedArrival: value.ExpectedArrival, Reason: value.Reason, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: timetableCompose.CareDate(value.ExceptionDate)}
}
func careArrivalExceptionFromRows(value timetableCompose.CareArrivalException) careplan.ArrivalException {
	return careplan.ArrivalException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, ExpectedArrival: value.ExpectedArrival, Reason: value.Reason, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: careplan.Date(value.ExceptionDate)}
}

func carePickupExceptionToRows(value careplan.PickupException) timetableCompose.CarePickupException {
	return timetableCompose.CarePickupException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, PickupTime: value.PickupTime, Reason: value.Reason, ExcusedFrom: value.ExcusedFrom, ExcusedReason: value.ExcusedReason, ExcusedCreatedBy: value.ExcusedCreatedBy, ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime, ExcusedAuto: value.ExcusedAuto, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: timetableCompose.CareDate(value.ExceptionDate)}
}
func carePickupExceptionFromRows(value timetableCompose.CarePickupException) careplan.PickupException {
	return careplan.PickupException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, PickupTime: value.PickupTime, Reason: value.Reason, ExcusedFrom: value.ExcusedFrom, ExcusedReason: value.ExcusedReason, ExcusedCreatedBy: value.ExcusedCreatedBy, ExcusedOwnsPickupTime: value.ExcusedOwnsPickupTime, ExcusedAuto: value.ExcusedAuto, Source: value.Source, CreatedBy: value.CreatedBy, CreatedByGuardian: value.CreatedByGuardian, ExceptionDate: careplan.Date(value.ExceptionDate)}
}

func careArrivalNoteToRows(value careplan.ArrivalNote) timetableCompose.CareArrivalNote {
	return timetableCompose.CareArrivalNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: timetableCompose.CareDate(value.NoteDate)}
}
func careArrivalNoteFromRows(value timetableCompose.CareArrivalNote) careplan.ArrivalNote {
	return careplan.ArrivalNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: careplan.Date(value.NoteDate)}
}

func carePickupNoteToRows(value careplan.PickupNote) timetableCompose.CarePickupNote {
	return timetableCompose.CarePickupNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Weekday: value.Weekday, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: timetableCompose.CareDate(value.NoteDate)}
}
func carePickupNoteFromRows(value timetableCompose.CarePickupNote) careplan.PickupNote {
	return careplan.PickupNote{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentID: value.StudentID, Weekday: value.Weekday, Content: value.Content, CreatedBy: value.CreatedBy, NoteDate: careplan.Date(value.NoteDate)}
}

func (r carePlanRecords) CreateArrivalSchedule(ctx context.Context, value timetableCompose.CareArrivalSchedule) (timetableCompose.CareArrivalSchedule, error) {
	created, err := r.capability.CreateArrivalSchedule(ctx, careArrivalScheduleFromRows(value))
	return careArrivalScheduleToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindArrivalSchedule(ctx context.Context, id int64) (timetableCompose.CareArrivalSchedule, error) {
	value, err := r.capability.FindArrivalSchedule(ctx, id)
	return careArrivalScheduleToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdateArrivalSchedule(ctx context.Context, value timetableCompose.CareArrivalSchedule) error {
	return carePlanScheduleError(r.capability.UpdateArrivalSchedule(ctx, careArrivalScheduleFromRows(value)))
}
func (r carePlanRecords) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalSchedule(ctx, id))
}
func (r carePlanRecords) DeleteArrivalSchedulesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalSchedulesByStudent(ctx, studentID))
}
func (r carePlanRecords) ListArrivalSchedules(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CareArrivalSchedule, error) {
	values, err := r.capability.ListArrivalSchedules(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, careArrivalScheduleToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) UpsertArrivalSchedule(ctx context.Context, value timetableCompose.CareArrivalSchedule) (timetableCompose.CareArrivalSchedule, error) {
	upserted, err := r.capability.UpsertArrivalSchedule(ctx, careArrivalScheduleFromRows(value))
	return careArrivalScheduleToRows(upserted), carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupSchedule(ctx context.Context, value timetableCompose.CarePickupSchedule) (timetableCompose.CarePickupSchedule, error) {
	created, err := r.capability.CreatePickupSchedule(ctx, carePickupScheduleFromRows(value))
	return carePickupScheduleToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindPickupSchedule(ctx context.Context, id int64) (timetableCompose.CarePickupSchedule, error) {
	value, err := r.capability.FindPickupSchedule(ctx, id)
	return carePickupScheduleToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdatePickupSchedule(ctx context.Context, value timetableCompose.CarePickupSchedule) error {
	return carePlanScheduleError(r.capability.UpdatePickupSchedule(ctx, carePickupScheduleFromRows(value)))
}
func (r carePlanRecords) DeletePickupSchedule(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeletePickupSchedule(ctx, id))
}
func (r carePlanRecords) DeletePickupSchedulesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupSchedulesByStudent(ctx, studentID))
}
func (r carePlanRecords) ListPickupSchedules(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CarePickupSchedule, error) {
	values, err := r.capability.ListPickupSchedules(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, carePickupScheduleToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) UpsertPickupSchedule(ctx context.Context, value timetableCompose.CarePickupSchedule) (timetableCompose.CarePickupSchedule, error) {
	upserted, err := r.capability.UpsertPickupSchedule(ctx, carePickupScheduleFromRows(value))
	return carePickupScheduleToRows(upserted), carePlanScheduleError(err)
}

func (r carePlanRecords) CreateArrivalException(ctx context.Context, value timetableCompose.CareArrivalException) (timetableCompose.CareArrivalException, error) {
	created, err := r.capability.CreateArrivalException(ctx, careArrivalExceptionFromRows(value))
	return careArrivalExceptionToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindArrivalException(ctx context.Context, id int64, lock bool) (timetableCompose.CareArrivalException, error) {
	value, err := r.capability.FindArrivalException(ctx, id, lock)
	return careArrivalExceptionToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdateArrivalException(ctx context.Context, value timetableCompose.CareArrivalException) error {
	return carePlanScheduleError(r.capability.UpdateArrivalException(ctx, careArrivalExceptionFromRows(value)))
}
func (r carePlanRecords) DeleteArrivalException(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalException(ctx, id))
}
func (r carePlanRecords) DeleteArrivalExceptionsByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalExceptionsByStudent(ctx, studentID))
}
func (r carePlanRecords) ListArrivalExceptions(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CareArrivalException, error) {
	values, err := r.capability.ListArrivalExceptions(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, careArrivalExceptionToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) DeleteArrivalExceptionsBefore(ctx context.Context, before timetableCompose.CareDate) (int64, error) {
	rows, err := r.capability.DeleteArrivalExceptionsBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupException(ctx context.Context, value timetableCompose.CarePickupException) (timetableCompose.CarePickupException, error) {
	created, err := r.capability.CreatePickupException(ctx, carePickupExceptionFromRows(value))
	return carePickupExceptionToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindPickupException(ctx context.Context, id int64, lock bool) (timetableCompose.CarePickupException, error) {
	value, err := r.capability.FindPickupException(ctx, id, lock)
	return carePickupExceptionToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdatePickupException(ctx context.Context, value timetableCompose.CarePickupException) error {
	return carePlanScheduleError(r.capability.UpdatePickupException(ctx, carePickupExceptionFromRows(value)))
}
func (r carePlanRecords) DeletePickupException(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeletePickupException(ctx, id))
}
func (r carePlanRecords) DeletePickupExceptionsByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupExceptionsByStudent(ctx, studentID))
}
func (r carePlanRecords) ListPickupExceptions(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CarePickupException, error) {
	values, err := r.capability.ListPickupExceptions(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, carePickupExceptionToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) DeletePickupExceptionsBefore(ctx context.Context, before timetableCompose.CareDate) (int64, error) {
	rows, err := r.capability.DeletePickupExceptionsBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreateArrivalNote(ctx context.Context, value timetableCompose.CareArrivalNote) (timetableCompose.CareArrivalNote, error) {
	created, err := r.capability.CreateArrivalNote(ctx, careArrivalNoteFromRows(value))
	return careArrivalNoteToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindArrivalNote(ctx context.Context, id int64) (timetableCompose.CareArrivalNote, error) {
	value, err := r.capability.FindArrivalNote(ctx, id)
	return careArrivalNoteToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdateArrivalNote(ctx context.Context, value timetableCompose.CareArrivalNote) error {
	return carePlanScheduleError(r.capability.UpdateArrivalNote(ctx, careArrivalNoteFromRows(value)))
}
func (r carePlanRecords) DeleteArrivalNote(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalNote(ctx, id))
}
func (r carePlanRecords) DeleteArrivalNotesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeleteArrivalNotesByStudent(ctx, studentID))
}
func (r carePlanRecords) ListArrivalNotes(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CareArrivalNote, error) {
	values, err := r.capability.ListArrivalNotes(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, careArrivalNoteToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) DeleteArrivalNotesBefore(ctx context.Context, before timetableCompose.CareDate) (int64, error) {
	rows, err := r.capability.DeleteArrivalNotesBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreatePickupNote(ctx context.Context, value timetableCompose.CarePickupNote) (timetableCompose.CarePickupNote, error) {
	created, err := r.capability.CreatePickupNote(ctx, carePickupNoteFromRows(value))
	return carePickupNoteToRows(created), carePlanScheduleError(err)
}
func (r carePlanRecords) FindPickupNote(ctx context.Context, id int64) (timetableCompose.CarePickupNote, error) {
	value, err := r.capability.FindPickupNote(ctx, id)
	return carePickupNoteToRows(value), carePlanScheduleError(err)
}
func (r carePlanRecords) UpdatePickupNote(ctx context.Context, value timetableCompose.CarePickupNote) error {
	return carePlanScheduleError(r.capability.UpdatePickupNote(ctx, carePickupNoteFromRows(value)))
}
func (r carePlanRecords) DeletePickupNote(ctx context.Context, id int64) error {
	return carePlanScheduleError(r.capability.DeletePickupNote(ctx, id))
}
func (r carePlanRecords) DeletePickupNotesByStudent(ctx context.Context, studentID int64) error {
	return carePlanScheduleError(r.capability.DeletePickupNotesByStudent(ctx, studentID))
}
func (r carePlanRecords) ListPickupNotes(ctx context.Context, filter timetableCompose.CareScheduleFilter) ([]timetableCompose.CarePickupNote, error) {
	values, err := r.capability.ListPickupNotes(ctx, carePlanScheduleFilter(filter))
	return carePlanSlice(values, carePickupNoteToRows), carePlanScheduleError(err)
}
func (r carePlanRecords) DeletePickupNotesBefore(ctx context.Context, before timetableCompose.CareDate) (int64, error) {
	rows, err := r.capability.DeletePickupNotesBefore(ctx, careplan.Date(before))
	return rows, carePlanScheduleError(err)
}

func (r carePlanRecords) CreateCareScheduleRequest(ctx context.Context, value timetableCompose.CareScheduleRequest) (timetableCompose.CareScheduleRequest, error) {
	created, err := r.capability.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest(value))
	return timetableCompose.CareScheduleRequest(created), carePlanRequestError(err)
}
func (r carePlanRecords) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (timetableCompose.CareScheduleRequest, error) {
	value, err := r.capability.FindCareScheduleRequest(ctx, id, lock)
	return timetableCompose.CareScheduleRequest(value), carePlanRequestError(err)
}
func (r carePlanRecords) FindPendingCareScheduleRequest(ctx context.Context, id int64) (timetableCompose.CareScheduleRequest, error) {
	value, err := r.capability.FindPendingCareScheduleRequest(ctx, id)
	return timetableCompose.CareScheduleRequest(value), carePlanRequestError(err)
}
func (r carePlanRecords) ListCareScheduleRequests(ctx context.Context, filter timetableCompose.CareScheduleRequestFilter) ([]timetableCompose.CareScheduleRequest, error) {
	ownerFilter := careplan.CareScheduleRequestFilter{StudentID: filter.StudentID, RequestKinds: filter.RequestKinds, Statuses: filter.Statuses, RecentSince: filter.RecentSince}
	if filter.Queue != nil {
		ownerFilter.Queue = publicQueueFilter(*filter.Queue)
	}
	values, err := r.capability.ListCareScheduleRequests(ctx, ownerFilter)
	return carePlanSlice(values, func(value careplan.CareScheduleChangeRequest) timetableCompose.CareScheduleRequest {
		return timetableCompose.CareScheduleRequest(value)
	}), carePlanRequestError(err)
}
func (r carePlanRecords) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) error {
	return carePlanRequestError(r.capability.UpdatePendingCareScheduleRequest(ctx, id, payload))
}
func (r carePlanRecords) DecideCareScheduleRequest(ctx context.Context, decision timetableCompose.CareScheduleRequestDecision) error {
	return carePlanRequestError(r.capability.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision(decision)))
}
func (r carePlanRecords) RedecideCareScheduleRequest(ctx context.Context, decision timetableCompose.CareScheduleRequestDecision) error {
	return carePlanRequestError(r.capability.RedecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision(decision)))
}
func (r carePlanRecords) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return carePlanRequestError(r.capability.UpdateCareScheduleRequestSnapshot(ctx, id, snapshot))
}
