package enrollment

import (
	"context"
	"errors"
)

type PhaseExpiryInput struct {
	AsOf            Date
	WarningThrough  Date
	OfferingsJSON   string
	BookingsJSON    string
	StudentIDs      []int64
	StudentStatuses []string
	EnrolledFrom    []string
	EnrolledUntil   []string
}

type PhaseExpirySnapshot struct {
	SourcePhaseID      int64
	SourcePhaseName    string
	SuccessorPhaseID   *int64
	SuccessorPhaseName *string
	FirstAffectedDate  Date
	AffectedChildren   int
	UnresolvedChildren int
}

func (m *Module) PhaseExpirySnapshots(ctx context.Context, input PhaseExpiryInput) ([]*PhaseExpirySnapshot, error) {
	if input.AsOf.IsZero() || input.WarningThrough.IsZero() {
		return nil, errors.New("phase expiry report dates are required")
	}
	if input.WarningThrough.Before(input.AsOf) {
		return nil, errors.New("phase expiry warning horizon must not be before the report date")
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PhaseExpirySnapshots(ctx, input)
}
