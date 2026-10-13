package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// CareScheduleRequestRows serve the retained models/schedule care-schedule
// change request repository over the Care Plan records (#3351).
type CareScheduleRequestRows struct{ rows CarePlanRows }

var _ scheduleModels.CareScheduleChangeRequestRepository = CareScheduleRequestRows{}

// NewCareScheduleRequestRows binds the request rows to the records.
func NewCareScheduleRequestRows(rows CarePlanRows) CareScheduleRequestRows {
	return CareScheduleRequestRows{rows: rows}
}

// careRequestQueue keeps the retained paging shape; the two vocabularies
// spell the same fields.
func careRequestQueue(value scheduleModels.RequestQueueFilters) *usersModels.RequestQueueFilters {
	result := usersModels.RequestQueueFilters(value)
	return &result
}

func (r CareScheduleRequestRows) Create(ctx context.Context, row *scheduleModels.CareScheduleChangeRequest) error {
	if row == nil {
		return errors.New("CareScheduleChangeRequest cannot be nil or zero value")
	}
	value, err := careScheduleRequestFromLegacy(row)
	if err != nil {
		return err
	}
	created, err := r.rows.Records.CreateCareScheduleRequest(ctx, value)
	if err != nil {
		return err
	}
	updated, err := careScheduleRequestFromPublic(created)
	if err == nil {
		*row = *updated
	}
	return err
}

func (r CareScheduleRequestRows) FindByID(ctx context.Context, raw any) (*scheduleModels.CareScheduleChangeRequest, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.rows.Records.FindCareScheduleRequest(ctx, id, false)
	if errors.Is(err, ErrCareRequestRecordNotFound) {
		return nil, r.rows.Errors.MissingRecordError("find by id")
	}
	if err != nil {
		return nil, err
	}
	return careScheduleRequestFromPublic(value)
}

func (r CareScheduleRequestRows) GetPendingForStudent(ctx context.Context, studentID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return r.GetPendingForStudentAndKind(ctx, studentID, scheduleModels.CareRequestKindWeeklySchedule)
}

func (r CareScheduleRequestRows) GetPendingForStudentAndKind(ctx context.Context, studentID int64, kind string) (*scheduleModels.CareScheduleChangeRequest, error) {
	values, err := r.list(ctx, CareScheduleRequestFilter{StudentID: studentID, RequestKinds: []string{kind}, Statuses: []string{scheduleModels.CareRequestStatusPending}})
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}

func (r CareScheduleRequestRows) ListPendingForTenant(ctx context.Context, filters scheduleModels.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, CareScheduleRequestFilter{Statuses: []string{scheduleModels.CareRequestStatusPending}, Queue: careRequestQueue(filters)})
}

func (r CareScheduleRequestRows) ListPendingForTenantAndKind(ctx context.Context, kind string, filters scheduleModels.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, CareScheduleRequestFilter{RequestKinds: []string{kind}, Statuses: []string{scheduleModels.CareRequestStatusPending}, Queue: careRequestQueue(filters)})
}

func (r CareScheduleRequestRows) ListRecentForStudentAndKind(ctx context.Context, studentID int64, kind string, since time.Time) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, CareScheduleRequestFilter{StudentID: studentID, RequestKinds: []string{kind}, RecentSince: since})
}

func (r CareScheduleRequestRows) ListDecidedForTenant(ctx context.Context, filters scheduleModels.RequestQueueFilters) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return r.list(ctx, CareScheduleRequestFilter{Statuses: []string{scheduleModels.CareRequestStatusApproved, scheduleModels.CareRequestStatusRejected, scheduleModels.CareRequestStatusWithdrawn}, Queue: careRequestQueue(filters)})
}

func (r CareScheduleRequestRows) list(ctx context.Context, filter CareScheduleRequestFilter) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	values, err := r.rows.Records.ListCareScheduleRequests(ctx, filter)
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

func (r CareScheduleRequestRows) FindPendingByIDForUpdate(ctx context.Context, id int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	value, err := r.rows.Records.FindPendingCareScheduleRequest(ctx, id)
	if err != nil {
		return nil, mapCareScheduleRequestError(err)
	}
	return careScheduleRequestFromPublic(value)
}

func (r CareScheduleRequestRows) FindByIDForUpdate(ctx context.Context, id int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	value, err := r.rows.Records.FindCareScheduleRequest(ctx, id, true)
	if err != nil {
		return nil, mapCareScheduleRequestError(err)
	}
	return careScheduleRequestFromPublic(value)
}

func (r CareScheduleRequestRows) UpdatePending(ctx context.Context, id int64, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return mapCareScheduleRequestError(r.rows.Records.UpdatePendingCareScheduleRequest(ctx, id, encoded))
}

func (r CareScheduleRequestRows) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return mapCareScheduleRequestError(r.rows.Records.DecideCareScheduleRequest(ctx, CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r CareScheduleRequestRows) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapCareScheduleRequestError(r.rows.Records.RedecideCareScheduleRequest(ctx, CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &reviewedBy, Applied: applied}))
}

func (r CareScheduleRequestRows) UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot *scheduleModels.CareRequestDecisionSnapshot) error {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return mapCareScheduleRequestError(r.rows.Records.UpdateCareScheduleRequestSnapshot(ctx, id, encoded))
}

func mapCareScheduleRequestError(err error) error {
	switch {
	case errors.Is(err, ErrCareRequestRecordNotFound):
		return scheduleModels.ErrCareRequestNotFound
	case errors.Is(err, ErrCareRequestRecordNotPending):
		return scheduleModels.ErrCareRequestNotPending
	case errors.Is(err, ErrCareRequestRecordNotDecided):
		return scheduleModels.ErrCareRequestNotDecided
	default:
		return err
	}
}

func careScheduleRequestFromLegacy(row *scheduleModels.CareScheduleChangeRequest) (CareScheduleRequest, error) {
	if row == nil {
		return CareScheduleRequest{}, fmt.Errorf("care schedule change request is required")
	}
	return careScheduleRequestToPublic(row)
}

func careScheduleRequestToPublic(row *scheduleModels.CareScheduleChangeRequest) (CareScheduleRequest, error) {
	var snapshot json.RawMessage
	if row.DecisionSnapshot != nil {
		var err error
		snapshot, err = json.Marshal(row.DecisionSnapshot)
		if err != nil {
			return CareScheduleRequest{}, fmt.Errorf("encode care schedule request decision snapshot: %w", err)
		}
	}
	payload, err := json.Marshal(row.Payload)
	if err != nil {
		return CareScheduleRequest{}, fmt.Errorf("encode care schedule request payload: %w", err)
	}
	return CareScheduleRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: snapshot}, nil
}

func careScheduleRequestFromPublic(value CareScheduleRequest) (*scheduleModels.CareScheduleChangeRequest, error) {
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
