package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type excusedRequestRepository struct{ capability careplan.Capability }
type careScheduleRequestRepository struct{ capability careplan.Capability }
type studentDataRequestRepository struct{ capability careplan.Capability }

func NewExcusedAbsenceRequestRepository(capability careplan.Capability) activeModels.ExcusedAbsenceRequestRepository {
	return excusedRequestRepository{capability: capability}
}

func NewCareScheduleChangeRequestRepository(capability careplan.Capability) scheduleModels.CareScheduleChangeRequestRepository {
	return careScheduleRequestRepository{capability: capability}
}

func NewStudentDataChangeRequestRepository(capability careplan.Capability) userModels.StudentDataChangeRequestRepository {
	return studentDataRequestRepository{capability: capability}
}

func (r excusedRequestRepository) Create(ctx context.Context, row *activeModels.ExcusedAbsenceRequest) error {
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

func (r excusedRequestRepository) LockStudentRequests(ctx context.Context, studentID int64) error {
	return r.capability.LockExcusedAbsenceRequests(ctx, studentID)
}

func (r excusedRequestRepository) FindByID(ctx context.Context, raw any) (*activeModels.ExcusedAbsenceRequest, error) {
	id, err := ScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindExcusedAbsenceRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrExcusedRequestNotFound) {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return excusedRequestFromPublic(value), nil
}

func (r excusedRequestRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*activeModels.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Options: CarePlanScheduleQueryOptions(options)})
	return excusedRequestsToLegacy(values), err
}

func (r excusedRequestRepository) ListPendingForStudent(ctx context.Context, studentID int64) ([]*activeModels.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, Statuses: []string{activeModels.ExcusedRequestStatusPending}})
	return excusedRequestsToLegacy(values), err
}

func (r excusedRequestRepository) ListRecentForStudent(ctx context.Context, studentID int64, since time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, RecentSince: since})
	return excusedRequestsToLegacy(values), err
}

func (r excusedRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*activeModels.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: []string{activeModels.ExcusedRequestStatusPending}, Queue: publicQueueFilter(filters)})
	return excusedRequestsToLegacy(values), err
}

func (r excusedRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*activeModels.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: []string{activeModels.ExcusedRequestStatusApproved, activeModels.ExcusedRequestStatusRejected, activeModels.ExcusedRequestStatusWithdrawn}, Queue: publicQueueFilter(filters)})
	return excusedRequestsToLegacy(values), err
}

func (r excusedRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	value, err := r.capability.FindPendingExcusedAbsenceRequest(ctx, id)
	if err != nil {
		return nil, mapExcusedRequestError(err)
	}
	return excusedRequestFromPublic(value), nil
}

func (r excusedRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	value, err := r.capability.FindExcusedAbsenceRequest(ctx, id, true)
	if err != nil {
		return nil, mapExcusedRequestError(err)
	}
	return excusedRequestFromPublic(value), nil
}

func (r excusedRequestRepository) UpdatePending(ctx context.Context, id int64, dates []timezone.Date, note, status string) error {
	publicDates := make([]careplan.Date, len(dates))
	for i := range dates {
		publicDates[i] = careplan.Date(dates[i])
	}
	return mapExcusedRequestError(r.capability.UpdatePendingExcusedAbsenceRequest(ctx, id, publicDates, note, status))
}

func (r excusedRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return mapExcusedRequestError(r.capability.DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r excusedRequestRepository) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapExcusedRequestError(r.capability.RedecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &reviewedBy, Applied: applied}))
}

func mapExcusedRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return activeModels.ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return activeModels.ErrExcusedRequestNotPending
	case errors.Is(err, careplan.ErrExcusedRequestNotDecided):
		return activeModels.ErrExcusedRequestNotDecided
	default:
		return err
	}
}

func (r careScheduleRequestRepository) Create(ctx context.Context, row *scheduleModels.CareScheduleChangeRequest) error {
	if row == nil {
		return errors.New("CareScheduleChangeRequest cannot be nil or zero value")
	}
	value, err := careScheduleRequestFromLegacy(row)
	if err != nil {
		return err
	}
	created, err := r.capability.CreateCareScheduleRequest(ctx, value)
	if err != nil {
		return err
	}
	updated, err := careScheduleRequestFromPublic(created)
	if err == nil {
		*row = *updated
	}
	return err
}

func (r careScheduleRequestRepository) FindByID(ctx context.Context, raw any) (*scheduleModels.CareScheduleChangeRequest, error) {
	id, err := ScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindCareScheduleRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrCareScheduleRequestNotFound) {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	if err != nil {
		return nil, err
	}
	return careScheduleRequestFromPublic(value)
}

func (r careScheduleRequestRepository) GetPendingForStudent(ctx context.Context, studentID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return r.GetPendingForStudentAndKind(ctx, studentID, scheduleModels.CareRequestKindWeeklySchedule)
}

func (r careScheduleRequestRepository) GetPendingForStudentAndKind(ctx context.Context, studentID int64, kind string) (*scheduleModels.CareScheduleChangeRequest, error) {
	values, err := r.list(ctx, careplan.CareScheduleRequestFilter{StudentID: studentID, RequestKinds: []string{kind}, Statuses: []string{scheduleModels.CareRequestStatusPending}})
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}

func (r careScheduleRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, careplan.CareScheduleRequestFilter{Statuses: []string{scheduleModels.CareRequestStatusPending}, Queue: publicQueueFilter(filters)})
}

func (r careScheduleRequestRepository) ListPendingForTenantAndKind(ctx context.Context, kind string, filters modelBase.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, careplan.CareScheduleRequestFilter{RequestKinds: []string{kind}, Statuses: []string{scheduleModels.CareRequestStatusPending}, Queue: publicQueueFilter(filters)})
}

func (r careScheduleRequestRepository) ListRecentForStudentAndKind(ctx context.Context, studentID int64, kind string, since time.Time) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, careplan.CareScheduleRequestFilter{StudentID: studentID, RequestKinds: []string{kind}, RecentSince: since})
}

func (r careScheduleRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, careplan.CareScheduleRequestFilter{Statuses: []string{scheduleModels.CareRequestStatusApproved, scheduleModels.CareRequestStatusRejected, scheduleModels.CareRequestStatusWithdrawn}, Queue: publicQueueFilter(filters)})
}

func (r careScheduleRequestRepository) list(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	values, err := r.capability.ListCareScheduleRequests(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*scheduleModels.CareScheduleChangeRequest, 0, len(values))
	for _, value := range values {
		row, convertErr := careScheduleRequestFromPublic(value)
		if convertErr != nil {
			return nil, convertErr
		}
		result = append(result, row)
	}
	return result, nil
}

func (r careScheduleRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	value, err := r.capability.FindPendingCareScheduleRequest(ctx, id)
	if err != nil {
		return nil, mapCareScheduleRequestError(err)
	}
	return careScheduleRequestFromPublic(value)
}

func (r careScheduleRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	value, err := r.capability.FindCareScheduleRequest(ctx, id, true)
	if err != nil {
		return nil, mapCareScheduleRequestError(err)
	}
	return careScheduleRequestFromPublic(value)
}

func (r careScheduleRequestRepository) UpdatePending(ctx context.Context, id int64, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return mapCareScheduleRequestError(r.capability.UpdatePendingCareScheduleRequest(ctx, id, encoded))
}

func (r careScheduleRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return mapCareScheduleRequestError(r.capability.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r careScheduleRequestRepository) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapCareScheduleRequestError(r.capability.RedecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &reviewedBy, Applied: applied}))
}

func (r careScheduleRequestRepository) UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot *scheduleModels.CareRequestDecisionSnapshot) error {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return mapCareScheduleRequestError(r.capability.UpdateCareScheduleRequestSnapshot(ctx, id, encoded))
}

func mapCareScheduleRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrCareScheduleRequestNotFound):
		return scheduleModels.ErrCareRequestNotFound
	case errors.Is(err, careplan.ErrCareScheduleRequestNotPending):
		return scheduleModels.ErrCareRequestNotPending
	case errors.Is(err, careplan.ErrCareScheduleRequestNotDecided):
		return scheduleModels.ErrCareRequestNotDecided
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
	id, err := ScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindStudentDataRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
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

func (r studentDataRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{userModels.DataChangeStatusPending}, Queue: publicQueueFilter(filters)})
}

func (r studentDataRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
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

func publicQueueFilter(value modelBase.RequestQueueFilters) *careplan.RequestQueueFilter {
	return &careplan.RequestQueueFilter{UrgentOnly: value.UrgentOnly, UrgentDate: value.UrgentDate, StudentIDs: value.StudentIDs, StudentID: value.StudentID, Search: value.Search, BeforeInstant: value.BeforeInstant, BeforeID: value.BeforeID, Limit: value.Limit}
}

func excusedRequestsToLegacy(values []careplan.ExcusedAbsenceRequest) []*activeModels.ExcusedAbsenceRequest {
	result := make([]*activeModels.ExcusedAbsenceRequest, 0, len(values))
	for _, value := range values {
		result = append(result, excusedRequestFromPublic(value))
	}
	return result
}

func careScheduleRequestFromLegacy(row *scheduleModels.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	if row == nil {
		return careplan.CareScheduleChangeRequest{}, fmt.Errorf("care schedule change request is required")
	}
	return careScheduleRequestToPublic(row)
}

func excusedRequestFromPublic(value careplan.ExcusedAbsenceRequest) *activeModels.ExcusedAbsenceRequest {
	dates := make([]timezone.Date, len(value.Dates))
	for i := range value.Dates {
		dates[i] = timezone.Date(value.Dates[i])
	}
	row := &activeModels.ExcusedAbsenceRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Dates: dates, Note: value.Note, AbsenceStatus: value.AbsenceStatus, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func excusedRequestToPublic(row *activeModels.ExcusedAbsenceRequest) careplan.ExcusedAbsenceRequest {
	dates := make([]careplan.Date, len(row.Dates))
	for i := range row.Dates {
		dates[i] = careplan.Date(row.Dates[i])
	}
	return careplan.ExcusedAbsenceRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Dates: dates, Note: row.Note, AbsenceStatus: row.AbsenceStatus, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func careScheduleRequestToPublic(row *scheduleModels.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	var snapshot json.RawMessage
	if row.DecisionSnapshot != nil {
		var err error
		snapshot, err = json.Marshal(row.DecisionSnapshot)
		if err != nil {
			return careplan.CareScheduleChangeRequest{}, fmt.Errorf("encode care schedule request decision snapshot: %w", err)
		}
	}
	payload, err := json.Marshal(row.Payload)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, fmt.Errorf("encode care schedule request payload: %w", err)
	}
	return careplan.CareScheduleChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: snapshot}, nil
}

func careScheduleRequestFromPublic(value careplan.CareScheduleChangeRequest) (*scheduleModels.CareScheduleChangeRequest, error) {
	var payload map[string]any
	if err := json.Unmarshal(value.Payload, &payload); err != nil {
		return nil, err
	}
	var snapshot *scheduleModels.CareRequestDecisionSnapshot
	if len(value.DecisionSnapshot) > 0 && string(value.DecisionSnapshot) != "null" {
		snapshot = new(scheduleModels.CareRequestDecisionSnapshot)
		if err := json.Unmarshal(value.DecisionSnapshot, snapshot); err != nil {
			return nil, err
		}
	}
	row := &scheduleModels.CareScheduleChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, RequestKind: value.RequestKind, Payload: payload, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt, DecisionSnapshot: snapshot}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row, nil
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *userModels.StudentDataChangeRequest {
	row := &userModels.StudentDataChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *userModels.StudentDataChangeRequest) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}
