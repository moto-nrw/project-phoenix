package config

import (
	"context"
	"errors"
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

type directRuntime struct{ db *bun.DB }

// NewRuntime provides the non-transactional runtime used by composition
// callers that only perform privileged reads. Application services replace it
// with their tenant-aware runtime before wiring settings repositories.
func NewRuntime(db *bun.DB) Runtime { return directRuntime{db: db} }

func (r directRuntime) DB(context.Context) bun.IDB   { return r.db }
func (directRuntime) TenantID(context.Context) int64 { return 0 }
func (directRuntime) TodayTime() time.Time           { return time.Now() }
func (directRuntime) LockStaffBalance(context.Context, int64) error {
	return errors.New("config repository transaction runtime is required")
}
