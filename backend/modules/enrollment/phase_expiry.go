package enrollment

import (
	"context"
	"errors"
)

type PhaseExpiryInput struct {
	AsOf            Date
	WarningThrough  Date
	OfferingsJSON   string
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
	var rows []*PhaseExpirySnapshot
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.PhaseExpirySnapshots(ctx, input)
		return err
	})
	return rows, err
}
