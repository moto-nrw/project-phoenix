package ports

import (
	"context"
	"time"
)

type Stats struct {
	Queries           int
	Rows              int64
	StatementDuration time.Duration
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats
	Err error
}

type Store interface {
	LockStaffSupervision(context.Context, int64, Date) ([]int64, Stats, error)
	UnclaimedStore
	AttendanceStore
	VisitStore
	LatestPresenceDate(context.Context, int64) (*string, Stats, error)
	CountAttendanceRecords(context.Context, int64) (int, Stats, error)
	LockOpenPresence(context.Context, []int64) (Stats, error)
	CloseOpenPresence(context.Context, []int64, time.Time) (Stats, error)
	ListOpenPresence(context.Context, []int64) ([]int64, Stats, error)
	LockOpenVisits(context.Context, int64) (Stats, error)
	RestoreVisits(context.Context, []int64) (Stats, error)
	LockOpenSupervisors(context.Context, int64) (Stats, error)
	LockSupervisors(context.Context, []int64) (Stats, error)
	RestoreGroup(context.Context, int64, time.Time) (Stats, error)
	RestoreSupervisors(context.Context, []int64) (Stats, error)
}

type Transaction interface {
	Run(context.Context, func(context.Context) error) error
	Require(context.Context) error
}
