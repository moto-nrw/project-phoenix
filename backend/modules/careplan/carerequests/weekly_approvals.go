package carerequests

import (
	"context"
	"errors"
)

var ErrCareDayManagedByBooking = errors.New("schedule: care day is managed by an offering booking")

// WeeklyApprovals merges a validated request into the current plan in the caller's tenant transaction.
type WeeklyApprovals interface {
	ApplyWeekly(context.Context, *Request, int64) (bool, error)
}
