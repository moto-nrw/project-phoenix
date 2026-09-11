package users

import (
	"context"
	"time"
)

// CareExitPresence joins the care-exit workflow's tenant transaction. School
// attendance and room visits stay owned by Student Presence, not People.
type CareExitPresence interface {
	ListOpenPresence(context.Context, []int64) ([]int64, error)
	LatestPresenceDate(context.Context, int64) (*string, error)
	LockOpenPresence(context.Context, []int64) error
	CloseOpenPresence(context.Context, []int64, time.Time) (int64, error)
}
