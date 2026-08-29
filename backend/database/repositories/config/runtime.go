package config

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// Runtime supplies ambient transaction and tenant state without coupling the
// settings Postgres adapter to the transaction implementation.
type Runtime interface {
	DB(context.Context) bun.IDB
	TenantID(context.Context) int64
	LockStaffBalance(context.Context, int64) error
	TodayTime() time.Time
}
