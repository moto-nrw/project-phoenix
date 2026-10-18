package api

import (
	owner0 "github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"

	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// ExcusedAbsenceRequestRepository serves the retained excused-absence request
// rows over the Care Plan capability. Its consumers own the ports they need.
// All methods MUST run inside a tenant transaction.
type ExcusedAbsenceRequestRepository struct{ capability careplan.Capability }

func NewExcusedAbsenceRequestRepository(capability careplan.Capability) *ExcusedAbsenceRequestRepository {
	return &ExcusedAbsenceRequestRepository{capability: capability}
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

func (r ExcusedAbsenceRequestRepository) ListWithOptions(ctx context.Context, options *peoplePorts.QueryOptions) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Options: legacyScheduleQueryOptions(options)})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListPendingForStudent(ctx context.Context, studentID int64) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, Statuses: careplan.PendingExcusedRequestStatuses()})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListRecentForStudent(ctx context.Context, studentID int64, since time.Time) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, RecentSince: since})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListPendingForTenant(ctx context.Context, filters peoplePorts.RequestQueueFilters) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: careplan.PendingExcusedRequestStatuses(), Queue: publicQueueFilter(filters)})
	return excusedRequestsToLegacy(values), err
}

func (r ExcusedAbsenceRequestRepository) ListDecidedForTenant(ctx context.Context, filters peoplePorts.RequestQueueFilters) ([]*absencerecords.ExcusedAbsenceRequest, error) {
	values, err := r.capability.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{Statuses: careplan.DecidedExcusedRequestStatuses(), Queue: publicQueueFilter(filters)})
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

func mapStudentDataRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrStudentDataRequestNotFound):
		return owner0.ErrReviewNotFound
	case errors.Is(err, careplan.ErrStudentDataRequestNotPending):
		return owner0.ErrReviewNotPending
	case errors.Is(err, careplan.ErrStudentDataRequestNotDecided):
		return owner0.ErrReviewNotDecided
	default:
		return err
	}
}

func publicQueueFilter(value peoplePorts.RequestQueueFilters) *careplan.RequestQueueFilter {
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
