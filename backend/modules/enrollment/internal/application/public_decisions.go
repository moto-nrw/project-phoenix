package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

var _ enrollment.Decisions = (*Decisions)(nil)

// errRequestReadsNotConfigured reports a composition without the read state.
var errRequestReadsNotConfigured = errors.New("enrollment: request read state not configured")

func (d *Decisions) reads() (enrollment.RequestReads, error) {
	if reads := d.RequestReads(); reads != nil {
		return reads, nil
	}
	return nil, errRequestReadsNotConfigured
}

func (d *Decisions) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	reads, err := d.reads()
	if err != nil {
		return 0, err
	}
	return reads.CountUnreadRequests(ctx, accountID)
}

func (d *Decisions) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	reads, err := d.reads()
	if err != nil {
		return nil, err
	}
	return reads.UnreadRequestIDs(ctx, accountID, requestIDs)
}

func (d *Decisions) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkRequestsRead(ctx, accountID, requestIDs)
}

func (d *Decisions) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkAllRequestsRead(ctx, accountID)
}

func (d *Decisions) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	reads, err := d.reads()
	if err != nil {
		return err
	}
	return reads.MarkRequestUnread(ctx, accountID, requestID)
}
