//go:build testhooks

package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CareScheduleRequestRows serve the retained models/schedule care-schedule
// change request repository over the Care Plan records (#3351).
type CareScheduleRequestRows struct{ rows ports.CarePlanRows }

var _ ports.CareScheduleChangeRequestRecords = CareScheduleRequestRows{}

// NewCareScheduleRequestRows binds the request rows to the records.
func NewCareScheduleRequestRows(rows ports.CarePlanRows) CareScheduleRequestRows {
	return CareScheduleRequestRows{rows: rows}
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

func (r CareScheduleRequestRows) FindByID(ctx context.Context, nativeRecordID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	var raw any = nativeRecordID
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.rows.Records.FindCareScheduleRequest(ctx, id)
	if errors.Is(err, ports.ErrCareRequestRecordNotFound) {
		return nil, r.rows.Errors.MissingRecordError("find by id")
	}
	if err != nil {
		return nil, err
	}
	return careScheduleRequestFromPublic(value)
}

func (r CareScheduleRequestRows) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return mapCareScheduleRequestError(r.rows.Records.DecideCareScheduleRequest(ctx, ports.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func mapCareScheduleRequestError(err error) error {
	switch {
	case errors.Is(err, ports.ErrCareRequestRecordNotFound):
		return scheduleModels.ErrCareRequestNotFound
	case errors.Is(err, ports.ErrCareRequestRecordNotPending):
		return scheduleModels.ErrCareRequestNotPending
	case errors.Is(err, ports.ErrCareRequestRecordNotDecided):
		return scheduleModels.ErrCareRequestNotDecided
	default:
		return err
	}
}

func careScheduleRequestFromLegacy(row *scheduleModels.CareScheduleChangeRequest) (ports.CareScheduleRequest, error) {
	if row == nil {
		return ports.CareScheduleRequest{}, fmt.Errorf("care schedule change request is required")
	}
	return careScheduleRequestToPublic(row)
}

func careScheduleRequestToPublic(row *scheduleModels.CareScheduleChangeRequest) (ports.CareScheduleRequest, error) {
	var snapshot json.RawMessage
	if row.DecisionSnapshot != nil {
		var err error
		snapshot, err = json.Marshal(row.DecisionSnapshot)
		if err != nil {
			return ports.CareScheduleRequest{}, fmt.Errorf("encode care schedule request decision snapshot: %w", err)
		}
	}
	payload, err := json.Marshal(row.Payload)
	if err != nil {
		return ports.CareScheduleRequest{}, fmt.Errorf("encode care schedule request payload: %w", err)
	}
	return ports.CareScheduleRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: snapshot}, nil
}

func careScheduleRequestFromPublic(value ports.CareScheduleRequest) (*scheduleModels.CareScheduleChangeRequest, error) {
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
