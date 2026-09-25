package enrollment

import (
	"context"
	"errors"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// childQuotaReachedCode is the stable code of a write School Membership
// refuses because the Kinderkontingent is reached (#3567). The worker reads
// the code instead of importing the owner's error type, the same way the
// HTTP adapters classify a business rejection.
const childQuotaReachedCode = "students.child_quota_reached"

func isChildQuotaReached(err error) bool {
	var coded interface{ ErrorCode() string }
	return errors.As(err, &coded) && coded.ErrorCode() == childQuotaReachedCode
}

// holdForChildQuota leaves a renewal the Kinderkontingent refused to the
// school (#3570): the row becomes submitted with the child_quota_reached
// review reason, so later ticks no longer approve it on their own and the
// admin area shows why it is open. The refused Decide was already rolled
// back to its savepoint; the hold gets its own so a failed hold cannot
// poison the rest of the run.
func (s *rolloverService) holdForChildQuota(ctx context.Context, phaseID, childID int64) (bool, error) {
	var held bool
	hold := func(holdCtx context.Context) error {
		var err error
		held, err = s.Children.HoldAutoRenewedChild(holdCtx, childID, enrollmentModels.ReviewReasonChildQuotaReached)
		return err
	}
	var err error
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		err = tenant.WithSavepoint(ctx, hold)
	} else {
		err = hold(ctx)
	}
	if err != nil {
		return false, err
	}
	if held {
		s.Logger.Info("rollover deadline: renewal held for the child quota",
			slog.Int64("phase_id", phaseID),
			slog.Int64("request_child_id", childID))
	}
	return held, nil
}
