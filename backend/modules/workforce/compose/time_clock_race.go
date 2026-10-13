package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// RacedCheckIn classifies a check-in that lost the race for the open block.
// A concurrent stamp of the same card is a state conflict, not a server
// fault; the duplicate INSERT left the request transaction aborted, so the
// rollback is requested explicitly. The Workforce store reports the violated
// open-block index (#2402) as ErrWorkSessionAlreadyOpen. Any other error is
// not a race and yields nil.
func RacedCheckIn(ctx context.Context, err error) error {
	if !errors.Is(err, workforce.ErrWorkSessionAlreadyOpen) {
		return nil
	}
	tenant.MarkRollback(ctx)
	return &workforce.TimeTrackingError{Kind: workforce.ErrCheckInRaced, Cause: err}
}
