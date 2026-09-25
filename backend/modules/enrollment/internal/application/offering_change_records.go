package application

import (
	"context"
	"errors"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Care Plan owns the offering change requests (#3561). The guardian portal's
// request sharing still reads them in the enrollment rows; OfferingChangeRecords
// serves Care Plan's values in those rows with the retained repository's
// error shape.

// CarePlanOfferingChanges is Care Plan's offering change lookup.
type CarePlanOfferingChanges interface {
	FindOfferingChange(ctx context.Context, id int64, lock bool) (careplan.OfferingChangeRequest, error)
}

// OfferingChangeRecords reads Care Plan's offering change requests in
// enrollment rows.
type OfferingChangeRecords struct {
	carePlan CarePlanOfferingChanges
	noRows   error
}

// NewOfferingChangeRecords binds the reads to Care Plan. noRows is the
// store's missing-row error a lookup miss carries, as the retained
// repository's did.
func NewOfferingChangeRecords(carePlan CarePlanOfferingChanges, noRows error) *OfferingChangeRecords {
	return &OfferingChangeRecords{carePlan: carePlan, noRows: noRows}
}

// FindByID loads one offering change request by the portal's untyped id. A
// miss answers the retained repository's not-found shape.
func (r *OfferingChangeRecords) FindByID(ctx context.Context, rawID any) (*enrollmentModels.OfferingChangeRequest, error) {
	const op = "find by id"
	id, err := untypedID(rawID)
	if err != nil {
		return nil, &offeringRecordError{op: op, err: err}
	}
	value, err := r.carePlan.FindOfferingChange(ctx, id, false)
	switch {
	case errors.Is(err, careplan.ErrOfferingChangeNotFound):
		return nil, &offeringRecordError{op: op, err: errors.Join(errRecordNotFound, r.noRows)}
	case err != nil:
		return nil, &offeringRecordError{op: op, err: err}
	}
	row := new(enrollmentModels.OfferingChangeRequest)
	if err := applyOfferingChangeToRow(row, value); err != nil {
		return nil, &offeringRecordError{op: op, err: err}
	}
	return row, nil
}

// offeringRecordError is the retained repository error shape, "database
// error during <op>: <cause>", with the cause kept in the chain.
type offeringRecordError struct {
	op  string
	err error
}

func (e *offeringRecordError) Error() string {
	if e.err == nil {
		return "database error during " + e.op
	}
	return "database error during " + e.op + ": " + e.err.Error()
}

func (e *offeringRecordError) Unwrap() error { return e.err }

// StoreFailure marks the error as the store's failure.
func (e *offeringRecordError) StoreFailure() bool { return true }

// errRecordNotFound is the repository not-found sentinel's shape: the same
// text and the RepositoryNotFound marker a missing row is recognized by.
var errRecordNotFound error = recordNotFoundError{}

type recordNotFoundError struct{}

func (recordNotFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found sentinel.
func (recordNotFoundError) RepositoryNotFound() {}

func untypedID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}

func applyOfferingChangeToRow(target *enrollmentModels.OfferingChangeRequest, value careplan.OfferingChangeRequest) error {
	target.ID, target.TenantID, target.CreatedAt, target.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	target.StudentID, target.RequestChildID, target.SubmittedBy = value.StudentID, value.RequestChildID, value.SubmittedBy
	target.CompleteWithdrawalConfirmed = value.CompleteWithdrawalConfirmed
	target.WithdrawalConfirmedBy, target.WithdrawalConfirmedAt = value.WithdrawalConfirmedBy, value.WithdrawalConfirmedAt
	target.ApprovedCompleteWithdrawal = value.ApprovedCompleteWithdrawal
	if err := unmarshalOptional(value.Payload, &target.Payload); err != nil {
		return fmt.Errorf("decode offering change payload: %w", err)
	}
	target.EffectiveFrom = enrollmentModels.OfferingChangeDate(value.EffectiveFrom)
	target.ParentNote, target.Status, target.DecisionReason = value.ParentNote, value.Status, value.DecisionReason
	if err := unmarshalOptional(value.DecisionSnapshot, &target.DecisionSnapshot); err != nil {
		return fmt.Errorf("decode offering change decision snapshot: %w", err)
	}
	target.ReviewedBy, target.ReviewedAt, target.AppliedAt = value.ReviewedBy, value.ReviewedAt, value.AppliedAt
	return nil
}
