package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// ExcusedAbsenceRequestRepository serves the retained excused-absence request
// rows over the Care Plan capability. Its consumers own the ports they need.
// All methods MUST run inside a tenant transaction.
type ExcusedAbsenceRequestRepository struct{ capability careplan.Capability }
type studentDataRequestRepository struct{ capability careplan.Capability }

func NewExcusedAbsenceRequestRepository(capability careplan.Capability) *ExcusedAbsenceRequestRepository {
	return &ExcusedAbsenceRequestRepository{capability: capability}
}

func NewCareScheduleChangeRequestRepository(capability careplan.Capability) timetableCompose.CareScheduleRequestRows {
	return timetableCompose.NewCareScheduleRequestRows(NewCarePlanRows(capability))
}

func NewStudentDataChangeRequestRepository(capability careplan.Capability) userModels.StudentDataChangeRequestRepository {
	return studentDataRequestRepository{capability: capability}
}

func (r ExcusedAbsenceRequestRepository) Create(ctx context.Context, row *absencerecords.ExcusedAbsenceRequest) error {
	if row == nil {
		return errors.New("ExcusedAbsenceRequest cannot be nil or zero value")
	}
	created, err := r.capability.CreateExcusedAbsenceRequest(ctx, excusedRequestToPublic(row))
	if err != nil {
		return err
	}
	*row = *excusedRequestFromPublic(created)
	return nil
}

func (r ExcusedAbsenceRequestRepository) LockStudentRequests(ctx context.Context, studentID int64) error {
	return r.capability.LockExcusedAbsenceRequests(ctx, studentID)
}

func (r ExcusedAbsenceRequestRepository) FindByID(ctx context.Context, raw any) (*absencerecords.ExcusedAbsenceRequest, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindExcusedAbsenceRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrExcusedRequestNotFound) {
		return nil, absencerecords.ErrExcusedRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return excusedRequestFromPublic(value), nil
}

func (r ExcusedAbsenceRequestRepository) ListWithOptions(ctx context.Context, options *userModels.QueryOptions) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Options: legacyScheduleQueryOptions(options)})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListPendingForStudent(ctx context.Context, studentID int64) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, Statuses: []string{absencerecords.ExcusedRequestStatusPending}})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListRecentForStudent(ctx context.Context, studentID int64, since time.Time) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, RecentSince: since})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListPendingForTenant(ctx context.Context, filters userModels.RequestQueueFilters) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: []string{absencerecords.ExcusedRequestStatusPending}, Queue: publicQueueFilter(filters)})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListDecidedForTenant(ctx context.Context, filters userModels.RequestQueueFilters) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: []string{absencerecords.ExcusedRequestStatusApproved, absencerecords.ExcusedRequestStatusRejected, absencerecords.ExcusedRequestStatusWithdrawn}, Queue: publicQueueFilter(filters)})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*absencerecords.ExcusedAbsenceRequest, error) {
	value, err := r.capability.FindPendingExcusedAbsenceRequest(ctx, id)
	if err != nil {
		return nil, mapExcusedRequestError(err)
	}
	return excusedRequestFromPublic(value), nil
}

func (r ExcusedAbsenceRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*absencerecords.ExcusedAbsenceRequest, error) {
	value, err := r.capability.FindExcusedAbsenceRequest(ctx, id, true)
	if err != nil {
		return nil, mapExcusedRequestError(err)
	}
	return excusedRequestFromPublic(value), nil
}

func (r ExcusedAbsenceRequestRepository) UpdatePending(ctx context.Context, id int64, dates []absencerecords.Date, note, status string) error {
	publicDates := make([]careplan.Date, len(dates))
	for i := range dates {
		publicDates[i] = careplan.Date(dates[i])
	}
	return mapExcusedRequestError(r.capability.UpdatePendingExcusedAbsenceRequest(ctx, id, publicDates, note, status))
}

func (r ExcusedAbsenceRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return mapExcusedRequestError(r.capability.DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r ExcusedAbsenceRequestRepository) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapExcusedRequestError(r.capability.RedecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &reviewedBy, Applied: applied}))
}

func mapExcusedRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return absencerecords.ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return absencerecords.ErrExcusedRequestNotPending
	case errors.Is(err, careplan.ErrExcusedRequestNotDecided):
		return absencerecords.ErrExcusedRequestNotDecided
	default:
		return err
	}
}

func (r studentDataRequestRepository) Create(ctx context.Context, row *userModels.StudentDataChangeRequest) error {
	if row == nil {
		return errors.New("StudentDataChangeRequest cannot be nil or zero value")
	}
	created, err := r.capability.CreateStudentDataRequest(ctx, studentDataRequestToPublic(row))
	if err != nil {
		return err
	}
	*row = *studentDataRequestFromPublic(created)
	return nil
}

func (r studentDataRequestRepository) FindByID(ctx context.Context, raw any) (*userModels.StudentDataChangeRequest, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindStudentDataRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
		return nil, retainedNotFound("find by id")
	}
	if err != nil {
		return nil, err
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{StudentID: studentID, Statuses: statuses, Limit: limit})
}

func (r studentDataRequestRepository) ListParentVisibleByStudent(ctx context.Context, studentID int64, limit int) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{StudentID: studentID, ParentVisible: true, Limit: limit})
}

func (r studentDataRequestRepository) ListPendingForTenant(ctx context.Context, filters userModels.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{userModels.DataChangeStatusPending}, Queue: publicQueueFilter(filters)})
}

func (r studentDataRequestRepository) ListDecidedForTenant(ctx context.Context, filters userModels.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{userModels.DataChangeStatusAutoApplied, userModels.DataChangeStatusApproved, userModels.DataChangeStatusRejected}, Queue: publicQueueFilter(filters)})
}

func (r studentDataRequestRepository) list(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]*userModels.StudentDataChangeRequest, error) {
	values, err := r.capability.ListStudentDataRequests(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*userModels.StudentDataChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, studentDataRequestFromPublic(value))
	}
	return result, nil
}

func (r studentDataRequestRepository) HasPendingForField(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return r.capability.HasPendingStudentDataRequest(ctx, studentID, target, field)
}

func (r studentDataRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*userModels.StudentDataChangeRequest, error) {
	value, err := r.capability.FindPendingStudentDataRequest(ctx, id)
	if err != nil {
		return nil, mapStudentDataRequestError(err)
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.StudentDataChangeRequest, error) {
	value, err := r.capability.FindStudentDataRequest(ctx, id, true)
	if err != nil {
		return nil, mapStudentDataRequestError(err)
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) UpdatePending(ctx context.Context, id int64, value json.RawMessage) error {
	return mapStudentDataRequestError(r.capability.UpdatePendingStudentDataRequest(ctx, id, value))
}

func (r studentDataRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapStudentDataRequestError(r.capability.DecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r studentDataRequestRepository) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapStudentDataRequestError(r.capability.RedecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func mapStudentDataRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrStudentDataRequestNotFound):
		return userModels.ErrChangeRequestNotFound
	case errors.Is(err, careplan.ErrStudentDataRequestNotPending):
		return userModels.ErrChangeRequestNotPending
	case errors.Is(err, careplan.ErrStudentDataRequestNotDecided):
		return userModels.ErrChangeRequestNotDecided
	default:
		return err
	}
}

func publicQueueFilter(value userModels.RequestQueueFilters) *careplan.RequestQueueFilter {
	return &careplan.RequestQueueFilter{UrgentOnly: value.UrgentOnly, UrgentDate: value.UrgentDate, StudentIDs: value.StudentIDs, StudentID: value.StudentID, Search: value.Search, BeforeInstant: value.BeforeInstant, BeforeID: value.BeforeID, Limit: value.Limit}
}

func excusedRequestsToLegacy(values []careplan.ExcusedAbsenceRequest) []*absencerecords.ExcusedAbsenceRequest {
	result := make([]*absencerecords.ExcusedAbsenceRequest, 0, len(values))
	for _, value := range values {
		result = append(result, excusedRequestFromPublic(value))
	}
	return result
}

func excusedRequestFromPublic(value careplan.ExcusedAbsenceRequest) *absencerecords.ExcusedAbsenceRequest {
	dates := make([]absencerecords.Date, len(value.Dates))
	for i := range value.Dates {
		dates[i] = absencerecords.Date(value.Dates[i])
	}
	row := &absencerecords.ExcusedAbsenceRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Dates: dates, Note: value.Note, AbsenceStatus: value.AbsenceStatus, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func excusedRequestToPublic(row *absencerecords.ExcusedAbsenceRequest) careplan.ExcusedAbsenceRequest {
	dates := make([]careplan.Date, len(row.Dates))
	for i := range row.Dates {
		dates[i] = careplan.Date(row.Dates[i])
	}
	return careplan.ExcusedAbsenceRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Dates: dates, Note: row.Note, AbsenceStatus: row.AbsenceStatus, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *userModels.StudentDataChangeRequest {
	row := &userModels.StudentDataChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *userModels.StudentDataChangeRequest) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}
